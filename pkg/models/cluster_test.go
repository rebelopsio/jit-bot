package models

import (
	"testing"
	"time"
)

func TestAccessStatus(t *testing.T) {
	// Test that all access statuses are defined
	statuses := []AccessStatus{
		AccessStatusPending,
		AccessStatusApproved,
		AccessStatusDenied,
		AccessStatusActive,
		AccessStatusExpired,
		AccessStatusRevoked,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("AccessStatus should not be empty: %v", status)
		}
	}
}

func TestClusterValidation(t *testing.T) {
	cluster := &Cluster{
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

	// Basic validation tests
	if cluster.ID == "" {
		t.Error("Cluster ID should not be empty")
	}

	if cluster.MaxDuration <= 0 {
		t.Error("Cluster MaxDuration should be positive")
	}

	if cluster.AWSAccount == "" {
		t.Error("Cluster AWSAccount should not be empty")
	}

	if !cluster.Enabled {
		t.Error("Cluster should be enabled")
	}

	if cluster.CreatedBy != "admin1" {
		t.Errorf("Cluster CreatedBy should be admin1, got %s", cluster.CreatedBy)
	}

	if cluster.Name != "test-cluster" {
		t.Errorf("Cluster Name should be test-cluster, got %s", cluster.Name)
	}

	if cluster.DisplayName != "Test Cluster" {
		t.Errorf("Cluster DisplayName should be Test Cluster, got %s", cluster.DisplayName)
	}

	if cluster.Region != "us-east-1" {
		t.Errorf("Cluster Region should be us-east-1, got %s", cluster.Region)
	}

	if cluster.Environment != "test" {
		t.Errorf("Cluster Environment should be test, got %s", cluster.Environment)
	}
}

func TestClusterAccessValidation(t *testing.T) {
	access := &ClusterAccess{
		ID:          "access-123",
		ClusterID:   "cluster-123",
		UserID:      "user-123",
		UserEmail:   "user@example.com",
		Reason:      "Testing access",
		Duration:    time.Hour,
		Status:      AccessStatusPending,
		RequestedAt: time.Now(),
	}

	// Basic validation tests
	if access.ID == "" {
		t.Error("ClusterAccess ID should not be empty")
	}

	if access.ClusterID == "" {
		t.Error("ClusterAccess ClusterID should not be empty")
	}

	if access.UserID == "" {
		t.Error("ClusterAccess UserID should not be empty")
	}

	if access.Duration <= 0 {
		t.Error("ClusterAccess Duration should be positive")
	}

	if access.Reason == "" {
		t.Error("ClusterAccess Reason should not be empty")
	}

	if access.UserEmail != "user@example.com" {
		t.Errorf("ClusterAccess UserEmail should be user@example.com, got %s", access.UserEmail)
	}

	if access.Status != AccessStatusPending {
		t.Errorf("ClusterAccess Status should be pending, got %s", access.Status)
	}

	if access.RequestedAt.IsZero() {
		t.Error("ClusterAccess RequestedAt should not be zero")
	}
}

func TestAccessRequest(t *testing.T) {
	request := &AccessRequest{
		ClusterID: "cluster-123",
		Reason:    "Testing",
		Duration:  time.Hour,
	}

	if request.ClusterID == "" {
		t.Error("AccessRequest ClusterID should not be empty")
	}

	if request.Reason == "" {
		t.Error("AccessRequest Reason should not be empty")
	}

	if request.Duration <= 0 {
		t.Error("AccessRequest Duration should be positive")
	}
}
