package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/swatv3nub/self-healing-ci-bot/internal/actions"
	"github.com/swatv3nub/self-healing-ci-bot/internal/classifier"
	"github.com/swatv3nub/self-healing-ci-bot/internal/config"
	"github.com/swatv3nub/self-healing-ci-bot/internal/providers"
	"github.com/swatv3nub/self-healing-ci-bot/internal/store"
	"github.com/swatv3nub/self-healing-ci-bot/pkg/logger"
)

var (
	cfg            *config.Config
	log            *logger.Logger
	ghProvider     providers.Provider
	glProvider     providers.Provider
	flakinessStore *store.FlakinessStore
	executor       *actions.Executor
	rateLimiter    *tokenBucket
)

// tokenBucket implements a simple token bucket rate limiter
type tokenBucket struct {
	mu       sync.Mutex
	rate     float64 // tokens per second
	burst    int
	tokens   float64
	lastTime time.Time
}

// newTokenBucket creates a new token bucket rate limiter
func newTokenBucket(rate float64, burst int) *tokenBucket {
	if rate <= 0 {
		return nil
	}
	return &tokenBucket{
		rate:   rate,
		burst:  burst,
		tokens: float64(burst),
		lastTime: time.Now(),
	}
}

// allow checks if a request is allowed and consumes a token
func (tb *tokenBucket) allow() bool {
	if tb == nil {
		return true // rate limiting disabled
	}
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastTime).Seconds()
	tb.tokens = min(tb.tokens+elapsed*tb.rate, float64(tb.burst))
	tb.lastTime = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// rateLimitMiddleware wraps a handler with rate limiting
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if rateLimiter != nil && !rateLimiter.allow() {
			log.Debug("Rate limit exceeded")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func main() {
	// Load configuration
	cfg = config.Load()
	log = logger.New(cfg.Debug)

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	log.Infof("Starting Self-Healing CI Bot on port %d", cfg.Port)

	// Initialize providers
	if cfg.GitHubToken != "" {
		ghProvider = providers.NewGitHubProvider(cfg.GitHubToken)
		log.Info("GitHub provider initialized")
	}

	if cfg.GitLabToken != "" {
		var err error
		glProvider, err = providers.NewGitLabProvider(cfg.GitLabToken)
		if err != nil {
			log.Errorf("Failed to initialize GitLab provider: %v", err)
		} else {
			log.Info("GitLab provider initialized")
		}
	}

	// Initialize stores and executors
	flakinessStore = store.NewFlakinessStore()
	var execProvider providers.Provider
	if ghProvider != nil {
		execProvider = ghProvider
	} else if glProvider != nil {
		execProvider = glProvider
	}
	if execProvider != nil {
		executor = actions.NewExecutor(execProvider, flakinessStore, cfg.MaxRetries, cfg.RetryBackoffMs)
	}

	// Initialize rate limiter
	rateLimiter = newTokenBucket(cfg.WebhookRateLimit, 10)

	// Setup HTTP routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/webhook/github", rateLimitMiddleware(webhookGitHubHandler))
	http.HandleFunc("/webhook/gitlab", rateLimitMiddleware(webhookGitLabHandler))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{Addr: addr}

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info("Shutdown signal received, stopping server...")
		if err := server.Close(); err != nil {
			log.Errorf("Server close error: %v", err)
		}
	}()

	log.Infof("Listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	log.Info("Server stopped")
}

// validateConfig validates the configuration
func validateConfig(cfg *config.Config) error {
	// At least one provider must be configured
	if cfg.GitHubToken == "" && cfg.GitLabToken == "" {
		return fmt.Errorf("at least one provider token must be configured (GITHUB_TOKEN or GITLAB_TOKEN)")
	}

	// Webhook secret is required when providers are configured
	if cfg.WebhookSecret == "" {
		return fmt.Errorf("BOT_SECRET is required for webhook signature verification")
	}

	// Validate port range
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid BOT_PORT: must be between 1 and 65535")
	}

	// Validate retry settings
	if cfg.MaxRetries < 0 {
		return fmt.Errorf("MAX_RETRIES must be non-negative")
	}
	if cfg.RetryBackoffMs < 0 {
		return fmt.Errorf("RETRY_BACKOFF_MS must be non-negative")
	}

	// Validate flaky threshold
	if cfg.FlakyThreshold < 0 || cfg.FlakyThreshold > 1 {
		return fmt.Errorf("FLAKY_THRESHOLD must be between 0.0 and 1.0")
	}

	return nil
}

