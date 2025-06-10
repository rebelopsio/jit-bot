# Troubleshooting Guide

This guide helps diagnose and resolve common issues with the JIT Access Tool.

## Quick Diagnostics

### System Health Check

```bash
# Check operator status
kubectl get pods -n jit-system
kubectl describe deployment jit-operator -n jit-system

# Check CRDs
kubectl get crds | grep jit.rebelops.io

# Check logs
kubectl logs deployment/jit-operator -n jit-system --tail=50
```

### Common Commands

```bash
# Get access requests
kubectl get jitaccessrequests -n jit-system

# Get access jobs  
kubectl get jitaccessjobs -n jit-system

# Describe a failing request
kubectl describe jitaccessrequest <request-name> -n jit-system

# Check operator metrics
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics
```

## Issue Categories

## 1. Operator Issues

### 1.1 Operator Pod Won't Start

**Symptoms:**
- Operator pod in `CrashLoopBackOff` or `Error` state
- No logs or startup errors in logs

**Diagnosis:**
```bash
kubectl describe pod -l app.kubernetes.io/name=jit-operator -n jit-system
kubectl logs deployment/jit-operator -n jit-system
```

**Common Causes & Solutions:**

1. **Missing RBAC permissions:**
   ```bash
   # Check ClusterRole and ClusterRoleBinding
   kubectl get clusterrole jit-operator-manager-role
   kubectl get clusterrolebinding jit-operator-manager-rolebinding
   
   # Fix: Reapply RBAC
   kubectl apply -f manifests/operator/rbac.yaml
   ```

2. **Invalid AWS credentials:**
   ```bash
   # Check AWS credentials secret
   kubectl get secret aws-credentials -n jit-system
   
   # Fix: Update credentials
   kubectl create secret generic aws-credentials \
     --from-literal=aws-access-key-id=YOUR_KEY \
     --from-literal=aws-secret-access-key=YOUR_SECRET \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

3. **Invalid configuration:**
   ```bash
   # Check ConfigMap
   kubectl get configmap jit-operator-config -n jit-system -o yaml
   
   # Fix: Validate YAML syntax and content
   kubectl apply -f manifests/operator/configmap.yaml
   ```

### 1.2 Operator Running But Not Processing Requests

**Symptoms:**
- Operator pod is running
- JITAccessRequests stuck in "Pending" phase
- No JITAccessJobs being created

**Diagnosis:**
```bash
# Check if operator is receiving events
kubectl logs deployment/jit-operator -n jit-system | grep "Reconciling"

# Check controller metrics
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics | grep controller
```

**Common Causes & Solutions:**

1. **Controller not watching resources:**
   ```bash
   # Check if CRDs are properly installed
   kubectl get crd jitaccessrequests.jit.rebelops.io
   kubectl get crd jitaccessjobs.jit.rebelops.io
   
   # Fix: Reinstall CRDs
   kubectl apply -f manifests/crds/
   ```

2. **Leader election issues:**
   ```bash
   # Check leader election logs
   kubectl logs deployment/jit-operator -n jit-system | grep "leader"
   
   # Check if multiple operator instances
   kubectl get pods -l app.kubernetes.io/name=jit-operator -n jit-system
   ```

## 2. AWS Integration Issues

### 2.1 AWS Authentication Failures

**Symptoms:**
- Error messages about AWS credentials
- STS assume role failures
- EKS access denied errors

**Diagnosis:**
```bash
# Check AWS-related logs
kubectl logs deployment/jit-operator -n jit-system | grep -i aws

# Test AWS connectivity from operator pod
kubectl exec deployment/jit-operator -n jit-system -- aws sts get-caller-identity
```

**Common Causes & Solutions:**

1. **Invalid credentials:**
   ```bash
   # Verify credentials secret
   kubectl get secret aws-credentials -n jit-system -o yaml
   
   # Test credentials locally
   aws sts get-caller-identity
   ```

2. **Cross-account role issues:**
   ```bash
   # Test role assumption
   aws sts assume-role \
     --role-arn arn:aws:iam::TARGET-ACCOUNT:role/JITCrossAccountRole \
     --role-session-name test-session
   ```

3. **EKS permissions:**
   ```bash
   # Check EKS cluster access
   aws eks describe-cluster --name cluster-name
   aws eks list-access-entries --cluster-name cluster-name
   ```

### 2.2 EKS Access Entry Failures

**Symptoms:**
- JITAccessJobs fail during "Creating" phase
- Errors about access entry creation
- EKS API errors in logs

**Diagnosis:**
```bash
# Check job status
kubectl describe jitaccessjob <job-name> -n jit-system

