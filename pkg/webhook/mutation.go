package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rebelopsio/jit-bot/pkg/controller"
)

const (
	envProduction = "production"
	envStaging    = "staging"
)

// JITAccessRequestMutator mutates JITAccessRequest resources
type JITAccessRequestMutator struct {
	Client  client.Client
	decoder admission.Decoder
}

// Handle mutates JITAccessRequest resources
func (m *JITAccessRequestMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	accessReq := &controller.JITAccessRequest{}
	err := m.decoder.Decode(req, accessReq)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Apply mutations
	m.setDefaults(accessReq)
	m.normalizeData(accessReq)
	m.injectMetadata(accessReq)
	m.setApprovers(accessReq)

	// Create patch
	marshaledReq, err := json.Marshal(accessReq)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledReq)
}

// InjectDecoder injects the decoder
func (m *JITAccessRequestMutator) InjectDecoder(d admission.Decoder) error {
	m.decoder = d
	return nil
}

// Mutation functions

func (m *JITAccessRequestMutator) setDefaults(req *controller.JITAccessRequest) {
	// Set default permissions if none specified
	if len(req.Spec.Permissions) == 0 {
		req.Spec.Permissions = []string{"view"}
	}

	// Set default duration if not specified
	if req.Spec.Duration == "" {
		req.Spec.Duration = "1h"
	}

	// Set RequestedAt if not set
	if req.Spec.RequestedAt.IsZero() {
		req.Spec.RequestedAt = metav1.Now()
	}

	// Set initial status phase
	if req.Status.Phase == "" {
		req.Status.Phase = controller.AccessPhasePending
	}

	// Initialize status message
	if req.Status.Message == "" {
		req.Status.Message = "Access request created and pending approval"
	}

	// Set default labels
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}

	// Ensure required labels are set
	req.Labels["jit.rebelops.io/type"] = "access-request"
	req.Labels["jit.rebelops.io/phase"] = string(req.Status.Phase)
}

func (m *JITAccessRequestMutator) normalizeData(req *controller.JITAccessRequest) {
	// Normalize cluster name (lowercase)
	req.Spec.TargetCluster.Name = strings.ToLower(req.Spec.TargetCluster.Name)

	// Normalize region (lowercase)
	req.Spec.TargetCluster.Region = strings.ToLower(req.Spec.TargetCluster.Region)

	// Normalize permissions (lowercase and deduplicate)
	normalizedPerms := make(map[string]bool)
	for _, perm := range req.Spec.Permissions {
		normalizedPerms[strings.ToLower(perm)] = true
	}

	perms := make([]string, 0, len(normalizedPerms))
	for perm := range normalizedPerms {
		perms = append(perms, perm)
	}
	req.Spec.Permissions = perms

	// Normalize namespaces (lowercase and deduplicate)
	if len(req.Spec.Namespaces) > 0 {
		normalizedNS := make(map[string]bool)
		for _, ns := range req.Spec.Namespaces {
			normalizedNS[strings.ToLower(ns)] = true
		}

		namespaces := make([]string, 0, len(normalizedNS))
		for ns := range normalizedNS {
			namespaces = append(namespaces, ns)
		}
		req.Spec.Namespaces = namespaces
	}

	// Trim whitespace from reason
	req.Spec.Reason = strings.TrimSpace(req.Spec.Reason)

	// Normalize duration format
	req.Spec.Duration = normalizeDuration(req.Spec.Duration)
}

func (m *JITAccessRequestMutator) injectMetadata(req *controller.JITAccessRequest) {
	// Add annotations
	if req.Annotations == nil {
		req.Annotations = make(map[string]string)
	}

	// Track creation source
	if req.Annotations["jit.rebelops.io/created-by"] == "" {
		req.Annotations["jit.rebelops.io/created-by"] = "webhook"
	}

	// Add request metadata
	req.Annotations["jit.rebelops.io/requested-at"] = req.Spec.RequestedAt.Format("2006-01-02T15:04:05Z")
	req.Annotations["jit.rebelops.io/duration"] = req.Spec.Duration
	req.Annotations["jit.rebelops.io/cluster"] = req.Spec.TargetCluster.Name

	// Add user metadata to labels
	req.Labels["jit.rebelops.io/user"] = req.Spec.UserID
	req.Labels["jit.rebelops.io/cluster"] = req.Spec.TargetCluster.Name

	// Add environment label based on cluster name
	env := determineEnvironment(req.Spec.TargetCluster.Name)
	req.Labels["jit.rebelops.io/environment"] = env
}

