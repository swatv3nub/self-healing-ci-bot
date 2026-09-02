package actions

import (
	"testing"

	"github.com/swatv3nub/self-healing-ci-bot/internal/classifier"
	"github.com/swatv3nub/self-healing-ci-bot/internal/providers"
	"github.com/swatv3nub/self-healing-ci-bot/internal/store"
	"github.com/stretchr/testify/assert"
)

// MockProvider for testing actions
type MockProvider struct {
	RetryJobFunc    func(runID, jobID string) error
	CommentOnPRFunc func(prNumber int, message string) error
	TagReviewersFunc func(prNumber int, reviewers []string) error
}

func (m *MockProvider) ParseWebhookPayload(payload []byte) (*providers.CIEvent, error) {
	return nil, nil
}

func (m *MockProvider) FetchLogs(runID, jobID string) (string, error) {
	return "", nil
}

func (m *MockProvider) RetryJob(runID, jobID string) error {
	if m.RetryJobFunc != nil {
		return m.RetryJobFunc(runID, jobID)
	}
	return nil
}

func (m *MockProvider) CommentOnPR(prNumber int, message string) error {
	if m.CommentOnPRFunc != nil {
		return m.CommentOnPRFunc(prNumber, message)
	}
	return nil
}

func (m *MockProvider) TagReviewers(prNumber int, reviewers []string) error {
	if m.TagReviewersFunc != nil {
		return m.TagReviewersFunc(prNumber, reviewers)
	}
	return nil
}

func TestRetryAction_Execute_Success(t *testing.T) {
	mock := &MockProvider{
		RetryJobFunc: func(runID, jobID string) error {
			assert.Equal(t, "run123", runID)
			assert.Equal(t, "job456", jobID)
			return nil
		},
	}

	action := &RetryAction{
		Provider:     mock,
		RunID:        "run123",
		JobID:        "job456",
		AttemptCount: 0,
		MaxRetries:   2,
		BackoffMs:    1, // Very small for test
	}

	err := action.Execute()
	assert.NoError(t, err)
}

func TestRetryAction_Execute_MaxRetriesExceeded(t *testing.T) {
	mock := &MockProvider{}

	action := &RetryAction{
		Provider:     mock,
		RunID:        "run123",
		JobID:        "job456",
		AttemptCount: 2,
		MaxRetries:   2,
		BackoffMs:    1,
	}

	err := action.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max retries")
}

func TestRetryAction_Execute_ProviderError(t *testing.T) {
	mock := &MockProvider{
		RetryJobFunc: func(runID, jobID string) error {
			return assert.AnError
		},
	}

	action := &RetryAction{
		Provider:     mock,
		RunID:        "run123",
		JobID:        "job456",
		AttemptCount: 0,
		MaxRetries:   2,
		BackoffMs:    1,
	}

	err := action.Execute()
	assert.Error(t, err)
}

func TestReportAction_Execute_Success(t *testing.T) {
	mock := &MockProvider{
		CommentOnPRFunc: func(prNumber int, message string) error {
			assert.Equal(t, 42, prNumber)
			assert.Contains(t, message, "CI Failure Report")
			assert.Contains(t, message, "real")
			return nil
		},
		TagReviewersFunc: func(prNumber int, reviewers []string) error {
			assert.Equal(t, 42, prNumber)
			assert.Equal(t, []string{"maintainers"}, reviewers)
			return nil
		},
	}

	classif := &classifier.ClassificationResult{
		Category:   classifier.CategoryReal,
		Confidence: 0.85,
		Reasoning:  "Test reasoning",
		Evidence:   []string{"Error: undefined reference"},
		Method:     "heuristic",
	}

	action := &ReportAction{
		Provider:       mock,
		PRNumber:       42,
		Classification: classif,
		Reviewers:      []string{"maintainers"},
	}

	err := action.Execute()
	assert.NoError(t, err)
}

func TestReportAction_Execute_NoPRNumber(t *testing.T) {
	mock := &MockProvider{}

	classif := &classifier.ClassificationResult{
		Category:  classifier.CategoryReal,
		Confidence: 0.85,
		Reasoning: "Test reasoning",
		Method:    "heuristic",
	}

	action := &ReportAction{
		Provider:       mock,
		PRNumber:       0,
		Classification: classif,
	}

	err := action.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no PR number")
}

func TestReportAction_Execute_CommentError(t *testing.T) {
	mock := &MockProvider{
		CommentOnPRFunc: func(prNumber int, message string) error {
			return assert.AnError
		},
	}

	classif := &classifier.ClassificationResult{
		Category:  classifier.CategoryReal,
		Confidence: 0.85,
		Reasoning: "Test reasoning",
		Method:    "heuristic",
	}

	action := &ReportAction{
		Provider:       mock,
		PRNumber:       42,
		Classification: classif,
	}

	err := action.Execute()
	assert.Error(t, err)
}

