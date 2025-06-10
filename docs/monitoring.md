# Monitoring Guide

This guide covers monitoring and observability for the JIT Bot system using Prometheus metrics, OpenTelemetry tracing, and Grafana dashboards.

## Overview

JIT Bot provides comprehensive observability through:

- **Prometheus Metrics**: Business, security, performance, and infrastructure metrics
- **OpenTelemetry Tracing**: Distributed tracing across all components
- **Grafana Dashboards**: Visual monitoring and alerting
- **AlertManager**: Automated alerting for critical issues
- **Structured Logging**: Correlation IDs and request tracking

## Quick Start

### 1. Deploy Monitoring Stack

```bash
# Deploy all monitoring components
kubectl apply -f config/monitoring/

# Verify deployment
kubectl get pods -l app=otel-collector -n jit-system
kubectl get servicemonitors -n jit-system
kubectl get prometheusrules -n jit-system
```

### 2. Enable Tracing

```bash
# Enable OpenTelemetry tracing on operator
kubectl set env deployment/jit-operator \
  --enable-tracing=true \
  --tracing-exporter=otlp \
  --tracing-endpoint=http://otel-collector.jit-system:4317 \
  -n jit-system
```

### 3. Access Dashboards

```bash
# Port forward to Grafana
kubectl port-forward svc/grafana 3000:3000 -n monitoring

# Open http://localhost:3000 and import dashboard
```

## Prometheus Metrics

### Business Metrics

Track core JIT Bot business functionality:

```prometheus
# Total access requests by cluster, user, environment
jit_access_requests_total{cluster="prod-east-1", user="U123USER", environment="production", permissions="edit"}

# Currently active access sessions
jit_active_access_sessions{cluster="staging-west-2", user="U456USER", environment="staging"}

# Approved requests counter
jit_access_requests_approved_total{cluster="prod-east-1", environment="production"}

# Denied requests counter
jit_access_requests_denied_total{cluster="dev-east-1", environment="development"}

# Request processing time distribution
jit_access_request_duration_seconds_bucket{cluster="prod-east-1", le="30"}
```

### Security Metrics

Monitor security violations and privilege escalation attempts:

```prometheus
# Security violations by type
jit_security_violations_total{violation_type="invalid_duration", cluster="prod-east-1"}

# Privilege escalation attempts
jit_privilege_escalation_attempts_total{user="U789USER", cluster="prod-east-1", permission="cluster-admin"}

# Webhook validation errors
jit_webhook_validation_errors_total{webhook_type="validating", error_type="duration_exceeded"}
```

### Performance Metrics

Track system performance and response times:

```prometheus
# Webhook response time
jit_webhook_request_duration_seconds_bucket{webhook_type="validating", operation="validate", le="1"}

# AWS API call rates and latency
jit_aws_api_calls_total{service="eks", operation="describe_cluster"}
jit_aws_api_call_duration_seconds_bucket{service="sts", operation="assume_role", le="5"}

# Slack command latency
jit_slack_command_duration_seconds_bucket{command="request", le="2"}
```

### Infrastructure Metrics

Monitor system health and resource usage:

```prometheus
# System health status (1=healthy, 0=unhealthy)
jit_system_health_status{component="webhook"}
jit_system_health_status{component="aws"}
jit_system_health_status{component="slack"}

# Error rates by component
jit_controller_errors_total{controller="JITAccessRequest"}
jit_aws_api_errors_total{service="eks", operation="describe_cluster"}
jit_slack_api_errors_total{endpoint="chat.postMessage"}
```

### Example Queries

**Request Rate Calculation:**
```prometheus
# Requests per second over 5 minutes
rate(jit_access_requests_total[5m])

# Approval rate over 1 hour
rate(jit_access_requests_approved_total[1h]) / 
(rate(jit_access_requests_approved_total[1h]) + rate(jit_access_requests_denied_total[1h]))
```

**Performance Analysis:**
```prometheus
# 95th percentile webhook latency
histogram_quantile(0.95, rate(jit_webhook_request_duration_seconds_bucket[5m]))

# Error rate by component
sum(rate(jit_controller_errors_total[5m])) by (controller)
```

## OpenTelemetry Tracing

### Configuration

Enable tracing with different exporters:

