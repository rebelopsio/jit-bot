package webhook

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		clusterName string
		want        string
	}{
		{
			name:        "production cluster with prod",
			clusterName: "prod-east-1",
			want:        "production",
		},
		{
			name:        "production cluster with production",
			clusterName: "production-west-2",
			want:        "production",
		},
		{
			name:        "staging cluster with stag",
			clusterName: "stag-east-1",
			want:        "staging",
		},
		{
			name:        "staging cluster with staging",
			clusterName: "staging-west-2",
			want:        "staging",
		},
		{
			name:        "development cluster with dev",
			clusterName: "dev-east-1",
			want:        "development",
		},
		{
			name:        "development cluster with development",
			clusterName: "development-west-2",
			want:        "development",
		},
		{
			name:        "unknown cluster pattern",
			clusterName: "unknown-cluster-name",
			want:        "production", // Default to production for safety
		},
		{
			name:        "cluster with mixed patterns",
			clusterName: "test-prod-staging",
			want:        "production", // First match wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineEnvironment(tt.clusterName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasElevatedPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{
			name:        "has cluster-admin",
			permissions: []string{"view", "cluster-admin"},
			want:        true,
		},
		{
			name:        "has admin",
			permissions: []string{"edit", "admin"},
			want:        true,
		},
		{
			name:        "has debug",
			permissions: []string{"view", "debug"},
			want:        true,
		},
		{
			name:        "has exec",
			permissions: []string{"logs", "exec"},
			want:        true,
		},
		{
			name:        "standard permissions only",
			permissions: []string{"view", "edit"},
			want:        true, // edit is considered elevated
		},
		{
			name:        "logs and port-forward only",
			permissions: []string{"logs", "port-forward"},
			want:        true, // port-forward is considered elevated
		},
		{
			name:        "empty permissions",
			permissions: []string{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasElevatedPermissions(tt.permissions)
			assert.Equal(t, tt.want, got)
		})
	}
}