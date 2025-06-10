package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	serviceName    = "jit-bot"
	serviceVersion = "1.0.0"
)

var (
	tracer oteltrace.Tracer
)

// TracingConfig holds configuration for tracing
type TracingConfig struct {
	Enabled     bool    `yaml:"enabled" json:"enabled"`
	Exporter    string  `yaml:"exporter" json:"exporter"` // "jaeger", "otlp", "console"
	Endpoint    string  `yaml:"endpoint" json:"endpoint"`
	ServiceName string  `yaml:"serviceName" json:"serviceName"`
	Environment string  `yaml:"environment" json:"environment"`
	SampleRate  float64 `yaml:"sampleRate" json:"sampleRate"`
}

// InitTracing initializes OpenTelemetry tracing
func InitTracing(ctx context.Context, config TracingConfig) (*trace.TracerProvider, error) {
	if !config.Enabled {
		// Return a no-op tracer provider
		tp := trace.NewTracerProvider()
		otel.SetTracerProvider(tp)
		tracer = tp.Tracer(serviceName)
		return tp, nil
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(getServiceName(config.ServiceName)),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(getEnvironment(config.Environment)),
			attribute.String("component", "jit-bot"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter trace.SpanExporter
	switch config.Exporter {
	case "jaeger":
		exporter, err = createJaegerExporter(config.Endpoint)
	case "otlp":
		exporter, err = createOTLPExporter(ctx, config.Endpoint)
	default:
		return nil, fmt.Errorf("unsupported exporter: %s", config.Exporter)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create tracer provider
	sampleRate := config.SampleRate
	if sampleRate <= 0 {
		sampleRate = 0.1 // Default 10% sampling
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.TraceIDRatioBased(sampleRate)),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Create tracer instance
	tracer = tp.Tracer(serviceName)

	return tp, nil
}

func createJaegerExporter(endpoint string) (trace.SpanExporter, error) {
	// Jaeger exporter is deprecated, using OTLP instead
	return createOTLPExporter(context.Background(), endpoint)
}

func createOTLPExporter(ctx context.Context, endpoint string) (trace.SpanExporter, error) {
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	return otlptrace.New(ctx, otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	))
}

func getServiceName(configName string) string {
	if configName != "" {
		return configName
	}
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return serviceName
}

func getEnvironment(configEnv string) string {
	if configEnv != "" {
		return configEnv
	}
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	return "development"
}

// Span helper functions

// StartSpan starts a new span with the given name and attributes
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, name, oteltrace.WithAttributes(attrs...))
}

// StartAccessRequestSpan starts a span for access request operations
func StartAccessRequestSpan(ctx context.Context, operation string, userID, cluster string) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, fmt.Sprintf("access_request.%s", operation),
		oteltrace.WithAttributes(
			attribute.String("jit.operation", operation),
			attribute.String("jit.user_id", userID),
			attribute.String("jit.cluster", cluster),
			attribute.String("jit.component", "access-request"),
		),
	)
}

// StartWebhookSpan starts a span for webhook operations
func StartWebhookSpan(ctx context.Context, webhookType, operation string) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, fmt.Sprintf("webhook.%s.%s", webhookType, operation),
		oteltrace.WithAttributes(
			attribute.String("jit.webhook_type", webhookType),
			attribute.String("jit.operation", operation),
			attribute.String("jit.component", "webhook"),
		),
	)
}

// StartAWSSpan starts a span for AWS operations
func StartAWSSpan(ctx context.Context, service, operation, region string) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, fmt.Sprintf("aws.%s.%s", service, operation),
		oteltrace.WithAttributes(
			attribute.String("aws.service", service),
			attribute.String("aws.operation", operation),
			attribute.String("aws.region", region),
			attribute.String("jit.component", "aws-integration"),
		),
	)
}

// StartSlackSpan starts a span for Slack operations
func StartSlackSpan(ctx context.Context, operation, command string) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, fmt.Sprintf("slack.%s", operation),
		oteltrace.WithAttributes(
			attribute.String("slack.operation", operation),
			attribute.String("slack.command", command),
			attribute.String("jit.component", "slack-integration"),
		),
	)
}

