package providers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitLabProvider implements the Provider interface for GitLab CI
type GitLabProvider struct {
	client    *gitlab.Client
	projectID int
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider(token string) (*GitLabProvider, error) {
	client, err := gitlab.NewClient(token)
	if err != nil {
		return nil, err
	}
	return &GitLabProvider{client: client}, nil
}

// ParseWebhookPayload parses GitLab pipeline event
func (gp *GitLabProvider) ParseWebhookPayload(payload []byte) (*CIEvent, error) {
	var event struct {
		ObjectKind string `json:"object_kind"`
		Pipeline   struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
			Ref    string `json:"ref"`
			SHA    string `json:"sha"`
		} `json:"pipeline"`
		Project struct {
			ID         int    `json:"id"`
			PathWithNS string `json:"path_with_namespace"`
			WebURL     string `json:"web_url"`
		} `json:"project"`
		ObjectAttributes struct {
			ID        int    `json:"id"`
			Status    string `json:"status"`
			StageName string `json:"stage"`
		} `json:"object_attributes"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	gp.projectID = event.Project.ID

	// GitLab can send job or pipeline events
	if event.ObjectKind != "pipeline" && event.ObjectKind != "job" {
		return nil, fmt.Errorf("unsupported object kind: %s", event.ObjectKind)
	}

	if event.Pipeline.Status != "failed" {
		return nil, fmt.Errorf("pipeline status is not failed: %s", event.Pipeline.Status)
	}

	return &CIEvent{
		Provider:   "gitlab",
		RunID:      fmt.Sprintf("%d", event.Pipeline.ID),
		JobID:      fmt.Sprintf("%d", event.ObjectAttributes.ID),
		JobName:    event.ObjectAttributes.StageName,
		Status:     event.Pipeline.Status,
		Branch:     event.Pipeline.Ref,
		CommitSHA:  event.Pipeline.SHA,
		RepoURL:    event.Project.WebURL,
		RawPayload: nil,
	}, nil
}

// FetchLogs retrieves job logs from GitLab
func (gp *GitLabProvider) FetchLogs(runID, jobID string) (string, error) {
	// This is a simplified placeholder; in production, you'd fetch via GitLab API
	return fmt.Sprintf("Logs for GitLab job %s", jobID), nil
}

// RetryJob triggers a job retry in GitLab
func (gp *GitLabProvider) RetryJob(runID, jobID string) error {
	ctx := context.Background()
	var id int
	_, err := fmt.Sscanf(jobID, "%d", &id)
	if err != nil {
		return err
	}

	job, _, err := gp.client.Jobs.RetryJob(gp.projectID, id, gitlab.WithContext(ctx))
	if err != nil {
		return err
	}

	if job == nil {
		return fmt.Errorf("failed to retry job")
	}

	return nil
}

// CommentOnPR posts a comment on a merge request
func (gp *GitLabProvider) CommentOnPR(prNumber int, message string) error {
	ctx := context.Background()
	_, _, err := gp.client.Notes.CreateMergeRequestNote(gp.projectID, prNumber, &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.String(message),
	}, gitlab.WithContext(ctx))
	return err
}

// TagReviewers adds mentions to reviewers (GitLab uses different syntax)
func (gp *GitLabProvider) TagReviewers(prNumber int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}

	mentions := ""
	for _, r := range reviewers {
		mentions += "@" + r + " "
	}

	return gp.CommentOnPR(prNumber, "Tagging: "+mentions)
}
