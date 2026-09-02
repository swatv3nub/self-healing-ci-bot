package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordResult(t *testing.T) {
	store := NewFlakinessStore()

	// Record some results
	store.RecordResult("test_a", true)  // pass
	store.RecordResult("test_a", false) // fail
	store.RecordResult("test_a", false) // fail

	rec := store.GetRecord("test_a")
	assert.NotNil(t, rec)
	assert.Equal(t, 3, rec.Runs)
	assert.Equal(t, 2, rec.Failures)
	assert.InDelta(t, 2.0/3.0, rec.FailRate, 0.01)
}

func TestIsFlakyByThreshold(t *testing.T) {
	store := NewFlakinessStore()

	// Fail 7 out of 10 times (fail rate = 0.7)
	for i := 0; i < 10; i++ {
		store.RecordResult("flaky_test", i < 3) // first 3 pass, rest fail (7 failures)
	}

	assert.False(t, store.IsFlakyByThreshold("flaky_test", 0.75))
	assert.True(t, store.IsFlakyByThreshold("flaky_test", 0.60))
	assert.True(t, store.IsFlakyByThreshold("flaky_test", 0.50))
}

func TestFlagAsFlaky(t *testing.T) {
	store := NewFlakinessStore()

	store.RecordResult("test_x", false)
	store.FlagAsFlaky("test_x")

	flagged := store.GetFlaggedTests()
	assert.Len(t, flagged, 1)
	assert.Equal(t, "test_x", flagged[0].TestName)
	assert.True(t, flagged[0].Flagged)
}

func TestGetStats(t *testing.T) {
	store := NewFlakinessStore()

	store.RecordResult("test1", true)
	store.RecordResult("test2", false)
	store.RecordResult("test3", false)
	store.FlagAsFlaky("test2")

	stats := store.GetStats()
	assert.Equal(t, 3, stats["total_tests"])
	assert.Equal(t, 3, stats["total_runs"])
	assert.Equal(t, 2, stats["total_failures"])
	assert.Equal(t, 1, stats["flagged_tests"])
}
