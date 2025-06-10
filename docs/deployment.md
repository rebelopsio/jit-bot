# Deployment Guide

This guide covers deploying the JIT Access Tool to a Kubernetes cluster.

## Prerequisites

### System Requirements

- **Kubernetes**: Version 1.24 or later
- **kubectl**: Configured to access your target cluster
- **Helm**: Version 3.x (optional, for Helm chart deployment)
- **Docker**: For building custom images (optional)

### AWS Requirements

- AWS Organizations setup with cross-account trust relationships
- EKS clusters in target AWS accounts
- IAM roles configured for JIT access
- SAML identity provider configured in AWS IAM

### Slack Requirements

- Slack workspace with administrative access
- Slack app with bot permissions
- Bot token and signing secret

## Installation Methods

### Method 1: Using Makefile (Recommended)

1. **Clone the repository**:
   ```bash
   git clone https://github.com/your-org/jit-bot.git
   cd jit-bot
   ```

2. **Deploy everything**:
   ```bash
   make deploy-all
   ```

   This will:
   - Install CRDs with OpenAPI validation
   - Create the `jit-system` namespace
   - Deploy the operator with RBAC
   - Configure admission webhooks

3. **Verify deployment**:
   ```bash
   kubectl get pods -n jit-system
   kubectl get crds | grep jit.rebelops.io
   ```

### Method 2: Manual kubectl Deployment

1. **Install CRDs**:
   ```bash
   kubectl apply -f manifests/crds/
   ```

2. **Setup webhook certificates** (choose one):
   
   **Option A: Using cert-manager**:
   ```bash
   # Install cert-manager
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   
   # Apply webhook configurations
   kubectl apply -f config/webhook/manifests.yaml
   ```
   
   **Option B: Generate self-signed certificates**:
   ```bash
   scripts/generate-webhook-certs.sh
   ```

3. **Deploy operator**:
   ```bash
   kubectl apply -f manifests/operator/
   ```

### Method 3: Helm Chart (Future)

```bash
helm repo add jit-access https://charts.jit-access.io
helm install jit-access jit-access/jit-operator -n jit-system --create-namespace
```

## Configuration

### 1. AWS Credentials

Create a secret with AWS credentials for the operator:

```bash
kubectl create secret generic aws-credentials \
  --from-literal=aws-access-key-id=AKIAIOSFODNN7EXAMPLE \
  --from-literal=aws-secret-access-key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
  --from-literal=aws-region=us-east-1 \
  -n jit-system
```

**Security Note**: Consider using IAM roles for service accounts (IRSA) instead of long-lived credentials.

### 2. Slack Configuration

Create a secret with Slack bot credentials:

```bash
kubectl create secret generic slack-config \
  --from-literal=bot-token=xoxb-your-slack-bot-token \
  --from-literal=signing-secret=your-slack-signing-secret \
  -n jit-system
```

### 3. Cluster Configuration

Edit the ConfigMap to define your EKS clusters:

```bash
kubectl edit configmap jit-operator-config -n jit-system
```

Update the `clusters.yaml` section:

```yaml
data:
  clusters.yaml: |
    clusters:
      - name: "prod-east-1"
        awsAccount: "123456789012"
        region: "us-east-1"
        endpoint: "https://ABC123.gr7.us-east-1.eks.amazonaws.com"
        maxDuration: "4h"
        requireApproval: true
        approvers:
          - "platform-team"
          - "sre-team"
      - name: "staging-east-1"
        awsAccount: "123456789012"
        region: "us-east-1"
        endpoint: "https://XYZ789.gr7.us-east-1.eks.amazonaws.com"
        maxDuration: "8h"
        requireApproval: false
      - name: "dev-west-2"
        awsAccount: "987654321098"
        region: "us-west-2"
        endpoint: "https://DEV456.gr7.us-west-2.eks.amazonaws.com"
        maxDuration: "12h"
        requireApproval: false
```

### 4. RBAC Configuration

Configure user roles by editing the RBAC system:

```bash
# This would typically be done through a ConfigMap or during operator startup
kubectl create configmap rbac-config \
  --from-literal=admins=U123ADMIN,U456ADMIN \
  --from-literal=approvers=U789APPROVER,U012APPROVER \
  -n jit-system
```

### 5. Webhook Validation

The system includes admission webhooks that provide:

**Validating Webhook:**
- Duration validation (15 minutes to 7 days)
- Permission validation (enum checking)
- Business rule enforcement
- AWS resource format validation

**Mutating Webhook:**
- Default value injection
- Data normalization
- Auto-approver assignment based on environment
- Metadata enrichment