**Jaeger Exporter:**
```bash
kubectl set env deployment/jit-operator \
  --enable-tracing=true \
  --tracing-exporter=jaeger \
  --tracing-endpoint=http://jaeger-collector.monitoring:14268/api/traces \
  -n jit-system
```

**OTLP Exporter:**
```bash
kubectl set env deployment/jit-operator \
  --enable-tracing=true \
  --tracing-exporter=otlp \
  --tracing-endpoint=http://otel-collector.jit-system:4317 \
  -n jit-system
```

### Sample Rates

Configure sampling based on environment:

| Environment | Sample Rate | Purpose |
|-------------|-------------|---------|
| Production  | 1%          | Minimal overhead |
| Staging     | 5%          | Testing validation |
| Development | 10%         | Full debugging |

### Trace Structure

JIT Bot creates traces for major operations:

**Access Request Flow:**
```
jit-request-span
├── validation-span
│   ├── webhook-validation-span
│   └── business-rules-span
├── aws-integration-span
│   ├── assume-role-span
│   └── create-kubeconfig-span
└── notification-span
    └── slack-notification-span
```

**Trace Attributes:**
- `jit.operation`: Operation type (request, approve, revoke)
- `jit.user_id`: User making the request
- `jit.cluster`: Target cluster name
- `jit.request_id`: Unique request identifier
- `jit.environment`: Environment (prod, staging, dev)

### Viewing Traces

**Jaeger UI:**
```bash
kubectl port-forward svc/jaeger-query 16686:16686 -n monitoring
# Open http://localhost:16686
```

**Query Examples:**
- Service: `jit-operator`
- Operation: `access_request.process`
- Tags: `jit.cluster=prod-east-1`

## Grafana Dashboards

### Dashboard Overview

The JIT Bot dashboard includes:

1. **Overview Panel**: Active sessions, requests today, approval rate, system health
2. **Access Requests**: Request rates, processing duration, sessions by cluster
3. **Performance**: Webhook latency, error rates, AWS API performance
4. **Security**: Violations, privilege escalation attempts
5. **Infrastructure**: System health status, resource usage

### Key Visualizations

**Single Stat Panels:**
```json
{
  "title": "Active Access Sessions",
  "targets": [{"expr": "sum(jit_active_access_sessions)"}],
  "thresholds": "50,100",
  "colors": ["green", "yellow", "red"]
}
```

**Time Series Graphs:**
```json
{
  "title": "Request Rate",
  "targets": [
    {"expr": "rate(jit_access_requests_total[5m])", "legendFormat": "Requests/sec"},
    {"expr": "rate(jit_access_requests_approved_total[5m])", "legendFormat": "Approved/sec"}
  ]
}
```

**Heatmaps:**
```json
{
  "title": "Request Processing Duration",
  "targets": [
    {"expr": "rate(jit_access_request_duration_seconds_bucket[5m])"}
  ],
  "yAxis": {"label": "Duration (seconds)"}
}
```

### Dashboard Import

1. Copy content from `config/monitoring/grafana-dashboard.json`
2. Open Grafana → Dashboards → Import
3. Paste JSON content and configure data source

## Alerting

### Alert Rules Categories

**Security Alerts (Critical):**
```yaml
- alert: HighPrivilegeEscalationAttempts
  expr: increase(jit_privilege_escalation_attempts_total[5m]) > 5
  for: 1m
  labels:
    severity: critical
    component: security
```

**Availability Alerts (Critical):**
```yaml
- alert: JITOperatorDown
  expr: up{job="jit-operator"} == 0
  for: 1m
  labels:
    severity: critical
    component: operator
```

**Performance Alerts (Warning):**
```yaml
- alert: HighWebhookLatency
  expr: histogram_quantile(0.95, rate(jit_webhook_request_duration_seconds_bucket[5m])) > 5
  for: 3m
  labels:
    severity: warning
    component: webhook
```

### Alert Notifications

**Slack Integration:**
```yaml
receivers:
- name: 'jit-bot-alerts'
  slack_configs:
  - api_url: '${SLACK_WEBHOOK_URL}'
    channel: '#jit-access-alerts'
    title: 'JIT Bot Alert'
    text: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

**Email Notifications:**
```yaml
- name: 'critical-alerts'
  email_configs:
  - to: 'oncall@company.com'
    subject: 'CRITICAL: JIT Bot Alert'
    body: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

## Recording Rules

Pre-computed metrics for faster dashboard queries:

