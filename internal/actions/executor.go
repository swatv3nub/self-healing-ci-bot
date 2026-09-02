package actions

import (
	"fmt"

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
	retry := &RetryAction{
		Provider:     ae.provider,
		RunID:        event.RunID,
		JobID:        event.JobID,
		AttemptCount: 0,
		MaxRetries:   ae.maxRetries,
		BackoffMs:    ae.retryBackoffMs,
	}
	return retry.Execute()
}

// handleFlakyFailure auto-retries flaky tests and updates flakiness tracking
func (ae *Executor) handleFlakyFailure(event *providers.CIEvent, classif *classifier.ClassificationResult) error {
	// Record the failure in flakiness store
	ae.flakinessStore.RecordResult(event.JobName, false)

	// Auto-retry
	retry := &RetryAction{
		Provider:     ae.provider,
		RunID:        event.RunID,
		JobID:        event.JobID,
		AttemptCount: 0,
		MaxRetries:   ae.maxRetries,
		BackoffMs:    ae.retryBackoffMs,
	}
	return retry.Execute()
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