# Check AWS EKS access entries
aws eks list-access-entries --cluster-name cluster-name
```

**Solutions:**

1. **Verify cluster configuration:**
   ```bash
   # Check if cluster supports access entries
   aws eks describe-cluster --name cluster-name \
     --query 'cluster.accessConfig.authenticationMode'
   ```

2. **Check IAM role trust relationships:**
   ```bash
   aws iam get-role --role-name JITAccessRole
   ```

## 3. Slack Integration Issues

### 3.1 Slash Commands Not Working

**Symptoms:**
- `/jit` command returns error or timeout
- No response from Slack bot
- Webhook timeout errors

**Diagnosis:**
```bash
# Check server logs
kubectl logs deployment/jit-server -n jit-system | grep slack

# Test webhook endpoint
curl -X POST https://your-domain.com/slack/commands \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "token=test&user_id=U123TEST&command=/jit&text=help"
```

**Common Causes & Solutions:**

1. **Invalid signing secret:**
   ```bash
   # Check Slack configuration
   kubectl get secret slack-config -n jit-system -o yaml
   
   # Verify signing secret in Slack app settings
   ```

2. **Network connectivity:**
   ```bash
   # Check if server is accessible
   kubectl get service jit-server -n jit-system
   kubectl get ingress jit-server -n jit-system
   ```

3. **Server not handling requests:**
   ```bash
   # Check server status
   kubectl get pods -l app=jit-server -n jit-system
   kubectl port-forward deployment/jit-server 8080:8080 -n jit-system
   ```

### 3.2 Permission Denied Errors

**Symptoms:**
- Users can't create requests
- Approval commands fail
- RBAC-related error messages

**Diagnosis:**
```bash
# Check user roles configuration
kubectl get configmap slack-user-roles -n jit-system -o yaml

# Check RBAC logs
kubectl logs deployment/jit-server -n jit-system | grep -i rbac
```

**Solutions:**

1. **Update user roles:**
   ```yaml
   # Update ConfigMap
   kubectl patch configmap slack-user-roles -n jit-system --type merge -p '
   data:
     roles.yaml: |
       roles:
         admins: ["U123ADMIN"]
         approvers: ["U456APPROVER"]
         requesters: ["U789USER"]
   '
   ```

2. **Get Slack user IDs:**
   ```bash
   # Use Slack API to get user info
   curl -H "Authorization: Bearer xoxb-your-bot-token" \
        "https://slack.com/api/users.info?user=U123456789"
   ```

## 4. Kubernetes Resource Issues

### 4.1 CRD Installation Problems

**Symptoms:**
- CRDs not found errors
- Schema validation failures
- Version conflicts

**Diagnosis:**
```bash
# Check CRD status
kubectl get crds | grep jit.rebelops.io
kubectl describe crd jitaccessrequests.jit.rebelops.io
```

**Solutions:**

1. **Reinstall CRDs:**
   ```bash
   kubectl delete -f manifests/crds/ --ignore-not-found=true
   kubectl apply -f manifests/crds/
   ```

2. **Check for version conflicts:**
   ```bash
   kubectl get crd jitaccessrequests.jit.rebelops.io -o yaml
   ```

### 4.2 Resource Stuck in Wrong State

**Symptoms:**
- JITAccessRequests stuck in "Pending"
- JITAccessJobs not progressing
- Status not updating

**Diagnosis:**
```bash
# Check resource status and conditions
kubectl describe jitaccessrequest <request-name> -n jit-system
kubectl describe jitaccessjob <job-name> -n jit-system

