package store

import (
	"sync"
	"time"
)

// TestResult represents a single test execution result
type TestResult struct {
	TestName  string
	Passed    bool
	Timestamp time.Time
}

// FlakinessRecord tracks historical failure rates
type FlakinessRecord struct {
	TestName string
	Runs     int
	Failures int
	FailRate float64 // failures / runs
	LastSeen time.Time
	Flagged  bool // marked as chronically flaky
}

// FlakinessStore manages test flakiness tracking
type FlakinessStore struct {
	mu      sync.RWMutex
	records map[string]*FlakinessRecord
}

// NewFlakinessStore creates a new flakiness store
func NewFlakinessStore() *FlakinessStore {
	return &FlakinessStore{
		records: make(map[string]*FlakinessRecord),
	}
}

// RecordResult records a test execution result
func (fs *FlakinessStore) RecordResult(testName string, passed bool) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if rec, exists := fs.records[testName]; exists {
		rec.Runs++
		if !passed {
			rec.Failures++
		}
		rec.LastSeen = time.Now()
		rec.FailRate = float64(rec.Failures) / float64(rec.Runs)
	} else {
		failures := 0
		if !passed {
			failures = 1
		}
		fs.records[testName] = &FlakinessRecord{
			TestName: testName,
			Runs:     1,
			Failures: failures,
			FailRate: float64(failures),
			LastSeen: time.Now(),
			Flagged:  false,
		}
	}
}

// GetRecord retrieves the flakiness record for a test
func (fs *FlakinessStore) GetRecord(testName string) *FlakinessRecord {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if rec, exists := fs.records[testName]; exists {
		// Return a copy to avoid concurrent modification
		copy := *rec
		return &copy
	}
	return nil
}

// IsFlakyByThreshold checks if a test is flaky based on fail rate threshold
func (fs *FlakinessStore) IsFlakyByThreshold(testName string, threshold float64) bool {
	rec := fs.GetRecord(testName)
	if rec == nil {
		return false
	}
	return rec.FailRate >= threshold
}

// FlagAsFlaky marks a test as chronically flaky
func (fs *FlakinessStore) FlagAsFlaky(testName string) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if rec, exists := fs.records[testName]; exists {
		rec.Flagged = true
	}
}

// GetFlaggedTests returns all tests marked as chronically flaky
func (fs *FlakinessStore) GetFlaggedTests() []*FlakinessRecord {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var flagged []*FlakinessRecord
	for _, rec := range fs.records {
		if rec.Flagged {
			copy := *rec
			flagged = append(flagged, &copy)
		}
	}
	return flagged
}

// GetStats returns overall statistics
func (fs *FlakinessStore) GetStats() map[string]interface{} {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	totalTests := len(fs.records)
	totalRuns := 0
	totalFailures := 0
	flaggedCount := 0

	for _, rec := range fs.records {
		totalRuns += rec.Runs
		totalFailures += rec.Failures
		if rec.Flagged {
			flaggedCount++
		}
	}

	return map[string]interface{}{
		"total_tests":    totalTests,
		"total_runs":     totalRuns,
		"total_failures": totalFailures,
		"flagged_tests":  flaggedCount,
	}
}
