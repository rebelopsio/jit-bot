# API Reference

This document provides a comprehensive reference for the JIT Access Tool APIs, including Kubernetes Custom Resources, REST endpoints, and Slack command interface.

## Table of Contents

1. [Kubernetes CRDs](#kubernetes-crds)
2. [REST API Endpoints](#rest-api-endpoints)
3. [Slack Command Interface](#slack-command-interface)
4. [Webhook Events](#webhook-events)

## Kubernetes CRDs

### JITAccessRequest

The `JITAccessRequest` resource represents a request for just-in-time access to an EKS cluster.

#### API Version

```yaml
apiVersion: jit.rebelops.io/v1
kind: JITAccessRequest
```

#### Spec Fields

| Field | Type | Required | Validation | Description |
|-------|------|----------|------------|-------------|
| `userID` | string | Yes | Pattern: `^U[A-Z0-9]{10}$` | Slack user ID of the requester |
| `userEmail` | string | Yes | Pattern: valid email format | Email address of the requesting user |
| `targetCluster` | [TargetCluster](#targetcluster) | Yes | See TargetCluster validation | EKS cluster to access |
| `reason` | string | Yes | Length: 10-500 chars, meaningful content | Business justification for access |
| `duration` | string | Yes | Pattern: `^(\d+[dhms])+$`, Range: 15m-7d | Requested access duration (e.g., "2h", "30m") |
| `permissions` | []string | Yes | Enum: view,edit,admin,cluster-admin,debug,logs,exec,port-forward | Requested permission levels |
| `namespaces` | []string | No | Pattern: valid k8s namespace names | Target Kubernetes namespaces (empty = cluster-wide) |
| `approvers` | []string | No | Auto-assigned if empty | Required approvers for this request |
| `slackChannel` | string | No | Pattern: `^C[A-Z0-9]{10}$` | Slack channel where request was made |
| `requestedAt` | metav1.Time | Yes | Auto-set by webhook | When the request was created |

#### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | [AccessPhase](#accessphase) | Current phase of the access request |
| `approvals` | [][Approval](#approval) | List of approvals received |
| `accessEntry` | [AccessEntryStatus](#accessentrystatus) | Details of granted access |
| `conditions` | []metav1.Condition | Detailed status conditions |
| `message` | string | Human-readable status message |

#### Example

```yaml
apiVersion: jit.rebelops.io/v1
kind: JITAccessRequest
metadata:
  name: jit-user123-1640995200
  namespace: jit-system
  labels:
    jit.rebelops.io/user: "U123USER"
    jit.rebelops.io/cluster: "prod-east-1"
spec:
  userID: "U123USER"
  userEmail: "john.doe@company.com"
  targetCluster:
    name: "prod-east-1"
    awsAccount: "123456789012"
    region: "us-east-1"
    endpoint: "https://ABC123.gr7.us-east-1.eks.amazonaws.com"
  reason: "Deploy critical hotfix for payment processing"
  duration: "2h"
  permissions: ["edit"]
  namespaces: ["payment-service", "monitoring"]
  approvers: ["platform-team", "sre-team"]
  slackChannel: "C123CHANNEL"
  requestedAt: "2023-12-31T23:00:00Z"
status:
  phase: "Active"
  approvals:
  - approver: "U456APPROVER"
    approvedAt: "2023-12-31T23:05:00Z"
    comment: "Approved for emergency deployment"
  accessEntry:
    principalArn: "arn:aws:sts::123456789012:assumed-role/JITAccessRole/jit-user123-prod-east-1-1640995200"
    sessionName: "jit-user123-prod-east-1"
    createdAt: "2023-12-31T23:06:00Z"
    expiresAt: "2024-01-01T01:06:00Z"
  conditions:
  - type: "Approved"
    status: "True"
    lastTransitionTime: "2023-12-31T23:05:00Z"
    reason: "RequiredApprovalsReceived"
    message: "JIT access request has been approved"
  message: "Access granted and active"
```

### JITAccessJob

The `JITAccessJob` resource manages the lifecycle of granted JIT access.

#### API Version

```yaml
apiVersion: jit.rebelops.io/v1
kind: JITAccessJob
```

#### Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `accessRequestRef` | [ObjectReference](#objectreference) | Yes | Reference to the JITAccessRequest |
| `targetCluster` | [TargetCluster](#targetcluster) | Yes | EKS cluster details |
| `duration` | string | Yes | Access duration |
| `jitRoleArn` | string | Yes | ARN of the JIT role to assume |
| `permissions` | []string | Yes | Permission levels |
| `namespaces` | []string | No | Target namespaces |
| `cleanupPolicy` | [CleanupPolicy](#cleanuppolicy) | No | When to cleanup access |

#### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | [JobPhase](#jobphase) | Current phase of the job |
| `startTime` | *metav1.Time | When the job started |
| `completionTime` | *metav1.Time | When the job completed |
| `expiryTime` | *metav1.Time | When the access expires |
| `accessEntry` | [JobAccessEntry](#jobaccessentry) | Created access entry details |
| `kubeConfigSecretRef` | [ObjectReference](#objectreference) | Reference to kubeconfig secret |
| `conditions` | []metav1.Condition | Detailed status conditions |

#### Example

```yaml
apiVersion: jit.rebelops.io/v1
kind: JITAccessJob
metadata:
  name: jit-user123-jit-user123-1640995200
  namespace: jit-system
  labels:
    jit.rebelops.io/request: "jit-user123-1640995200"
    jit.rebelops.io/user: "U123USER"
    jit.rebelops.io/cluster: "prod-east-1"
spec:
  accessRequestRef:
    name: "jit-user123-1640995200"
    namespace: "jit-system"
  targetCluster:
    name: "prod-east-1"
    awsAccount: "123456789012"
    region: "us-east-1"
  duration: "2h"
  jitRoleArn: "arn:aws:iam::123456789012:role/JITAccessRole"
  permissions: ["edit"]
  namespaces: ["payment-service", "monitoring"]
  cleanupPolicy: "OnExpiry"
status:
  phase: "Active"
  startTime: "2023-12-31T23:06:00Z"
  expiryTime: "2024-01-01T01:06:00Z"
  accessEntry:
    principalArn: "arn:aws:sts::123456789012:assumed-role/JITAccessRole/jit-user123-prod-east-1-1640995200"
    sessionName: "jit-user123-prod-east-1"
    credentialsSecretRef:
      name: "jit-credentials-jit-user123-jit-user123-1640995200"
      namespace: "jit-system"
  kubeConfigSecretRef:
    name: "jit-kubeconfig-jit-user123-jit-user123-1640995200"
    namespace: "jit-system"
  conditions:
  - type: "AccessGranted"
    status: "True"
    lastTransitionTime: "2023-12-31T23:06:00Z"
    reason: "AccessCreated"
    message: "JIT access has been successfully created"
```

### Type Definitions

#### TargetCluster

```yaml
name: string          # EKS cluster name (1-100 chars, required)
awsAccount: string    # AWS account ID (12 digits, pattern: ^\d{12}$)
region: string        # AWS region (pattern: ^[a-z]{2}-[a-z]+-\d{1}$)
endpoint: string      # EKS cluster endpoint (optional, must start with https://)
```

**Validation Rules:**
- `name`: Required, 1-100 characters
- `awsAccount`: Required, exactly 12 digits
- `region`: Required, valid AWS region format (e.g., "us-east-1")
- `endpoint`: Optional, must be valid HTTPS URL if provided

#### AccessPhase

```yaml
enum:
  - "Pending"   # Request pending approval
  - "Approved"  # Request approved, provisioning access
  - "Denied"    # Request denied
  - "Active"    # Access granted and active
  - "Expired"   # Access has expired
  - "Revoked"   # Access manually revoked
```

#### JobPhase

```yaml
enum:
  - "Pending"    # Job created, not started
  - "Creating"   # Creating AWS access
  - "Active"     # Access active
  - "Expiring"   # Access expiring, cleaning up
  - "Completed" # Job completed successfully
  - "Failed"     # Job failed
```

#### Approval

```yaml
approver: string      # User ID who approved
approvedAt: metav1.Time  # When approval was given
comment: string       # Optional approval comment
```

#### AccessEntryStatus

```yaml
principalArn: string  # ARN of the principal granted access
sessionName: string   # STS session name
createdAt: metav1.Time   # When access was granted
expiresAt: metav1.Time   # When access expires
```

#### JobAccessEntry

```yaml
principalArn: string     # ARN of the principal
sessionName: string      # STS session name
credentialsSecretRef:    # Reference to credentials secret
  name: string
  namespace: string
```

#### ObjectReference

```yaml
name: string          # Resource name
namespace: string     # Resource namespace
```

#### CleanupPolicy

```yaml
enum:
  - "OnExpiry"  # Cleanup when access expires
  - "OnDelete"  # Cleanup when job is deleted
  - "Manual"    # Manual cleanup required
```

## Admission Webhook Validation

The system uses Kubernetes admission webhooks to validate and mutate JIT access requests.

### Validating Webhook

The validating webhook enforces business rules that cannot be expressed in OpenAPI schema:

#### Duration Validation
- **Minimum**: 15 minutes
- **Maximum**: 7 days
- **Format**: `(\d+[dhms])+` (e.g., "2h", "30m", "1d", "2h30m")

#### Permission Validation
- **Valid permissions**: `view`, `edit`, `admin`, `cluster-admin`, `debug`, `logs`, `exec`, `port-forward`
- **Escalation rules**: `cluster-admin` cannot be combined with other permissions
- **Minimum**: At least one permission required

#### Reason Validation
- **Length**: 10-500 characters
- **Content**: Must be meaningful (blocks generic terms like "test", "debug", etc.)

#### Business Rules
- Production clusters require approval for elevated permissions
- Namespaces cannot be specified with `cluster-admin` permission
- AWS account ID must be exactly 12 digits
- Slack user ID must match pattern `^U[A-Z0-9]{10}$`

### Mutating Webhook

The mutating webhook automatically sets defaults and normalizes data:

#### Default Values
- **Permissions**: `["view"]` if not specified
- **Duration**: `"1h"` if not specified
- **Labels**: Adds tracking labels for user, cluster, environment
- **Timestamps**: Sets `requestedAt` if not provided

#### Data Normalization
- **Cluster names**: Converted to lowercase
- **Permissions**: Deduplicated and normalized
- **Namespaces**: Deduplicated and validated format

#### Auto-Assignment
- **Approvers**: Automatically assigned based on:
  - **Production clusters**: `platform-team`, `sre-team`
  - **Elevated permissions**: Additional `security-team` approval
  - **Staging clusters**: Approval required only for elevated permissions
  - **Development clusters**: No approval required for basic access

#### Environment Detection
- **Production**: Cluster names containing "prod" or "production"
- **Staging**: Cluster names containing "stag" or "staging"  
- **Development**: Cluster names containing "dev" or "development"
- **Default**: Production (for safety)

### Webhook Endpoints

| Webhook | Path | Purpose |
|---------|------|---------|
| Validating | `/validate-jit-rebelops-io-v1alpha1-jitaccessrequest` | Validate business rules |
| Mutating (Request) | `/mutate-jit-rebelops-io-v1alpha1-jitaccessrequest` | Set defaults and normalize |
| Mutating (Job) | `/mutate-jit-rebelops-io-v1alpha1-jitaccessjob` | Job resource mutation |

## REST API Endpoints

The JIT server provides REST endpoints for integration and management.

### Base URL

```
https://your-domain.com
```

### Authentication

All API endpoints require authentication via:
- Slack signature verification (for Slack endpoints)
- Kubernetes ServiceAccount tokens (for admin endpoints)

### Endpoints

#### Slack Integration

##### POST /slack/commands

Handle Slack slash commands.

**Request Headers:**
```
Content-Type: application/x-www-form-urlencoded
X-Slack-Request-Timestamp: <timestamp>
X-Slack-Signature: <signature>
```

**Request Body:**
```
token=<slack-token>&
user_id=<user-id>&
user_name=<username>&
command=/jit&
text=<command-text>&
channel_id=<channel-id>&
response_url=<response-url>
```

**Response:**
```json
{
  "response_type": "ephemeral",
  "text": "Command response"
}
```

##### POST /slack/interactive

Handle Slack interactive components (buttons, modals).

**Request Format:** Same as commands endpoint

##### POST /slack/events

Handle Slack events (mentions, DMs).

**Request Format:** JSON event payload

#### Admin Endpoints

##### GET /api/v1/requests

List all access requests.

**Query Parameters:**
- `user` (optional): Filter by user ID
- `cluster` (optional): Filter by cluster name
- `status` (optional): Filter by status
- `limit` (optional): Limit results (default: 50)

**Response:**
```json
{
  "items": [
    {
      "id": "jit-user123-1640995200",
      "userId": "U123USER",
      "userEmail": "john.doe@company.com",
      "cluster": "prod-east-1",
      "reason": "Deploy critical hotfix",
      "duration": "2h",
      "status": "Active",
      "requestedAt": "2023-12-31T23:00:00Z",
      "expiresAt": "2024-01-01T01:06:00Z"
    }
  ],
  "total": 1
}
```

##### GET /api/v1/requests/{id}

Get specific access request details.

**Response:**
```json
{
  "id": "jit-user123-1640995200",
  "userId": "U123USER",
  "userEmail": "john.doe@company.com",
  "cluster": "prod-east-1",
  "reason": "Deploy critical hotfix",
  "duration": "2h",
  "permissions": ["edit"],
  "namespaces": ["payment-service"],
  "status": "Active",
  "requestedAt": "2023-12-31T23:00:00Z",
  "approvals": [
    {
      "approver": "U456APPROVER",
      "approvedAt": "2023-12-31T23:05:00Z",
      "comment": "Approved for emergency"
    }
  ],
  "accessDetails": {
    "principalArn": "arn:aws:sts::123456789012:assumed-role/JITAccessRole/session",
    "createdAt": "2023-12-31T23:06:00Z",
    "expiresAt": "2024-01-01T01:06:00Z"
  }
}
```

##### POST /api/v1/requests/{id}/approve

Approve an access request.

**Request Body:**
```json
{
  "approver": "U456APPROVER",
  "comment": "Approved for emergency deployment"
}
```

**Response:**
```json
{
  "status": "approved",
  "message": "Request approved successfully"
}
```

##### POST /api/v1/requests/{id}/deny

Deny an access request.

**Request Body:**
```json
{
  "approver": "U456APPROVER",
  "reason": "Insufficient justification"
}
```

##### DELETE /api/v1/requests/{id}

Revoke active access.

**Response:**
```json
{
  "status": "revoked",
  "message": "Access revoked successfully"
}
```

#### Health and Metrics

##### GET /healthz

Health check endpoint.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2023-12-31T23:00:00Z"
}
```

##### GET /metrics

Prometheus metrics endpoint.

**Response:** Prometheus format metrics

## Slack Command Interface

### Command Format

```
/jit <subcommand> [arguments] [flags]
```

### Subcommands

#### request

Request JIT access to a cluster.

**Syntax:**
```
/jit request <cluster> <duration> <reason> [--permissions=<perms>] [--namespaces=<ns>]
```

**Arguments:**
- `cluster`: EKS cluster name
- `duration`: Access duration (e.g., "2h", "30m")
- `reason`: Business justification

**Flags:**
- `--permissions`: Comma-separated permissions (default: "view")
- `--namespaces`: Comma-separated namespaces (default: cluster-wide)

**Examples:**
```
/jit request prod-east-1 2h "Deploy hotfix"
/jit request staging-west-2 4h "Feature testing" --permissions=edit --namespaces=default,testing
```

#### approve

Approve a pending access request.

**Syntax:**
```
/jit approve <request-id> [comment]
```

**Arguments:**
- `request-id`: ID of the request to approve
- `comment`: Optional approval comment

**Example:**
```
/jit approve jit-user123-1640995200 "Approved for emergency deployment"
```

#### deny

Deny a pending access request.

**Syntax:**
```
/jit deny <request-id> <reason>
```

**Arguments:**
- `request-id`: ID of the request to deny
- `reason`: Reason for denial

**Example:**
```
/jit deny jit-user123-1640995200 "Insufficient business justification"
```

#### list

List access requests.

**Syntax:**
```
/jit list [mine|all] [--status=<status>] [--cluster=<cluster>]
```

**Arguments:**
- `mine`: Show only your requests (default)
- `all`: Show all requests (admin only)

**Flags:**
- `--status`: Filter by status
- `--cluster`: Filter by cluster

**Examples:**
```
/jit list mine
/jit list all --status=pending
/jit list --cluster=prod-east-1
```

#### revoke

Revoke active access.

**Syntax:**
```
/jit revoke <request-id> [reason]
```

**Arguments:**
- `request-id`: ID of the active request
- `reason`: Optional revocation reason

**Example:**
```
/jit revoke jit-user123-1640995200 "No longer needed"
```

#### status

Check status of a specific request.

**Syntax:**
```
/jit status <request-id>
```

**Example:**
```
/jit status jit-user123-1640995200
```

#### help

Show help information.

**Syntax:**
```
/jit help [subcommand]
```

**Examples:**
```
/jit help
/jit help request
```

### Response Format

All Slack commands return responses in this format:

#### Success Response

```json
{
  "response_type": "in_channel",
  "text": "✅ Success message",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "Detailed information"
      }
    }
  ]
}
```

#### Error Response

```json
{
  "response_type": "ephemeral",
  "text": "❌ Error message",
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "Error details and suggested actions"
      }
    }
  ]
}
```

## Webhook Events

### Slack Events

The system listens for various Slack events:

#### Command Events

Triggered by slash commands.

```json
{
  "type": "slash_command",
  "user": {
    "id": "U123USER",
    "name": "john.doe"
  },
  "command": "/jit",
  "text": "request prod-east-1 2h Deploy hotfix",
  "channel": {
    "id": "C123CHANNEL",
    "name": "engineering"
  },
  "response_url": "https://hooks.slack.com/commands/...",
  "trigger_id": "123.456.789"
}
```

#### Interactive Events

Triggered by button clicks, form submissions.

```json
{
  "type": "interactive_message",
  "user": {
    "id": "U456APPROVER",
    "name": "jane.smith"
  },
  "actions": [
    {
      "name": "approve",
      "type": "button",
      "value": "jit-user123-1640995200"
    }
  ],
  "callback_id": "approve_request",
  "response_url": "https://hooks.slack.com/actions/..."
}
```

### Kubernetes Events

The operator emits Kubernetes events for important state changes:

#### Request Events

```yaml
apiVersion: v1
kind: Event
metadata:
  name: jit-user123-1640995200.request-created
  namespace: jit-system
type: Normal
reason: RequestCreated
message: "JIT access request created for user U123USER"
involvedObject:
  apiVersion: jit.rebelops.io/v1
  kind: JITAccessRequest
  name: jit-user123-1640995200
  namespace: jit-system
```

#### Access Events

```yaml
apiVersion: v1
kind: Event
type: Normal
reason: AccessGranted
message: "JIT access granted to user U123USER for cluster prod-east-1"
involvedObject:
  apiVersion: jit.rebelops.io/v1
  kind: JITAccessJob
```

## Error Codes

### HTTP Status Codes

- `200`: Success
- `201`: Created
- `400`: Bad Request
- `401`: Unauthorized
- `403`: Forbidden
- `404`: Not Found
- `409`: Conflict
- `429`: Rate Limited
- `500`: Internal Server Error

### Application Error Codes

| Code | Description |
|------|-------------|
| `INVALID_CLUSTER` | Specified cluster not found |
| `INVALID_DURATION` | Invalid duration format |
| `PERMISSION_DENIED` | User lacks required permissions |
| `APPROVAL_REQUIRED` | Request requires approval |
| `ALREADY_APPROVED` | Request already approved |
| `ALREADY_DENIED` | Request already denied |
| `ACCESS_EXPIRED` | Access has expired |
| `AWS_ERROR` | AWS API error |
| `K8S_ERROR` | Kubernetes API error |

## Rate Limits

- Slack commands: 1 request per second per user
- API endpoints: 10 requests per minute per user
- Webhook endpoints: 100 requests per minute total

## SDK Examples

### Go Client

```go
import (
    "context"
    "k8s.io/client-go/kubernetes/scheme"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "github.com/your-org/jit-access/pkg/controller"
)

// Create a JIT access request
func createAccessRequest(ctx context.Context, k8sClient client.Client) error {
    request := &controller.JITAccessRequest{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "my-request",
            Namespace: "jit-system",
        },
        Spec: controller.JITAccessRequestSpec{
            UserID:    "U123USER",
            UserEmail: "user@company.com",
            TargetCluster: controller.TargetCluster{
                Name:       "prod-east-1",
                AWSAccount: "123456789012",
                Region:     "us-east-1",
            },
            Reason:      "Deploy hotfix",
            Duration:    "2h",
            Permissions: []string{"edit"},
            RequestedAt: metav1.Now(),
        },
    }
    
    return k8sClient.Create(ctx, request)
}
```

### kubectl Examples

```bash
# Create access request
kubectl apply -f - <<EOF
apiVersion: jit.rebelops.io/v1
kind: JITAccessRequest
metadata:
  name: my-request
  namespace: jit-system
spec:
  userID: "U123USER"
  userEmail: "user@company.com"
  targetCluster:
    name: "prod-east-1"
    awsAccount: "123456789012"
    region: "us-east-1"
  reason: "Deploy hotfix"
  duration: "2h"
  permissions: ["edit"]
  requestedAt: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EOF

# List all requests
kubectl get jitaccessrequests -n jit-system

# Get request details
kubectl describe jitaccessrequest my-request -n jit-system

# Update request status (approval)
kubectl patch jitaccessrequest my-request -n jit-system --type=merge -p='
status:
  approvals:
  - approver: "U456APPROVER"
    approvedAt: "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    comment: "Approved"
'
```