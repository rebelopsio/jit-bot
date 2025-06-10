# AWS Setup Guide

This guide covers the AWS infrastructure setup required for the JIT Access Tool.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    AWS Organizations                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ Management      │  │ Production      │  │ Development     │  │
│  │ Account         │  │ Account         │  │ Account         │  │
│  │                 │  │                 │  │                 │  │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │
│  │ │ JIT Operator│ │  │ │ EKS Cluster │ │  │ │ EKS Cluster │ │  │
│  │ │ & Server    │ │  │ │             │ │  │ │             │ │  │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │  │
│  │                 │  │                 │  │                 │  │
│  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │ ┌─────────────┐ │  │
│  │ │SAML Provider│ │  │ │ JIT Access  │ │  │ │ JIT Access  │ │  │
│  │ │(Google)     │ │  │ │ Role        │ │  │ │ Role        │ │  │
│  │ └─────────────┘ │  │ └─────────────┘ │  │ └─────────────┘ │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- AWS Organizations setup with multiple accounts
- Administrative access to all AWS accounts
- Google Workspace (for SAML integration)
- AWS CLI configured with appropriate credentials

## 1. AWS Organizations Setup

### 1.1 Verify Organizations Structure

```bash
# List all accounts in your organization
aws organizations list-accounts

# Get organization details
aws organizations describe-organization
```

### 1.2 Enable Required Services

Ensure the following services are enabled across all accounts:
- AWS IAM Identity Center (formerly AWS SSO)
- AWS CloudTrail
- AWS EKS
- AWS STS

## 2. SAML Identity Provider Setup

### 2.1 Google Workspace Configuration

1. **In Google Admin Console**:
   - Navigate to Apps > Web and mobile apps
   - Add a custom SAML app
   - Configure the ACS URL: `https://signin.aws.amazon.com/saml`
   - Set the Entity ID: `urn:amazon:webservices`

2. **Download SAML Metadata**:
   - Download the metadata XML file
   - Save it for AWS configuration

### 2.2 AWS SAML Provider Configuration

For each AWS account, create a SAML identity provider:

```bash
# Create SAML provider in management account
aws iam create-saml-provider \
  --name GoogleSAML \
  --saml-metadata-document file://google-saml-metadata.xml

# Note the ARN returned - you'll need this for role configuration
# Example: arn:aws:iam::123456789012:saml-provider/GoogleSAML
```

## 3. Cross-Account IAM Roles

### 3.1 Management Account - Operator Role

Create a role for the JIT operator to assume roles in other accounts:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "eks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

**Policy for Operator Role**:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sts:AssumeRole"
      ],
      "Resource": "arn:aws:iam::*:role/JITCrossAccountRole"
    },
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters"
      ],
      "Resource": "*"
    }
  ]
}
```

### 3.2 Target Accounts - Cross-Account Access Role

Create this role in each target AWS account:

**Trust Policy** (`JITCrossAccountRole-trust-policy.json`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::MANAGEMENT-ACCOUNT-ID:role/JITOperatorRole"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

**Permissions Policy** (`JITCrossAccountRole-permissions.json`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:CreateAccessEntry",
        "eks:DeleteAccessEntry",
        "eks:DescribeAccessEntry",
        "eks:ListAccessEntries",
        "eks:UpdateAccessEntry",
        "eks:AssociateAccessPolicy",
        "eks:DisassociateAccessPolicy",
        "eks:ListAssociatedAccessPolicies"
      ],
      "Resource": "arn:aws:eks:*:*:cluster/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "eks:DescribeCluster",
        "eks:ListClusters"
      ],
      "Resource": "*"
    }
  ]
}
```

**Create the role**:

```bash
# Create the role
aws iam create-role \
  --role-name JITCrossAccountRole \
  --assume-role-policy-document file://JITCrossAccountRole-trust-policy.json

# Attach the permissions policy
aws iam put-role-policy \
  --role-name JITCrossAccountRole \
  --policy-name JITCrossAccountPermissions \
  --policy-document file://JITCrossAccountRole-permissions.json
```

