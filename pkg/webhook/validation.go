package webhook

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rebelopsio/jit-bot/pkg/controller"
)

// JITAccessRequestValidator validates JITAccessRequest resources
type JITAccessRequestValidator struct {
	Client  client.Client
	decoder admission.Decoder
}

// Handle validates JITAccessRequest resources
func (v *JITAccessRequestValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	accessReq := &controller.JITAccessRequest{}
	err := v.decoder.Decode(req, accessReq)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Validate duration format
	if err := validateDuration(accessReq.Spec.Duration); err != nil {
		return admission.Denied(fmt.Sprintf("invalid duration: %v", err))
	}

	// Validate permissions
	if err := validatePermissions(accessReq.Spec.Permissions); err != nil {
		return admission.Denied(fmt.Sprintf("invalid permissions: %v", err))
	}

	// Validate cluster configuration
	if err := validateCluster(accessReq.Spec.TargetCluster); err != nil {
		return admission.Denied(fmt.Sprintf("invalid cluster configuration: %v", err))
	}

	// Validate reason is provided and meaningful
	if err := validateReason(accessReq.Spec.Reason); err != nil {
		return admission.Denied(fmt.Sprintf("invalid reason: %v", err))
	}

	// Validate approvers if specified
	if err := validateApprovers(accessReq.Spec.Approvers); err != nil {
		return admission.Denied(fmt.Sprintf("invalid approvers: %v", err))
	}

	// Check if namespaces are valid when specified
	if err := validateNamespaces(accessReq.Spec.Namespaces, accessReq.Spec.Permissions); err != nil {
		return admission.Denied(fmt.Sprintf("invalid namespaces: %v", err))
	}

	return admission.Allowed("")
}

// InjectDecoder injects the decoder
func (v *JITAccessRequestValidator) InjectDecoder(d admission.Decoder) error {
	v.decoder = d
	return nil
}

// Validation helper functions

func validateDuration(duration string) error {
	// Support formats: 1h, 30m, 2h30m, 1d, etc.
	durationRegex := regexp.MustCompile(`^(\d+[dhms])+$`)
	if !durationRegex.MatchString(duration) {
		return fmt.Errorf("duration must be in format like '1h', '30m', '2h30m', '1d'")
	}

	// Parse duration to ensure it's valid
	parsedDuration, err := parseDuration(duration)
	if err != nil {
		return err
	}

	// Check duration limits
	minDuration := 15 * time.Minute
	maxDuration := 7 * 24 * time.Hour // 7 days

	if parsedDuration < minDuration {
		return fmt.Errorf("duration must be at least %v", minDuration)
	}

	if parsedDuration > maxDuration {
		return fmt.Errorf("duration cannot exceed %v", maxDuration)
	}

	return nil
}

func parseDuration(duration string) (time.Duration, error) {
	// Convert formats like "1d" to "24h"
	duration = strings.ReplaceAll(duration, "d", "h*24")
	
	// Handle multiplication in the string
	if strings.Contains(duration, "*") {
		parts := strings.Split(duration, "*")
		if len(parts) == 2 {
			baseValue := parts[0]
			multiplier := parts[1]
			
			// Extract numeric value from base
			var num int
			fmt.Sscanf(baseValue, "%dh", &num)
			
			// Parse multiplier
			var mult int
			fmt.Sscanf(multiplier, "%d", &mult)
			
			duration = fmt.Sprintf("%dh", num*mult)
		}
	}

	return time.ParseDuration(duration)
}

func validatePermissions(permissions []string) error {
	if len(permissions) == 0 {
		return fmt.Errorf("at least one permission must be specified")
	}

	validPermissions := map[string]bool{
		"view":          true,
		"edit":          true,
		"admin":         true,
		"cluster-admin": true,
		"debug":         true,
		"logs":          true,
		"exec":          true,
		"port-forward":  true,
	}

	for _, perm := range permissions {
		if !validPermissions[perm] {
			return fmt.Errorf("invalid permission '%s'. Valid permissions are: %v", 
				perm, getKeys(validPermissions))
		}
	}

	// Check for permission escalation
	if contains(permissions, "cluster-admin") && len(permissions) > 1 {
		return fmt.Errorf("cluster-admin permission cannot be combined with other permissions")
	}

	return nil
}

