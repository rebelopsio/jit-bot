package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/rebelopsio/jit-bot/pkg/metrics"
	"github.com/rebelopsio/jit-bot/pkg/telemetry"
)

const (
	statusSuccess = "success"
	statusError   = "error"
)

var logger = log.Log.WithName("monitoring")

// Config holds monitoring configuration
type Config struct {
	MetricsEnabled bool                    `yaml:"metrics" json:"metrics"`
	MetricsPort    int                     `yaml:"metricsPort" json:"metricsPort"`
	HealthPort     int                     `yaml:"healthPort" json:"healthPort"`
	Tracing        telemetry.TracingConfig `yaml:"tracing" json:"tracing"`
}

// Monitor manages metrics and tracing
type Monitor struct {
	config         Config
	tracerProvider *trace.TracerProvider
	metricsServer  *http.Server
	healthServer   *http.Server
}

// NewMonitor creates a new monitoring instance
func NewMonitor(config Config) *Monitor {
	return &Monitor{
		config: config,
	}
}

// Start initializes and starts monitoring services
func (m *Monitor) Start(ctx context.Context) error {
	// Initialize tracing
	if m.config.Tracing.Enabled {
		tp, err := telemetry.InitTracing(ctx, m.config.Tracing)
		if err != nil {
			return fmt.Errorf("failed to initialize tracing: %w", err)
		}
		m.tracerProvider = tp
		logger.Info("Tracing initialized", "exporter", m.config.Tracing.Exporter, "endpoint", m.config.Tracing.Endpoint)
	}

	// Start metrics server
	if m.config.MetricsEnabled {
		if err := m.startMetricsServer(); err != nil {
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
		logger.Info("Metrics server started", "port", m.config.MetricsPort)
	}

	// Start health server
	if err := m.startHealthServer(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}
	logger.Info("Health server started", "port", m.config.HealthPort)

	// Initialize system health checks
	m.initializeHealthChecks()

	return nil
}

// Stop gracefully shuts down monitoring services
func (m *Monitor) Stop(ctx context.Context) error {
	var errs []error

	// Shutdown metrics server
	if m.metricsServer != nil {
		if err := m.metricsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("metrics server shutdown failed: %w", err))
		}
	}

	// Shutdown health server
	if m.healthServer != nil {
		if err := m.healthServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("health server shutdown failed: %w", err))
		}
	}

	// Shutdown tracer provider
	if m.tracerProvider != nil {
		if err := m.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown failed: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("monitoring shutdown errors: %v", errs)
	}

	return nil
}

func (m *Monitor) startMetricsServer() error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	m.metricsServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", m.config.MetricsPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := m.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Metrics server failed")
		}
	}()

	return nil
}

func (m *Monitor) startHealthServer() error {
	mux := http.NewServeMux()

	// Add health check endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			logger.Error(err, "Failed to write health check response")
		}
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Check if all components are ready
		if m.isSystemReady() {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("ready")); err != nil {
				logger.Error(err, "Failed to write readiness response")
			}
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err := w.Write([]byte("not ready")); err != nil {
				logger.Error(err, "Failed to write not ready response")
			}
		}
	})

	mux.HandleFunc("/livez", func(w http.ResponseWriter, r *http.Request) {
		// Check if system is alive
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("alive")); err != nil {
			logger.Error(err, "Failed to write liveness response")
		}
	})

	m.healthServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", m.config.HealthPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if err := m.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Health server failed")
		}
	}()

	return nil
}

func (m *Monitor) initializeHealthChecks() {
	// Set initial health status
	metrics.SetSystemHealthStatus("operator", true)
	metrics.SetSystemHealthStatus("webhook", true)
	metrics.SetSystemHealthStatus("aws", true)
	metrics.SetSystemHealthStatus("slack", true)
}

func (m *Monitor) isSystemReady() bool {
	// Implement system readiness checks
	// This could check database connections, external services, etc.
	return true
}

// Monitoring wrapper functions that combine metrics and tracing

// TrackAccessRequest tracks an access request operation with both metrics and tracing
func (m *Monitor) TrackAccessRequest(ctx context.Context, operation, userID, cluster, environment string, permissions []string, fn func(context.Context) error) error {
	// Record metrics
	metrics.RecordAccessRequest(cluster, userID, environment, permissions)

	// Add tracing
	return telemetry.InstrumentAccessRequest(ctx, operation, userID, cluster, fn)
}

