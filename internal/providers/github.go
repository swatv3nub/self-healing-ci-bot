package providers

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// GitHubProvider implements the Provider interface for GitHub Actions
type GitHubProvider struct {
	client    *github.Client
	repoOwner string
	repoName  string
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(token string) *GitHubProvider {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &GitHubProvider{
		client: client,
	}
}

// ParseWebhookPayload parses GitHub Actions workflow_run event
func (gp *GitHubProvider) ParseWebhookPayload(payload []byte) (*CIEvent, error) {
	var event struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			ID         int64  `json:"id"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HeadBranch string `json:"head_branch"`
			HeadSHA    string `json:"head_sha"`
			HeadCommit struct {
				Message string `json:"message"`
			} `json:"head_commit"`
			PullRequests []struct {
				Number int `json:"number"`
			} `json:"pull_requests"`
		} `json:"workflow_run"`
		Repository struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			HTMLURL string `json:"html_url"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, err
	}

	if event.WorkflowRun.Conclusion != "failure" {
		return nil, fmt.Errorf("workflow run is not a failure: %s", event.WorkflowRun.Conclusion)
	}

	gp.repoOwner = event.Repository.Owner.Login
	gp.repoName = event.Repository.Name

	prNum := 0
	if len(event.WorkflowRun.PullRequests) > 0 {
		prNum = event.WorkflowRun.PullRequests[0].Number
	}

	return &CIEvent{
		Provider:   "github",
		RunID:      fmt.Sprintf("%d", event.WorkflowRun.ID),
		JobName:    "workflow_run", // GitHub Actions groups jobs; we get more detail from logs
		Status:     event.WorkflowRun.Status,
		Conclusion: event.WorkflowRun.Conclusion,
		Branch:     event.WorkflowRun.HeadBranch,
		CommitSHA:  event.WorkflowRun.HeadSHA,
		RepoURL:    event.Repository.HTMLURL,
		PRNumber:   prNum,
		RawPayload: nil,
	}, nil
}

// FetchLogs retrieves the workflow run logs from GitHub
func (gp *GitHubProvider) FetchLogs(runID, jobID string) (string, error) {
	ctx := context.Background()
	var runIDInt int64
	_, err := fmt.Sscanf(runID, "%d", &runIDInt)
	if err != nil {
		return "", err
	}

	// Get the logs download URL
	logsURL, _, err := gp.client.Actions.GetWorkflowRunLogs(ctx, gp.repoOwner, gp.repoName, runIDInt, 5)
	if err != nil {
		return "", err
	}

	// Download the logs (zip file)
	resp, err := http.Get(logsURL.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download logs: %s", resp.Status)
	}

	// Read the zip content
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Extract and concatenate log files
	return extractLogsFromZip(zipData)
}

// extractLogsFromZip extracts text content from GitHub Actions logs zip
func extractLogsFromZip(zipData []byte) (string, error) {
	reader, err := zip.NewReader(strings.NewReader(string(zipData)), int64(len(zipData)))
	if err != nil {
		return "", err
	}

	var logs strings.Builder
	for _, file := range reader.File {
		// Skip directories
		if file.FileInfo().IsDir() {
			continue
		}

		// Open file in zip
		rc, err := file.Open()
		if err != nil {
			continue
		}

		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			continue
		}

		logs.WriteString(fmt.Sprintf("=== %s ===\n", file.Name))
		logs.Write(content)
		logs.WriteString("\n")
	}

	return logs.String(), nil
}

// RetryJob triggers a workflow_run re-run
func (gp *GitHubProvider) RetryJob(runID, jobID string) error {
	ctx := context.Background()
	var id int64
	_, err := fmt.Sscanf(runID, "%d", &id)
	if err != nil {
		return err
	}

	// Rerun the workflow (rerun all jobs)
	_, err = gp.client.Actions.RerunWorkflowByID(ctx, gp.repoOwner, gp.repoName, id)
	if err != nil {
		return err
	}

	return nil
}

// CommentOnPR posts a comment on the pull request
func (gp *GitHubProvider) CommentOnPR(prNumber int, message string) error {
	ctx := context.Background()
	_, _, err := gp.client.Issues.CreateComment(ctx, gp.repoOwner, gp.repoName, prNumber, &github.IssueComment{
		Body: github.String(message),
	})
	return err
}

// TagReviewers adds @mentions to a message
func (gp *GitHubProvider) TagReviewers(prNumber int, reviewers []string) error {
	// This would typically be called as part of CommentOnPR
	// by including @mentions in the comment body
	if len(reviewers) == 0 {
		return nil
	}

	mentions := ""
	for _, r := range reviewers {
		mentions += "@" + r + " "
	}

	return gp.CommentOnPR(prNumber, "Tagging: "+mentions)
}
