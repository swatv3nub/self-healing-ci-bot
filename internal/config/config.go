package config

import (
	"os"
	"strconv"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port int
	// Secrets & Tokens
	WebhookSecret string
	GitHubToken   string
	GitLabToken   string
	// LLM
	OpenAIAPIKey string
	// Behavior
	MaxRetries      int
	FlakyThreshold  float64 // fail rate above which to flag as flaky (0.0-1.0)
	RetryBackoffMs  int
	WebhookRateLimit float64 // requests per second (0 = disabled)
	// Debug
	Debug bool
}

// Load reads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Port:             parseInt(os.Getenv("BOT_PORT"), 8080),
		WebhookSecret:    os.Getenv("BOT_SECRET"),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		GitLabToken:      os.Getenv("GITLAB_TOKEN"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		MaxRetries:       parseInt(os.Getenv("MAX_RETRIES"), 2),
		FlakyThreshold:   parseFloat(os.Getenv("FLAKY_THRESHOLD"), 0.4),
		RetryBackoffMs:   parseInt(os.Getenv("RETRY_BACKOFF_MS"), 1000),
		WebhookRateLimit: parseFloat(os.Getenv("WEBHOOK_RATE_LIMIT"), 0),
		Debug:            parseBool(os.Getenv("DEBUG"), false),
	}
	return cfg
}

func parseInt(s string, defaultVal int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return defaultVal
}

func parseFloat(s string, defaultVal float64) float64 {
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return defaultVal
}

func parseBool(s string, defaultVal bool) bool {
	switch s {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return defaultVal
	}
}