func TestExecutor_ExecuteActions_Infra(t *testing.T) {
	mock := &MockProvider{
		RetryJobFunc: func(runID, jobID string) error {
			assert.Equal(t, "run123", runID)
			assert.Equal(t, "job456", jobID)
			return nil
		},
	}

	flakinessStore := store.NewFlakinessStore()
	executor := NewExecutor(mock, flakinessStore, 2, 1)

	event := &providers.CIEvent{
		Provider: "github",
		RunID:    "run123",
		JobID:    "job456",
		JobName:  "test-job",
	}

	classif := &classifier.ClassificationResult{
		Category:  classifier.CategoryInfra,
		Confidence: 0.95,
		Reasoning: "Rate limit",
		Method:    "heuristic",
	}

	err := executor.ExecuteActions(event, classif)
	assert.NoError(t, err)
}

func TestExecutor_ExecuteActions_Flaky(t *testing.T) {
	mock := &MockProvider{
		RetryJobFunc: func(runID, jobID string) error {
			return nil
		},
	}

	flakinessStore := store.NewFlakinessStore()
	executor := NewExecutor(mock, flakinessStore, 2, 1)

	event := &providers.CIEvent{
		Provider: "github",
		RunID:    "run123",
		JobID:    "job456",
		JobName:  "TestFlakyTest",
	}

	classif := &classifier.ClassificationResult{
		Category:  classifier.CategoryFlaky,
		Confidence: 0.85,
		Reasoning: "Race condition",
		Method:    "heuristic",
	}

	err := executor.ExecuteActions(event, classif)
	assert.NoError(t, err)

	// Verify flakiness was recorded (initial failure + retry success)
	rec := flakinessStore.GetRecord("TestFlakyTest")
	assert.NotNil(t, rec)
	assert.Equal(t, 2, rec.Runs)
	assert.Equal(t, 1, rec.Failures)
}

func TestExecutor_ExecuteActions_Real(t *testing.T) {
	mock := &MockProvider{
		CommentOnPRFunc: func(prNumber int, message string) error {
			assert.Equal(t, 42, prNumber)
			assert.Contains(t, message, "real")
			return nil
		},
	}

	flakinessStore := store.NewFlakinessStore()
	executor := NewExecutor(mock, flakinessStore, 2, 1)

	event := &providers.CIEvent{
		Provider:  "github",
		RunID:     "run123",
		JobID:     "job456",
		JobName:   "test-job",
		PRNumber:  42,
	}

	classif := &classifier.ClassificationResult{
		Category:    classifier.CategoryReal,
		Confidence:  0.60,
		Reasoning:   "Build error",
		CulpritLine: &classifier.CulpritLine{File: "main.go", LineNo: 10, Content: "error"},
		Method:      "heuristic",
	}

	err := executor.ExecuteActions(event, classif)
	assert.NoError(t, err)
}

func TestExecutor_ExecuteActions_UnknownCategory(t *testing.T) {
	mock := &MockProvider{}
	flakinessStore := store.NewFlakinessStore()
	executor := NewExecutor(mock, flakinessStore, 2, 1)

	event := &providers.CIEvent{
		Provider: "github",
		RunID:    "run123",
		JobID:    "job456",
	}

	classif := &classifier.ClassificationResult{
		Category:  classifier.FailureCategory("unknown"),
		Confidence: 0.50,
		Reasoning: "Unknown",
		Method:    "heuristic",
	}

	err := executor.ExecuteActions(event, classif)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown failure category")
}

func TestFormatFailureReport(t *testing.T) {
	classif := &classifier.ClassificationResult{
		Category:   classifier.CategoryReal,
		Confidence: 0.75,
		Reasoning:  "Build failed due to missing function",
		Evidence:   []string{"Error: undefined: GetUser", "at main.go:42"},
		CulpritLine: &classifier.CulpritLine{
			File:     "main.go",
			LineNo:   42,
			Function: "main.handleRequest",
			Content:  "Error: undefined: GetUser",
		},
		Method: "heuristic",
	}

	report := formatFailureReport(classif)
	assert.Contains(t, report, "CI Failure Report")
	assert.Contains(t, report, "real")
	assert.Contains(t, report, "75%")
	assert.Contains(t, report, "Build failed due to missing function")
	assert.Contains(t, report, "main.go:42")
	assert.Contains(t, report, "Error: undefined: GetUser")
	// Function is not included in current report format
}