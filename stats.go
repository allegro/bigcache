package bigcache

// Stats stores cache statistics
type Stats struct {
	// Hits is a number of successfully found keys
	Hits int32 `json:"hits"`
	// Misses is a number of not found keys
	Misses int32 `json:"misses"`
	// DelHits is a number of successfully deleted keys
	DelHits int32 `json:"delete_hits"`
	// DelMisses is a number of not deleted keys
	DelMisses int32 `json:"delete_misses"`
	// Collisions is a number of happened key-collisions
	Collisions int32 `json:"collisions"`
}
