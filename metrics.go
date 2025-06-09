package bigcache

import (
	"sync"
	"time"
)

// Metrics structure used for exposing metrics from cleanup operations
type Metrics struct {
	// The average duration of cleanup calls in microseconds
	AverageCleanupDurationMicros int64

	// Number of entries removed during the cleanup
	TotalEntriesRemoved int64

	// Number of cleanup operations performed
	CleanupOperationsCount int64

	// For compatibility with stress tests
	CleanupCount        int64
	TotalCleanupTime    time.Duration
	TotalEvictedEntries int64
}

// Clone creates a copy of the metrics structure
func (m Metrics) Clone() Metrics {
	return Metrics{
		AverageCleanupDurationMicros: m.AverageCleanupDurationMicros,
		TotalEntriesRemoved:          m.TotalEntriesRemoved,
		CleanupOperationsCount:       m.CleanupOperationsCount,
		CleanupCount:                 m.CleanupOperationsCount, // Map to existing field
		TotalCleanupTime:             m.TotalCleanupTime,
		TotalEvictedEntries:          m.TotalEntriesRemoved, // Map to existing field
	}
}

type metrics struct {
	sync.RWMutex
	enabled bool
	metrics Metrics
}

func newMetrics(enabled bool) *metrics {
	return &metrics{
		enabled: enabled,
		metrics: Metrics{},
	}
}

// Get returns a clone of the current metrics
func (m *metrics) Get() Metrics {
	if !m.enabled {
		return Metrics{}
	}

	m.RLock()
	defer m.RUnlock()
	return m.metrics.Clone()
}

// recordCleanup updates metrics with data from a cleanup operation
func (m *metrics) recordCleanup(duration time.Duration, entriesRemoved int) {
	if !m.enabled {
		return
	}

	m.Lock()
	defer m.Unlock()

	durationMicros := duration.Microseconds()
	m.metrics.CleanupOperationsCount++
	m.metrics.TotalEntriesRemoved += int64(entriesRemoved)
	m.metrics.TotalCleanupTime += duration
	m.metrics.TotalEvictedEntries += int64(entriesRemoved)

	// Calculate running average for cleanup duration
	if m.metrics.CleanupOperationsCount > 1 {
		totalDuration := m.metrics.AverageCleanupDurationMicros * (m.metrics.CleanupOperationsCount - 1)
		m.metrics.AverageCleanupDurationMicros = (totalDuration + durationMicros) / m.metrics.CleanupOperationsCount
	} else {
		m.metrics.AverageCleanupDurationMicros = durationMicros
	}
}
