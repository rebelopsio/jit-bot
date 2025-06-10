package models

import (
	"time"
)

type Cluster struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	AWSAccount  string            `json:"aws_account"`
	Region      string            `json:"region"`
	Environment string            `json:"environment"`
	Tags        map[string]string `json:"tags"`
	MaxDuration time.Duration     `json:"max_duration"`
	RequiredApprovers int          `json:"required_approvers"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CreatedBy   string            `json:"created_by"`
}

type ClusterAccess struct {
	ID            string        `json:"id"`
	ClusterID     string        `json:"cluster_id"`
	UserID        string        `json:"user_id"`
	UserEmail     string        `json:"user_email"`
	Reason        string        `json:"reason"`
	Duration      time.Duration `json:"duration"`
	Status        AccessStatus  `json:"status"`
	ApprovedBy    []string      `json:"approved_by"`
	RequestedAt   time.Time     `json:"requested_at"`
	ApprovedAt    *time.Time    `json:"approved_at,omitempty"`
	ExpiresAt     *time.Time    `json:"expires_at,omitempty"`
	RevokedAt     *time.Time    `json:"revoked_at,omitempty"`
	RevokedBy     string        `json:"revoked_by,omitempty"`
	RevokeReason  string        `json:"revoke_reason,omitempty"`
}

type AccessStatus string

const (
	AccessStatusPending  AccessStatus = "pending"
	AccessStatusApproved AccessStatus = "approved"
	AccessStatusDenied   AccessStatus = "denied"
	AccessStatusActive   AccessStatus = "active"
	AccessStatusExpired  AccessStatus = "expired"
	AccessStatusRevoked  AccessStatus = "revoked"
)

type AccessRequest struct {
	ClusterID string        `json:"cluster_id"`
	Reason    string        `json:"reason"`
	Duration  time.Duration `json:"duration"`
}