package bigcache

import (
	"sync"
	"time"
)

// Metrics holds metrics for the cache operations
type Metrics struct {
	// Number of times cleanup was executed
	CleanupCount int64
	
	// Total time spent on cleanup operations
	TotalCleanupTime time.Duration
	
	// Total number of entries evicted during cleanup
	TotalEvictedEntries int64
}

// metrics is an internal structure to track cache performance
type metrics struct {
	mu                 sync.RWMutex
	enabled            bool
	cleanupCount       int64
	totalCleanupTime   time.Duration
	totalEvictedEntries int64
}

// newMetrics creates a new metrics instance
func newMetrics(enabled bool) *metrics {
	return &metrics{
		enabled: enabled,
	}
}

// recordCleanup records metrics for a cleanup operation
func (m *metrics) recordCleanup(duration time.Duration, evictedEntries int) {
	if !m.enabled {
		return
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.cleanupCount++
	m.totalCleanupTime += duration
	m.totalEvictedEntries += int64(evictedEntries)
}

// Get returns a copy of the current metrics
func (m *metrics) Get() Metrics {
	if !m.enabled {
		return Metrics{}
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return Metrics{
		CleanupCount:       m.cleanupCount,
		TotalCleanupTime:   m.totalCleanupTime,
		TotalEvictedEntries: m.totalEvictedEntries,
	}
}