func validateCluster(cluster controller.TargetCluster) error {
	if cluster.Name == "" {
		return fmt.Errorf("cluster name is required")
	}

	if cluster.AWSAccount == "" {
		return fmt.Errorf("AWS account ID is required")
	}

	// Validate AWS account ID format (12 digits)
	accountRegex := regexp.MustCompile(`^\d{12}$`)
	if !accountRegex.MatchString(cluster.AWSAccount) {
		return fmt.Errorf("AWS account ID must be 12 digits")
	}

	if cluster.Region == "" {
		return fmt.Errorf("AWS region is required")
	}

	// Validate AWS region format
	regionRegex := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d{1}$`)
	if !regionRegex.MatchString(cluster.Region) {
		return fmt.Errorf("invalid AWS region format")
	}

	return nil
}

func validateReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("reason cannot be empty")
	}

	if len(reason) < 10 {
		return fmt.Errorf("reason must be at least 10 characters long")
	}

	if len(reason) > 500 {
		return fmt.Errorf("reason cannot exceed 500 characters")
	}

	// Check for generic/placeholder reasons
	genericReasons := []string{
		"test",
		"testing",
		"debug",
		"debugging",
		"temp",
		"temporary",
		"asdf",
		"xxx",
		"...",
		"n/a",
	}

	lowerReason := strings.ToLower(reason)
	for _, generic := range genericReasons {
		if lowerReason == generic {
			return fmt.Errorf("please provide a meaningful business reason for access")
		}
	}

	return nil
}

func validateApprovers(approvers []string) error {
	// If no approvers specified, that's okay (will be determined by policy)
	if len(approvers) == 0 {
		return nil
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, approver := range approvers {
		if seen[approver] {
			return fmt.Errorf("duplicate approver: %s", approver)
		}
		seen[approver] = true
	}

	// Validate approver format (Slack user ID or team name)
	for _, approver := range approvers {
		if !isValidApprover(approver) {
			return fmt.Errorf("invalid approver format: %s", approver)
		}
	}

	return nil
}

func validateNamespaces(namespaces []string, permissions []string) error {
	// If cluster-admin permission, namespaces should be empty
	if contains(permissions, "cluster-admin") && len(namespaces) > 0 {
		return fmt.Errorf("cluster-admin permission applies cluster-wide, namespaces should not be specified")
	}

	// Validate namespace names
	namespaceRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	for _, ns := range namespaces {
		if !namespaceRegex.MatchString(ns) {
			return fmt.Errorf("invalid namespace name: %s", ns)
		}
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, ns := range namespaces {
		if seen[ns] {
			return fmt.Errorf("duplicate namespace: %s", ns)
		}
		seen[ns] = true
	}

	return nil
}

// Helper functions

func isValidApprover(approver string) bool {
	// Slack user ID format: U1234567890
	if strings.HasPrefix(approver, "U") && len(approver) == 11 {
		return true
	}
	
	// Team name format: lowercase with hyphens
	teamRegex := regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)
	return teamRegex.MatchString(approver)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ValidateDelete ensures JITAccessRequests can be safely deleted
func (v *JITAccessRequestValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	req, ok := obj.(*controller.JITAccessRequest)
	if !ok {
		return fmt.Errorf("expected JITAccessRequest but got %T", obj)
	}

	// Prevent deletion of active requests
	if req.Status.Phase == controller.AccessPhaseActive {
		return fmt.Errorf("cannot delete active access request - revoke access first")
	}

	return nil
}