### 3.3 JIT Access Roles (Per Account)

Create roles that users will temporarily assume:

**Trust Policy** (`JITAccessRole-trust-policy.json`):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT-ID:saml-provider/GoogleSAML"
      },
      "Action": "sts:AssumeRoleWithSAML",
      "Condition": {
        "StringEquals": {
          "SAML:aud": "https://signin.aws.amazon.com/saml"
        }
      }
    }
  ]
}
```

**Create JIT Access Roles with different permission levels**:

```bash
# Create read-only role
aws iam create-role \
  --role-name JITAccessRole-ReadOnly \
  --assume-role-policy-document file://JITAccessRole-trust-policy.json

aws iam attach-role-policy \
  --role-name JITAccessRole-ReadOnly \
  --policy-arn arn:aws:iam::aws:policy/ReadOnlyAccess

# Create developer role
aws iam create-role \
  --role-name JITAccessRole-Developer \
  --assume-role-policy-document file://JITAccessRole-trust-policy.json

aws iam attach-role-policy \
  --role-name JITAccessRole-Developer \
  --policy-arn arn:aws:iam::aws:policy/PowerUserAccess

# Create admin role (use sparingly)
aws iam create-role \
  --role-name JITAccessRole-Admin \
  --assume-role-policy-document file://JITAccessRole-trust-policy.json

aws iam attach-role-policy \
  --role-name JITAccessRole-Admin \
  --policy-arn arn:aws:iam::aws:policy/AdministratorAccess
```

## 4. EKS Cluster Configuration

### 4.1 Enable Access Entries

For each EKS cluster, ensure access entries are enabled:

```bash
# Check current cluster configuration
aws eks describe-cluster --name your-cluster-name

# Update cluster to use access entries (if needed)
aws eks update-cluster-config \
  --name your-cluster-name \
  --access-config authenticationMode=API_AND_CONFIG_MAP
```

### 4.2 Configure OIDC Provider (if not already done)

```bash
# Get cluster OIDC issuer URL
CLUSTER_NAME="your-cluster-name"
OIDC_ISSUER=$(aws eks describe-cluster --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text)

# Create OIDC identity provider
aws iam create-open-id-connect-provider \
  --url $OIDC_ISSUER \
  --thumbprint-list 9e99a48a9960b14926bb7f3b02e22da2b0ab7280 \
  --client-id-list sts.amazonaws.com
```

### 4.3 Create Kubernetes RBAC Policies

Apply these policies to each EKS cluster:

**View Access Policy** (`rbac-view.yaml`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jit-view-access
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps", "extensions"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: jit-view-access-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: jit-view-access
subjects:
- kind: User
  name: jit-view-user
  apiGroup: rbac.authorization.k8s.io
```