# Check controller reconciliation
kubectl logs deployment/jit-operator -n jit-system | grep <request-name>
```

**Solutions:**

1. **Force reconciliation:**
   ```bash
   # Add annotation to trigger reconciliation
   kubectl annotate jitaccessrequest <request-name> -n jit-system \
     force-sync="$(date +%s)"
   ```

2. **Restart operator:**
   ```bash
   kubectl rollout restart deployment/jit-operator -n jit-system
   ```

## 5. Performance Issues

### 5.1 Slow Request Processing

**Symptoms:**
- Long delays between request and job creation
- Slow approval processing
- Timeout errors

**Diagnosis:**
```bash
# Check operator resource usage
kubectl top pod -l app.kubernetes.io/name=jit-operator -n jit-system

# Check metrics
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics | grep -E "(http_request_duration|reconcile_duration)"
```

**Solutions:**

1. **Increase resources:**
   ```yaml
   # Update deployment resources
   resources:
     limits:
       cpu: 1000m
       memory: 512Mi
     requests:
       cpu: 200m
       memory: 256Mi
   ```

2. **Tune reconciliation frequency:**
   ```bash
   # Add environment variable to operator
   kubectl set env deployment/jit-operator \
     RECONCILE_PERIOD=30s -n jit-system
   ```

### 5.2 High Memory Usage

**Symptoms:**
- OOMKilled pods
- Memory limit exceeded
- Slow performance

**Diagnosis:**
```bash
# Check memory usage
kubectl top pod -l app.kubernetes.io/name=jit-operator -n jit-system

# Check for memory leaks
kubectl logs deployment/jit-operator -n jit-system | grep -i memory
```

**Solutions:**

1. **Increase memory limits:**
   ```bash
   kubectl patch deployment jit-operator -n jit-system -p '
   spec:
     template:
       spec:
         containers:
         - name: manager
           resources:
             limits:
               memory: 1Gi
   '
   ```

## 6. Cleanup Issues

### 6.1 Resources Not Being Cleaned Up

**Symptoms:**
- Expired access still active
- Secrets not deleted
- AWS access entries remain

**Diagnosis:**
```bash
# Check for expired jobs
kubectl get jitaccessjobs -n jit-system \
  -o custom-columns=NAME:.metadata.name,PHASE:.status.phase,EXPIRES:.status.expiryTime

# Check secrets
kubectl get secrets -l jit.rebelops.io/type -n jit-system
```

**Solutions:**

1. **Force cleanup:**
   ```bash
   # Delete expired job to trigger cleanup
   kubectl delete jitaccessjob <job-name> -n jit-system
   ```

2. **Manual cleanup:**
   ```bash
   # Clean up AWS access entries
   aws eks delete-access-entry \
     --cluster-name cluster-name \
     --principal-arn arn:aws:iam::account:role/JITAccessRole
   
   # Clean up secrets
   kubectl delete secrets -l jit.rebelops.io/type=credentials -n jit-system
   ```

## 7. Development and Testing Issues

### 7.1 Local Development Problems

**Symptoms:**
- Cannot connect to Kubernetes cluster
- Build failures
- Test failures

**Solutions:**

1. **Check kubeconfig:**
   ```bash
   kubectl config current-context
   kubectl cluster-info
   ```

2. **Build issues:**
   ```bash
   # Clean and rebuild
   make clean
   make build-all
   
   # Check Go version
   go version
   ```

3. **Test failures:**
   ```bash
   # Run tests with verbose output
   go test -v ./...
   
   # Check test dependencies
   go mod tidy
   ```

## 8. Monitoring and Observability Issues

### 8.1 Metrics Not Appearing

**Symptoms:**
- Prometheus not scraping JIT Bot metrics
- Missing metrics in Grafana dashboard
- ServiceMonitor not working

**Diagnosis:**
```bash
# Check metrics endpoint
kubectl port-forward deployment/jit-operator 8080:8080 -n jit-system
curl http://localhost:8080/metrics | grep jit_

# Check ServiceMonitor
kubectl get servicemonitor jit-bot-metrics -n jit-system
kubectl describe servicemonitor jit-bot-metrics -n jit-system

