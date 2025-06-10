package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Access Request Metrics
	accessRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_access_requests_total",
			Help: "Total number of JIT access requests created",
		},
		[]string{"cluster", "user", "environment", "permissions"},
	)

	accessRequestsApproved = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_access_requests_approved_total",
			Help: "Total number of JIT access requests approved",
		},
		[]string{"cluster", "user", "environment", "approver"},
	)

	accessRequestsDenied = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_access_requests_denied_total",
			Help: "Total number of JIT access requests denied",
		},
		[]string{"cluster", "user", "environment", "reason"},
	)

	accessRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_access_request_duration_seconds",
			Help:    "Time from request creation to approval/denial",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
		},
		[]string{"cluster", "environment", "status"},
	)

	// Active Access Metrics
	activeAccessSessions = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jit_active_access_sessions",
			Help: "Number of currently active JIT access sessions",
		},
		[]string{"cluster", "environment", "permission_level"},
	)

	accessSessionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_access_session_duration_seconds",
			Help:    "Duration of completed access sessions",
			Buckets: prometheus.ExponentialBuckets(60, 2, 12), // 1min to ~68hrs
		},
		[]string{"cluster", "environment", "permissions"},
	)

	// Webhook Metrics
	webhookRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_webhook_requests_total",
			Help: "Total number of webhook requests processed",
		},
		[]string{"webhook_type", "operation", "status"},
	)

	webhookRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_webhook_request_duration_seconds",
			Help:    "Duration of webhook request processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"webhook_type", "operation"},
	)

	webhookValidationErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_webhook_validation_errors_total",
			Help: "Total number of webhook validation errors",
		},
		[]string{"webhook_type", "error_type", "field"},
	)

	// AWS Integration Metrics
	awsApiCalls = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_aws_api_calls_total",
			Help: "Total number of AWS API calls made",
		},
		[]string{"service", "operation", "status", "region"},
	)

	awsApiDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_aws_api_duration_seconds",
			Help:    "Duration of AWS API calls",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "operation", "region"},
	)

	awsApiErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_aws_api_errors_total",
			Help: "Total number of AWS API errors",
		},
		[]string{"service", "operation", "error_code", "region"},
	)

	// Slack Integration Metrics
	slackCommandsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_slack_commands_total",
			Help: "Total number of Slack commands processed",
		},
		[]string{"command", "user", "channel", "status"},
	)

	slackCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_slack_command_duration_seconds",
			Help:    "Duration of Slack command processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	slackApiErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_slack_api_errors_total",
			Help: "Total number of Slack API errors",
		},
		[]string{"operation", "error_type"},
	)

	// Controller Metrics
	controllerReconcileTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_controller_reconcile_total",
			Help: "Total number of controller reconciliation attempts",
		},
		[]string{"controller", "result"},
	)

	controllerReconcileDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "jit_controller_reconcile_duration_seconds",
			Help:    "Duration of controller reconciliation",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller"},
	)

	controllerErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_controller_errors_total",
			Help: "Total number of controller errors",
		},
		[]string{"controller", "error_type"},
	)

	// Security Metrics
	securityViolationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_security_violations_total",
			Help: "Total number of security violations detected",
		},
		[]string{"violation_type", "user", "cluster"},
	)

	privilegeEscalationAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "jit_privilege_escalation_attempts_total",
			Help: "Total number of privilege escalation attempts",
		},
		[]string{"user", "from_permission", "to_permission", "cluster"},
	)

	// System Health Metrics
	systemHealthStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "jit_system_health_status",
			Help: "System health status (1=healthy, 0=unhealthy)",
		},
		[]string{"component"},
	)

	lastSuccessfulBackup = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "jit_last_successful_backup_timestamp",
			Help: "Timestamp of last successful backup",
		},
	)
)

func init() {
	// Register metrics with controller-runtime
	metrics.Registry.MustRegister(
		accessRequestsTotal,
		accessRequestsApproved,
		accessRequestsDenied,
		accessRequestDuration,
		activeAccessSessions,
		accessSessionDuration,
		webhookRequestsTotal,
		webhookRequestDuration,
		webhookValidationErrors,
		awsApiCalls,
		awsApiDuration,
		awsApiErrors,
		slackCommandsTotal,
		slackCommandDuration,
		slackApiErrors,
		controllerReconcileTotal,
		controllerReconcileDuration,
		controllerErrors,
		securityViolationsTotal,
		privilegeEscalationAttempts,
		systemHealthStatus,
		lastSuccessfulBackup,
	)
}

// Access Request Metrics Functions

func RecordAccessRequest(cluster, user, environment string, permissions []string) {
	permList := joinPermissions(permissions)
	accessRequestsTotal.WithLabelValues(cluster, user, environment, permList).Inc()
}

