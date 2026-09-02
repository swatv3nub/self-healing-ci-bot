.PHONY: help build run test clean

help:
	@echo "Self-Healing CI Bot - Makefile targets"
	@echo ""
	@echo "  build       - Build the bot binary"
	@echo "  run         - Run the bot (requires .env)"
	@echo "  test        - Run tests"
	@echo "  test-cover  - Run tests with coverage"
	@echo "  clean       - Remove build artifacts"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter (requires golangci-lint)"

build:
	@mkdir -p bin
	go build -o bin/bot ./cmd/bot

run: build
	./bin/bot

test:
	go test -v ./...

test-cover:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf bin/ dist/ coverage.out coverage.html

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

.PHONY: docker docker-run
docker:
	docker build -t self-healing-ci-bot:latest .

docker-run:
	docker run -p 8080:8080 --env-file .env self-healing-ci-bot:latest
