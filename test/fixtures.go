package test

// Sample webhook payloads and test data

// GitHubWorkflowRunPayload is an example GitHub Actions workflow_run event
const GitHubWorkflowRunPayload = `{
  "action": "completed",
  "workflow_run": {
    "id": 12345678,
    "name": "Tests",
    "node_id": "MDg6Q2hlY2tSdW4xMjM0NTY3OA==",
    "head_branch": "feature/new-api",
    "head_sha": "abc123def456",
    "path": ".github/workflows/test.yml",
    "run_number": 42,
    "event": "pull_request",
    "status": "completed",
    "conclusion": "failure",
    "workflow_id": 87654321,
    "check_suite_id": 11111111,
    "check_suite_node_id": "MDEwOlB1bGxSZXF1ZXN0MQ==",
    "head_commit": {
      "id": "abc123def456",
      "tree_id": "xyz789",
      "message": "Add new API endpoint",
      "timestamp": "2026-06-21T10:00:00Z",
      "author": {
        "name": "Developer",
        "email": "dev@example.com"
      }
    },
    "pull_requests": [
      {
        "id": 1,
        "number": 42,
        "head": {
          "ref": "feature/new-api",
          "sha": "abc123def456"
        },
        "base": {
          "ref": "main",
          "sha": "main123main456"
        }
      }
    ]
  },
  "repository": {
    "id": 123456789,
    "name": "my-app",
    "full_name": "myorg/my-app",
    "owner": {
      "login": "myorg",
      "id": 1,
      "type": "Organization"
    },
    "html_url": "https://github.com/myorg/my-app",
    "description": "My awesome application"
  },
  "sender": {
    "login": "github-actions[bot]",
    "id": 41898282,
    "type": "Bot"
  }
}`

// GitLabPipelinePayload is an example GitLab CI pipeline event
const GitLabPipelinePayload = `{
  "object_kind": "pipeline",
  "object_attributes": {
    "id": 987654,
    "iid": 42,
    "name": "CI Pipeline",
    "source": "push",
    "status": "failed",
    "stage": "test",
    "created_at": "2026-06-21T10:00:00.000Z",
    "finished_at": "2026-06-21T10:05:30.000Z",
    "duration": 330,
    "queued_duration": 5
  },
  "pipeline": {
    "id": 987654,
    "iid": 42,
    "status": "failed",
    "source": "push",
    "created_at": "2026-06-21T10:00:00Z",
    "finished_at": "2026-06-21T10:05:30Z",
    "ref": "feature/new-api",
    "sha": "abc123def456",
    "before_sha": "main123main456"
  },
  "project": {
    "id": 123456,
    "name": "my-app",
    "description": "My awesome application",
    "web_url": "https://gitlab.com/myorg/my-app",
    "avatar_url": null,
    "git_ssh_url": "git@gitlab.com:myorg/my-app.git",
    "git_http_url": "https://gitlab.com/myorg/my-app.git",
    "namespace": "myorg",
    "visibility": "private",
    "path_with_namespace": "myorg/my-app"
  },
  "commit": {
    "id": "abc123def456",
    "short_id": "abc123de",
    "title": "Add new API endpoint",
    "message": "Add new API endpoint\n\nCloses #10",
    "author_name": "Developer",
    "author_email": "dev@example.com",
    "created_at": "2026-06-21T09:55:00Z"
  },
  "user": {
    "id": 1,
    "username": "developer",
    "email": "dev@example.com",
    "name": "Developer",
    "avatar_url": "https://www.gravatar.com/avatar/..."
  }
}`

// SampleFailureLogs contains various failure log examples

const LogsRateLimitError = `
Setting up job...
Running build...
Step 1: Install dependencies
  npm install
  > fetching packages from registry...
  HTTP/1.1 429 Too Many Requests
  X-RateLimit-Limit: 1000
  X-RateLimit-Remaining: 0
  X-RateLimit-Reset: 1234567890
  
  Error: npm ERR! code E429
  npm ERR! 429 Too Many Requests - GET https://registry.npmjs.org/...
  npm ERR! You have exceeded the rate limit for your IP address.
  npm ERR! Please try again later.
  npm ERR! npm notice For more information, run: npm audit
`

const LogsConnectionReset = `
Setting up job...
Running integration tests...
Error: ECONNRESET
Error: socket hang up
  at Socket.onclose (_http_client.js:361:8)
  at Socket.emit (events.js:315:10)
  at TCP.onread [as oncb] (internal/stream_base:1032:12)
Error: Connection was reset by peer
Test aborted.
`

const LogsContextDeadline = `
Setting up job...
Running API tests...
Starting server on :8080
Making requests...
context deadline exceeded
  at time.Time.String (time.go:487)
  at main.waitForServer (main.go:124)
Error: Request timed out after 30s
Process exited with code 124
`

const LogsRealFailure = `
Setting up job...
Running build...
Step 1: Compile Go code
go build -o app ./cmd/app
# github.com/myorg/my-app/internal/api
internal/api/handler.go:42:9: undefined: GetUserByID
ld returned 1 exit code
Build failed
`

const LogsFlakyTest = `
Setting up job...
Running tests...
--- FAIL: TestConcurrentWrite (2.345s)
    race_detector.go:15: Warning: DATA RACE
    Write by goroutine 23 at 0x00c420018200:
        sync.(*Mutex).Unlock()
    Previous write by goroutine 22 at 0x00c420018200:
        sync.(*Mutex).Lock()
    Goroutine 22 (running) created at:
        main.concurrentWriter()
exit status 1
FAIL
`
