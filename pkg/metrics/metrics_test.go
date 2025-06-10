package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordAccessRequest(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	tests := []struct {
		name        string
		cluster     string
		user        string
		environment string
		permissions []string
		expectCount int
	}{
		{
			name:        "single request",
			cluster:     "prod-east-1",
			user:        "U123456789A",
			environment: "production",
			permissions: []string{"edit"},
			expectCount: 1,
		},
		{
			name:        "multiple permissions",
			cluster:     "staging-west-2",
			user:        "U987654321B",
			environment: "staging",
			permissions: []string{"view", "logs"},
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics before each test case
			resetMetrics()
			RecordAccessRequest(tt.cluster, tt.user, tt.environment, tt.permissions)

			// Check that the metric was incremented
			metricName := "jit_access_requests_total"
			expected := `
				# HELP jit_access_requests_total Total number of JIT access requests created
				# TYPE jit_access_requests_total counter
				jit_access_requests_total{cluster="` + tt.cluster + `",environment="` + tt.environment + `",permissions="` + joinPermissions(tt.permissions) + `",user="` + tt.user + `"} ` + string(rune(tt.expectCount+48)) + `
			`
			err := testutil.CollectAndCompare(accessRequestsTotal, strings.NewReader(expected), metricName)
			assert.NoError(t, err)
		})
	}
}

func TestRecordAccessRequestApproval(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	tests := []struct {
		name        string
		cluster     string
		user        string
		environment string
		approver    string
		expectCount int
	}{
		{
			name:        "single approval",
			cluster:     "prod-east-1",
			user:        "U123456789A",
			environment: "production",
			approver:    "U456789012C",
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordAccessRequestApproval(tt.cluster, tt.user, tt.environment, tt.approver, time.Now())

			metricName := "jit_access_requests_approved_total"
			expected := `
				# HELP jit_access_requests_approved_total Total number of JIT access requests approved
				# TYPE jit_access_requests_approved_total counter
				jit_access_requests_approved_total{approver="` + tt.approver + `",cluster="` + tt.cluster + `",environment="` + tt.environment + `",user="` + tt.user + `"} ` + string(rune(tt.expectCount+48)) + `
			`
			err := testutil.CollectAndCompare(accessRequestsApproved, strings.NewReader(expected), metricName)
			assert.NoError(t, err)
		})
	}
}

