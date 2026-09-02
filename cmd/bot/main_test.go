package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swatv3nub/self-healing-ci-bot/internal/classifier"
	"github.com/swatv3nub/self-healing-ci-bot/test"
)

func TestHealthEndpoint(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]string
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
	assert.Equal(t, "self-healing-ci-bot", result["service"])
}

func TestWebhookSignatureVerification(t *testing.T) {
	secret := "test-secret"
	payload := []byte(`{"test": "data"}`)

	// Compute valid signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	validSig := "sha256=" + hex.EncodeToString(h.Sum(nil))

	// Test valid signature
	assert.True(t, verifyWebhookSignature(payload, validSig, secret))

	// Test invalid signature
	assert.False(t, verifyWebhookSignature(payload, "sha256=invalid", secret))

	// Test empty secret
	assert.False(t, verifyWebhookSignature(payload, validSig, ""))
}

func TestClassificationPipeline(t *testing.T) {
	tests := []struct {
		name     string
		logs     string
		expected classifier.FailureCategory
	}{
		{
			name:     "rate_limit",
			logs:     test.LogsRateLimitError,
			expected: classifier.CategoryInfra,
		},
		{
			name:     "connection_reset",
			logs:     test.LogsConnectionReset,
			expected: classifier.CategoryInfra,
		},
		{
			name:     "context_deadline",
			logs:     test.LogsContextDeadline,
			expected: classifier.CategoryInfra,
		},
		{
			name:     "real_failure",
			logs:     test.LogsRealFailure,
			expected: classifier.CategoryReal,
		},
		{
			name:     "flaky_test",
			logs:     test.LogsFlakyTest,
			expected: classifier.CategoryFlaky,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyHeuristic(classifier.FailureLogInput{
				JobName: "test_job",
				Logs:    tt.logs,
			})
			assert.Equal(t, tt.expected, result.Category)
		})
	}
}

func BenchmarkClassification(b *testing.B) {
	input := classifier.FailureLogInput{
		Provider: "github",
		JobName:  "test_job",
		Logs:     test.LogsRealFailure,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.ClassifyHeuristic(input)
	}
}