# Check if Prometheus is discovering targets
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
# Open http://localhost:9090/targets and look for jit-bot targets
```

**Solutions:**

1. **Fix ServiceMonitor configuration:**
   ```bash
   # Verify service labels match ServiceMonitor selector
   kubectl get service jit-operator -n jit-system --show-labels
   
   # Update ServiceMonitor if needed
   kubectl apply -f config/monitoring/monitoring-stack.yaml
   ```

2. **Check Prometheus RBAC:**
   ```bash
   # Ensure Prometheus can discover ServiceMonitors
   kubectl get clusterrole prometheus-operator
   kubectl get clusterrolebinding prometheus-operator
   ```

### 8.2 OpenTelemetry Tracing Issues

**Symptoms:**
- No traces appearing in Jaeger/OTLP collector
- Tracing instrumentation not working
- Trace spans missing or incomplete

**Diagnosis:**
```bash
# Check tracing configuration
kubectl get deployment jit-operator -n jit-system -o yaml | grep -A 10 -B 5 tracing

# Check OpenTelemetry Collector
kubectl get pods -l app=otel-collector -n jit-system
kubectl logs deployment/otel-collector -n jit-system

# Check trace export in operator logs
kubectl logs deployment/jit-operator -n jit-system | grep -i trace
```

**Solutions:**

1. **Enable tracing on operator:**
   ```bash
   kubectl set env deployment/jit-operator \
     --enable-tracing=true \
     --tracing-exporter=otlp \
     --tracing-endpoint=http://otel-collector.jit-system:4317 \
     -n jit-system
   ```

2. **Fix OTLP Collector configuration:**
   ```bash
   # Check collector configuration
   kubectl get configmap otel-collector-config -n jit-system -o yaml
   
   # Restart collector if needed
   kubectl rollout restart deployment/otel-collector -n jit-system
   ```

3. **Debug trace sampling:**
   ```bash
   # Increase sampling rate for debugging
   kubectl set env deployment/jit-operator ENVIRONMENT=development -n jit-system
   # This sets 10% sampling rate
   ```

### 8.3 Grafana Dashboard Issues

**Symptoms:**
- Dashboard not loading data
- Panels showing "No data"
- Query errors in dashboard

**Diagnosis:**
```bash
# Check Grafana connectivity to Prometheus
kubectl logs deployment/grafana -n monitoring | grep -i prometheus

# Test Prometheus queries manually
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
# Test queries like: up{job="jit-operator"}
```

**Solutions:**

1. **Import dashboard correctly:**
   ```bash
   # Copy dashboard JSON
   cat config/monitoring/grafana-dashboard.json
   
   # Import via Grafana UI:
   # 1. Go to Dashboards → Import
   # 2. Paste JSON content
   # 3. Configure Prometheus data source
   ```

2. **Fix data source configuration:**
   ```yaml
   # Ensure Prometheus data source URL is correct
   # Usually: http://prometheus:9090
   ```

### 8.4 AlertManager Not Firing Alerts

**Symptoms:**
- Critical alerts not triggering
- No notifications in Slack/email
- Alert rules not evaluating

**Diagnosis:**
```bash
# Check PrometheusRule status
kubectl get prometheusrules jit-bot-alerts -n jit-system
kubectl describe prometheusrules jit-bot-alerts -n jit-system

# Check AlertManager configuration
kubectl get configmap alertmanager-jit-bot -n monitoring -o yaml

# Check alert status in Prometheus
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
# Open http://localhost:9090/alerts
```

**Solutions:**

1. **Verify alert rule syntax:**
   ```bash
   # Test alert queries manually
   kubectl port-forward svc/prometheus 9090:9090 -n monitoring
   # Test: increase(jit_privilege_escalation_attempts_total[5m]) > 5
   ```

2. **Check AlertManager routing:**
   ```yaml
   # Verify alert routing configuration
   route:
     group_by: ['alertname', 'component']
     receiver: 'jit-bot-alerts'
     routes:
     - match:
         severity: critical
       receiver: 'critical-alerts'
   ```

### 8.5 High Cardinality Metrics

**Symptoms:**
- Prometheus storage issues
- High memory usage
- Query timeouts

**Diagnosis:**
```bash
# Check metric cardinality
kubectl port-forward svc/prometheus 9090:9090 -n monitoring
# Query: {__name__=~"jit_.*"} 
# Count unique label combinations

