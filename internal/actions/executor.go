package actions

import (
	"fmt"
	"strings"

	"github.com/swatv3nub/self-healing-ci-bot/internal/classifier"
	"github.com/swatv3nub/self-healing-ci-bot/internal/providers"
	"github.com/swatv3nub/self-healing-ci-bot/internal/store"
)

// Executor handles orchestration of actions based on classification
type Executor struct {
	provider       providers.Provider
	flakinessStore *store.FlakinessStore
	maxRetries     int
	retryBackoffMs int
}

// NewExecutor creates a new action executor
func NewExecutor(provider providers.Provider, store *store.FlakinessStore, maxRetries, retryBackoffMs int) *Executor {
	return &Executor{
		provider:       provider,
		flakinessStore: store,
		maxRetries:     maxRetries,
		retryBackoffMs: retryBackoffMs,
	}
}

// ExecuteActions determines and runs appropriate actions based on classification
func (ae *Executor) ExecuteActions(event *providers.CIEvent, classif *classifier.ClassificationResult) error {
	switch classif.Category {
	case classifier.CategoryInfra:
		return ae.handleInfraFailure(event, classif)
	case classifier.CategoryFlaky:
		return ae.handleFlakyFailure(event, classif)
	case classifier.CategoryReal:
		return ae.handleRealFailure(event, classif)
	default:
		return fmt.Errorf("unknown failure category: %s", classif.Category)
	}
}

// handleInfraFailure auto-retries infrastructure failures
func (ae *Executor) handleInfraFailure(event *providers.CIEvent, classif *classifier.ClassificationResult) error {
	testName := extractTestName(event, classif)
	
	retry := &RetryAction{
		Provider:     ae.provider,
		RunID:        event.RunID,
		JobID:        event.JobID,
		AttemptCount: 0,
		MaxRetries:   ae.maxRetries,
		BackoffMs:    ae.retryBackoffMs,
	}
	
	success, err := retry.ExecuteWithResult()
	if err != nil {
		return err
	}
	
	// Record retry result for flakiness tracking
	ae.flakinessStore.RecordResult(testName, success)
	
	return nil
}

// handleFlakyFailure auto-retries flaky tests and updates flakiness tracking
func (ae *Executor) handleFlakyFailure(event *providers.CIEvent, classif *classifier.ClassificationResult) error {
	// Extract test name from classification evidence or culprit line
	testName := extractTestName(event, classif)

	// Record the failure in flakiness store
	ae.flakinessStore.RecordResult(testName, false)

	// Auto-retry
	retry := &RetryAction{
		Provider:     ae.provider,
		RunID:        event.RunID,
		JobID:        event.JobID,
		AttemptCount: 0,
		MaxRetries:   ae.maxRetries,
		BackoffMs:    ae.retryBackoffMs,
	}
	
	success, err := retry.ExecuteWithResult()
	if err != nil {
		return err
	}
	
	// Record retry result for flakiness tracking
	ae.flakinessStore.RecordResult(testName, success)
	
	return nil
}

// extractTestName extracts test name from classification result or falls back to job name
func extractTestName(event *providers.CIEvent, classif *classifier.ClassificationResult) string {
	// Try to get test name from culprit line
	if classif.CulpritLine != nil && classif.CulpritLine.Function != "" {
		return classif.CulpritLine.Function
	}

	// Try to extract from evidence lines (look for test function names)
	for _, evidence := range classif.Evidence {
		// Look for test function patterns like "TestXxx" or "Test_Xxx"
		if strings.HasPrefix(strings.TrimSpace(evidence), "--- FAIL: ") {
			parts := strings.Split(evidence, " ")
			if len(parts) >= 3 {
				return strings.TrimSpace(parts[2])
			}
		}
		// Look for "FAIL: TestName" pattern
		if strings.Contains(evidence, "FAIL: ") {
			parts := strings.Split(evidence, "FAIL: ")
			if len(parts) >= 2 {
				testPart := strings.Fields(parts[1])
				if len(testPart) > 0 {
					return testPart[0]
				}
			}
		}
	}

	// Fall back to job name
	return event.JobName
}

// handleRealFailure reports the real failure to PR
func (ae *Executor) handleRealFailure(event *providers.CIEvent, classif *classifier.ClassificationResult) error {
	// Don't retry; report instead
	report := &ReportAction{
		Provider:       ae.provider,
		PRNumber:       event.PRNumber,
		Classification: classif,
		Reviewers:      determineReviewers(event, classif),
	}
	return report.Execute()
}

// determineReviewers identifies who should be tagged in the failure report
func determineReviewers(event *providers.CIEvent, classif *classifier.ClassificationResult) []string {
	// In a real implementation, you'd query CODEOWNERS or git blame
	// For now, return a placeholder
	reviewers := []string{}

	if classif.CulpritLine != nil && classif.CulpritLine.File != "" {
		// Could query git blame or CODEOWNERS for this file
		reviewers = append(reviewers, "maintainers") // placeholder
	}

	return reviewers
}
