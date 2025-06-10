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

	// Validate user ID format
	if validationErr := validateUserID(accessReq.Spec.UserID); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid user ID format: %v", validationErr))
	}

	// Validate email format
	if validationErr := validateEmail(accessReq.Spec.UserEmail); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid email format: %v", validationErr))
	}

	// Validate duration format
	if validationErr := validateDuration(accessReq.Spec.Duration); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid duration: %v", validationErr))
	}

	// Validate permissions
	if validationErr := validatePermissions(accessReq.Spec.Permissions); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid permissions: %v", validationErr))
	}

	// Validate cluster configuration
	if validationErr := validateCluster(accessReq.Spec.TargetCluster); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid cluster configuration: %v", validationErr))
	}

	// Validate reason is provided and meaningful
	if validationErr := validateReason(accessReq.Spec.Reason); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid reason: %v", validationErr))
	}

	// Validate reason is sufficient for elevated permissions
	if validationErr := validateReasonForPermissions(accessReq.Spec.Reason, accessReq.Spec.Permissions); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid reason: %v", validationErr))
	}

	// Validate approvers if specified
	if validationErr := validateApprovers(accessReq.Spec.Approvers); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid approvers: %v", validationErr))
	}

	// Check if namespaces are valid when specified
	if validationErr := validateNamespaces(accessReq.Spec.Namespaces, accessReq.Spec.Permissions); validationErr != nil {
		return admission.Denied(fmt.Sprintf("invalid namespaces: %v", validationErr))
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
	// Parse duration to ensure it's valid
	parsedDuration, err := parseDuration(duration)
	if err != nil {
		return fmt.Errorf("invalid duration format - duration must be in format like '1h', '30m', '2h30m', '1d'")
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
	if duration == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}

	// Parse duration components using regexp
	re := regexp.MustCompile(`(\d+)([dhms])`)
	matches := re.FindAllStringSubmatch(duration, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	var total time.Duration
	for _, match := range matches {
		value := 0
		if _, err := fmt.Sscanf(match[1], "%d", &value); err != nil {
			return 0, fmt.Errorf("invalid duration value: %s", match[1])
		}
		unit := match[2]

		switch unit {
		case "d":
			total += time.Duration(value) * 24 * time.Hour
		case "h":
			total += time.Duration(value) * time.Hour
		case "m":
			total += time.Duration(value) * time.Minute
		case "s":
			total += time.Duration(value) * time.Second
		default:
			return 0, fmt.Errorf("invalid duration unit: %s", unit)
		}
	}

	// Verify that the parsed string matches the original (no invalid parts)
	rebuiltString := ""
	for _, match := range matches {
		rebuiltString += match[0]
	}
	if rebuiltString != duration {
		return 0, fmt.Errorf("invalid duration format: %s", duration)
	}

	return total, nil
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
		return fmt.Errorf("invalid AWS account ID format - AWS account ID must be 12 digits")
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
			return fmt.Errorf("invalid namespace format - invalid namespace name: %s", ns)
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

// validateUserID validates the user ID format
func validateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Slack user ID format: U followed by 10 alphanumeric characters
	userIDRegex := regexp.MustCompile(`^U[A-Z0-9]{10}$`)
	if !userIDRegex.MatchString(userID) {
		return fmt.Errorf("must be a valid Slack user ID (e.g., U1234567890)")
	}

	return nil
}

// validateEmail validates the email format
func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Basic email validation
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("must be a valid email address")
	}

	return nil
}

// validateReasonForPermissions validates that the reason is sufficient for the requested permissions
func validateReasonForPermissions(reason string, permissions []string) error {
	lowerReason := strings.ToLower(reason)
	genericPhrases := []string{
		"need access",
		"want access",
		"need to debug",
		"want to debug",
		"need to check",
		"want to check",
		"testing something",
		"trying to",
	}

	// Check if cluster-admin permission requires detailed justification
	if contains(permissions, "cluster-admin") {
		// Check for generic phrases first for cluster-admin
		for _, phrase := range genericPhrases {
			if strings.Contains(lowerReason, phrase) {
				return fmt.Errorf("cluster-admin permission requires detailed justification")
			}
		}

		if len(reason) < 50 {
			return fmt.Errorf("cluster-admin permission requires detailed justification (at least 50 characters)")
		}
	} else {
		// Check for generic reasons for any permission
		for _, phrase := range genericPhrases {
			if strings.Contains(lowerReason, phrase) {
				return fmt.Errorf("reason appears generic - please provide specific business justification")
			}
		}
	}

	return nil
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