func TestRecordAccessRequestDenial(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	cluster := "prod-east-1"
	user := "U123456789A"
	environment := "production"
	reason := "insufficient-permissions"

	RecordAccessRequestDenial(cluster, user, environment, reason, time.Now())

	metricName := "jit_access_requests_denied_total"
	expected := `
		# HELP jit_access_requests_denied_total Total number of JIT access requests denied
		# TYPE jit_access_requests_denied_total counter
		jit_access_requests_denied_total{cluster="` + cluster + `",environment="` + environment + `",reason="` + reason + `",user="` + user + `"} 1
	`
	err := testutil.CollectAndCompare(accessRequestsDenied, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestSetActiveAccessSessions(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	tests := []struct {
		name            string
		cluster         string
		environment     string
		permissionLevel string
		count           int
	}{
		{
			name:            "set active sessions",
			cluster:         "prod-east-1",
			environment:     "production",
			permissionLevel: "edit",
			count:           5,
		},
		{
			name:            "update active sessions",
			cluster:         "staging-west-2",
			environment:     "staging",
			permissionLevel: "view",
			count:           3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset metrics before each test case
			resetMetrics()
			SetActiveAccessSessions(tt.cluster, tt.environment, tt.permissionLevel, tt.count)

			metricName := "jit_active_access_sessions"
			expected := `
				# HELP jit_active_access_sessions Number of currently active JIT access sessions
				# TYPE jit_active_access_sessions gauge
				jit_active_access_sessions{cluster="` + tt.cluster + `",environment="` + tt.environment + `",permission_level="` + tt.permissionLevel + `"} ` + string(rune(tt.count+48)) + `
			`
			err := testutil.CollectAndCompare(activeAccessSessions, strings.NewReader(expected), metricName)
			assert.NoError(t, err)
		})
	}
}

func TestRecordWebhookRequest(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	webhookType := "validating"
	operation := "validate"
	status := "success"

	RecordWebhookRequest(webhookType, operation, status, time.Millisecond*100)

	metricName := "jit_webhook_requests_total"
	expected := `
		# HELP jit_webhook_requests_total Total number of webhook requests processed
		# TYPE jit_webhook_requests_total counter
		jit_webhook_requests_total{operation="` + operation + `",status="` + status + `",webhook_type="` + webhookType + `"} 1
	`
	err := testutil.CollectAndCompare(webhookRequestsTotal, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordWebhookValidationError(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	webhookType := "validating"
	errorType := "invalid_duration"
	field := "duration"

	RecordWebhookValidationError(webhookType, errorType, field)

	metricName := "jit_webhook_validation_errors_total"
	expected := `
		# HELP jit_webhook_validation_errors_total Total number of webhook validation errors
		# TYPE jit_webhook_validation_errors_total counter
		jit_webhook_validation_errors_total{error_type="` + errorType + `",field="` + field + `",webhook_type="` + webhookType + `"} 1
	`
	err := testutil.CollectAndCompare(webhookValidationErrors, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordAWSAPICall(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	service := "eks"
	operation := "describe_cluster"
	status := "success"
	region := "us-east-1"

	RecordAWSAPICall(service, operation, status, region, time.Millisecond*500)

	metricName := "jit_aws_api_calls_total"
	expected := `
		# HELP jit_aws_api_calls_total Total number of AWS API calls made
		# TYPE jit_aws_api_calls_total counter
		jit_aws_api_calls_total{operation="` + operation + `",region="` + region + `",service="` + service + `",status="` + status + `"} 1
	`
	err := testutil.CollectAndCompare(awsApiCalls, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordAWSAPIError(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	service := "sts"
	operation := "assume_role"
	errorCode := "AccessDenied"
	region := "us-east-1"

	RecordAWSAPIError(service, operation, errorCode, region)

	metricName := "jit_aws_api_errors_total"
	expected := `
		# HELP jit_aws_api_errors_total Total number of AWS API errors
		# TYPE jit_aws_api_errors_total counter
		jit_aws_api_errors_total{error_code="` + errorCode + `",operation="` + operation + `",region="` + region + `",service="` + service + `"} 1
	`
	err := testutil.CollectAndCompare(awsApiErrors, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordSlackCommand(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	command := "request"
	user := "U123456789A"
	channel := "C123456789"
	status := "success"

	RecordSlackCommand(command, user, channel, status, time.Millisecond*200)

	metricName := "jit_slack_commands_total"
	expected := `
		# HELP jit_slack_commands_total Total number of Slack commands processed
		# TYPE jit_slack_commands_total counter
		jit_slack_commands_total{channel="` + channel + `",command="` + command + `",status="` + status + `",user="` + user + `"} 1
	`
	err := testutil.CollectAndCompare(slackCommandsTotal, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordSlackAPIError(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	operation := "chat.postMessage"
	errorType := "rate_limited"

	RecordSlackAPIError(operation, errorType)

	metricName := "jit_slack_api_errors_total"
	expected := `
		# HELP jit_slack_api_errors_total Total number of Slack API errors
		# TYPE jit_slack_api_errors_total counter
		jit_slack_api_errors_total{error_type="` + errorType + `",operation="` + operation + `"} 1
	`
	err := testutil.CollectAndCompare(slackApiErrors, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordSecurityViolation(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	violationType := "privilege_escalation"
	user := "U123456789A"
	cluster := "prod-east-1"

	RecordSecurityViolation(violationType, user, cluster)

	metricName := "jit_security_violations_total"
	expected := `
		# HELP jit_security_violations_total Total number of security violations detected
		# TYPE jit_security_violations_total counter
		jit_security_violations_total{cluster="` + cluster + `",user="` + user + `",violation_type="` + violationType + `"} 1
	`
	err := testutil.CollectAndCompare(securityViolationsTotal, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordPrivilegeEscalationAttempt(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	user := "U123456789A"
	fromPerm := "edit"
	toPerm := "cluster-admin"
	cluster := "prod-east-1"

	RecordPrivilegeEscalationAttempt(user, fromPerm, toPerm, cluster)

	metricName := "jit_privilege_escalation_attempts_total"
	expected := `
		# HELP jit_privilege_escalation_attempts_total Total number of privilege escalation attempts
		# TYPE jit_privilege_escalation_attempts_total counter
		jit_privilege_escalation_attempts_total{cluster="` + cluster + `",from_permission="` + fromPerm + `",to_permission="` + toPerm + `",user="` + user + `"} 1
	`
	err := testutil.CollectAndCompare(privilegeEscalationAttempts, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordControllerReconcile(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	controller := "JITAccessRequest"
	result := "success"
	duration := time.Millisecond * 150

	RecordControllerReconcile(controller, result, duration)

	metricName := "jit_controller_reconcile_total"
	expected := `
		# HELP jit_controller_reconcile_total Total number of controller reconciliation attempts
		# TYPE jit_controller_reconcile_total counter
		jit_controller_reconcile_total{controller="` + controller + `",result="` + result + `"} 1
	`
	err := testutil.CollectAndCompare(controllerReconcileTotal, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestRecordControllerError(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	controller := "JITAccessRequest"
	errorType := "reconcile_failed"

	RecordControllerError(controller, errorType)

	metricName := "jit_controller_errors_total"
	expected := `
		# HELP jit_controller_errors_total Total number of controller errors
		# TYPE jit_controller_errors_total counter
		jit_controller_errors_total{controller="` + controller + `",error_type="` + errorType + `"} 1
	`
	err := testutil.CollectAndCompare(controllerErrors, strings.NewReader(expected), metricName)
	assert.NoError(t, err)
}

func TestSetSystemHealthStatus(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	tests := []struct {
		name      string
		component string
		healthy   bool
		expected  float64
	}{
		{
			name:      "healthy component",
			component: "webhook",
			healthy:   true,
			expected:  1.0,
		},
		{
			name:      "unhealthy component",
			component: "aws",
			healthy:   false,
			expected:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetSystemHealthStatus(tt.component, tt.healthy)

			gauge := systemHealthStatus.WithLabelValues(tt.component)
			metric := &dto.Metric{}
			err := gauge.Write(metric)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, metric.GetGauge().GetValue())
		})
	}
}

func TestJoinPermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		expected    string
	}{
		{
			name:        "single permission",
			permissions: []string{"view"},
			expected:    "view",
		},
		{
			name:        "multiple permissions",
			permissions: []string{"view", "edit", "admin"},
			expected:    "view,edit,admin",
		},
		{
			name:        "empty permissions",
			permissions: []string{},
			expected:    "none",
		},
		{
			name:        "nil permissions",
			permissions: nil,
			expected:    "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinPermissions(tt.permissions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered
	// This is implicitly tested by the other tests, but we can also check explicitly

	registry := prometheus.NewRegistry()
	
	// Register all metrics with the test registry
	err := registry.Register(accessRequestsTotal)
	assert.NoError(t, err, "accessRequestsTotal should register successfully")

	err = registry.Register(accessRequestsApproved)
	assert.NoError(t, err, "accessRequestsApproved should register successfully")

	err = registry.Register(activeAccessSessions)
	assert.NoError(t, err, "activeAccessSessions should register successfully")

	// Try to register the same metric again - should fail
	err = registry.Register(accessRequestsTotal)
	assert.Error(t, err, "registering the same metric twice should fail")
}

func TestMetricsConcurrency(t *testing.T) {
	// Reset metrics before test
	resetMetrics()

	// Test concurrent access to metrics
	const numGoroutines = 10
	const numIncrements = 100

	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < numIncrements; j++ {
				RecordAccessRequest("test-cluster", "test-user", "test-env", []string{"view"})
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Check that the metric has the expected value
	counter := accessRequestsTotal.WithLabelValues("test-cluster", "test-user", "test-env", "view")
	metric := &dto.Metric{}
	err := counter.Write(metric)
	require.NoError(t, err)

	expected := float64(numGoroutines * numIncrements)
	assert.Equal(t, expected, metric.GetCounter().GetValue())
}

// resetMetrics is a helper function to reset all metrics for testing
// In a real implementation, you might want to create new instances or use a testing registry
func resetMetrics() {
	// Reset all counters and gauges to zero
	// This is a simplified approach for testing
	accessRequestsTotal.Reset()
	accessRequestsApproved.Reset()
	accessRequestsDenied.Reset()
	activeAccessSessions.Reset()
	accessRequestDuration.Reset()
	webhookRequestsTotal.Reset()
	webhookRequestDuration.Reset()
	webhookValidationErrors.Reset()
	awsApiCalls.Reset()
	awsApiDuration.Reset()
	awsApiErrors.Reset()
	slackCommandsTotal.Reset()
	slackCommandDuration.Reset()
	slackApiErrors.Reset()
	securityViolationsTotal.Reset()
	privilegeEscalationAttempts.Reset()
	controllerReconcileTotal.Reset()
	controllerReconcileDuration.Reset()
	controllerErrors.Reset()
	systemHealthStatus.Reset()
}