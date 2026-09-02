package providers

// CIEvent represents a normalized CI failure event from any provider
type CIEvent struct {
	Provider   string
	RunID      string
	JobID      string
	JobName    string
	Status     string // "failure", "error", etc.
	Logs       string
	Branch     string
	CommitSHA  string
	RepoURL    string
	PRNumber   int // 0 if not a PR
	Conclusion string
	RawPayload map[string]interface{}
}

// Provider interface for CI platform integrations
type Provider interface {
	// ParseWebhookPayload converts raw webhook payload to CIEvent
	ParseWebhookPayload(payload []byte) (*CIEvent, error)

	// FetchLogs retrieves full job/run logs from the CI system
	FetchLogs(runID, jobID string) (string, error)

	// RetryJob triggers a retry of the failed job
	RetryJob(runID, jobID string) error

	// CommentOnPR posts a comment to the PR associated with the failure
	CommentOnPR(prNumber int, message string) error

	// TagReviewers adds mentions to reviewers in a PR comment
	TagReviewers(prNumber int, reviewers []string) error
}

// WebhookRequest represents the incoming webhook request
type WebhookRequest struct {
	Provider string
	Payload  []byte
	Secret   string // webhook secret for verification
}
