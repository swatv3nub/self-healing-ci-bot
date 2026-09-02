package classifier

import (
	"regexp"
	"strings"
)

var (
	// Infra failure patterns
	infraPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"connection_reset", regexp.MustCompile(`(?i)ECONNRESET|connection.*reset|socket.*hang`)},
		{"connection_refused", regexp.MustCompile(`(?i)ECONNREFUSED|connection.*refused`)},
		{"timeout", regexp.MustCompile(`(?i)context.*deadline.*exceeded|timeout|timed out`)},
		{"rate_limit", regexp.MustCompile(`(?i)rate.*limit|429|too many requests`)},
		{"dns_failure", regexp.MustCompile(`(?i)ENOTFOUND|name.*resolution|dns.*failed`)},
		{"registry_error", regexp.MustCompile(`(?i)registry.*error|docker pull|image pull|image not found`)},
		{"network_error", regexp.MustCompile(`(?i)network.*unreachable|no route|packet loss`)},
		{"disk_full", regexp.MustCompile(`(?i)no space left on device|disk full|ENOSPC`)},
		{"memory_error", regexp.MustCompile(`(?i)out of memory|OOM|memory limit exceeded`)},
		{"signal_error", regexp.MustCompile(`(?i)killed|signal.*terminated|sigkill|exit code 137`)},
	}

	// Flaky patterns (test-specific)
	flakyPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"race_condition", regexp.MustCompile(`(?i)data race|race condition|concurrent.*access`)},
		{"timing_sensitive", regexp.MustCompile(`(?i)flaky|timing.*sensitive|race condition|intermittent`)},
		{"test_isolation", regexp.MustCompile(`(?i)teardown.*failed|cleanup.*error|port already in use`)},
	}
)

// ClassifyHeuristic performs fast, pattern-based failure classification
func ClassifyHeuristic(input FailureLogInput) ClassificationResult {
	logs := strings.ToLower(input.Logs)

	// Check infra patterns
	for _, p := range infraPatterns {
		if matches := p.pattern.FindAllString(input.Logs, -1); len(matches) > 0 {
			return ClassificationResult{
				Category:   CategoryInfra,
				Confidence: 0.95,
				Evidence:   matches[:min(len(matches), 3)],
				Reasoning:  "Matched infrastructure failure pattern: " + p.name,
				Method:     "heuristic",
			}
		}
	}

	// Check flaky patterns
	for _, p := range flakyPatterns {
		if matches := p.pattern.FindAllString(input.Logs, -1); len(matches) > 0 {
			return ClassificationResult{
				Category:   CategoryFlaky,
				Confidence: 0.85,
				Evidence:   matches[:min(len(matches), 3)],
				Reasoning:  "Matched flaky failure pattern: " + p.name,
				Method:     "heuristic",
			}
		}
	}

	// Check if test is historically flaky
	if input.PreviousRuns > 0 {
		// If this test has failed recently, it's likely flaky
		// (In practice, you'd look this up from the store)
		if input.PreviousRuns >= 2 {
			return ClassificationResult{
				Category:   CategoryFlaky,
				Confidence: 0.70,
				Evidence:   []string{"Test has failed " + strings.Title(strings.ToLower(stringifyInt(input.PreviousRuns))) + " recent times"},
				Reasoning:  "Historical failure pattern suggests flakiness",
				Method:     "heuristic",
			}
		}
	}

	// Default: assume real failure
	lines := extractRelevantLines(input.Logs)
	culprit := extractCulpritLine(lines)

	return ClassificationResult{
		Category:    CategoryReal,
		Confidence:  0.60, // Lower confidence for unknown patterns
		Evidence:    lines[:min(len(lines), 3)],
		CulpritLine: culprit,
		Reasoning:   "No recognized infra or flaky pattern; likely a real failure",
		Method:      "heuristic",
	}
}

// extractRelevantLines pulls the most interesting lines from logs
func extractRelevantLines(logs string) []string {
	lines := strings.Split(logs, "\n")
	var relevant []string

	keywordPatterns := []string{
		"error", "panic", "failed", "exception", "stack", "trace",
		"assert", "expected", "got", "undefined", "fatal",
	}

	for _, line := range lines {
		if len(relevant) >= 5 {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, kw := range keywordPatterns {
			if strings.Contains(strings.ToLower(line), kw) {
				relevant = append(relevant, line)
				break
			}
		}
	}

	if len(relevant) == 0 && len(lines) > 0 {
		// Fallback: return last few non-empty lines
		for i := len(lines) - 1; i >= 0 && len(relevant) < 3; i-- {
			if line := strings.TrimSpace(lines[i]); line != "" {
				relevant = append(relevant, line)
			}
		}
	}

	return relevant
}

// extractCulpritLine tries to identify the specific code line that failed
func extractCulpritLine(lines []string) *CulpritLine {
	// Look for stack trace patterns
	filePattern := regexp.MustCompile(`^\s*at\s+([^\s]+):(\d+)(?::(\d+))?\s*in\s+(.*)$|^\s*(\S+\.go):(\d+):\s*(.*)$`)

	for _, line := range lines {
		if matches := filePattern.FindStringSubmatch(line); len(matches) > 0 {
			return &CulpritLine{
				File:    matches[1],
				LineNo:  parseLineNo(matches[2]),
				Content: strings.TrimSpace(line),
			}
		}
	}

	return nil
}

func parseLineNo(s string) int {
	var n int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			break
		}
	}
	return n
}

func stringifyInt(n int) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			switch r {
			case '0':
				return '0'
			case '1':
				return '1'
			case '2':
				return '2'
			case '3':
				return '3'
			case '4':
				return '4'
			case '5':
				return '5'
			case '6':
				return '6'
			case '7':
				return '7'
			case '8':
				return '8'
			case '9':
				return '9'
			}
		}
		return r
	}, string(rune(n)+rune('0')))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