```yaml
rules:
# Request rate aggregations
- record: jit:access_request_rate_5m
  expr: rate(jit_access_requests_total[5m])

- record: jit:access_approval_rate_1h
  expr: |
    rate(jit_access_requests_approved_total[1h]) /
    (rate(jit_access_requests_approved_total[1h]) + rate(jit_access_requests_denied_total[1h]))

# Error rate aggregations
- record: jit:error_rate_5m
  expr: |
    (
      rate(jit_webhook_validation_errors_total[5m]) +
      rate(jit_aws_api_errors_total[5m]) +
      rate(jit_slack_api_errors_total[5m]) +
      rate(jit_controller_errors_total[5m])
    )
```

## Troubleshooting Monitoring

### Common Issues

**Metrics Not Appearing:**
```bash
# Check ServiceMonitor
kubectl get servicemonitors -n jit-system
kubectl describe servicemonitor jit-bot-metrics -n jit-system

# Check operator metrics endpoint
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics
```

**Traces Not Showing:**
```bash
# Check tracing configuration
kubectl get deployment jit-operator -n jit-system -o yaml | grep -A 5 -B 5 tracing

# Check OpenTelemetry Collector
kubectl logs deployment/otel-collector -n jit-system
kubectl describe pod -l app=otel-collector -n jit-system
```

**Dashboard Loading Issues:**
```bash
# Check Grafana data source
kubectl logs deployment/grafana -n monitoring

# Verify Prometheus connectivity
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
# Test query: up{job="jit-operator"}
```

### Debug Mode

Enable verbose logging for monitoring components:

```bash
# Enable debug logging on operator
kubectl set env deployment/jit-operator LOG_LEVEL=debug -n jit-system

# Check OpenTelemetry Collector debug
kubectl set env deployment/otel-collector OTEL_LOG_LEVEL=debug -n jit-system
```

## Best Practices

### Metric Naming

Follow Prometheus naming conventions:
- Use `_total` suffix for counters
- Use `_seconds` suffix for time measurements
- Include relevant labels for filtering
- Avoid high cardinality labels

### Dashboard Design

- Group related metrics in panels
- Use appropriate visualization types
- Set meaningful thresholds and colors
- Include documentation in panel descriptions

### Alerting Strategy

- **Critical**: Requires immediate action (system down, security breach)
- **Warning**: Needs attention but not urgent (performance degradation)
- **Info**: Informational only (capacity planning)

### Retention Policies

Configure appropriate retention:
- **Metrics**: 30-90 days depending on volume
- **Traces**: 7-14 days for detailed analysis
- **Logs**: 30 days with structured indexing

## Integration Examples

### Automated Incident Response

```bash
# Create incident based on alert
curl -X POST "https://api.pagerduty.com/incidents" \
  -H "Authorization: Token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "incident": {
      "type": "incident",
      "title": "JIT Bot High Privilege Escalation Attempts",
      "service": {"id": "SERVICE_ID", "type": "service_reference"},
      "urgency": "high"
    }
  }'
```

### Custom Webhook Integration

```python
import requests
import json

def send_jit_alert(alert_data):
    webhook_url = "https://your-webhook-endpoint.com/alerts"
    payload = {
        "alert_name": alert_data["alert"],
        "severity": alert_data["severity"], 
        "component": alert_data["component"],
        "description": alert_data["description"],
        "timestamp": alert_data["timestamp"]
    }
    
    response = requests.post(webhook_url, json=payload)
    return response.status_code == 200
```

## Advanced Configuration

### Custom Metrics

Add application-specific metrics:

```go
var customMetric = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "jit_custom_events_total",
        Help: "Total custom events",
    },
    []string{"event_type", "cluster"},
)

// Increment metric
customMetric.WithLabelValues("user_login", clusterName).Inc()
```

### Trace Sampling

Implement custom sampling logic:

```go
func customSampler(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
    // Sample 100% of error traces
    if strings.Contains(p.Name, "error") {
        return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
    }
    
    // Use environment-based sampling for normal traces
    rate := getSampleRateForEnvironment()
    if rand.Float64() < rate {
        return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
    }
    
    return sdktrace.SamplingResult{Decision: sdktrace.Drop}
}
```

This monitoring setup provides comprehensive observability for the JIT Bot system, enabling proactive issue detection and efficient troubleshooting.