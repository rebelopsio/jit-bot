package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

type AdminHandler struct {
	rbac  *auth.RBAC
	store *store.MemoryStore
}

func NewAdminHandler(rbac *auth.RBAC, store *store.MemoryStore) *AdminHandler {
	return &AdminHandler{
		rbac:  rbac,
		store: store,
	}
}

func (h *AdminHandler) CreateCluster(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	if err := h.rbac.ValidatePermission(userID, auth.PermissionManageClusters); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var req models.Cluster
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cluster := &models.Cluster{
		ID:                uuid.New().String(),
		Name:              req.Name,
		DisplayName:       req.DisplayName,
		AWSAccount:        req.AWSAccount,
		Region:            req.Region,
		Environment:       req.Environment,
		Tags:              req.Tags,
		MaxDuration:       req.MaxDuration,
		RequiredApprovers: req.RequiredApprovers,
		Enabled:           req.Enabled,
		CreatedBy:         userID,
	}

	if cluster.MaxDuration == 0 {
		cluster.MaxDuration = 1 * time.Hour
	}

	if err := h.store.CreateCluster(cluster); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cluster); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AdminHandler) ListClusters(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	if err := h.rbac.ValidatePermission(userID, auth.PermissionViewRequests); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	clusters, err := h.store.ListClusters()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(clusters); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AdminHandler) UpdateCluster(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	if err := h.rbac.ValidatePermission(userID, auth.PermissionManageClusters); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var cluster models.Cluster
	if err := json.NewDecoder(r.Body).Decode(&cluster); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.store.UpdateCluster(&cluster); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cluster); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AdminHandler) DeleteCluster(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	if err := h.rbac.ValidatePermission(userID, auth.PermissionManageClusters); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	clusterID := r.URL.Query().Get("id")
	if clusterID == "" {
		http.Error(w, "missing cluster ID", http.StatusBadRequest)
		return
	}

	if err := h.store.DeleteCluster(clusterID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) ManageUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-Slack-User-Id")
	if userID == "" {
		http.Error(w, "missing user ID", http.StatusUnauthorized)
		return
	}

	if err := h.rbac.ValidatePermission(userID, auth.PermissionManageUsers); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	var req struct {
		UserID string    `json:"user_id"`
		Role   auth.Role `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	h.rbac.SetUserRole(req.UserID, req.Role)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "user role updated",
		"user_id": req.UserID,
		"role":    string(req.Role),
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
