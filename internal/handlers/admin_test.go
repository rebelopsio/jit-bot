package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

func TestNewAdminHandler(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()

	handler := NewAdminHandler(rbac, memStore)
	if handler == nil {
		t.Fatal("NewAdminHandler returned nil")
	}

	if handler.rbac != rbac {
		t.Error("RBAC not set correctly")
	}

	if handler.store != memStore {
		t.Error("Store not set correctly")
	}
}

func TestCreateCluster(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	cluster := models.Cluster{
		Name:              "test-cluster",
		DisplayName:       "Test Cluster",
		AWSAccount:        "123456789012",
		Region:            "us-east-1",
		Environment:       "test",
		MaxDuration:       time.Hour,
		RequiredApprovers: 1,
		Enabled:           true,
	}

	body, _ := json.Marshal(cluster)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-User-Id", "admin1")

	rr := httptest.NewRecorder()
	handler.CreateCluster(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response models.Cluster
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Name != cluster.Name {
		t.Errorf("Expected cluster name %s, got %s", cluster.Name, response.Name)
	}

	if response.ID == "" {
		t.Error("Cluster ID should be generated")
	}

	if response.CreatedBy != "admin1" {
		t.Errorf("Expected CreatedBy to be admin1, got %s", response.CreatedBy)
	}
}

func TestCreateClusterUnauthorized(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	cluster := models.Cluster{Name: "test-cluster"}
	body, _ := json.Marshal(cluster)

	// Test without user ID
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler.CreateCluster(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Test with non-admin user
	req = httptest.NewRequest(http.MethodPost, "/api/v1/clusters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-User-Id", "requester1")

	rr = httptest.NewRecorder()
	handler.CreateCluster(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}

func TestListClusters(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	rbac.SetUserRole("requester1", auth.RoleRequester)
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	// Add test clusters
	cluster1 := &models.Cluster{
		ID:        "cluster1",
		Name:      "cluster1",
		CreatedBy: "admin1",
	}
	cluster2 := &models.Cluster{
		ID:        "cluster2",
		Name:      "cluster2",
		CreatedBy: "admin1",
	}

	if err := memStore.CreateCluster(cluster1); err != nil {
		t.Fatalf("Failed to create cluster1: %v", err)
	}
	if err := memStore.CreateCluster(cluster2); err != nil {
		t.Fatalf("Failed to create cluster2: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	req.Header.Set("X-Slack-User-Id", "requester1")

	rr := httptest.NewRecorder()
	handler.ListClusters(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var clusters []*models.Cluster
	if err := json.NewDecoder(rr.Body).Decode(&clusters); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}
}

func TestUpdateCluster(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	// Create initial cluster
	cluster := &models.Cluster{
		ID:          "test-cluster",
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		CreatedBy:   "admin1",
	}
	if err := memStore.CreateCluster(cluster); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	// Update cluster
	cluster.DisplayName = "Updated Test Cluster"
	body, _ := json.Marshal(cluster)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/clusters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-User-Id", "admin1")

	rr := httptest.NewRecorder()
	handler.UpdateCluster(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response models.Cluster
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.DisplayName != "Updated Test Cluster" {
		t.Errorf("Expected DisplayName to be updated, got %s", response.DisplayName)
	}
}

func TestDeleteCluster(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	// Create cluster to delete
	cluster := &models.Cluster{
		ID:        "test-cluster",
		Name:      "test-cluster",
		CreatedBy: "admin1",
	}
	if err := memStore.CreateCluster(cluster); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters?id=test-cluster", nil)
	req.Header.Set("X-Slack-User-Id", "admin1")

	rr := httptest.NewRecorder()
	handler.DeleteCluster(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, rr.Code)
	}

	// Verify cluster is deleted
	_, err := memStore.GetCluster("test-cluster")
	if err == nil {
		t.Error("Cluster should be deleted")
	}
}

func TestDeleteClusterMissingID(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/clusters", nil)
	req.Header.Set("X-Slack-User-Id", "admin1")

	rr := httptest.NewRecorder()
	handler.DeleteCluster(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestManageUser(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	userRequest := struct {
		UserID string    `json:"user_id"`
		Role   auth.Role `json:"role"`
	}{
		UserID: "user123",
		Role:   auth.RoleApprover,
	}

	body, _ := json.Marshal(userRequest)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-User-Id", "admin1")

	rr := httptest.NewRecorder()
	handler.ManageUser(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify role was set
	if rbac.GetUserRole("user123") != auth.RoleApprover {
		t.Error("User role should be set to approver")
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["user_id"] != "user123" {
		t.Errorf("Expected user_id user123, got %s", response["user_id"])
	}
}

func TestManageUserUnauthorized(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	rbac.SetUserRole("approver1", auth.RoleApprover)
	memStore := store.NewMemoryStore()
	handler := NewAdminHandler(rbac, memStore)

	userRequest := struct {
		UserID string    `json:"user_id"`
		Role   auth.Role `json:"role"`
	}{
		UserID: "user123",
		Role:   auth.RoleApprover,
	}

	body, _ := json.Marshal(userRequest)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/role", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-User-Id", "approver1") // Approver trying to manage users

	rr := httptest.NewRecorder()
	handler.ManageUser(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, rr.Code)
	}
}