# Check Prometheus metrics
curl http://localhost:9090/api/v1/label/__name__/values | grep jit_
```

**Solutions:**

1. **Reduce label cardinality:**
   ```go
   // Avoid user-specific labels in high-frequency metrics
   // Bad: counter.WithLabelValues(userID, cluster, timestamp)
   // Good: counter.WithLabelValues(cluster, environment)
   ```

2. **Configure metric retention:**
   ```yaml
   # Set appropriate retention policy
   prometheus:
     retention: "30d"
     retentionSize: "10GB"
   ```

### 8.6 Monitoring Setup Issues

**Key Prometheus Queries for Troubleshooting:**

```bash
# Request rate monitoring
rate(jit_access_requests_total[5m])

# Request duration percentiles
histogram_quantile(0.95, rate(jit_access_request_duration_seconds_bucket[5m]))

# Error rate monitoring
rate(jit_controller_errors_total[5m])
rate(jit_webhook_validation_errors_total[5m])

# Active sessions monitoring
sum(jit_active_access_sessions) by (cluster)

# Security monitoring
increase(jit_security_violations_total[1h])
increase(jit_privilege_escalation_attempts_total[5m])

# System health monitoring
jit_system_health_status
```

**Key Metrics to Monitor:**

- **Business KPIs**: Request creation rate, approval latency, active sessions
- **Security**: Privilege escalation attempts, security violations, validation errors
- **Performance**: Webhook latency, AWS API response times, processing duration
- **Infrastructure**: System health, error rates, resource usage
- **Availability**: Component uptime, failed reconciliations, timeout errors

## 9. Emergency Procedures

### 9.1 Emergency Access Revocation

```bash
# Immediately revoke all access for a user
kubectl delete jitaccessjobs -l jit.rebelops.io/user=USER_ID -n jit-system

# Remove AWS access entries
aws eks list-access-entries --cluster-name cluster-name
aws eks delete-access-entry --cluster-name cluster-name --principal-arn PRINCIPAL_ARN
```

### 9.2 System Recovery

```bash
# Complete system restart
kubectl delete deployment jit-operator -n jit-system
kubectl delete deployment jit-server -n jit-system
kubectl apply -f manifests/operator/

# Reset all requests (emergency only)
kubectl delete jitaccessrequests --all -n jit-system
kubectl delete jitaccessjobs --all -n jit-system
```

## 10. Getting Help

### 10.1 Gathering Debug Information

```bash
#!/bin/bash
# debug-collect.sh - Collect debug information

echo "=== JIT Access Tool Debug Information ===" > debug-info.txt
echo "Date: $(date)" >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Operator Status ===" >> debug-info.txt
kubectl get pods -n jit-system >> debug-info.txt
echo "" >> debug-info.txt

echo "=== CRDs ===" >> debug-info.txt
kubectl get crds | grep jit.rebelops.io >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Access Requests ===" >> debug-info.txt
kubectl get jitaccessrequests -n jit-system >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Access Jobs ===" >> debug-info.txt
kubectl get jitaccessjobs -n jit-system >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Recent Operator Logs ===" >> debug-info.txt
kubectl logs deployment/jit-operator -n jit-system --tail=100 >> debug-info.txt
echo "" >> debug-info.txt

echo "=== ConfigMaps ===" >> debug-info.txt
kubectl get configmaps -n jit-system >> debug-info.txt
echo "" >> debug-info.txt

echo "Debug information collected in debug-info.txt"
```

## 10. Webhook Issues

### 10.1 Access Request Creation Fails

**Symptoms:**
- JITAccessRequest creation is rejected
- Validation errors when creating resources
- "admission webhook denied the request" errors

**Diagnosis:**
```bash
# Check webhook configurations
kubectl get validatingadmissionwebhook jit-bot-validating-webhook
kubectl get mutatingadmissionwebhook jit-bot-mutating-webhook

# Check webhook service
kubectl get service jit-operator-webhook-service -n jit-system

# Check webhook endpoint connectivity
kubectl exec -it deployment/jit-operator -n jit-system -- netstat -ln | grep 9443

