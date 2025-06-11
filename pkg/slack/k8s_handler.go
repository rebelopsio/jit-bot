package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/controller"
)

// SlackResponse represents a response to a Slack command
type SlackResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
}

// K8sCommandHandler handles Slack commands by creating Kubernetes resources
type K8sCommandHandler struct {
	client    client.Client
	rbac      *auth.RBAC
	namespace string
}

func NewK8sCommandHandler(client client.Client, rbac *auth.RBAC, namespace string) *K8sCommandHandler {
	return &K8sCommandHandler{
		client:    client,
		rbac:      rbac,
		namespace: namespace,
	}
}

// HandleRequestCommand processes /jit request commands
func (h *K8sCommandHandler) HandleRequestCommand(
	ctx context.Context,
	cmd SlackCommand,
	args []string,
) (*SlackResponse, error) {
	// Check user permissions
	if !h.rbac.UserHasPermission(cmd.UserID, auth.PermissionCreateRequests) {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "❌ You don't have permission to create JIT access requests",
		}, nil
	}

	// Parse command arguments
	if len(args) < 3 {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "❌ Usage: /jit request <cluster> <duration> <reason> [permissions] [namespaces]",
		}, nil
	}

	clusterName := args[0]
	duration := args[1]
	reason := strings.Join(args[2:], " ")

	// Parse optional parameters
	permissions := []string{"view"} // Default permission
	var namespaces []string

	// Look for --permissions and --namespaces flags
	for i, arg := range args {
		if arg == "--permissions" && i+1 < len(args) {
			permissions = strings.Split(args[i+1], ",")
		}
		if arg == "--namespaces" && i+1 < len(args) {
			namespaces = strings.Split(args[i+1], ",")
		}
	}

	// Create JITAccessRequest
	request := &controller.JITAccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("jit-%s-%d", cmd.UserID, time.Now().Unix()),
			Namespace: h.namespace,
			Labels: map[string]string{
				"jit.rebelops.io/user":    cmd.UserID,
				"jit.rebelops.io/cluster": clusterName,
				"jit.rebelops.io/channel": cmd.ChannelID,
			},
		},
		Spec: controller.JITAccessRequestSpec{
			UserID:    cmd.UserID,
			UserEmail: fmt.Sprintf("%s@company.com", cmd.UserName), // This should come from user profile
			TargetCluster: controller.TargetCluster{
				Name:       clusterName,
				AWSAccount: h.getClusterAccount(clusterName),
				Region:     h.getClusterRegion(clusterName),
			},
			Reason:       reason,
			Duration:     duration,
			Permissions:  permissions,
			Namespaces:   namespaces,
			Approvers:    h.getRequiredApprovers(clusterName, permissions),
			SlackChannel: cmd.ChannelID,
			RequestedAt:  metav1.Now(),
		},
	}

	if err := h.client.Create(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to create JIT access request: %w", err)
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Text: fmt.Sprintf(
			"✅ JIT access request created: `%s`\n🎯 Cluster: %s\n⏱️ Duration: %s\n📝 Reason: %s",
			request.Name,
			clusterName,
			duration,
			reason,
		),
	}, nil
}

// HandleApproveCommand processes /jit approve commands
func (h *K8sCommandHandler) HandleApproveCommand(
	ctx context.Context,
	cmd SlackCommand,
	args []string,
) (*SlackResponse, error) {
	// Check user permissions
	if !h.rbac.UserHasPermission(cmd.UserID, auth.PermissionApproveRequests) {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "❌ You don't have permission to approve JIT access requests",
		}, nil
	}

	if len(args) < 1 {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "❌ Usage: /jit approve <request-name> [comment]",
		}, nil
	}

	requestName := args[0]
	comment := ""
	if len(args) > 1 {
		comment = strings.Join(args[1:], " ")
	}

	// Fetch the request
	var request controller.JITAccessRequest
	if err := h.client.Get(ctx, client.ObjectKey{Name: requestName, Namespace: h.namespace}, &request); err != nil {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         fmt.Sprintf("❌ Request not found: %s", requestName),
		}, err
	}

	// Add approval
	approval := controller.Approval{
		Approver:   cmd.UserID,
		ApprovedAt: metav1.Now(),
		Comment:    comment,
	}

	request.Status.Approvals = append(request.Status.Approvals, approval)

	if err := h.client.Status().Update(ctx, &request); err != nil {
		return nil, fmt.Errorf("failed to update request approval: %w", err)
	}

	return &SlackResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("✅ Approved request: `%s` by <@%s>", requestName, cmd.UserID),
	}, nil
}

// HandleListCommand processes /jit list commands
func (h *K8sCommandHandler) HandleListCommand(
	ctx context.Context,
	cmd SlackCommand,
	args []string,
) (*SlackResponse, error) {
	var requestList controller.JITAccessRequestList
	listOpts := []client.ListOption{
		client.InNamespace(h.namespace),
	}

	// Filter by user if requested
	if len(args) > 0 && args[0] == "mine" {
		listOpts = append(listOpts, client.MatchingLabels{"jit.rebelops.io/user": cmd.UserID})
	}

	if err := h.client.List(ctx, &requestList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}

	if len(requestList.Items) == 0 {
		return &SlackResponse{
			ResponseType: "ephemeral",
			Text:         "📋 No JIT access requests found",
		}, nil
	}

	var response strings.Builder
	response.WriteString("📋 JIT Access Requests:\n")

	for _, req := range requestList.Items {
		status := "❓"
		switch req.Status.Phase {
		case controller.AccessPhasePending:
			status = "⏳"
		case controller.AccessPhaseApproved:
			status = "✅"
		case controller.AccessPhaseDenied:
			status = "❌"
		case controller.AccessPhaseActive:
			status = "🟢"
		case controller.AccessPhaseExpired:
			status = "⏰"
		case controller.AccessPhaseRevoked:
			status = "🔴"
		}

		response.WriteString(fmt.Sprintf("%s `%s` - %s (%s) - %s\n",
			status, req.Name, req.Spec.TargetCluster.Name, req.Spec.Duration, req.Status.Phase))
	}

	return &SlackResponse{
		ResponseType: "ephemeral",
		Text:         response.String(),
	}, nil
}

// Helper functions
func (h *K8sCommandHandler) getClusterAccount(clusterName string) string {
	// This should be configurable via ConfigMap
	clusterConfigs := map[string]string{
		"prod-east-1":    "123456789012",
		"staging-east-1": "123456789012",
		"dev-west-2":     "987654321098",
	}

	if account, exists := clusterConfigs[clusterName]; exists {
		return account
	}
	return "123456789012" // Default account
}

func (h *K8sCommandHandler) getClusterRegion(clusterName string) string {
	// This should be configurable via ConfigMap
	clusterConfigs := map[string]string{
		"prod-east-1":    "us-east-1",
		"staging-east-1": "us-east-1",
		"dev-west-2":     "us-west-2",
	}

	if region, exists := clusterConfigs[clusterName]; exists {
		return region
	}
	return "us-east-1" // Default region
}

func (h *K8sCommandHandler) getRequiredApprovers(clusterName string, permissions []string) []string {
	// Define approval policies
	if strings.Contains(clusterName, "prod") {
		return []string{"platform-team", "sre-team"}
	}

	// Check for elevated permissions
	for _, perm := range permissions {
		if perm == "admin" || perm == "cluster-admin" {
			return []string{"platform-team"}
		}
	}

	return []string{} // No approval required for basic access to non-prod
}