// StartControllerSpan starts a span for controller operations
func StartControllerSpan(ctx context.Context, controller, operation string) (context.Context, oteltrace.Span) {
	return tracer.Start(ctx, fmt.Sprintf("controller.%s.%s", controller, operation),
		oteltrace.WithAttributes(
			attribute.String("k8s.controller", controller),
			attribute.String("k8s.operation", operation),
			attribute.String("jit.component", "controller"),
		),
	)
}

// Span event and status helper functions

// AddSpanEvent adds an event to the current span
func AddSpanEvent(span oteltrace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, oteltrace.WithAttributes(attrs...))
}

// SetSpanStatus sets the status of a span
func SetSpanStatus(span oteltrace.Span, err error, message string) {
	if err != nil {
		span.SetStatus(codes.Error, message)
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, message)
	}
}

// SetSpanAttributes sets attributes on a span
func SetSpanAttributes(span oteltrace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// Common attribute helpers

func UserAttributes(userID, userEmail string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("user.email", userEmail),
	}
}

func ClusterAttributes(cluster, account, region string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("aws.cluster", cluster),
		attribute.String("aws.account", account),
		attribute.String("aws.region", region),
	}
}

func PermissionAttributes(permissions []string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.StringSlice("jit.permissions", permissions),
		attribute.Int("jit.permission_count", len(permissions)),
	}
}

func RequestAttributes(requestID, reason, duration string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("jit.request_id", requestID),
		attribute.String("jit.reason", reason),
		attribute.String("jit.duration", duration),
	}
}

// Instrumentation helpers for common operations

// InstrumentAccessRequest wraps access request operations with tracing
func InstrumentAccessRequest(ctx context.Context, operation, userID, cluster string, fn func(context.Context) error) error {
	ctx, span := StartAccessRequestSpan(ctx, operation, userID, cluster)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	// Add timing
	span.SetAttributes(attribute.Int64("jit.duration_ms", duration.Milliseconds()))

	if err != nil {
		SetSpanStatus(span, err, fmt.Sprintf("Failed to %s access request", operation))
		return err
	}

	SetSpanStatus(span, nil, fmt.Sprintf("Successfully %sed access request", operation))
	return nil
}

// InstrumentWebhook wraps webhook operations with tracing
func InstrumentWebhook(ctx context.Context, webhookType, operation string, fn func(context.Context) error) error {
	ctx, span := StartWebhookSpan(ctx, webhookType, operation)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	span.SetAttributes(attribute.Int64("jit.duration_ms", duration.Milliseconds()))

	if err != nil {
		SetSpanStatus(span, err, fmt.Sprintf("Webhook %s %s failed", webhookType, operation))
		return err
	}

	SetSpanStatus(span, nil, fmt.Sprintf("Webhook %s %s succeeded", webhookType, operation))
	return nil
}

// InstrumentAWSCall wraps AWS API calls with tracing
func InstrumentAWSCall(ctx context.Context, service, operation, region string, fn func(context.Context) error) error {
	ctx, span := StartAWSSpan(ctx, service, operation, region)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	span.SetAttributes(attribute.Int64("aws.duration_ms", duration.Milliseconds()))

	if err != nil {
		SetSpanStatus(span, err, fmt.Sprintf("AWS %s %s failed", service, operation))
		return err
	}

	SetSpanStatus(span, nil, fmt.Sprintf("AWS %s %s succeeded", service, operation))
	return nil
}

// InstrumentSlackCommand wraps Slack command processing with tracing
func InstrumentSlackCommand(ctx context.Context, command string, fn func(context.Context) error) error {
	ctx, span := StartSlackSpan(ctx, "command", command)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	duration := time.Since(start)

	span.SetAttributes(attribute.Int64("slack.duration_ms", duration.Milliseconds()))

	if err != nil {
		SetSpanStatus(span, err, fmt.Sprintf("Slack command %s failed", command))
		return err
	}

	SetSpanStatus(span, nil, fmt.Sprintf("Slack command %s succeeded", command))
	return nil
}

// TraceID returns the trace ID from the current span context
func TraceID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanID returns the span ID from the current span context
func SpanID(ctx context.Context) string {
	span := oteltrace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// Context propagation helpers

// InjectTraceContext injects trace context into a map (for HTTP headers, etc.)
func InjectTraceContext(ctx context.Context, carrier map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(carrier))
}

// ExtractTraceContext extracts trace context from a map
func ExtractTraceContext(ctx context.Context, carrier map[string]string) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(carrier))
}