# Check operator logs for webhook errors
kubectl logs deployment/jit-operator -n jit-system | grep webhook
```

**Common Causes & Solutions:**

1. **Certificate issues:**
   ```bash
   # Check certificates
   kubectl get secret jit-operator-webhook-certs -n jit-system
   
   # Regenerate certificates if needed
   scripts/generate-webhook-certs.sh
   ```

2. **Webhook validation errors:**
   ```bash
   # Test with invalid data to see validation in action
   kubectl apply -f - <<EOF
   apiVersion: jit.rebelops.io/v1alpha1
   kind: JITAccessRequest
   metadata:
     name: test-validation
     namespace: jit-system
   spec:
     userID: "invalid"
     userEmail: "not-an-email"
     reason: "too short"
     duration: "999d"
     permissions: ["invalid-permission"]
     targetCluster:
       name: "test"
       awsAccount: "123"
       region: "invalid"
   EOF
   ```

3. **Webhook server not running:**
   ```bash
   # Check if webhook server is listening
   kubectl port-forward deployment/jit-operator 9443:9443 -n jit-system
   
   # Test webhook endpoint
   curl -k https://localhost:9443/validate-jit-rebelops-io-v1alpha1-jitaccessrequest
   ```

### 10.2 Webhook Timeouts

**Symptoms:**
- "context deadline exceeded" errors
- Long delays when creating access requests
- Webhook timeouts in admission review

**Solutions:**

1. **Check webhook performance:**
   ```bash
   # Monitor webhook response times
   kubectl logs deployment/jit-operator -n jit-system | grep "webhook.*duration"
   ```

2. **Increase timeout:**
   ```bash
   # Edit webhook configuration
   kubectl patch validatingadmissionwebhook jit-bot-validating-webhook \
     --type='merge' -p='{"webhooks":[{"name":"jit-bot-validating-webhook","timeoutSeconds":30}]}'
   ```

### 10.3 Auto-Approver Assignment Issues

**Symptoms:**
- Requests not getting expected approvers
- Wrong approval requirements
- Environment detection not working

**Diagnosis:**
```bash
# Check how cluster environments are detected
kubectl get jitaccessrequest <request-name> -n jit-system -o yaml | grep -A10 labels

# Check mutation webhook logs
kubectl logs deployment/jit-operator -n jit-system | grep mutation
```

**Common Issues:**

1. **Cluster name doesn't match environment patterns:**
   - Production: Must contain "prod" or "production"
   - Staging: Must contain "stag" or "staging"
   - Development: Must contain "dev" or "development"

2. **Approver assignment not working:**
   ```bash
   # Manually verify mutation is working
   kubectl apply -f - <<EOF
   apiVersion: jit.rebelops.io/v1alpha1
   kind: JITAccessRequest
   metadata:
     name: test-mutation
     namespace: jit-system
   spec:
     userID: "U123456789A"
     userEmail: "test@company.com"
     reason: "Testing mutation webhook functionality"
     duration: "1h"
     permissions: ["admin"]
     targetCluster:
       name: "prod-east-1"
       awsAccount: "123456789012"
       region: "us-east-1"
   EOF
   
   # Check if approvers were auto-assigned
   kubectl get jitaccessrequest test-mutation -n jit-system -o yaml | grep approvers
   ```

### 10.4 Webhook Certificate Issues

**Symptoms:**
- "x509: certificate signed by unknown authority" errors
- Webhook admission failures
- TLS handshake errors

**Solutions:**

1. **Using cert-manager:**
   ```bash
   # Check cert-manager
   kubectl get pods -n cert-manager
   
   # Check certificate status
   kubectl get certificate jit-operator-serving-cert -n jit-system
   kubectl describe certificate jit-operator-serving-cert -n jit-system
   ```

2. **Manual certificate troubleshooting:**
   ```bash
   # Check certificate validity
   kubectl get secret jit-operator-webhook-certs -n jit-system -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
   
   # Regenerate certificates
   NAMESPACE=jit-system scripts/generate-webhook-certs.sh
   ```

## 11. Support

### 11.1 Support Channels

1. **GitHub Issues**: Report bugs and feature requests
2. **Internal Slack**: `#jit-access-support` channel
3. **Documentation**: Check this troubleshooting guide first
4. **Logs**: Always include relevant logs with support requests

### 11.2 Before Contacting Support

1. Check this troubleshooting guide
2. Verify system requirements
3. Collect debug information
4. Try basic remediation steps
5. Document the exact error messages and steps to reproduce