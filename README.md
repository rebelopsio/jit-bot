# JIT Bot - Just-In-Time Access Tool for AWS EKS

A comprehensive Just-In-Time (JIT) access management solution for AWS EKS clusters with Slack integration, built using Kubernetes operators and custom resources.

## 🏗️ Architecture

The system consists of two main components:

1. **JIT Server**: Handles Slack interactions and provides web endpoints
2. **JIT Operator**: Kubernetes operator that manages access lifecycle using Custom Resource Definitions (CRDs)

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Slack       │◄──►│   JIT Server    │◄──►│  JIT Operator   │
│   Commands      │    │  (Web/Slack)    │    │ (K8s Controller)│
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │  Google SAML    │    │ JITAccessRequest│
                       │ Authentication  │    │  & JITAccessJob │
                       └─────────────────┘    │      CRDs       │
                                │             └─────────────────┘
                                ▼                       │
                       ┌─────────────────┐              ▼
                       │  AWS EKS/STS    │    ┌─────────────────┐
                       │   Multi-Account │    │   Kubernetes    │
                       └─────────────────┘    │    Secrets      │
                                              └─────────────────┘
```

## ✨ Features

- **Kubernetes-Native**: Uses CRDs and controllers for access management
- **Input Validation**: Admission webhooks validate requests and enforce business rules
- **Smart Defaults**: Mutating webhooks automatically set defaults and normalize data
- **Slack Integration**: Complete `/jit` command interface for requests and approvals
- **Multi-Account AWS**: Support for AWS Organizations with cross-account access
- **Time-Limited Access**: Automatic expiration and cleanup of access grants
- **Approval Workflow**: Configurable approval requirements per cluster with auto-assignment
- **Audit Trail**: Complete tracking of all access requests and approvals
- **Security**: Temporary credentials stored in Kubernetes secrets
- **Auto-Cleanup**: Automated revocation of expired access and resource cleanup
- **REST API**: Complete HTTP API for programmatic access management
- **AWS Integration**: Full STS and EKS integration with temporary credential generation
- **KubeConfig Generation**: Automatic kubeconfig creation with embedded temporary credentials

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster (1.24+)
- Go 1.21+ (for development)
- AWS CLI configured with appropriate permissions
- Slack workspace with bot permissions
- Google SAML provider configured in AWS IAM

### Installation

1. **Install CRDs and Operator**:
   ```bash
   make deploy-all
   ```

2. **Setup Webhook Certificates** (choose one option):
   
   **Option A: Using cert-manager (recommended)**:
   ```bash
   # Install cert-manager first
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   
   # Deploy webhook configurations
   kubectl apply -f config/webhook/manifests.yaml
   ```
   
   **Option B: Manual certificate generation**:
   ```bash
   # Generate self-signed certificates
   scripts/generate-webhook-certs.sh
   ```

3. **Configure AWS Credentials**:
   ```bash
   kubectl create secret generic aws-credentials \
     --from-literal=aws-access-key-id=YOUR_ACCESS_KEY \
     --from-literal=aws-secret-access-key=YOUR_SECRET_KEY \
     -n jit-system
   ```

4. **Configure Slack Integration**:
   ```bash
   kubectl create secret generic slack-config \
     --from-literal=bot-token=xoxb-your-bot-token \
     --from-literal=signing-secret=your-signing-secret \
     -n jit-system
   ```

## 📖 Usage

### Slack Commands

#### Request Access
```
/jit request prod-east-1 2h "Deploy hotfix" --permissions=edit --namespaces=production
```

**Validation Rules:**
- Duration: 15 minutes to 7 days (e.g., `15m`, `2h`, `1d`)
- Permissions: `view`, `edit`, `admin`, `cluster-admin`, `debug`, `logs`, `exec`, `port-forward`
- Reason: Minimum 10 characters, meaningful business justification
- Auto-approvers assigned based on cluster environment and permission level

#### Approve Request
```
/jit approve jit-user123-1234567890 "Approved for hotfix deployment"
```

#### List Requests
```
/jit list mine          # Your requests
/jit list              # All requests (admin only)
```

#### Get Help
```
/jit help
```

### Kubernetes Resources

#### Check Access Requests
```bash
kubectl get jitaccessrequests -n jit-system
kubectl describe jitaccessrequest jit-user123-1234567890 -n jit-system
```

#### Monitor Access Jobs
```bash
kubectl get jitaccessjobs -n jit-system
kubectl logs deployment/jit-operator -n jit-system
```

### REST API

The JIT Server provides a complete REST API for programmatic access management:

#### Grant Access
```bash
POST /api/v1/access/grant
Content-Type: application/json
X-Slack-User-Id: U1234567890

{
  "cluster_id": "cluster-123",
  "user_id": "U1234567890",
  "user_email": "user@company.com",
  "permissions": ["edit"],
  "namespaces": ["production"],
  "duration": "2h",
  "reason": "Deploy hotfix for critical bug",
  "jit_role_arn": "arn:aws:iam::123456789012:role/JITAccessRole"
}
```

**Response:**
```json
{
  "access_id": "access-abc123",
  "cluster_name": "prod-east-1",
  "user_id": "U1234567890",
  "kubeconfig": "apiVersion: v1\nkind: Config...",
  "cluster_endpoint": "https://ABC123.gr7.us-east-1.eks.amazonaws.com",
  "expires_at": "2025-06-11T16:00:00Z",
  "temporary_credentials": {
    "access_key_id": "ASIA...",
    "secret_access_key": "...",
    "session_token": "...",
    "expiration": "2025-06-11T16:00:00Z"
  }
}
```

#### Revoke Access
```bash
POST /api/v1/access/revoke
Content-Type: application/json
X-Slack-User-Id: U1234567890

