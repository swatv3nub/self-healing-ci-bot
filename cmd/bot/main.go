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
	"syscall"

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
)

func main() {
	// Load configuration
	cfg = config.Load()
	log = logger.New(cfg.Debug)

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
	if ghProvider != nil {
		executor = actions.NewExecutor(ghProvider, flakinessStore, cfg.MaxRetries, cfg.RetryBackoffMs)
	}

	// Setup HTTP routes
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/webhook/github", webhookGitHubHandler)
	http.HandleFunc("/webhook/gitlab", webhookGitLabHandler)

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

	// Classify the failure (heuristic first)
	classif := classifier.ClassifyHeuristic(classifier.FailureLogInput{
		Provider: "github",
		JobName:  event.JobName,
		Logs:     event.Logs,
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

	// Execute actions
	exec := actions.NewExecutor(glProvider, flakinessStore, cfg.MaxRetries, cfg.RetryBackoffMs)
	if err := exec.ExecuteActions(event, &classif); err != nil {
		log.Errorf("Action execution failed: %v", err)
		http.Error(w, "Action failed", http.StatusInternalServerError)
		return
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

// computeHMAC calculates HMAC-SHA256
func computeHMAC(payload []byte, secret string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return h.Sum(nil)
}