// TrackWebhookRequest tracks a webhook request with both metrics and tracing
func (m *Monitor) TrackWebhookRequest(ctx context.Context, webhookType, operation string, fn func(context.Context) error) error {
	start := time.Now()

	// Add tracing
	err := telemetry.InstrumentWebhook(ctx, webhookType, operation, fn)

	// Record metrics
	status := statusSuccess
	if err != nil {
		status = statusError
	}
	metrics.RecordWebhookRequest(webhookType, operation, status, time.Since(start))

	return err
}

// TrackAWSCall tracks an AWS API call with both metrics and tracing
func (m *Monitor) TrackAWSCall(ctx context.Context, service, operation, region string, fn func(context.Context) error) error {
	start := time.Now()

	// Add tracing
	err := telemetry.InstrumentAWSCall(ctx, service, operation, region, fn)

	// Record metrics
	status := statusSuccess
	if err != nil {
		status = statusError
		// Try to extract AWS error code
		// This would need AWS SDK specific error handling
		metrics.RecordAWSAPIError(service, operation, "unknown", region)
	}
	metrics.RecordAWSAPICall(service, operation, status, region, time.Since(start))

	return err
}

// TrackSlackCommand tracks a Slack command with both metrics and tracing
func (m *Monitor) TrackSlackCommand(ctx context.Context, command, userID, channelID string, fn func(context.Context) error) error {
	start := time.Now()

	// Add tracing
	err := telemetry.InstrumentSlackCommand(ctx, command, fn)

	// Record metrics
	status := statusSuccess
	if err != nil {
		status = statusError
	}
	metrics.RecordSlackCommand(command, userID, channelID, status, time.Since(start))

	return err
}

// TrackControllerReconcile tracks a controller reconciliation with both metrics and tracing
func (m *Monitor) TrackControllerReconcile(ctx context.Context, controller string, fn func(context.Context) error) error {
	start := time.Now()

	// Add tracing
	ctx, span := telemetry.StartControllerSpan(ctx, controller, "reconcile")
	defer span.End()

	err := fn(ctx)
	duration := time.Since(start)

	// Record metrics
	result := "success"
	if err != nil {
		result = "error"
		telemetry.SetSpanStatus(span, err, "Reconciliation failed")
		metrics.RecordControllerError(controller, "reconcile_error")
	} else {
		telemetry.SetSpanStatus(span, nil, "Reconciliation succeeded")
	}

	metrics.RecordControllerReconcile(controller, result, duration)

	return err
}

// Health check functions

func (m *Monitor) SetComponentHealth(component string, healthy bool) {
	metrics.SetSystemHealthStatus(component, healthy)
}

func (m *Monitor) RecordSecurityViolation(violationType, userID, cluster string) {
	metrics.RecordSecurityViolation(violationType, userID, cluster)
}

func (m *Monitor) RecordPrivilegeEscalation(userID, fromPerm, toPerm, cluster string) {
	metrics.RecordPrivilegeEscalationAttempt(userID, fromPerm, toPerm, cluster)
}

// Instrumentation middleware for HTTP handlers

func (m *Monitor) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Extract trace context from headers
		ctx := telemetry.ExtractTraceContext(r.Context(), extractHeadersMap(r.Header))
		r = r.WithContext(ctx)

		// Start span
		ctx, span := telemetry.StartSpan(ctx, fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path),
			attribute.String("http.method", r.Method),
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.user_agent", r.UserAgent()),
		)
		defer span.End()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}

		// Call next handler
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Record metrics and span attributes
		duration := time.Since(start)
		span.SetAttributes(
			attribute.Int("http.status_code", wrapped.statusCode),
			attribute.Int64("http.duration_ms", duration.Milliseconds()),
		)

		if wrapped.statusCode >= 400 {
			telemetry.SetSpanStatus(span, nil, fmt.Sprintf("HTTP %d", wrapped.statusCode))
		} else {
			telemetry.SetSpanStatus(span, nil, "HTTP request completed")
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func extractHeadersMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// Default configurations

func DefaultConfig() Config {
	return Config{
		MetricsEnabled: true,
		MetricsPort:    8080,
		HealthPort:     8081,
		Tracing: telemetry.TracingConfig{
			Enabled:     false, // Disabled by default
			Exporter:    "jaeger",
			Endpoint:    "",
			ServiceName: "jit-bot",
			Environment: "development",
			SampleRate:  0.1,
		},
	}
}

func ProductionConfig() Config {
	return Config{
		MetricsEnabled: true,
		MetricsPort:    8080,
		HealthPort:     8081,
		Tracing: telemetry.TracingConfig{
			Enabled:     true,
			Exporter:    "otlp",
			Endpoint:    "", // Will use environment variables
			ServiceName: "jit-bot",
			Environment: "production",
			SampleRate:  0.05, // Lower sampling in production
		},
	}
}