**Verify webhooks are working:**
```bash
# Check webhook configurations
kubectl get validatingadmissionwebhook jit-bot-validating-webhook
kubectl get mutatingadmissionwebhook jit-bot-mutating-webhook

# Test validation (should fail with validation error)
kubectl apply -f - <<EOF
apiVersion: jit.rebelops.io/v1alpha1
kind: JITAccessRequest
metadata:
  name: test-invalid
  namespace: jit-system
spec:
  userID: "invalid"
  userEmail: "invalid-email"
  reason: "short"
  duration: "999d"
  permissions: ["invalid-perm"]
  targetCluster:
    name: "test"
    awsAccount: "invalid"
    region: "invalid"
EOF
```

## JIT Server Deployment

The JIT server handles Slack interactions and can be deployed separately:

### 1. Create Server Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jit-server
  namespace: jit-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: jit-server
  template:
    metadata:
      labels:
        app: jit-server
    spec:
      containers:
      - name: jit-server
        image: jit-server:latest
        ports:
        - containerPort: 8080
        env:
        - name: SLACK_BOT_TOKEN
          valueFrom:
            secretKeyRef:
              name: slack-config
              key: bot-token
        - name: SLACK_SIGNING_SECRET
          valueFrom:
            secretKeyRef:
              name: slack-config
              key: signing-secret
        - name: K8S_NAMESPACE
          value: "jit-system"
---
apiVersion: v1
kind: Service
metadata:
  name: jit-server
  namespace: jit-system
spec:
  selector:
    app: jit-server
  ports:
  - port: 80
    targetPort: 8080
  type: ClusterIP
```

### 2. Expose via Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: jit-server
  namespace: jit-system
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - jit.your-domain.com
    secretName: jit-server-tls
  rules:
  - host: jit.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: jit-server
            port:
              number: 80
```

## Verification

### 1. Check Operator Status

```bash
# Check operator pod
kubectl get pods -n jit-system
kubectl logs deployment/jit-operator -n jit-system

# Check CRDs
kubectl get crds | grep jit.rebelops.io

# Check operator metrics
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics
```

### 2. Test CRD Creation

```bash
# Create a test access request
cat <<EOF | kubectl apply -f -
apiVersion: jit.rebelops.io/v1
kind: JITAccessRequest
metadata:
  name: test-request
  namespace: jit-system
spec:
  userID: "U123TEST"
  userEmail: "test@company.com"
  targetCluster:
    name: "staging-east-1"
    awsAccount: "123456789012"
    region: "us-east-1"
  reason: "Testing deployment"
  duration: "1h"
  permissions: ["view"]
  requestedAt: "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
EOF

# Check the request
kubectl get jitaccessrequests -n jit-system
kubectl describe jitaccessrequest test-request -n jit-system
```

### 3. Test Slack Integration

```bash
# Check server logs
kubectl logs deployment/jit-server -n jit-system

# Test webhook endpoint
curl -X POST https://jit.your-domain.com/slack/commands \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "token=test&user_id=U123TEST&command=/jit&text=help"
```

## Monitoring and Observability

### 1. Monitoring Stack Deployment

Deploy the complete monitoring stack including Prometheus, Grafana, AlertManager, and OpenTelemetry:

```bash
# Deploy monitoring components
kubectl apply -f config/monitoring/

# Verify monitoring stack
kubectl get servicemonitors -n jit-system
kubectl get prometheusrules -n jit-system
kubectl get pods -l app=otel-collector -n jit-system
```

### 2. Prometheus Metrics

The operator exposes comprehensive metrics on port 8080:

```bash
# Port forward to access metrics
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system

# View metrics
curl http://localhost:8080/metrics
```

**Key Metrics Categories:**
- **Business**: Access requests, sessions, approval rates
- **Security**: Violations, escalation attempts, validation errors  
- **Performance**: Webhook latency, AWS API calls, processing time
- **Infrastructure**: System health, error rates, resource usage

### 3. OpenTelemetry Tracing

Configure distributed tracing for detailed request flow analysis:

**Operator Configuration:**
```bash
# Enable tracing with Jaeger
kubectl set env deployment/jit-operator \
  ENABLE_TRACING=true \
  TRACING_EXPORTER=jaeger \
  TRACING_ENDPOINT=http://jaeger-collector.monitoring:14268/api/traces \
  -n jit-system

# Enable tracing with OTLP  
kubectl set env deployment/jit-operator \
  ENABLE_TRACING=true \
  TRACING_EXPORTER=otlp \
  TRACING_ENDPOINT=http://otel-collector.jit-system:4317 \
  -n jit-system
```

