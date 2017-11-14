package bigcache

// Stats stores cache statistics
type Stats struct {
	// Hits is a number of successfully found keys
	Hits int64
	// Misses is a number of not found keys
	Misses int64
	// DelHits is a number of successfully deleted keys
	DelHits int64
	// DelMisses is a number of not deleted keys
	DelMisses int64
}