// healthHandler returns service health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "self-healing-ci-bot",
	})
}

// metricsHandler returns flakiness metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := flakinessStore.GetStats()
	flagged := flakinessStore.GetFlaggedTests()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stats":         stats,
		"flagged_tests": flagged,
	})
}

// webhookGitHubHandler processes GitHub Actions webhooks
func webhookGitHubHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify webhook signature
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !verifyWebhookSignature(payload, r.Header.Get("X-Hub-Signature-256"), cfg.WebhookSecret) {
		log.Error("GitHub webhook signature verification failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if ghProvider == nil {
		http.Error(w, "GitHub provider not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse webhook
	event, err := ghProvider.ParseWebhookPayload(payload)
	if err != nil {
		log.Debugf("GitHub webhook parse error: %v", err)
		w.WriteHeader(http.StatusOK) // Accept but don't process
		return
	}

	log.Infof("GitHub failure detected: run %s, PR %d", event.RunID, event.PRNumber)

	// Fetch logs if not present in event
	logs := event.Logs
	if logs == "" && event.RunID != "" && event.JobID != "" {
		fetchedLogs, err := ghProvider.FetchLogs(event.RunID, event.JobID)
		if err != nil {
			log.Errorf("Failed to fetch GitHub logs: %v", err)
		} else {
			logs = fetchedLogs
		}
	}

	// Classify the failure (heuristic first)
	classif := classifier.ClassifyHeuristic(classifier.FailureLogInput{
		Provider: "github",
		JobName:  event.JobName,
		Logs:     logs,
	})

	log.Infof("Classification: %s (confidence: %.2f)", classif.Category, classif.Confidence)

	// Execute actions
	if executor != nil {
		if err := executor.ExecuteActions(event, &classif); err != nil {
			log.Errorf("Action execution failed: %v", err)
			http.Error(w, "Action failed", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "processed",
		"classification": classif,
	})
}

// webhookGitLabHandler processes GitLab CI webhooks
func webhookGitLabHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !verifyGitLabSignature(r.Header.Get("X-Gitlab-Token"), cfg.WebhookSecret) {
		log.Error("GitLab webhook signature verification failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if glProvider == nil {
		http.Error(w, "GitLab provider not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse webhook
	event, err := glProvider.ParseWebhookPayload(payload)
	if err != nil {
		log.Debugf("GitLab webhook parse error: %v", err)
		w.WriteHeader(http.StatusOK) // Accept but don't process
		return
	}

	log.Infof("GitLab failure detected: run %s, job %s", event.RunID, event.JobName)

	// Classify the failure
	classif := classifier.ClassifyHeuristic(classifier.FailureLogInput{
		Provider: "gitlab",
		JobName:  event.JobName,
		Logs:     event.Logs,
	})

	log.Infof("Classification: %s (confidence: %.2f)", classif.Category, classif.Confidence)

	// Execute actions using shared executor
	if executor != nil {
		if err := executor.ExecuteActions(event, &classif); err != nil {
			log.Errorf("Action execution failed: %v", err)
			http.Error(w, "Action failed", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "processed",
		"classification": classif,
	})
}

// verifyWebhookSignature validates GitHub webhook HMAC-SHA256 signature
func verifyWebhookSignature(payload []byte, signature, secret string) bool {
	if secret == "" {
		return false
	}

	expected := "sha256=" + hex.EncodeToString(computeHMAC(payload, secret))
	return hmac.Equal([]byte(signature), []byte(expected))
}

// verifyGitLabSignature validates GitLab webhook token
func verifyGitLabSignature(token, secret string) bool {
	if secret == "" || token == "" {
		return false
	}
	return hmac.Equal([]byte(token), []byte(secret))
}

// computeHMAC calculates HMAC-SHA256
func computeHMAC(payload []byte, secret string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return h.Sum(nil)
}
