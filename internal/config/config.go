package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Slack  SlackConfig  `mapstructure:"slack"`
	AWS    AWSConfig    `mapstructure:"aws"`
	Access AccessConfig `mapstructure:"access"`
	Log    LogConfig    `mapstructure:"log"`
	Auth   AuthConfig   `mapstructure:"auth"`
}

type ServerConfig struct {
	Port         string        `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"readTimeout"`
	WriteTimeout time.Duration `mapstructure:"writeTimeout"`
	IdleTimeout  time.Duration `mapstructure:"idleTimeout"`
}

type SlackConfig struct {
	Token         string `mapstructure:"token"`
	SigningSecret string `mapstructure:"signingSecret"`
}

type AWSConfig struct {
	Region           string   `mapstructure:"region"`
	AccountIDs       []string `mapstructure:"accountIds"`
	SAMLProviderArn  string   `mapstructure:"samlProviderArn"`
	EKSClusterPrefix string   `mapstructure:"eksClusterPrefix"`
}

type AccessConfig struct {
	MaxDuration      time.Duration `mapstructure:"maxDuration"`
	ApprovalRequired bool          `mapstructure:"approvalRequired"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type AuthConfig struct {
	AdminUsers []string `mapstructure:"adminUsers"`
	Approvers  []string `mapstructure:"approvers"`
}

func LoadFromViper() (*Config, error) {
	setDefaults()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.readTimeout", "15s")
	viper.SetDefault("server.writeTimeout", "15s")
	viper.SetDefault("server.idleTimeout", "60s")

	viper.SetDefault("aws.region", "us-east-1")

	viper.SetDefault("access.maxDuration", "1h")
	viper.SetDefault("access.approvalRequired", true)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}

func validate(cfg *Config) error {
	if cfg.Slack.Token == "" {
		return fmt.Errorf("slack.token is required")
	}

	if cfg.Slack.SigningSecret == "" {
		return fmt.Errorf("slack.signingSecret is required")
	}

	if cfg.Server.Port == "" {
		return fmt.Errorf("server.port is required")
	}

	return nil
}

func (c *Config) Port() string {
	return c.Server.Port
}

func (c *Config) SlackToken() string {
	return c.Slack.Token
}

func (c *Config) SlackSigningSecret() string {
	return c.Slack.SigningSecret
}