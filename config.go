package bigcache

import "time"

// Config for BigCache
type Config struct {
	// Number of cache shards
	// Proper value must be a power of two
	Shards int
	// Time after which entry can be evicted
	LifeWindow time.Duration
	// Max number of entries in life window. Used to allocate proper size of cache in every shard.
	// When proper value is set then cache will not allocate additional memory
	MaxEntriesInWindow int
	// Max size of entry in bytes. Used to allocate proper size of cache in every shard.
	MaxEntrySize int
	// Verbose mode prints information about new memory allocation
	Verbose bool
	// Hasher used to map between string keys and unsigned 64bit integers, by default fnv64 hashing is used.
	Hasher Hasher
}

// DefaultConfig initializes config with default values.
// When load for BigCache can be predicted in advance then it is better to use custom config.
func DefaultConfig(eviction time.Duration) Config {
	return Config{
		Shards:             1024,
		LifeWindow:         eviction,
		MaxEntriesInWindow: 1000 * 10 * 60,
		MaxEntrySize:       500,
		Verbose:            true,
		Hasher:             newDefaultHasher(),
	}
}