**Sample Rates by Environment:**
- Production: 1% (`ENVIRONMENT=production`)
- Staging: 5% (`ENVIRONMENT=staging`)
- Development: 10% (`ENVIRONMENT=development`)

### 4. Grafana Dashboard

Access the Grafana dashboard to monitor JIT Bot performance:

```bash
# Port forward to Grafana (if not using ingress)
kubectl port-forward svc/grafana 3000:3000 -n monitoring

# Open browser to http://localhost:3000
# Import dashboard from config/monitoring/grafana-dashboard.json
```

**Dashboard Sections:**
- Overview: Active sessions, requests today, approval rate, system health
- Access Requests: Request rates, processing duration, sessions by cluster
- Performance: Webhook latency, error rates, AWS API performance
- Security: Violations, privilege escalation attempts
- Infrastructure: System health status, resource usage

### 5. Alerting

AlertManager rules are automatically configured for:

**Critical Alerts:**
- JIT Operator/Server down
- AWS integration failure
- Webhook service unavailable
- High privilege escalation attempts

**Warning Alerts:**
- High error rates (AWS API, Slack API, webhook validation)
- Performance degradation (high latency, slow processing)
- Security issues (violation spikes, suspicious patterns)

**View Active Alerts:**
```bash
# Check AlertManager status
kubectl get pods -l app=alertmanager -n monitoring

# View Prometheus rules
kubectl get prometheusrules jit-bot-alerts -n jit-system -o yaml
```

### 6. Structured Logging

Both components use structured logging with correlation IDs:

```bash
# Operator logs with tracing context
kubectl logs deployment/jit-operator -n jit-system -f

# Server logs with request correlation
kubectl logs deployment/jit-server -n jit-system -f

# Filter for specific trace ID
kubectl logs deployment/jit-operator -n jit-system | grep "trace_id=abc123"

# Filter for specific request
kubectl logs deployment/jit-operator -n jit-system | grep "request_id=jit-user123-1234567890"
```

### 7. Health Checks

Configure comprehensive health monitoring:

```bash
# Check operator health endpoint
kubectl port-forward deployment/jit-operator 8081:8081 -n jit-system
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz

# Monitor health metrics
curl http://localhost:8080/metrics | grep jit_system_health_status
```

## Troubleshooting

### Common Issues

1. **CRDs not installing**:
   ```bash
   kubectl get crds | grep jit
   kubectl describe crd jitaccessrequests.jit.rebelops.io
   ```

2. **Operator not starting**:
   ```bash
   kubectl describe pod -l app.kubernetes.io/name=jit-operator -n jit-system
   kubectl logs deployment/jit-operator -n jit-system
   ```

3. **AWS permissions issues**:
   ```bash
   kubectl logs deployment/jit-operator -n jit-system | grep "AWS"
   ```

4. **Slack webhook failures**:
   ```bash
   kubectl logs deployment/jit-server -n jit-system | grep "slack"
   ```

### Debug Mode

Enable debug logging by setting environment variables:

```bash
kubectl set env deployment/jit-operator LOG_LEVEL=debug -n jit-system
kubectl set env deployment/jit-server LOG_LEVEL=debug -n jit-system
```

## Upgrading

### 1. Using Makefile

```bash
# Update CRDs
make install-crds

# Update operator
make deploy-operator
```

### 2. Manual Upgrade

```bash
# Apply new CRDs
kubectl apply -f manifests/crds/

# Update operator
kubectl apply -f manifests/operator/

# Restart operator to pick up changes
kubectl rollout restart deployment/jit-operator -n jit-system
```

## Uninstallation

To completely remove the JIT Access Tool:

```bash
# Remove all deployments
make undeploy-all

# Or manually:
kubectl delete -f manifests/operator/
kubectl delete -f manifests/crds/

# Remove secrets (optional)
kubectl delete secret aws-credentials slack-config -n jit-system

# Remove namespace
kubectl delete namespace jit-system
```

## Security Considerations

### 1. Network Policies

Implement network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: jit-operator-netpol
  namespace: jit-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: jit-operator
  policyTypes:
  - Ingress
  - Egress
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # HTTPS to AWS APIs
  - to: []
    ports:
    - protocol: TCP
      port: 6443  # Kubernetes API
```

### 2. Pod Security Standards

Apply pod security standards:

```bash
kubectl label namespace jit-system pod-security.kubernetes.io/enforce=restricted
kubectl label namespace jit-system pod-security.kubernetes.io/audit=restricted
kubectl label namespace jit-system pod-security.kubernetes.io/warn=restricted
```

### 3. Resource Limits

Set appropriate resource limits:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```