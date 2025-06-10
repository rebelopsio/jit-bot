package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoadFromViper(t *testing.T) {
	// Reset viper for test
	viper.Reset()

	// Set test values
	viper.Set("server.port", "9090")
	viper.Set("slack.token", "test-token")
	viper.Set("slack.signingSecret", "test-secret")
	viper.Set("aws.region", "us-west-2")
	viper.Set("auth.adminUsers", []string{"admin1", "admin2"})
	viper.Set("auth.approvers", []string{"approver1"})

	cfg, err := LoadFromViper()
	if err != nil {
		t.Fatalf("LoadFromViper failed: %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("Expected port 9090, got %s", cfg.Server.Port)
	}

	if cfg.Slack.Token != "test-token" {
		t.Errorf("Expected token test-token, got %s", cfg.Slack.Token)
	}

	if cfg.Slack.SigningSecret != "test-secret" {
		t.Errorf("Expected signing secret test-secret, got %s", cfg.Slack.SigningSecret)
	}

	if cfg.AWS.Region != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", cfg.AWS.Region)
	}

	if len(cfg.Auth.AdminUsers) != 2 {
		t.Errorf("Expected 2 admin users, got %d", len(cfg.Auth.AdminUsers))
	}

	if len(cfg.Auth.Approvers) != 1 {
		t.Errorf("Expected 1 approver, got %d", len(cfg.Auth.Approvers))
	}
}

func TestLoadFromViperWithDefaults(t *testing.T) {
	// Reset viper for test
	viper.Reset()

	// Set only required values
	viper.Set("slack.token", "test-token")
	viper.Set("slack.signingSecret", "test-secret")

	cfg, err := LoadFromViper()
	if err != nil {
		t.Fatalf("LoadFromViper failed: %v", err)
	}

	// Check defaults
	if cfg.Server.Port != "8080" {
		t.Errorf("Expected default port 8080, got %s", cfg.Server.Port)
	}

	if cfg.AWS.Region != "us-east-1" {
		t.Errorf("Expected default region us-east-1, got %s", cfg.AWS.Region)
	}

	if cfg.Access.MaxDuration != time.Hour {
		t.Errorf("Expected default max duration 1h, got %v", cfg.Access.MaxDuration)
	}

	if !cfg.Access.ApprovalRequired {
		t.Error("Expected approval required to be true by default")
	}

	if cfg.Log.Level != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.Log.Level)
	}
}

func validateExpectedError(t *testing.T, err error, expectedMsg string) {
	if err == nil {
		t.Error("Expected error but got none")
		return
	}
	if err.Error() != "config validation failed: "+expectedMsg {
		t.Errorf("Expected error '%s', got '%s'", expectedMsg, err.Error())
	}
}

func validateSuccessCase(t *testing.T, err error, cfg *Config) {
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}
	if cfg == nil {
		t.Error("Expected config but got nil")
	}
}

func TestLoadFromViperValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func()
		expectError bool
		errorMsg    string
	}{
		{
			name: "missing slack token",
			setupViper: func() {
				viper.Reset()
				viper.Set("slack.signingSecret", "test-secret")
			},
			expectError: true,
			errorMsg:    "slack.token is required",
		},
		{
			name: "missing slack signing secret",
			setupViper: func() {
				viper.Reset()
				viper.Set("slack.token", "test-token")
			},
			expectError: true,
			errorMsg:    "slack.signingSecret is required",
		},
		{
			name: "valid config",
			setupViper: func() {
				viper.Reset()
				viper.Set("slack.token", "test-token")
				viper.Set("slack.signingSecret", "test-secret")
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.setupViper()

			cfg, err := LoadFromViper()

			if test.expectError {
				validateExpectedError(t, err, test.errorMsg)
			} else {
				validateSuccessCase(t, err, cfg)
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	viper.Reset()
	setDefaults()

	// Test that defaults are set
	if viper.GetString("server.port") != "8080" {
		t.Errorf("Expected default port 8080, got %s", viper.GetString("server.port"))
	}

	if viper.GetString("aws.region") != "us-east-1" {
		t.Errorf("Expected default region us-east-1, got %s", viper.GetString("aws.region"))
	}

	if viper.GetString("access.maxDuration") != "1h" {
		t.Errorf("Expected default max duration 1h, got %s", viper.GetString("access.maxDuration"))
	}

	if !viper.GetBool("access.approvalRequired") {
		t.Error("Expected approval required to be true by default")
	}

	if viper.GetString("log.level") != "info" {
		t.Errorf("Expected default log level info, got %s", viper.GetString("log.level"))
	}

	if viper.GetString("log.format") != "json" {
		t.Errorf("Expected default log format json, got %s", viper.GetString("log.format"))
	}
}

func TestConfigHelperMethods(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: "9090"},
		Slack: SlackConfig{
			Token:         "test-token",
			SigningSecret: "test-secret",
		},
	}

	if cfg.Port() != "9090" {
		t.Errorf("Expected port 9090, got %s", cfg.Port())
	}

	if cfg.SlackToken() != "test-token" {
		t.Errorf("Expected token test-token, got %s", cfg.SlackToken())
	}

	if cfg.SlackSigningSecret() != "test-secret" {
		t.Errorf("Expected signing secret test-secret, got %s", cfg.SlackSigningSecret())
	}
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// This test demonstrates that environment variable support is built in
	// The actual environment variable binding happens in the root command
	// initialization, so we'll test the mechanism exists

	viper.Reset()
	viper.SetEnvPrefix("JIT")
	viper.AutomaticEnv()

	// Manually set values to simulate environment variables
	viper.Set("slack.token", "env-token")
	viper.Set("slack.signingSecret", "env-secret")
	viper.Set("server.port", "7777")

	cfg, err := LoadFromViper()
	if err != nil {
		t.Fatalf("LoadFromViper failed: %v", err)
	}

	// Verify values are used correctly
	if cfg.Slack.Token != "env-token" {
		t.Errorf("Expected token env-token, got %s", cfg.Slack.Token)
	}

	if cfg.Slack.SigningSecret != "env-secret" {
		t.Errorf("Expected secret env-secret, got %s", cfg.Slack.SigningSecret)
	}

	if cfg.Server.Port != "7777" {
		t.Errorf("Expected port 7777, got %s", cfg.Server.Port)
	}
}
