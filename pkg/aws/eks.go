package aws

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

type EKSService struct {
	client *eks.Client
	region string
}

type AccessEntry struct {
	ClusterName    string
	PrincipalArn   string
	Username       string
	Groups         []string
	AccessPolicies []AccessPolicy
	CreatedAt      time.Time
	ModifiedAt     time.Time
	Tags           map[string]string
}

type AccessPolicy struct {
	PolicyArn   string
	AccessScope AccessScope
}

type AccessScope struct {
	Type       string
	Namespaces []string
}

func NewEKSService(region string) (*EKSService, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &EKSService{
		client: eks.NewFromConfig(cfg),
		region: region,
	}, nil
}

func (e *EKSService) CreateAccessEntry(ctx context.Context, entry AccessEntry) error {
	input := &eks.CreateAccessEntryInput{
		ClusterName:  aws.String(entry.ClusterName),
		PrincipalArn: aws.String(entry.PrincipalArn),
		Username:     aws.String(entry.Username),
		Tags:         entry.Tags,
	}

	if len(entry.Groups) > 0 {
		input.KubernetesGroups = entry.Groups
	}

	_, err := e.client.CreateAccessEntry(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create access entry: %w", err)
	}

	// Associate access policies if provided
	for _, policy := range entry.AccessPolicies {
		err = e.AssociateAccessPolicy(ctx, entry.ClusterName, entry.PrincipalArn, policy)
		if err != nil {
			// Log warning but don't fail the entire operation
			slog.Warn("Failed to associate policy", "policy_arn", policy.PolicyArn, "error", err)
		}
	}

	return nil
}

func (e *EKSService) AssociateAccessPolicy(ctx context.Context, clusterName, principalArn string, policy AccessPolicy) error {
	input := &eks.AssociateAccessPolicyInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(principalArn),
		PolicyArn:    aws.String(policy.PolicyArn),
		AccessScope: &types.AccessScope{
			Type: types.AccessScopeType(policy.AccessScope.Type),
		},
	}

	if len(policy.AccessScope.Namespaces) > 0 {
		input.AccessScope.Namespaces = policy.AccessScope.Namespaces
	}

	_, err := e.client.AssociateAccessPolicy(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to associate access policy: %w", err)
	}

	return nil
}

func (e *EKSService) DeleteAccessEntry(ctx context.Context, clusterName, principalArn string) error {
	input := &eks.DeleteAccessEntryInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(principalArn),
	}

	_, err := e.client.DeleteAccessEntry(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete access entry: %w", err)
	}

	return nil
}

func (e *EKSService) DescribeAccessEntry(ctx context.Context, clusterName, principalArn string) (*AccessEntry, error) {
	input := &eks.DescribeAccessEntryInput{
		ClusterName:  aws.String(clusterName),
		PrincipalArn: aws.String(principalArn),
	}

	result, err := e.client.DescribeAccessEntry(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe access entry: %w", err)
	}

	entry := &AccessEntry{
		ClusterName:  clusterName,
		PrincipalArn: principalArn,
		Username:     aws.ToString(result.AccessEntry.Username),
		Groups:       result.AccessEntry.KubernetesGroups,
		CreatedAt:    aws.ToTime(result.AccessEntry.CreatedAt),
		ModifiedAt:   aws.ToTime(result.AccessEntry.ModifiedAt),
		Tags:         result.AccessEntry.Tags,
	}

	return entry, nil
}

func (e *EKSService) ListAccessEntries(ctx context.Context, clusterName string) ([]string, error) {
	input := &eks.ListAccessEntriesInput{
		ClusterName: aws.String(clusterName),
	}

	var allEntries []string
	paginator := eks.NewListAccessEntriesPaginator(e.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list access entries: %w", err)
		}
		allEntries = append(allEntries, page.AccessEntries...)
	}

	return allEntries, nil
}

func (e *EKSService) DescribeCluster(ctx context.Context, clusterName string) (*types.Cluster, error) {
	input := &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	}

	result, err := e.client.DescribeCluster(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	return result.Cluster, nil
}

// Common EKS access policies
const (
	EKSViewerPolicy    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSViewPolicy"
	EKSEditorPolicy    = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSEditPolicy"
	EKSAdminPolicy     = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSClusterAdminPolicy"
	EKSNamespacePolicy = "arn:aws:eks::aws:cluster-access-policy/AmazonEKSAdminViewPolicy"
)

// CreateJITAccessEntry creates a temporary access entry for JIT access
func (e *EKSService) CreateJITAccessEntry(ctx context.Context, clusterName, principalArn, username string, permissions []string, namespaces []string) error {
	// Determine appropriate policies based on permissions
	var accessPolicies []AccessPolicy

	for _, permission := range permissions {
		switch permission {
		case "view":
			accessPolicies = append(accessPolicies, AccessPolicy{
				PolicyArn: EKSViewerPolicy,
				AccessScope: AccessScope{
					Type:       "namespace",
					Namespaces: namespaces,
				},
			})
		case "edit":
			accessPolicies = append(accessPolicies, AccessPolicy{
				PolicyArn: EKSEditorPolicy,
				AccessScope: AccessScope{
					Type:       "namespace",
					Namespaces: namespaces,
				},
			})
		case "admin":
			accessPolicies = append(accessPolicies, AccessPolicy{
				PolicyArn: EKSAdminPolicy,
				AccessScope: AccessScope{
					Type: "cluster",
				},
			})
		}
	}

	entry := AccessEntry{
		ClusterName:    clusterName,
		PrincipalArn:   principalArn,
		Username:       username,
		AccessPolicies: accessPolicies,
		Tags: map[string]string{
			"Purpose":   "JITAccess",
			"CreatedBy": "jit-server",
			"Temporary": "true",
		},
	}

	return e.CreateAccessEntry(ctx, entry)
}
