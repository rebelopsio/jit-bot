package store

import (
	"testing"
	"time"

	"github.com/rebelopsio/jit-bot/pkg/models"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}

	if store.clusters == nil {
		t.Error("clusters map should be initialized")
	}

	if store.accesses == nil {
		t.Error("accesses map should be initialized")
	}
}

func TestCreateCluster(t *testing.T) {
	store := NewMemoryStore()
	cluster := &models.Cluster{
		ID:          "test-cluster",
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		AWSAccount:  "123456789012",
		Region:      "us-east-1",
		Environment: "test",
		MaxDuration: time.Hour,
		Enabled:     true,
		CreatedBy:   "admin1",
	}

	err := store.CreateCluster(cluster)
	if err != nil {
		t.Fatalf("CreateCluster failed: %v", err)
	}

	// Verify cluster was created with timestamps
	if cluster.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if cluster.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}

	// Try to create duplicate cluster
	err = store.CreateCluster(cluster)
	if err == nil {
		t.Error("Creating duplicate cluster should return error")
	}
}

func TestGetCluster(t *testing.T) {
	store := NewMemoryStore()
	originalCluster := &models.Cluster{
		ID:          "test-cluster",
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		AWSAccount:  "123456789012",
		Region:      "us-east-1",
		Environment: "test",
		MaxDuration: time.Hour,
		Enabled:     true,
		CreatedBy:   "admin1",
	}

	store.CreateCluster(originalCluster)

	// Get existing cluster
	cluster, err := store.GetCluster("test-cluster")
	if err != nil {
		t.Fatalf("GetCluster failed: %v", err)
	}

	if cluster.ID != originalCluster.ID {
		t.Errorf("Expected cluster ID %s, got %s", originalCluster.ID, cluster.ID)
	}

	// Get non-existent cluster
	_, err = store.GetCluster("non-existent")
	if err == nil {
		t.Error("Getting non-existent cluster should return error")
	}
}

func TestListClusters(t *testing.T) {
	store := NewMemoryStore()

	// List empty clusters
	clusters, err := store.ListClusters()
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}

	if len(clusters) != 0 {
		t.Errorf("Expected 0 clusters, got %d", len(clusters))
	}

	// Add clusters
	cluster1 := &models.Cluster{ID: "cluster1", Name: "cluster1", CreatedBy: "admin1"}
	cluster2 := &models.Cluster{ID: "cluster2", Name: "cluster2", CreatedBy: "admin1"}

	store.CreateCluster(cluster1)
	store.CreateCluster(cluster2)

	clusters, err = store.ListClusters()
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}

	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}
}

func TestUpdateCluster(t *testing.T) {
	store := NewMemoryStore()
	cluster := &models.Cluster{
		ID:          "test-cluster",
		Name:        "test-cluster",
		DisplayName: "Test Cluster",
		AWSAccount:  "123456789012",
		Region:      "us-east-1",
		Environment: "test",
		MaxDuration: time.Hour,
		Enabled:     true,
		CreatedBy:   "admin1",
	}

	store.CreateCluster(cluster)
	originalUpdatedAt := cluster.UpdatedAt

	// Update cluster
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	cluster.DisplayName = "Updated Test Cluster"
	err := store.UpdateCluster(cluster)
	if err != nil {
		t.Fatalf("UpdateCluster failed: %v", err)
	}

	// Verify UpdatedAt was changed
	if !cluster.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}

	// Try to update non-existent cluster
	nonExistent := &models.Cluster{ID: "non-existent"}
	err = store.UpdateCluster(nonExistent)
	if err == nil {
		t.Error("Updating non-existent cluster should return error")
	}
}

func TestDeleteCluster(t *testing.T) {
	store := NewMemoryStore()
	cluster := &models.Cluster{
		ID:        "test-cluster",
		Name:      "test-cluster",
		CreatedBy: "admin1",
	}

	store.CreateCluster(cluster)

	// Delete existing cluster
	err := store.DeleteCluster("test-cluster")
	if err != nil {
		t.Fatalf("DeleteCluster failed: %v", err)
	}

	// Verify cluster is deleted
	_, err = store.GetCluster("test-cluster")
	if err == nil {
		t.Error("Cluster should be deleted")
	}

	// Try to delete non-existent cluster
	err = store.DeleteCluster("non-existent")
	if err == nil {
		t.Error("Deleting non-existent cluster should return error")
	}
}

func TestCreateAccess(t *testing.T) {
	store := NewMemoryStore()
	access := &models.ClusterAccess{
		ID:          "access-123",
		ClusterID:   "cluster-123",
		UserID:      "user-123",
		UserEmail:   "user@example.com",
		Reason:      "Testing",
		Duration:    time.Hour,
		Status:      models.AccessStatusPending,
		RequestedAt: time.Now(),
	}

	err := store.CreateAccess(access)
	if err != nil {
		t.Fatalf("CreateAccess failed: %v", err)
	}

	// Try to create duplicate access
	err = store.CreateAccess(access)
	if err == nil {
		t.Error("Creating duplicate access should return error")
	}
}

func TestGetAccess(t *testing.T) {
	store := NewMemoryStore()
	originalAccess := &models.ClusterAccess{
		ID:          "access-123",
		ClusterID:   "cluster-123",
		UserID:      "user-123",
		UserEmail:   "user@example.com",
		Reason:      "Testing",
		Duration:    time.Hour,
		Status:      models.AccessStatusPending,
		RequestedAt: time.Now(),
	}

	store.CreateAccess(originalAccess)

	// Get existing access
	access, err := store.GetAccess("access-123")
	if err != nil {
		t.Fatalf("GetAccess failed: %v", err)
	}

	if access.ID != originalAccess.ID {
		t.Errorf("Expected access ID %s, got %s", originalAccess.ID, access.ID)
	}

	// Get non-existent access
	_, err = store.GetAccess("non-existent")
	if err == nil {
		t.Error("Getting non-existent access should return error")
	}
}

func TestListUserAccesses(t *testing.T) {
	store := NewMemoryStore()

	access1 := &models.ClusterAccess{
		ID:        "access-1",
		ClusterID: "cluster-1",
		UserID:    "user-123",
		Status:    models.AccessStatusPending,
	}

	access2 := &models.ClusterAccess{
		ID:        "access-2",
		ClusterID: "cluster-2",
		UserID:    "user-123",
		Status:    models.AccessStatusActive,
	}

	access3 := &models.ClusterAccess{
		ID:        "access-3",
		ClusterID: "cluster-1",
		UserID:    "user-456",
		Status:    models.AccessStatusPending,
	}

	store.CreateAccess(access1)
	store.CreateAccess(access2)
	store.CreateAccess(access3)

	// List accesses for user-123
	accesses, err := store.ListUserAccesses("user-123")
	if err != nil {
		t.Fatalf("ListUserAccesses failed: %v", err)
	}

	if len(accesses) != 2 {
		t.Errorf("Expected 2 accesses for user-123, got %d", len(accesses))
	}

	// List accesses for user with no accesses
	accesses, err = store.ListUserAccesses("user-789")
	if err != nil {
		t.Fatalf("ListUserAccesses failed: %v", err)
	}

	if len(accesses) != 0 {
		t.Errorf("Expected 0 accesses for user-789, got %d", len(accesses))
	}
}