**Edit Access Policy** (`rbac-edit.yaml`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: jit-edit-access
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["apps", "extensions"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["batch"]
  resources: ["*"]
  verbs: ["*"]
- apiGroups: ["networking.k8s.io"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: jit-edit-access-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: jit-edit-access
subjects:
- kind: User
  name: jit-edit-user
  apiGroup: rbac.authorization.k8s.io
```

**Apply to clusters**:

```bash
# Apply to each cluster
kubectl apply -f rbac-view.yaml
kubectl apply -f rbac-edit.yaml
```

## 5. CloudTrail Configuration

### 5.1 Enable CloudTrail for Audit

Create a CloudTrail for auditing JIT access:

```bash
# Create CloudTrail for JIT access auditing
aws cloudtrail create-trail \
  --name JITAccessAuditTrail \
  --s3-bucket-name your-audit-bucket \
  --include-global-service-events \
  --is-multi-region-trail \
  --enable-log-file-validation

# Start logging
aws cloudtrail start-logging --name JITAccessAuditTrail
```

### 5.2 CloudWatch Log Group (Optional)

```bash
# Create log group for CloudTrail
aws logs create-log-group --log-group-name /aws/cloudtrail/jit-access

# Update CloudTrail to send to CloudWatch
aws cloudtrail put-event-selectors \
  --trail-name JITAccessAuditTrail \
  --event-selectors ReadWriteType=All,IncludeManagementEvents=true
```

## 6. Testing and Validation

### 6.1 Test Cross-Account Access

```bash
# Test assuming cross-account role
aws sts assume-role \
  --role-arn arn:aws:iam::TARGET-ACCOUNT:role/JITCrossAccountRole \
  --role-session-name test-session

# Use the temporary credentials to test EKS access
export AWS_ACCESS_KEY_ID=<from-assume-role>
export AWS_SECRET_ACCESS_KEY=<from-assume-role>
export AWS_SESSION_TOKEN=<from-assume-role>

aws eks describe-cluster --name your-cluster-name
```

### 6.2 Test SAML Authentication

```bash
# Test SAML authentication (this would normally be done through a browser)
aws sts assume-role-with-saml \
  --role-arn arn:aws:iam::ACCOUNT-ID:role/JITAccessRole-ReadOnly \
  --principal-arn arn:aws:iam::ACCOUNT-ID:saml-provider/GoogleSAML \
  --saml-assertion <base64-encoded-saml-assertion>
```

### 6.3 Test EKS Access Entries

```bash
# Create a test access entry
aws eks create-access-entry \
  --cluster-name your-cluster-name \
  --principal-arn arn:aws:iam::ACCOUNT-ID:role/JITAccessRole-ReadOnly \
  --username jit-test-user

# Associate access policy
aws eks associate-access-policy \
  --cluster-name your-cluster-name \
  --principal-arn arn:aws:iam::ACCOUNT-ID:role/JITAccessRole-ReadOnly \
  --policy-arn arn:aws:eks::aws:cluster-access-policy/AmazonEKSViewPolicy \
  --access-scope type=cluster

# Clean up test entry
aws eks delete-access-entry \
  --cluster-name your-cluster-name \
  --principal-arn arn:aws:iam::ACCOUNT-ID:role/JITAccessRole-ReadOnly
```

## 7. Security Best Practices

### 7.1 Least Privilege Access

- Create specific roles for different access levels (read-only, developer, admin)
- Use namespace-scoped access where possible
- Regularly review and rotate credentials

### 7.2 Monitoring and Alerting

- Set up CloudWatch alarms for unusual STS activity
- Monitor EKS access entry creation/deletion
- Alert on failed authentication attempts

### 7.3 Compliance

- Ensure CloudTrail logging is enabled and monitored
- Implement data retention policies
- Regular access reviews and audits

## 8. Troubleshooting

### Common Issues

1. **Cross-account assume role failures**:
   ```bash
   # Check trust relationships
   aws iam get-role --role-name JITCrossAccountRole
   ```

2. **EKS access entry failures**:
   ```bash
   # Check cluster authentication mode
   aws eks describe-cluster --name cluster-name --query 'cluster.accessConfig'
   ```

3. **SAML authentication issues**:
   ```bash
   # Validate SAML provider
   aws iam get-saml-provider --saml-provider-arn arn:aws:iam::account:saml-provider/GoogleSAML
   ```

### Debug Commands

```bash
# Test role assumption
aws sts get-caller-identity

# Check available clusters
aws eks list-clusters

# Test EKS access
aws eks describe-cluster --name cluster-name

# Check access entries
aws eks list-access-entries --cluster-name cluster-name
```

## Environment-Specific Configurations

### Development Environment

- Longer access durations (up to 12 hours)
- Auto-approval for view/edit permissions
- Relaxed monitoring

### Staging Environment  

- Medium access durations (up to 8 hours)
- Auto-approval for view, manual approval for edit
- Standard monitoring

### Production Environment

- Short access durations (up to 4 hours)
- Manual approval required for all access
- Enhanced monitoring and alerting