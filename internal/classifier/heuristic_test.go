package classifier

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyHeuristicInfra(t *testing.T) {
	tests := []struct {
		name    string
		input   FailureLogInput
		want    FailureCategory
		wantMin float64 // min confidence
	}{
		{
			name: "rate_limit_error",
			input: FailureLogInput{
				JobName: "test_job",
				Logs:    "HTTP 429: API rate limit exceeded. Retry after 60s",
			},
			want:    CategoryInfra,
			wantMin: 0.90,
		},
		{
			name: "connection_reset",
			input: FailureLogInput{
				JobName: "test_job",
				Logs:    "Error: ECONNRESET - Connection reset by peer",
			},
			want:    CategoryInfra,
			wantMin: 0.90,
		},
		{
			name: "context_deadline",
			input: FailureLogInput{
				JobName: "test_job",
				Logs:    "context deadline exceeded",
			},
			want:    CategoryInfra,
			wantMin: 0.90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHeuristic(tt.input)
			assert.Equal(t, tt.want, result.Category)
			assert.GreaterOrEqual(t, result.Confidence, tt.wantMin)
		})
	}
}

func TestClassifyHeuristicFlaky(t *testing.T) {
	tests := []struct {
		name  string
		input FailureLogInput
		want  FailureCategory
	}{
		{
			name: "race_condition",
			input: FailureLogInput{
				JobName: "test_job",
				Logs:    "ERROR: data race detected in TestConcurrentWrite",
			},
			want: CategoryFlaky,
		},
		{
			name: "timing_sensitive",
			input: FailureLogInput{
				JobName: "test_job",
				Logs:    "Test is flaky - timeout after 1s but sometimes passes",
			},
			want: CategoryFlaky,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHeuristic(tt.input)
			assert.Equal(t, tt.want, result.Category)
		})
	}
}

func TestClassifyHeuristicReal(t *testing.T) {
	input := FailureLogInput{
		JobName: "test_job",
		Logs: `Error: undefined reference to 'main.GetUser'
at src/api.go:42 in main.fetchUserData
Build failed with exit code 2`,
	}

	result := ClassifyHeuristic(input)
	assert.Equal(t, CategoryReal, result.Category)
	assert.NotNil(t, result.Evidence)
	assert.Len(t, result.Evidence, 2)
}

func TestExtractRelevantLines(t *testing.T) {
	logs := `Running tests...
Test 1: PASS
Test 2: PASS
Test 3: FAIL
Error: assertion failed: expected 5, got 3
Stack trace:
  at src/test_helper.go:123
Cleanup finished`

	lines := extractRelevantLines(logs)
	assert.Greater(t, len(lines), 0)

	// Should contain lines with keywords
	found := false
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "error") ||
			strings.Contains(strings.ToLower(line), "fail") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should extract at least one error/fail line")
}

func TestExtractCulpritLine(t *testing.T) {
	lines := []string{
		"at src/api.go:42 in main.handleRequest",
		"panic: runtime error",
	}

	culprit := extractCulpritLine(lines)
	assert.NotNil(t, culprit)
	assert.Equal(t, "src/api.go", culprit.File)
	assert.Equal(t, 42, culprit.LineNo)
}
