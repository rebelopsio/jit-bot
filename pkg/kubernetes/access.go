package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/rebelopsio/jit-bot/pkg/aws"
	"github.com/rebelopsio/jit-bot/pkg/models"
)

type AccessManager struct {
	stsService *aws.STSService
	eksService *aws.EKSService
	region     string
}

type GrantAccessRequest struct {
	ClusterAccess *models.ClusterAccess
	Cluster       *models.Cluster
	UserEmail     string
	Permissions   []string
	Namespaces    []string
	JITRoleArn    string
	AssumeRoleArn string
}

type AccessCredentials struct {
	TemporaryCredentials *aws.Credentials
	KubeConfig           string
	ClusterEndpoint      string
	ExpiresAt            time.Time
}

func NewAccessManager(region string) (*AccessManager, error) {
	stsService, err := aws.NewSTSService(region)
	if err != nil {
		return nil, fmt.Errorf("failed to create STS service: %w", err)
	}

	eksService, err := aws.NewEKSService(region)
	if err != nil {
		return nil, fmt.Errorf("failed to create EKS service: %w", err)
	}

	return &AccessManager{
		stsService: stsService,
		eksService: eksService,
		region:     region,
	}, nil
}

func (am *AccessManager) GrantAccess(ctx context.Context, req GrantAccessRequest) (*AccessCredentials, error) {
	// Step 1: Create temporary IAM role session
	sessionName := aws.GenerateJITSessionName(req.ClusterAccess.UserID, req.Cluster.ID)
	policy := aws.CreateJITPolicy(req.Cluster.Name, "", req.Permissions)

	// Assume the JIT role with limited permissions
	creds, err := am.stsService.AssumeRole(ctx, aws.AssumeRoleInput{
		RoleArn:         req.JITRoleArn,
		SessionName:     sessionName,
		DurationSeconds: int32(req.ClusterAccess.Duration.Seconds()),
		Policy:          policy,
		Tags: []ststypes.Tag{
			{Key: awssdk.String("Purpose"), Value: awssdk.String("JITAccess")},
			{Key: awssdk.String("UserID"), Value: awssdk.String(req.ClusterAccess.UserID)},
			{Key: awssdk.String("ClusterID"), Value: awssdk.String(req.Cluster.ID)},
			{Key: awssdk.String("RequestID"), Value: awssdk.String(req.ClusterAccess.ID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assume JIT role: %w", err)
	}

	// Step 2: Create EKS access entry
	principalArn := fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/%s",
		req.Cluster.AWSAccount,
		extractRoleName(req.JITRoleArn),
		sessionName)

	username := fmt.Sprintf("jit:%s", req.ClusterAccess.UserID)

	err = am.eksService.CreateJITAccessEntry(ctx,
		req.Cluster.Name,
		principalArn,
		username,
		req.Permissions,
		req.Namespaces)
	if err != nil {
		return nil, fmt.Errorf("failed to create EKS access entry: %w", err)
	}

	// Step 3: Get cluster details for kubeconfig
	cluster, err := am.eksService.DescribeCluster(ctx, req.Cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	// Step 4: Generate kubeconfig
	kubeConfig := am.generateKubeConfig(cluster, creds, req.Cluster.Region)

	return &AccessCredentials{
		TemporaryCredentials: creds,
		KubeConfig:           kubeConfig,
		ClusterEndpoint:      awssdk.ToString(cluster.Endpoint),
		ExpiresAt:            creds.Expiration,
	}, nil
}

func (am *AccessManager) RevokeAccess(ctx context.Context, clusterAccess *models.ClusterAccess, cluster *models.Cluster, jitRoleArn string) error {
	// Calculate the principal ARN that was created during access grant
	sessionName := aws.GenerateJITSessionName(clusterAccess.UserID, cluster.ID)
	principalArn := fmt.Sprintf("arn:aws:sts::%s:assumed-role/%s/%s",
		cluster.AWSAccount,
		extractRoleName(jitRoleArn),
		sessionName)

	// Remove EKS access entry
	err := am.eksService.DeleteAccessEntry(ctx, cluster.Name, principalArn)
	if err != nil {
		return fmt.Errorf("failed to delete EKS access entry: %w", err)
	}

	return nil
}

func (am *AccessManager) ListActiveAccess(ctx context.Context, clusterName string) ([]string, error) {
	entries, err := am.eksService.ListAccessEntries(ctx, clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to list access entries: %w", err)
	}

	// Filter for JIT access entries
	var jitEntries []string
	for _, entryArn := range entries {
		// Check if this is a JIT-created entry (contains assumed-role and jit session name)
		if isJITEntry(entryArn) {
			jitEntries = append(jitEntries, entryArn)
		}
	}

	return jitEntries, nil
}

func (am *AccessManager) CleanupExpiredAccess(ctx context.Context, clusterName string) error {
	entries, err := am.ListActiveAccess(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("failed to list active access: %w", err)
	}

	for _, entryArn := range entries {
		// Check if entry is expired (this would require storing expiration metadata)
		// For now, we'll rely on IAM session expiration
		entry, err := am.eksService.DescribeAccessEntry(ctx, clusterName, entryArn)
		if err != nil {
			continue // Skip if we can't describe it
		}

		// Check if entry is tagged as temporary and old enough
		if entry.Tags["Temporary"] == "true" {
			// You could implement logic here to check age and clean up
			// For now, we'll leave this as a manual process
		}
	}

	return nil
}

func (am *AccessManager) generateKubeConfig(cluster *ekstypes.Cluster, creds *aws.Credentials, region string) string {
	clusterName := awssdk.ToString(cluster.Name)
	endpoint := awssdk.ToString(cluster.Endpoint)
	ca := awssdk.ToString(cluster.CertificateAuthority.Data)

	kubeConfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - %s
        - --region
        - %s
      env:
        - name: AWS_ACCESS_KEY_ID
          value: %s
        - name: AWS_SECRET_ACCESS_KEY
          value: %s
        - name: AWS_SESSION_TOKEN
          value: %s
`, ca, endpoint, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, region, creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken)

	return kubeConfig
}

// Helper functions
func extractRoleName(roleArn string) string {
	// Extract role name from ARN: arn:aws:iam::123456789012:role/RoleName
	// This is a simplified implementation
	parts := strings.Split(roleArn, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "unknown"
}

func isJITEntry(entryArn string) bool {
	// Check if the ARN contains indicators of JIT access
	return strings.Contains(entryArn, "assumed-role") && strings.Contains(entryArn, "jit-")
}
