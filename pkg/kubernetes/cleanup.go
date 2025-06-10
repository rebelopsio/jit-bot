package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

type CleanupService struct {
	accessManager *AccessManager
	store         *store.MemoryStore
	region        string
}

func NewCleanupService(region string, store *store.MemoryStore) (*CleanupService, error) {
	accessManager, err := NewAccessManager(region)
	if err != nil {
		return nil, fmt.Errorf("failed to create access manager: %w", err)
	}

	return &CleanupService{
		accessManager: accessManager,
		store:         store,
		region:        region,
	}, nil
}

// StartCleanupWorker starts a background worker that periodically cleans up expired access
func (cs *CleanupService) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Starting cleanup worker", "interval", interval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Cleanup worker stopped")
			return
		case <-ticker.C:
			if err := cs.performCleanup(ctx); err != nil {
				slog.Error("Cleanup error", "error", err)
			}
		}
	}
}

func (cs *CleanupService) performCleanup(ctx context.Context) error {
	slog.Info("Starting cleanup cycle")

	// Get all clusters to check for expired access
	clusters, err := cs.store.ListClusters()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range clusters {
		if cleanupErr := cs.cleanupClusterAccess(ctx, cluster); cleanupErr != nil {
			slog.Error("Failed to cleanup cluster", "cluster", cluster.Name, "error", cleanupErr)
		}
	}

	return nil
}

func (cs *CleanupService) cleanupClusterAccess(ctx context.Context, cluster *models.Cluster) error {
	// Get all access entries for this cluster
	entries, err := cs.accessManager.ListActiveAccess(ctx, cluster.Name)
	if err != nil {
		return fmt.Errorf("failed to list active access for cluster %s: %w", cluster.Name, err)
	}

	for _, entryArn := range entries {
		// Extract session information from the ARN
		sessionInfo := extractSessionInfo(entryArn)
		if sessionInfo == nil {
			continue // Skip non-JIT entries
		}

		// Check if we have a corresponding access record
		access, err := cs.findAccessBySession(sessionInfo)
		if err != nil {
			slog.Warn("No access record found for session", "session", sessionInfo.SessionName, "error", err)
			// Cleanup orphaned entries
			if err := cs.accessManager.eksService.DeleteAccessEntry(ctx, cluster.Name, entryArn); err != nil {
				slog.Error("Failed to delete orphaned access entry", "entry_arn", entryArn, "error", err)
			}
			continue
		}

		// Check if access has expired
		if cs.isAccessExpired(access) {
			slog.Info("Cleaning up expired access", "user", access.UserID, "cluster", cluster.Name)

			if err := cs.revokeExpiredAccess(ctx, access, cluster, entryArn); err != nil {
				slog.Error("Failed to revoke expired access", "error", err)
			}
		}
	}

	return nil
}

func (cs *CleanupService) isAccessExpired(access *models.ClusterAccess) bool {
	if access.ExpiresAt == nil {
		// If no expiration time set, calculate from requested time + duration
		expirationTime := access.RequestedAt.Add(access.Duration)
		return time.Now().After(expirationTime)
	}
	return time.Now().After(*access.ExpiresAt)
}

func (cs *CleanupService) revokeExpiredAccess(ctx context.Context, access *models.ClusterAccess, cluster *models.Cluster, entryArn string) error {
	// Delete the EKS access entry
	if err := cs.accessManager.eksService.DeleteAccessEntry(ctx, cluster.Name, entryArn); err != nil {
		return fmt.Errorf("failed to delete access entry: %w", err)
	}

	// Update access status in store
	access.Status = models.AccessStatusExpired
	expiredAt := time.Now()
	access.RevokedAt = &expiredAt
	access.RevokeReason = "Automatic expiration"

	// Note: In a real implementation, you'd want to update the store here
	// For the memory store, we might need to add an UpdateAccess method

	slog.Info("Successfully revoked expired access", "user", access.UserID)
	return nil
}

func (cs *CleanupService) findAccessBySession(sessionInfo *SessionInfo) (*models.ClusterAccess, error) {
	// This is a simplified implementation
	// In practice, you'd want to maintain a mapping between session names and access IDs
	// or store the session name in the access record

	// For now, we'll try to extract the user ID from the session name
	// Session names are in format: jit-{userID}-{clusterID}-{timestamp}
	if sessionInfo.UserID == "" {
		return nil, fmt.Errorf("could not extract user ID from session")
	}

	// This would need a more sophisticated lookup in a real implementation
	return nil, fmt.Errorf("access lookup not fully implemented")
}

type SessionInfo struct {
	SessionName string
	UserID      string
	ClusterID   string
	Timestamp   string
}

func extractSessionInfo(entryArn string) *SessionInfo {
	// Extract session information from ARN like:
	// arn:aws:sts::123456789012:assumed-role/JITAccessRole/jit-user123-cluster456-20240610-143022

	if !strings.Contains(entryArn, "assumed-role") {
		return nil
	}

	parts := strings.Split(entryArn, "/")
	if len(parts) < 3 {
		return nil
	}

	sessionName := parts[len(parts)-1]
	if !strings.HasPrefix(sessionName, "jit-") {
		return nil
	}

	// Parse session name: jit-{userID}-{clusterID}-{timestamp}
	sessionParts := strings.Split(sessionName, "-")
	if len(sessionParts) < 4 {
		return nil
	}

	return &SessionInfo{
		SessionName: sessionName,
		UserID:      sessionParts[1],
		ClusterID:   sessionParts[2],
		Timestamp:   strings.Join(sessionParts[3:], "-"),
	}
}

// ForceCleanupCluster removes all JIT access entries for a cluster
func (cs *CleanupService) ForceCleanupCluster(ctx context.Context, clusterName string) error {
	entries, err := cs.accessManager.ListActiveAccess(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list active access: %w", err)
	}

	for _, entryArn := range entries {
		if err := cs.accessManager.eksService.DeleteAccessEntry(ctx, clusterName, entryArn); err != nil {
			slog.Error("Failed to delete access entry", "entry_arn", entryArn, "error", err)
		} else {
			slog.Info("Deleted access entry", "entry_arn", entryArn)
		}
	}

	return nil
}

// CleanupUserAccess removes all access entries for a specific user
func (cs *CleanupService) CleanupUserAccess(ctx context.Context, userID string) error {
	clusters, err := cs.store.ListClusters()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range clusters {
		entries, err := cs.accessManager.ListActiveAccess(ctx, cluster.Name)
		if err != nil {
			slog.Error("Failed to list access for cluster", "cluster", cluster.Name, "error", err)
			continue
		}

		for _, entryArn := range entries {
			sessionInfo := extractSessionInfo(entryArn)
			if sessionInfo != nil && sessionInfo.UserID == userID {
				if err := cs.accessManager.eksService.DeleteAccessEntry(ctx, cluster.Name, entryArn); err != nil {
					slog.Error("Failed to delete user access entry", "entry_arn", entryArn, "error", err)
				} else {
					slog.Info("Deleted user access entry", "user", userID, "entry_arn", entryArn)
				}
			}
		}
	}

	return nil
}
