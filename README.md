# Self-Healing CI Pipeline Bot

A Go-based webhook server that automatically triages and responds to CI failures. It classifies failures as infra issues, flaky tests, or real bugs—and takes appropriate action (retry, alert, or report).

## Features

- **Multi-provider support**: GitHub Actions and GitLab CI webhooks
- **Intelligent failure classification**: 
  - Heuristic-based pattern matching (fast, cost-effective)
  - Optional LLM classification for ambiguous cases
- **Automatic remediation**:
  - Auto-retry for infrastructure and flaky failures (capped at configurable limit)
  - PR comments with culprit analysis for real failures
  - Reviewer tagging based on failure type
- **Flakiness tracking**: Per-test failure rates to identify chronically unreliable tests

## Quick Start

### Prerequisites

- Go 1.22+
- GitHub/GitLab personal access tokens (if using those providers)
- (Optional) OpenAI API key for LLM classification

### Installation & Setup

1. Clone the repository:
   ```bash
   git clone <repo-url>
   cd self-healing-ci-bot
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Configure environment variables (see `.env.example`):
   ```bash
   cp .env.example .env
   # Edit .env with your tokens and API keys
   ```

4. Run the server:
   ```bash
   go run ./cmd/bot/main.go
   ```

   The server listens on `http://localhost:8080` by default.

5. (Optional) Run tests:
   ```bash
   go test ./...
   ```

## Project Structure

```
.
├── cmd/
│   └── bot/
│       └── main.go              # Entry point: webhook server setup
├── internal/
│   ├── classifier/
│   │   ├── heuristic.go         # Pattern-based failure classification
│   │   ├── llm.go               # Optional LLM-based classification
│   │   └── types.go             # Shared types (Failure, Classification)
│   ├── providers/
│   │   ├── github.go            # GitHub Actions webhook handler & API client
│   │   ├── gitlab.go            # GitLab CI webhook handler & API client
│   │   └── types.go             # Provider-agnostic event types
│   ├── store/
│   │   ├── flakiness.go         # In-memory (or persistent) flakiness tracking
│   │   └── types.go             # Store types and interfaces
│   ├── actions/
│   │   ├── retry.go             # Retry logic with exponential backoff
│   │   ├── reporter.go          # PR comment & reviewer tagging
│   │   └── types.go             # Action types
│   └── config/
│       └── config.go            # Configuration management from env/flags
├── pkg/
│   └── logger/
│       └── logger.go            # Structured logging
├── test/
│   └── fixtures/
│       └── (test data & mocks)
├── go.mod                       # Go module definition
├── go.sum                       # Locked dependencies
├── README.md                    # This file
├── .env.example                 # Example environment variables
└── Makefile                     # Build and development tasks
```

## Configuration

See `.env.example` for all configurable options:

- `BOT_PORT`: Server listening port (default: 8080)
- `BOT_SECRET`: Webhook secret for signature verification
- `GITHUB_TOKEN`: GitHub API authentication token
- `GITLAB_TOKEN`: GitLab API authentication token
- `OPENAI_API_KEY`: Optional, for LLM classification
- `MAX_RETRIES`: Maximum auto-retry attempts per failure (default: 2)
- `FLAKY_THRESHOLD`: Test fail rate above which to flag as flaky (default: 0.4)

## Webhook Integration

### GitHub Actions

1. Navigate to your repo → **Settings** → **Webhooks**
2. Click **Add webhook**
3. Set payload URL to your bot's endpoint (e.g., `https://your-bot.com/webhook/github`)
4. Set content type to `application/json`
5. Select events: **Workflow runs** and optionally **Pulls**
6. Add a secret and update `BOT_SECRET` in config

### GitLab CI

1. Navigate to your project → **Settings** → **Webhooks**
2. Fill in the webhook URL (e.g., `https://your-bot.com/webhook/gitlab`)
3. Check **Pipeline events**
4. Add a secret token and update `BOT_SECRET` in config

## API Endpoints

- `POST /webhook/github` – GitHub Actions failure webhook
- `POST /webhook/gitlab` – GitLab CI failure webhook
- `GET /health` – Health check
- `GET /metrics` – Flakiness metrics (optional Prometheus format)

## How It Works

### 1. Webhook Reception
The bot receives CI failure events and extracts job/run metadata and failure logs.

### 2. Classification
- **Heuristic pass**: Pattern-match against known infra signatures (ECONNRESET, context deadline, rate limits, etc.) and test-specific flakiness patterns
- **LLM pass** (optional): Send ambiguous cases to OpenAI/Claude for structured classification with reasoning

### 3. Action
- **Infra/Flaky**: Auto-retry up to `MAX_RETRIES` times
- **Real failure**: Post a PR comment with the likely culprit and tag reviewers
- **All cases**: Update flakiness score for the test

### 4. Tracking
Store failure patterns and test flakiness rates to identify chronic problems.

## Development

### Running Tests

```bash
go test ./... -v
go test ./internal/classifier -v  # Test classifier specifically
```

### Building

```bash
go build -o bin/bot ./cmd/bot
```

### Debugging

Set `DEBUG=true` in your `.env` for verbose logging:
```bash
DEBUG=true go run ./cmd/bot/main.go
```

## Example: Real vs. Flaky Failure

**Real failure** (pattern not recognized as infra/flaky):
```
Error: undefined reference to `main.GetUser`
  at src/api.go:42
```
→ Post PR comment: "Build failed at src/api.go:42 — missing function `GetUser`. @reviewer-name please check."

**Flaky failure** (test fails ~60% of the time across runs):
```
FAIL: TestConcurrentWrite (timeout after 2s)
```
→ Auto-retry (attempt 1/2). If it passes: mark test as flaky (60% fail rate). If it fails again: still retry, update flakiness to flagged.

**Infra failure** (rate limit hit):
```
HTTP 429: API rate limit exceeded
```
→ Auto-retry with exponential backoff (attempt 1/2).

## Contributing

1. Fork the repo
2. Create a feature branch (`git checkout -b feature/your-feature`)
3. Commit changes (`git commit -m 'Add feature'`)
4. Push to the branch (`git push origin feature/your-feature`)
5. Open a pull request

## License

MIT

## Support

For issues, questions, or feature requests, please open an issue on GitHub.
