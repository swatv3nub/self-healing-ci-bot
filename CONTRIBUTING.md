# Contributing to Self-Healing CI Bot

Thank you for your interest in contributing! Here's how you can help.

## Development Setup

1. **Clone the repository:**
   ```bash
   git clone <repo-url>
   cd self-healing-ci-bot
   ```

2. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env with your test tokens
   ```

3. **Run tests:**
   ```bash
   make test
   ```

4. **Build locally:**
   ```bash
   make build
   ```

## Code Standards

- Use `gofmt` for formatting: `make fmt`
- Run linter: `make lint` (requires `golangci-lint`)
- Write tests for new features
- Keep functions small and focused
- Add documentation for public APIs

## Common Tasks

### Adding a New CI Provider

1. Create a new file in `internal/providers/` (e.g., `jenkins.go`)
2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       ParseWebhookPayload(payload []byte) (*CIEvent, error)
       FetchLogs(runID, jobID string) (string, error)
       RetryJob(runID, jobID string) error
       CommentOnPR(prNumber int, message string) error
       TagReviewers(prNumber int, reviewers []string) error
   }
   ```
3. Register in `main.go`
4. Add webhook handler (e.g., `/webhook/jenkins`)
5. Add tests

### Improving Classification

1. Add new patterns to `internal/classifier/heuristic.go`
2. Add corresponding tests to `internal/classifier/heuristic_test.go`
3. Test against sample logs from your CI system

### Adding an LLM Provider

1. Implement the `LLMClassifier` interface in `internal/classifier/llm.go`
2. Add configuration for the new provider
3. Update `main.go` to initialize the provider

## Testing

Run all tests:
```bash
make test
```

Run with coverage:
```bash
make test-cover
```

Test a specific package:
```bash
go test -v ./internal/classifier
```

## Submitting Changes

1. Create a feature branch: `git checkout -b feature/your-feature`
2. Commit with clear messages: `git commit -m "Add feature: ..."`
3. Push and open a pull request
4. Ensure all tests pass and linter is clean

## Reporting Issues

Found a bug? Please open an issue with:
- Description of the bug
- Steps to reproduce
- Expected vs actual behavior
- Logs (if applicable)

## Questions?

Feel free to open a discussion or reach out to maintainers.
