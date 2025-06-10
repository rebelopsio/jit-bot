package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

type STSService struct {
	client *sts.Client
	region string
}

type AssumeRoleInput struct {
	RoleArn         string
	SessionName     string
	DurationSeconds int32
	ExternalId      string
	Policy          string
	Tags            []types.Tag
}

type Credentials struct {
	AccessKeyId     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

func NewSTSService(region string) (*STSService, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &STSService{
		client: sts.NewFromConfig(cfg),
		region: region,
	}, nil
}

func (s *STSService) AssumeRole(ctx context.Context, input AssumeRoleInput) (*Credentials, error) {
	assumeRoleInput := &sts.AssumeRoleInput{
		RoleArn:         aws.String(input.RoleArn),
		RoleSessionName: aws.String(input.SessionName),
		DurationSeconds: aws.Int32(input.DurationSeconds),
	}

	if input.ExternalId != "" {
		assumeRoleInput.ExternalId = aws.String(input.ExternalId)
	}

	if input.Policy != "" {
		assumeRoleInput.Policy = aws.String(input.Policy)
	}

	if len(input.Tags) > 0 {
		assumeRoleInput.Tags = input.Tags
	}

	result, err := s.client.AssumeRole(ctx, assumeRoleInput)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %w", err)
	}

	return &Credentials{
		AccessKeyId:     *result.Credentials.AccessKeyId,
		SecretAccessKey: *result.Credentials.SecretAccessKey,
		SessionToken:    *result.Credentials.SessionToken,
		Expiration:      *result.Credentials.Expiration,
	}, nil
}

func (s *STSService) AssumeRoleWithWebIdentity(ctx context.Context, roleArn, webIdentityToken, sessionName string, durationSeconds int32) (*Credentials, error) {
	input := &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleArn),
		RoleSessionName:  aws.String(sessionName),
		WebIdentityToken: aws.String(webIdentityToken),
		DurationSeconds:  aws.Int32(durationSeconds),
	}

	result, err := s.client.AssumeRoleWithWebIdentity(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to assume role with web identity: %w", err)
	}

	return &Credentials{
		AccessKeyId:     *result.Credentials.AccessKeyId,
		SecretAccessKey: *result.Credentials.SecretAccessKey,
		SessionToken:    *result.Credentials.SessionToken,
		Expiration:      *result.Credentials.Expiration,
	}, nil
}

func (s *STSService) GetCallerIdentity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	return s.client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
}

// GenerateJITSessionName creates a unique session name for JIT access
func GenerateJITSessionName(userID, clusterID string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("jit-%s-%s-%s", userID, clusterID, timestamp)
}

// CreateJITPolicy generates an IAM policy for limited EKS access
func CreateJITPolicy(clusterName, namespace string, permissions []string) string {
	policy := `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "eks:AccessKubernetesApi"
      ],
      "Resource": "arn:aws:eks:*:*:cluster/%s"
    }
  ]
}`
	return fmt.Sprintf(policy, clusterName)
}