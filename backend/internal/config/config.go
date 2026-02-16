// Package config provides configuration loading and validation.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration.
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Auth       AuthConfig
	MLSidecar  MLSidecarConfig
	AWS        AWSConfig
	Azure      AzureConfig
	Jobs       JobsConfig
	Resilience   ResilienceConfig
	Logging      LoggingConfig
	Notification NotificationConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Host         string
	Port         int
	User         string
	Password     string
	Name         string
	SSLMode      string
	MaxOpenConns int
	MaxIdleConns int
	MaxLifetime  time.Duration
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret     string
	TokenExpiry   time.Duration
	APIKeyEnabled bool
}

// MLSidecarConfig holds ML sidecar connection settings.
type MLSidecarConfig struct {
	URL            string
	Enabled        bool
	Timeout        time.Duration
	MaxRetries     int
	CircuitBreaker CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker settings.
type CircuitBreakerConfig struct {
	MaxFailures   int
	ResetTimeout  time.Duration
	HalfOpenLimit int
}

// AWSConfig holds AWS provider settings.
type AWSConfig struct {
	Enabled       bool
	Region        string
	AccessKeyID   string
	SecretKey     string
	AssumeRoleARN string
	ExternalID    string
}

// AzureConfig holds Azure provider settings.
type AzureConfig struct {
	Enabled        bool
	TenantID       string
	ClientID       string
	ClientSecret   string
	SubscriptionID string
}

// JobsConfig holds background job settings.
type JobsConfig struct {
	CostSyncSchedule       string
	AnomalyDetectSchedule  string
	ForecastSchedule       string
	BudgetCheckSchedule    string
	RecommendationSchedule string
}

// ResilienceConfig holds resilience settings.
type ResilienceConfig struct {
	RetryMaxAttempts int
	RetryBaseDelay   time.Duration
	RetryMaxDelay    time.Duration
	RateLimitRPS     float64
	RateLimitBurst   int
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string
	Format string
}

// NotificationConfig holds notification settings.
type NotificationConfig struct {
	SlackWebhookURL string
	EmailSMTPHost   string
	EmailSMTPPort   int
	EmailFrom       string
	EmailPassword   string
	WebhookURLs     string // comma-separated
}

// Load loads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvInt("SERVER_PORT", 8080),
			ReadTimeout:     getEnvDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getEnvDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:         getEnv("DB_HOST", "localhost"),
			Port:         getEnvInt("DB_PORT", 5432),
			User:         getEnv("DB_USER", "finopsmind"),
			Password:     getEnv("DB_PASSWORD", ""),
			Name:         getEnv("DB_NAME", "finopsmind"),
			SSLMode:      getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns: getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("DB_MAX_IDLE_CONNS", 5),
			MaxLifetime:  getEnvDuration("DB_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", ""),
			TokenExpiry:   getEnvDuration("JWT_EXPIRY", 24*time.Hour),
			APIKeyEnabled: getEnvBool("API_KEY_ENABLED", true),
		},
		MLSidecar: MLSidecarConfig{
			URL:        getEnv("ML_SIDECAR_URL", "http://localhost:8000"),
			Enabled:    getEnvBool("ML_SIDECAR_ENABLED", true),
			Timeout:    getEnvDuration("ML_SIDECAR_TIMEOUT", 30*time.Second),
			MaxRetries: getEnvInt("ML_SIDECAR_MAX_RETRIES", 3),
			CircuitBreaker: CircuitBreakerConfig{
				MaxFailures:   getEnvInt("CB_MAX_FAILURES", 5),
				ResetTimeout:  getEnvDuration("CB_RESET_TIMEOUT", 30*time.Second),
				HalfOpenLimit: getEnvInt("CB_HALF_OPEN_LIMIT", 1),
			},
		},
		AWS: AWSConfig{
			Enabled:       getEnvBool("AWS_ENABLED", false),
			Region:        getEnv("AWS_REGION", "us-east-1"),
			AccessKeyID:   getEnv("AWS_ACCESS_KEY_ID", ""),
			SecretKey:     getEnv("AWS_SECRET_ACCESS_KEY", ""),
			AssumeRoleARN: getEnv("AWS_ASSUME_ROLE_ARN", ""),
			ExternalID:    getEnv("AWS_EXTERNAL_ID", ""),
		},
		Azure: AzureConfig{
			Enabled:        getEnvBool("AZURE_ENABLED", false),
			TenantID:       getEnv("AZURE_TENANT_ID", ""),
			ClientID:       getEnv("AZURE_CLIENT_ID", ""),
			ClientSecret:   getEnv("AZURE_CLIENT_SECRET", ""),
			SubscriptionID: getEnv("AZURE_SUBSCRIPTION_ID", ""),
		},
		Jobs: JobsConfig{
			CostSyncSchedule:       getEnv("JOB_COST_SYNC", "0 */6 * * *"),
			AnomalyDetectSchedule:  getEnv("JOB_ANOMALY_DETECT", "0 1 * * *"),
			ForecastSchedule:       getEnv("JOB_FORECAST", "0 2 * * *"),
			BudgetCheckSchedule:    getEnv("JOB_BUDGET_CHECK", "0 * * * *"),
			RecommendationSchedule: getEnv("JOB_RECOMMENDATIONS", "0 3 * * *"),
		},
		Resilience: ResilienceConfig{
			RetryMaxAttempts: getEnvInt("RETRY_MAX_ATTEMPTS", 3),
			RetryBaseDelay:   getEnvDuration("RETRY_BASE_DELAY", 1*time.Second),
			RetryMaxDelay:    getEnvDuration("RETRY_MAX_DELAY", 30*time.Second),
			RateLimitRPS:     getEnvFloat("RATE_LIMIT_RPS", 100),
			RateLimitBurst:   getEnvInt("RATE_LIMIT_BURST", 200),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Notification: NotificationConfig{
			SlackWebhookURL: getEnv("NOTIFICATION_SLACK_WEBHOOK", ""),
			EmailSMTPHost:   getEnv("NOTIFICATION_EMAIL_SMTP_HOST", ""),
			EmailSMTPPort:   getEnvInt("NOTIFICATION_EMAIL_SMTP_PORT", 587),
			EmailFrom:       getEnv("NOTIFICATION_EMAIL_FROM", ""),
			EmailPassword:   getEnv("NOTIFICATION_EMAIL_PASSWORD", ""),
			WebhookURLs:     getEnv("NOTIFICATION_WEBHOOK_URLS", ""),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}

// DSN returns the database connection string.
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// Helper functions
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