func (m *JITAccessRequestMutator) setApprovers(req *controller.JITAccessRequest) {
	// If approvers are already set, respect them
	if len(req.Spec.Approvers) > 0 {
		return
	}

	// Determine required approvers based on cluster and permissions
	env := determineEnvironment(req.Spec.TargetCluster.Name)
	hasElevatedPerms := hasElevatedPermissions(req.Spec.Permissions)

	approvers := []string{}

	// Production clusters always require approval
	switch env {
	case envProduction:
		approvers = append(approvers, "platform-team", "sre-team")

		// Additional approval for elevated permissions in prod
		if hasElevatedPerms {
			approvers = append(approvers, "security-team")
		}
	case envStaging:
		// Staging requires approval for elevated permissions
		if hasElevatedPerms {
			approvers = append(approvers, "platform-team")
		}
	}
	// Development environments don't require approval for basic access

	// Remove duplicates
	uniqueApprovers := make(map[string]bool)
	for _, approver := range approvers {
		uniqueApprovers[approver] = true
	}

	finalApprovers := make([]string, 0, len(uniqueApprovers))
	for approver := range uniqueApprovers {
		finalApprovers = append(finalApprovers, approver)
	}

	req.Spec.Approvers = finalApprovers

	// Update annotations to track auto-assigned approvers
	if len(finalApprovers) > 0 {
		req.Annotations["jit.rebelops.io/auto-approvers"] = strings.Join(finalApprovers, ",")
	}
}

// Helper functions

func normalizeDuration(duration string) string {
	// Normalize common duration formats
	replacements := map[string]string{
		"min":     "m",
		"mins":    "m",
		"minute":  "m",
		"minutes": "m",
		"hour":    "h",
		"hours":   "h",
		"day":     "d",
		"days":    "d",
	}

	normalized := strings.ToLower(duration)
	for old, new := range replacements {
		normalized = strings.ReplaceAll(normalized, old, new)
	}

	// Remove spaces
	normalized = strings.ReplaceAll(normalized, " ", "")

	return normalized
}

func determineEnvironment(clusterName string) string {
	lowerName := strings.ToLower(clusterName)

	if strings.Contains(lowerName, "prod") || strings.Contains(lowerName, "production") {
		return envProduction
	}
	if strings.Contains(lowerName, "stag") || strings.Contains(lowerName, "staging") {
		return envStaging
	}
	if strings.Contains(lowerName, "dev") || strings.Contains(lowerName, "development") {
		return "development"
	}
	if strings.Contains(lowerName, "qa") || strings.Contains(lowerName, "test") {
		return "qa"
	}

	// Default to production for safety
	return envProduction
}

func hasElevatedPermissions(permissions []string) bool {
	elevatedPerms := map[string]bool{
		"admin":         true,
		"cluster-admin": true,
		"edit":          true,
		"exec":          true,
		"port-forward":  true,
		"debug":         true,
	}

	for _, perm := range permissions {
		if elevatedPerms[strings.ToLower(perm)] {
			return true
		}
	}

	return false
}

// MutateJITAccessJob mutates JITAccessJob resources
type JITAccessJobMutator struct {
	Client  client.Client
	decoder admission.Decoder
}

// Handle mutates JITAccessJob resources
func (j *JITAccessJobMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	job := &controller.JITAccessJob{}
	err := j.decoder.Decode(req, job)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Set defaults for job
	if job.Spec.CleanupPolicy == "" {
		job.Spec.CleanupPolicy = controller.CleanupPolicyOnExpiry
	}

	// Set default labels
	if job.Labels == nil {
		job.Labels = make(map[string]string)
	}

	job.Labels["jit.rebelops.io/type"] = "access-job"
	job.Labels["jit.rebelops.io/request"] = job.Spec.AccessRequestRef.Name
	job.Labels["jit.rebelops.io/cluster"] = job.Spec.TargetCluster.Name

	// Create patch
	marshaledJob, err := json.Marshal(job)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledJob)
}

// InjectDecoder injects the decoder
func (j *JITAccessJobMutator) InjectDecoder(d admission.Decoder) error {
	j.decoder = d
	return nil
}