{
  "access_id": "access-abc123"
}
```

#### List Access Records
```bash
GET /api/v1/access?user_id=U1234567890&active=true
X-Slack-User-Id: U1234567890
```

#### Get Access Status
```bash
GET /api/v1/access/status?access_id=access-abc123
X-Slack-User-Id: U1234567890
```

#### Cleanup Expired Access
```bash
POST /api/v1/access/cleanup?cluster_id=cluster-123
X-Slack-User-Id: U1234567890  # Admin only
```

## 🔧 Configuration

### Cluster Configuration

Edit the ConfigMap to define your EKS clusters:

```yaml
# manifests/operator/configmap.yaml
data:
  clusters.yaml: |
    clusters:
      - name: "prod-east-1"
        awsAccount: "123456789012"
        region: "us-east-1"
        maxDuration: "4h"
        requireApproval: true
        approvers: ["platform-team", "sre-team"]
```

### RBAC Configuration

The system supports role-based access control:

- **Admin**: Can approve requests, manage system
- **Approver**: Can approve requests for specific clusters
- **Requester**: Can create access requests

## 🛠️ Development

### Build from Source

```bash
# Build both components
make build-all

# Run tests
make test

# Run linting
make lint

# Format code
make fmt
```

### Local Development

1. **Run the operator locally**:
   ```bash
   make operator-run
   ```

2. **Run the server locally**:
   ```bash
   make run
   ```

### Docker Images

```bash
# Build operator image
make operator-docker-build

# Build server image  
make docker-build
```

## 📁 Project Structure

```
├── cmd/
│   ├── jit-server/          # Slack server application
│   └── operator/            # Kubernetes operator
├── pkg/
│   ├── auth/               # RBAC and authentication
│   ├── aws/                # AWS EKS/STS integration
│   ├── controller/         # Kubernetes controllers and CRDs
│   ├── kubernetes/         # Access management logic
│   ├── models/             # Data models
│   ├── slack/              # Slack integration
│   ├── store/              # Data storage
│   └── webhook/            # Admission webhooks for validation
├── manifests/
│   ├── crds/               # Custom Resource Definitions
│   └── operator/           # Operator deployment manifests
├── config/
│   └── webhook/            # Webhook configurations and certificates
├── charts/                 # Helm charts
├── internal/               # Internal packages
├── scripts/                # Utility scripts
└── docs/                   # Documentation
```

## 🔒 Security Considerations

- **Input Validation**: Admission webhooks prevent malformed or malicious requests
- **Least Privilege**: Users receive minimal required permissions
- **Time-Limited**: All access automatically expires (enforced by validation)
- **Audit Trail**: Complete logging of all access requests and approvals
- **Secrets Management**: Credentials stored securely in Kubernetes secrets
- **RBAC**: Role-based access control for different user types
- **Approval Workflow**: Required approvals for production clusters with auto-assignment
- **Business Rules**: Webhooks enforce security policies and prevent escalation

## 📊 Monitoring & Observability

JIT Bot provides comprehensive monitoring and observability through Prometheus metrics and OpenTelemetry tracing.

### Prometheus Metrics

The operator exposes metrics on `:8080/metrics` and health checks on `:8081/healthz`.

**Key Business Metrics:**
- `jit_access_requests_total` - Total access requests by cluster, user, environment
- `jit_active_access_sessions` - Currently active access sessions
- `jit_access_requests_approved_total` - Approved requests counter
- `jit_access_requests_denied_total` - Denied requests counter
- `jit_access_request_duration_seconds` - Request processing time histogram

**Security Metrics:**
- `jit_security_violations_total` - Security violations by type
- `jit_privilege_escalation_attempts_total` - Privilege escalation attempts
- `jit_webhook_validation_errors_total` - Webhook validation failures

**Performance Metrics:**
- `jit_webhook_request_duration_seconds` - Webhook response time
- `jit_aws_api_calls_total` - AWS API call rates
- `jit_slack_command_duration_seconds` - Slack command latency

### OpenTelemetry Tracing

Distributed tracing is available for detailed request flow analysis:

**Configuration Options:**
- **Jaeger**: `--tracing-exporter=jaeger --tracing-endpoint=http://jaeger:14268/api/traces`
- **OTLP**: `--tracing-exporter=otlp --tracing-endpoint=http://otel-collector:4317`

**Sample Rates by Environment:**
- Production: 1%
- Staging: 5% 
- Development: 10%

### Monitoring Stack Deployment

Deploy the complete monitoring stack including Prometheus, Grafana, and AlertManager:

```bash
kubectl apply -f config/monitoring/
```

**Includes:**
- ServiceMonitors for automatic metrics collection
- Grafana dashboard with JIT Bot visualizations
- AlertManager rules for security and performance alerts
- OpenTelemetry Collector for trace aggregation

### Dashboard Access

Access the Grafana dashboard at `http://grafana.monitoring.svc.cluster.local:3000` to view:
- Access request rates and approval metrics
- Active session monitoring by cluster
- Security violation alerts
- Performance and error rate tracking

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test lint`
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Troubleshooting

See [docs/troubleshooting.md](docs/troubleshooting.md) for common issues and solutions.

## 📚 Additional Documentation

- [Deployment Guide](docs/deployment.md)
- [AWS Setup](docs/aws-setup.md)
- [Slack Configuration](docs/slack-setup.md)
- [API Reference](docs/api-reference.md)