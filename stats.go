package bigcache

import "sync/atomic"

// Stats stores cache statistics
type Stats struct {
	// Hits is a number of successfully found keys
	Hits int64 `json:"hits"`
	// Misses is a number of not found keys
	Misses int64 `json:"misses"`
	// DelHits is a number of successfully deleted keys
	DelHits int64 `json:"delete_hits"`
	// DelMisses is a number of not deleted keys
	DelMisses int64 `json:"delete_misses"`
	// Collisions is a number of happened key-collisions
	Collisions int64 `json:"collisions"`
}

// shardStats uses atomic.Int64 so the counters stay 8-byte aligned on 32-bit platforms.
type shardStats struct {
	hits       atomic.Int64
	misses     atomic.Int64
	delHits    atomic.Int64
	delMisses  atomic.Int64
	collisions atomic.Int64
}
