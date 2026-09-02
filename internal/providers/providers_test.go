package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitHubProvider_ParseWebhookPayload(t *testing.T) {
	provider := NewGitHubProvider("test-token")

	payload := []byte(`{
		"action": "completed",
		"workflow_run": {
			"id": 12345678,
			"status": "completed",
			"conclusion": "failure",
			"head_branch": "feature/test",
			"head_sha": "abc123",
			"pull_requests": [{"number": 42}]
		},
		"repository": {
			"name": "test-repo",
			"owner": {"login": "test-owner"},
			"html_url": "https://github.com/test-owner/test-repo"
		}
	}`)

	event, err := provider.ParseWebhookPayload(payload)
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, "github", event.Provider)
	assert.Equal(t, "12345678", event.RunID)
	assert.Equal(t, "feature/test", event.Branch)
	assert.Equal(t, "abc123", event.CommitSHA)
	assert.Equal(t, 42, event.PRNumber)
}

func TestGitHubProvider_ParseWebhookPayload_NonFailure(t *testing.T) {
	provider := NewGitHubProvider("test-token")

	payload := []byte(`{
		"action": "completed",
		"workflow_run": {
			"id": 12345678,
			"status": "completed",
			"conclusion": "success",
			"head_branch": "main",
			"head_sha": "abc123"
		},
		"repository": {
			"name": "test-repo",
			"owner": {"login": "test-owner"},
			"html_url": "https://github.com/test-owner/test-repo"
		}
	}`)

	event, err := provider.ParseWebhookPayload(payload)
	assert.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "not a failure")
}

func TestGitLabProvider_ParseWebhookPayload(t *testing.T) {
	provider, _ := NewGitLabProvider("test-token")

	payload := []byte(`{
		"object_kind": "pipeline",
		"pipeline": {
			"id": 987654,
			"status": "failed",
			"ref": "feature/test",
			"sha": "abc123"
		},
		"project": {
			"id": 123456,
			"path_with_namespace": "test-owner/test-repo",
			"web_url": "https://gitlab.com/test-owner/test-repo"
		},
		"object_attributes": {
			"id": 555555,
			"status": "failed",
			"stage": "test"
		}
	}`)

	event, err := provider.ParseWebhookPayload(payload)
	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, "gitlab", event.Provider)
	assert.Equal(t, "987654", event.RunID)
	assert.Equal(t, "555555", event.JobID)
	assert.Equal(t, "test", event.JobName)
	assert.Equal(t, "feature/test", event.Branch)
	assert.Equal(t, "abc123", event.CommitSHA)
}

func TestGitLabProvider_ParseWebhookPayload_NonFailure(t *testing.T) {
	provider, _ := NewGitLabProvider("test-token")

	payload := []byte(`{
		"object_kind": "pipeline",
		"pipeline": {
			"id": 987654,
			"status": "success",
			"ref": "main",
			"sha": "abc123"
		},
		"project": {
			"id": 123456,
			"path_with_namespace": "test-owner/test-repo",
			"web_url": "https://gitlab.com/test-owner/test-repo"
		},
		"object_attributes": {
			"id": 555555,
			"status": "success",
			"stage": "test"
		}
	}`)

	event, err := provider.ParseWebhookPayload(payload)
	assert.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "not failed")
}

func TestGitLabProvider_ParseWebhookPayload_UnsupportedType(t *testing.T) {
	provider, _ := NewGitLabProvider("test-token")

	payload := []byte(`{
		"object_kind": "merge_request",
		"pipeline": {
			"id": 987654,
			"status": "failed",
			"ref": "feature/test",
			"sha": "abc123"
		},
		"project": {
			"id": 123456,
			"path_with_namespace": "test-owner/test-repo",
			"web_url": "https://gitlab.com/test-owner/test-repo"
		}
	}`)

	event, err := provider.ParseWebhookPayload(payload)
	assert.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "unsupported object kind")
}

// MockProvider for testing
type MockProvider struct {
	RetryJobFunc    func(runID, jobID string) error
	CommentOnPRFunc func(prNumber int, message string) error
	TagReviewersFunc func(prNumber int, reviewers []string) error
}

func (m *MockProvider) ParseWebhookPayload(payload []byte) (*CIEvent, error) {
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

func TestMockProvider(t *testing.T) {
	mock := &MockProvider{
		RetryJobFunc: func(runID, jobID string) error {
			assert.Equal(t, "run123", runID)
			assert.Equal(t, "job456", jobID)
			return nil
		},
		CommentOnPRFunc: func(prNumber int, message string) error {
			assert.Equal(t, 42, prNumber)
			assert.Contains(t, message, "CI Failure Report")
			return nil
		},
	}

	err := mock.RetryJob("run123", "job456")
	assert.NoError(t, err)

	err = mock.CommentOnPR(42, "CI Failure Report\n**Classification**: real")
	assert.NoError(t, err)
}

func TestCIEvent_JSON(t *testing.T) {
	event := &CIEvent{
		Provider:  "github",
		RunID:     "123",
		JobID:     "456",
		JobName:   "test-job",
		Status:    "failure",
		Branch:    "main",
		CommitSHA: "abc123",
		RepoURL:   "https://github.com/test/repo",
		PRNumber:  42,
		Conclusion: "failure",
	}

	data, err := json.Marshal(event)
	assert.NoError(t, err)

	var parsed CIEvent
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, event.Provider, parsed.Provider)
	assert.Equal(t, event.RunID, parsed.RunID)
	assert.Equal(t, event.PRNumber, parsed.PRNumber)
}