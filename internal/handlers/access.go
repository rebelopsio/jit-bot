package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/kubernetes"
	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

type AccessHandler struct {
	rbac          *auth.RBAC
	store         *store.MemoryStore
	accessManager *kubernetes.AccessManager
	region        string
}

type GrantAccessRequest struct {
	ClusterID     string   `json:"cluster_id"`
	UserID        string   `json:"user_id"`
	UserEmail     string   `json:"user_email"`
	Permissions   []string `json:"permissions"`
	Namespaces    []string `json:"namespaces"`
	Duration      string   `json:"duration"`
	Reason        string   `json:"reason"`
	JITRoleArn    string   `json:"jit_role_arn"`
	AssumeRoleArn string   `json:"assume_role_arn,omitempty"`
}

type AccessResponse struct {
	AccessID             string    `json:"access_id"`
	ClusterName          string    `json:"cluster_name"`
	UserID               string    `json:"user_id"`
	KubeConfig           string    `json:"kubeconfig"`
	ClusterEndpoint      string    `json:"cluster_endpoint"`
	ExpiresAt            time.Time `json:"expires_at"`
	TemporaryCredentials struct {
		AccessKeyID     string    `json:"access_key_id"`
		SecretAccessKey string    `json:"secret_access_key"`
		SessionToken    string    `json:"session_token"`
		Expiration      time.Time `json:"expiration"`
	} `json:"temporary_credentials"`
}

type RevokeAccessRequest struct {
	AccessID string `json:"access_id"`
}

func NewAccessHandler(
	rbac *auth.RBAC,
	store *store.MemoryStore,
	region string,
) (*AccessHandler, error) {
	accessManager, err := kubernetes.NewAccessManager(region)
	if err != nil {
		return nil, fmt.Errorf("failed to create access manager: %w", err)
	}

	return &AccessHandler{
		rbac:          rbac,
		store:         store,
		accessManager: accessManager,
		region:        region,
	}, nil
}