func RecordAccessRequestApproval(cluster, user, environment, approver string, requestTime time.Time) {
	accessRequestsApproved.WithLabelValues(cluster, user, environment, approver).Inc()
	accessRequestDuration.WithLabelValues(cluster, environment, "approved").Observe(time.Since(requestTime).Seconds())
}

func RecordAccessRequestDenial(cluster, user, environment, reason string, requestTime time.Time) {
	accessRequestsDenied.WithLabelValues(cluster, user, environment, reason).Inc()
	accessRequestDuration.WithLabelValues(cluster, environment, "denied").Observe(time.Since(requestTime).Seconds())
}

func SetActiveAccessSessions(cluster, environment, permissionLevel string, count int) {
	activeAccessSessions.WithLabelValues(cluster, environment, permissionLevel).Set(float64(count))
}

func RecordAccessSessionCompletion(cluster, environment string, permissions []string, duration time.Duration) {
	permList := joinPermissions(permissions)
	accessSessionDuration.WithLabelValues(cluster, environment, permList).Observe(duration.Seconds())
}

// Webhook Metrics Functions

func RecordWebhookRequest(webhookType, operation, status string, duration time.Duration) {
	webhookRequestsTotal.WithLabelValues(webhookType, operation, status).Inc()
	webhookRequestDuration.WithLabelValues(webhookType, operation).Observe(duration.Seconds())
}

func RecordWebhookValidationError(webhookType, errorType, field string) {
	webhookValidationErrors.WithLabelValues(webhookType, errorType, field).Inc()
}

// AWS Metrics Functions

func RecordAWSAPICall(service, operation, status, region string, duration time.Duration) {
	awsApiCalls.WithLabelValues(service, operation, status, region).Inc()
	awsApiDuration.WithLabelValues(service, operation, region).Observe(duration.Seconds())
}

func RecordAWSAPIError(service, operation, errorCode, region string) {
	awsApiErrors.WithLabelValues(service, operation, errorCode, region).Inc()
}

// Slack Metrics Functions

func RecordSlackCommand(command, user, channel, status string, duration time.Duration) {
	slackCommandsTotal.WithLabelValues(command, user, channel, status).Inc()
	slackCommandDuration.WithLabelValues(command).Observe(duration.Seconds())
}

func RecordSlackAPIError(operation, errorType string) {
	slackApiErrors.WithLabelValues(operation, errorType).Inc()
}

// Controller Metrics Functions

func RecordControllerReconcile(controller, result string, duration time.Duration) {
	controllerReconcileTotal.WithLabelValues(controller, result).Inc()
	controllerReconcileDuration.WithLabelValues(controller).Observe(duration.Seconds())
}

func RecordControllerError(controller, errorType string) {
	controllerErrors.WithLabelValues(controller, errorType).Inc()
}

// Security Metrics Functions

func RecordSecurityViolation(violationType, user, cluster string) {
	securityViolationsTotal.WithLabelValues(violationType, user, cluster).Inc()
}

func RecordPrivilegeEscalationAttempt(user, fromPerm, toPerm, cluster string) {
	privilegeEscalationAttempts.WithLabelValues(user, fromPerm, toPerm, cluster).Inc()
}

// System Health Functions

func SetSystemHealthStatus(component string, healthy bool) {
	status := 0.0
	if healthy {
		status = 1.0
	}
	systemHealthStatus.WithLabelValues(component).Set(status)
}

func SetLastSuccessfulBackup(timestamp time.Time) {
	lastSuccessfulBackup.Set(float64(timestamp.Unix()))
}

// Helper Functions

func joinPermissions(permissions []string) string {
	if len(permissions) == 0 {
		return "none"
	}
	result := ""
	for i, perm := range permissions {
		if i > 0 {
			result += ","
		}
		result += perm
	}
	return result
}

// Metric Collection for Dashboard

type MetricsSummary struct {
	TotalRequests       int64            `json:"total_requests"`
	ActiveSessions      int64            `json:"active_sessions"`
	ApprovalRate        float64          `json:"approval_rate"`
	AverageApprovalTime float64          `json:"avg_approval_time_seconds"`
	TopClusters         []ClusterMetrics `json:"top_clusters"`
	SecurityAlerts      int64            `json:"security_alerts"`
	SystemHealth        map[string]bool  `json:"system_health"`
}

type ClusterMetrics struct {
	Name           string  `json:"name"`
	RequestCount   int64   `json:"request_count"`
	ActiveSessions int64   `json:"active_sessions"`
	ApprovalRate   float64 `json:"approval_rate"`
}
