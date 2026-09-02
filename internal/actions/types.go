package actions

import (
	"fmt"
	"time"

	"github.com/swatv3nub/self-healing-ci-bot/internal/classifier"
	"github.com/swatv3nub/self-healing-ci-bot/internal/providers"
)

// Action represents an action to take based on classification
type Action interface {
	Execute() error
}

// RetryAction triggers a job retry with backoff
type RetryAction struct {
	Provider     providers.Provider
	RunID        string
	JobID        string
	AttemptCount int
	MaxRetries   int
	BackoffMs    int
}

// Execute performs the retry
func (ra *RetryAction) Execute() error {
	if ra.AttemptCount >= ra.MaxRetries {
		return fmt.Errorf("max retries (%d) exceeded", ra.MaxRetries)
	}

	// Exponential backoff: backoff * 2^attempt
	backoff := time.Duration(ra.BackoffMs*(1<<uint(ra.AttemptCount))) * time.Millisecond
	time.Sleep(backoff)

	return ra.Provider.RetryJob(ra.RunID, ra.JobID)
}

// ReportAction posts a failure report to a PR
type ReportAction struct {
	Provider       providers.Provider
	PRNumber       int
	Classification *classifier.ClassificationResult
	Reviewers      []string
}

// Execute posts the failure report
func (ra *ReportAction) Execute() error {
	if ra.PRNumber == 0 {
		return fmt.Errorf("no PR number to report to")
	}

	message := formatFailureReport(ra.Classification)

	if err := ra.Provider.CommentOnPR(ra.PRNumber, message); err != nil {
		return err
	}

	if len(ra.Reviewers) > 0 {
		return ra.Provider.TagReviewers(ra.PRNumber, ra.Reviewers)
	}

	return nil
}

// formatFailureReport creates a markdown-formatted failure report
func formatFailureReport(classif *classifier.ClassificationResult) string {
	report := fmt.Sprintf("## CI Failure Report\n\n")
	report += fmt.Sprintf("**Classification**: %s\n\n", classif.Category)
	report += fmt.Sprintf("**Confidence**: %.0f%%\n\n", classif.Confidence*100)
	report += fmt.Sprintf("**Reasoning**: %s\n\n", classif.Reasoning)

	if classif.CulpritLine != nil {
		report += fmt.Sprintf("**Culprit Line**: `%s:%d`\n```\n%s\n```\n\n",
			classif.CulpritLine.File, classif.CulpritLine.LineNo, classif.CulpritLine.Content)
	}

	if len(classif.Evidence) > 0 {
		report += "**Evidence**:\n"
		for _, ev := range classif.Evidence {
			report += fmt.Sprintf("- %s\n", ev)
		}
	}

	return report
}
