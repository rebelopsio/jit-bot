package main

import (
	"context"
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/controller"
	"github.com/rebelopsio/jit-bot/pkg/kubernetes"
	"github.com/rebelopsio/jit-bot/pkg/monitoring"
	"github.com/rebelopsio/jit-bot/pkg/telemetry"
	webhookpkg "github.com/rebelopsio/jit-bot/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(controller.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var awsRegion string
	var webhookPort int
	var certDir string
	var enableTracing bool
	var tracingExporter string
	var tracingEndpoint string
	
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&awsRegion, "aws-region", "", "AWS region for accessing AWS services.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port the webhook server binds to.")
	flag.StringVar(&certDir, "cert-dir", "", "The directory that contains the webhook server certificates.")
	flag.BoolVar(&enableTracing, "enable-tracing", false, "Enable OpenTelemetry tracing.")
	flag.StringVar(&tracingExporter, "tracing-exporter", "jaeger", "Tracing exporter (jaeger, otlp).")
	flag.StringVar(&tracingEndpoint, "tracing-endpoint", "", "Tracing endpoint URL.")
	
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Initialize monitoring
	monitoringConfig := monitoring.Config{
		MetricsEnabled: true,
		MetricsPort:    8080,
		HealthPort:     8081,
		Tracing: telemetry.TracingConfig{
			Enabled:     enableTracing,
			Exporter:    tracingExporter,
			Endpoint:    tracingEndpoint,
			ServiceName: "jit-operator",
			Environment: getEnvironment(),
			SampleRate:  getSampleRate(),
		},
	}

	monitor := monitoring.NewMonitor(monitoringConfig)
	ctx := context.Background()
	
	if err := monitor.Start(ctx); err != nil {
		setupLog.Error(err, "unable to start monitoring")
		os.Exit(1)
	}
	defer func() {
		if err := monitor.Stop(ctx); err != nil {
			setupLog.Error(err, "failed to stop monitoring")
		}
	}()

	// Configure webhook server options
	webhookOpts := ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "jit-operator.rebelops.io",
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		}),
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), webhookOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize AWS services
	if awsRegion == "" {
		awsRegion = os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-east-1" // Default
		}
	}

	accessManager, err := kubernetes.NewAccessManager(awsRegion)
	if err != nil {
		setupLog.Error(err, "unable to create access manager")
		os.Exit(1)
	}

	// Initialize RBAC
	rbac := auth.NewRBAC([]string{})

	// Setup controllers
	if err = (&controller.JITAccessRequestReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		RBAC:   rbac,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "JITAccessRequest")
		os.Exit(1)
	}
	
	if err = (&controller.JITAccessJobReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		AccessManager: accessManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "JITAccessJob")
		os.Exit(1)
	}

	// Setup webhooks
	if err = webhookpkg.SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup webhooks")
		os.Exit(1)
	}

	// Add health check endpoints
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func getEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("GO_ENV"); env != "" {
		return env
	}
	return "development"
}

func getSampleRate() float64 {
	env := getEnvironment()
	switch env {
	case "production":
		return 0.01 // 1% sampling in production
	case "staging":
		return 0.05 // 5% sampling in staging
	default:
		return 0.1 // 10% sampling in development
	}
}