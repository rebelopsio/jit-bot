package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

func TestNewCommandHandler(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()

	handler := NewCommandHandler(rbac, memStore)
	if handler == nil {
		t.Fatal("NewCommandHandler returned nil")
	}

	if handler.rbac != rbac {
		t.Error("RBAC not set correctly")
	}

	if handler.store != memStore {
		t.Error("Store not set correctly")
	}
}

func createTestRequest(text, userID string) *http.Request {
	formData := url.Values{}
	formData.Set("command", "/jit")
	formData.Set("text", text)
	formData.Set("user_id", userID)
	formData.Set("user_name", "testuser")

	req := httptest.NewRequest(http.MethodPost, "/slack/commands", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestHandleJITCommandHelp(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("help", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["response_type"] != "ephemeral" {
		t.Error("Help response should be ephemeral")
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "JIT Access Commands") {
		t.Error("Help text should contain command information")
	}
}

func TestHandleJITCommandEmptyText(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Should return help when no subcommand provided
	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "JIT Access Commands") {
		t.Error("Should return help for empty command")
	}
}

func TestHandleRequestAccess(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	// Create test cluster
	cluster := &models.Cluster{
		ID:          "test-cluster",
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		MaxDuration: time.Hour,
		Enabled:     true,
		CreatedBy:   "admin1",
	}
	if err := memStore.CreateCluster(cluster); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	req := createTestRequest("request test-cluster debugging issue #1234", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["response_type"] != "in_channel" {
		t.Error("Access request response should be in_channel")
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Access request submitted") {
		t.Error("Should confirm access request submission")
	}
}

func TestHandleRequestAccessInvalidCluster(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("request nonexistent-cluster debugging", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "not found") {
		t.Error("Should return error for non-existent cluster")
	}
}

func TestHandleRequestAccessDisabledCluster(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	// Create disabled cluster
	cluster := &models.Cluster{
		ID:          "disabled-cluster",
		Name:        "disabled-cluster",
		DisplayName: "Disabled Cluster",
		MaxDuration: time.Hour,
		Enabled:     false,
		CreatedBy:   "admin1",
	}
	if err := memStore.CreateCluster(cluster); err != nil {
		t.Fatalf("Failed to create cluster: %v", err)
	}

	req := createTestRequest("request disabled-cluster debugging", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "disabled") {
		t.Error("Should return error for disabled cluster")
	}
}

func TestHandleRequestAccessInsufficientArgs(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("request cluster-only", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Usage:") {
		t.Error("Should return usage information for insufficient args")
	}
}

func TestHandleListClusters(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	// Create test clusters
	cluster1 := &models.Cluster{
		ID:          "cluster1",
		Name:        "cluster1",
		DisplayName: "Cluster 1",
		Environment: "prod",
		MaxDuration: time.Hour,
		Enabled:     true,
		CreatedBy:   "admin1",
	}
	cluster2 := &models.Cluster{
		ID:          "cluster2",
		Name:        "cluster2",
		DisplayName: "Cluster 2",
		Environment: "dev",
		MaxDuration: 30 * time.Minute,
		Enabled:     true,
		CreatedBy:   "admin1",
	}

	if err := memStore.CreateCluster(cluster1); err != nil {
		t.Fatalf("Failed to create cluster1: %v", err)
	}
	if err := memStore.CreateCluster(cluster2); err != nil {
		t.Fatalf("Failed to create cluster2: %v", err)
	}

	req := createTestRequest("list", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["response_type"] != "ephemeral" {
		t.Error("List response should be ephemeral")
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Available clusters") {
		t.Error("Should show available clusters")
	}
}

func TestHandleListClustersEmpty(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("list", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "No clusters available") {
		t.Error("Should show no clusters message")
	}
}

func TestHandleStatus(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	// Create test access
	access := &models.ClusterAccess{
		ID:          "access-123",
		ClusterID:   "cluster-123",
		UserID:      "user123",
		UserEmail:   "user@example.com",
		Reason:      "Testing",
		Duration:    time.Hour,
		Status:      models.AccessStatusPending,
		RequestedAt: time.Now(),
	}
	if err := memStore.CreateAccess(access); err != nil {
		t.Fatalf("Failed to create access: %v", err)
	}

	req := createTestRequest("status", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["response_type"] != "ephemeral" {
		t.Error("Status response should be ephemeral")
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Your access requests") {
		t.Error("Should show user's access requests")
	}
}

func TestHandleStatusEmpty(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("status", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "no active or pending") {
		t.Error("Should show no access requests message")
	}
}

func TestHandleAdmin(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("admin", "admin1")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Admin commands") {
		t.Error("Should show admin commands")
	}
}

func TestHandleAdminUnauthorized(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("admin", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "don't have admin permissions") {
		t.Error("Should deny access for non-admin user")
	}
}

func TestHandleUnknownCommand(t *testing.T) {
	rbac := auth.NewRBAC([]string{"admin1"})
	memStore := store.NewMemoryStore()
	handler := NewCommandHandler(rbac, memStore)

	req := createTestRequest("unknown-command", "user123")
	rr := httptest.NewRecorder()

	handler.HandleJITCommand(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	text, ok := response["text"].(string)
	if !ok || !strings.Contains(text, "Unknown command") {
		t.Error("Should return error for unknown command")
	}
}