func (h *AccessHandler) GrantAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	// Check permissions
	if err := h.rbac.ValidatePermission(userID, auth.PermissionCreateRequests); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var req GrantAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ClusterID == "" || req.UserID == "" || req.UserEmail == "" {
		http.Error(
			w,
			"missing required fields: cluster_id, user_id, user_email",
			http.StatusBadRequest,
		)
		return
	}

	// Get cluster information
	cluster, err := h.store.GetCluster(req.ClusterID)
	if err != nil {
		http.Error(w, fmt.Sprintf("cluster not found: %s", req.ClusterID), http.StatusNotFound)
		return
	}

	// Parse duration
	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		http.Error(
			w,
			fmt.Sprintf("invalid duration format: %s", req.Duration),
			http.StatusBadRequest,
		)
		return
	}

	// Validate duration against cluster limits
	if duration > cluster.MaxDuration {
		http.Error(w, fmt.Sprintf("requested duration %s exceeds cluster limit %s",
			duration, cluster.MaxDuration), http.StatusBadRequest)
		return
	}

	// Set defaults
	if len(req.Permissions) == 0 {
		req.Permissions = []string{"view"}
	}
	if req.JITRoleArn == "" {
		// This should be configured per cluster or globally
		req.JITRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/JITAccessRole", cluster.AWSAccount)
	}

	// Create access record
	accessID := uuid.New().String()
	expiresAt := time.Now().Add(duration)
	clusterAccess := &models.ClusterAccess{
		ID:          accessID,
		UserID:      req.UserID,
		UserEmail:   req.UserEmail,
		ClusterID:   req.ClusterID,
		Duration:    duration,
		Status:      models.AccessStatusActive,
		RequestedAt: time.Now(),
		ExpiresAt:   &expiresAt,
		Reason:      req.Reason,
	}

	// Grant actual access through AWS
	accessRequest := kubernetes.GrantAccessRequest{
		ClusterAccess: clusterAccess,
		Cluster:       cluster,
		UserEmail:     req.UserEmail,
		Permissions:   req.Permissions,
		Namespaces:    req.Namespaces,
		JITRoleArn:    req.JITRoleArn,
		AssumeRoleArn: req.AssumeRoleArn,
	}

	credentials, err := h.accessManager.GrantAccess(ctx, accessRequest)
	if err != nil {
		http.Error(
			w,
			fmt.Sprintf("failed to grant access: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	// Store the access record
	if storeErr := h.store.CreateClusterAccess(clusterAccess); storeErr != nil {
		// Log error but don't fail the request since AWS access was already granted
		// TODO: Implement rollback mechanism
		http.Error(
			w,
			fmt.Sprintf("access granted but failed to store record: %v", storeErr),
			http.StatusInternalServerError,
		)
		return
	}

	// Prepare response
	response := AccessResponse{
		AccessID:        accessID,
		ClusterName:     cluster.Name,
		UserID:          req.UserID,
		KubeConfig:      credentials.KubeConfig,
		ClusterEndpoint: credentials.ClusterEndpoint,
		ExpiresAt:       credentials.ExpiresAt,
	}

	response.TemporaryCredentials.AccessKeyID = credentials.TemporaryCredentials.AccessKeyID
	response.TemporaryCredentials.SecretAccessKey = credentials.TemporaryCredentials.SecretAccessKey
	response.TemporaryCredentials.SessionToken = credentials.TemporaryCredentials.SessionToken
	response.TemporaryCredentials.Expiration = credentials.TemporaryCredentials.Expiration

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(response); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AccessHandler) RevokeAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	var req RevokeAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get access record
	clusterAccess, err := h.store.GetClusterAccess(req.AccessID)
	if err != nil {
		http.Error(w, fmt.Sprintf("access record not found: %s", req.AccessID), http.StatusNotFound)
		return
	}

	// Check permissions - user can revoke their own access or admins can revoke any
	if clusterAccess.UserID != userID {
		if permErr := h.rbac.ValidatePermission(userID, auth.PermissionRevokeAccess); permErr != nil {
			http.Error(w, permErr.Error(), http.StatusForbidden)
			return
		}
	}

	// Get cluster information
	cluster, err := h.store.GetCluster(clusterAccess.ClusterID)
	if err != nil {
		http.Error(
			w,
			fmt.Sprintf("cluster not found: %s", clusterAccess.ClusterID),
			http.StatusNotFound,
		)
		return
	}

	// Revoke access through AWS
	jitRoleArn := fmt.Sprintf("arn:aws:iam::%s:role/JITAccessRole", cluster.AWSAccount)
	if revokeErr := h.accessManager.RevokeAccess(ctx, clusterAccess, cluster, jitRoleArn); revokeErr != nil {
		http.Error(
			w,
			fmt.Sprintf("failed to revoke access: %v", revokeErr),
			http.StatusInternalServerError,
		)
		return
	}

	// Update access record
	clusterAccess.Status = models.AccessStatusRevoked
	clusterAccess.RevokedAt = &time.Time{}
	*clusterAccess.RevokedAt = time.Now()

	if updateErr := h.store.UpdateClusterAccess(clusterAccess); updateErr != nil {
		// Log error but don't fail since AWS access was revoked
		http.Error(w, fmt.Sprintf("access revoked but failed to update record: %v", updateErr),
			http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AccessHandler) ListAccess(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	// Check permissions
	if err := h.rbac.ValidatePermission(userID, auth.PermissionViewRequests); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Get query parameters
	filterUserID := r.URL.Query().Get("user_id")
	filterClusterID := r.URL.Query().Get("cluster_id")
	activeOnly := r.URL.Query().Get("active") == "true"

	accessList, err := h.store.ListClusterAccess()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Apply filters
	var filteredAccess []*models.ClusterAccess
	for _, access := range accessList {
		// Filter by user if specified
		if filterUserID != "" && access.UserID != filterUserID {
			continue
		}

		// Filter by cluster if specified
		if filterClusterID != "" && access.ClusterID != filterClusterID {
			continue
		}

		// Filter active only if specified
		if activeOnly && access.Status != models.AccessStatusActive {
			continue
		}

		// Skip expired access if active only is requested
		if activeOnly && access.ExpiresAt != nil && time.Now().After(*access.ExpiresAt) {
			continue
		}

		filteredAccess = append(filteredAccess, access)
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(filteredAccess); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AccessHandler) CleanupExpiredAccess(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	// Check permissions - only admins can trigger cleanup
	if err := h.rbac.ValidatePermission(userID, auth.PermissionManageClusters); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	clusterID := r.URL.Query().Get("cluster_id")
	if clusterID == "" {
		http.Error(w, "missing cluster_id parameter", http.StatusBadRequest)
		return
	}

	cluster, err := h.store.GetCluster(clusterID)
	if err != nil {
		http.Error(w, fmt.Sprintf("cluster not found: %s", clusterID), http.StatusNotFound)
		return
	}

	// Clean up expired access through AWS
	if cleanupErr := h.accessManager.CleanupExpiredAccess(ctx, cluster.Name); cleanupErr != nil {
		http.Error(
			w,
			fmt.Sprintf("failed to cleanup expired access: %v", cleanupErr),
			http.StatusInternalServerError,
		)
		return
	}

	// Update local records to mark them as expired
	accessList, err := h.store.ListClusterAccess()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cleanedCount := 0
	for _, access := range accessList {
		if access.ClusterID == clusterID &&
			access.Status == models.AccessStatusActive &&
			access.ExpiresAt != nil &&
			time.Now().After(*access.ExpiresAt) {
			access.Status = models.AccessStatusExpired
			if updateErr := h.store.UpdateClusterAccess(access); updateErr != nil {
				// Log error but continue
				continue
			}
			cleanedCount++
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       "cleanup completed",
		"cluster_name":  cluster.Name,
		"cleaned_count": cleanedCount,
	}); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AccessHandler) GetAccessStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	accessID := r.URL.Query().Get("access_id")
	if accessID == "" {
		http.Error(w, "missing access_id parameter", http.StatusBadRequest)
		return
	}

	clusterAccess, err := h.store.GetClusterAccess(accessID)
	if err != nil {
		http.Error(w, fmt.Sprintf("access record not found: %s", accessID), http.StatusNotFound)
		return
	}

	// Check permissions - user can view their own access or admins can view any
	if clusterAccess.UserID != userID {
		if permErr := h.rbac.ValidatePermission(userID, auth.PermissionViewRequests); permErr != nil {
			http.Error(w, permErr.Error(), http.StatusForbidden)
			return
		}
	}

	// Update status if expired
	if clusterAccess.ExpiresAt != nil && time.Now().After(*clusterAccess.ExpiresAt) &&
		clusterAccess.Status == models.AccessStatusActive {
		clusterAccess.Status = models.AccessStatusExpired
		_ = h.store.UpdateClusterAccess(clusterAccess) // Ignore error for status check
	}

	w.Header().Set("Content-Type", "application/json")
	if encodeErr := json.NewEncoder(w).Encode(clusterAccess); encodeErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
