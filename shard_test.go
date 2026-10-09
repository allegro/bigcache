package bigcache

import (
	"context"
	"testing"
	"time"
)

func TestInitNewShardWithoutStats(t *testing.T) {
	t.Parallel()

	config := Config{
		Shards:             10,
		LifeWindow:         10 * time.Second,
		MaxEntriesInWindow: 1000,
		MaxEntrySize:       500,
		StatsEnabled:       false,
		Hasher:             newDefaultHasher(),
	}

	shard := initNewShard(config, func(wrappedEntry []byte, reason RemoveReason) {}, &systemClock{})

	if shard.hashmap == nil {
		t.Fatal("expected hashmap to be allocated, got nil")
	}
	if shard.hashmapStats != nil {
		t.Fatalf("expected hashmapStats to be nil when stats are disabled, got non-nil map of length %d", len(shard.hashmapStats))
	}

	metadata := shard.getKeyMetadata(123)
	assertEqual(t, uint32(0), metadata.RequestCount)

	metadataWithLock := shard.getKeyMetadataWithLock(123)
	assertEqual(t, uint32(0), metadataWithLock.RequestCount)

	shard.hit(123)
	shard.hitWithoutLock(123)
	assertEqual(t, int64(2), shard.getStats().Hits)
	if shard.hashmapStats != nil {
		t.Fatal("expected hashmapStats to remain nil after hits when stats are disabled")
	}
}

func TestInitNewShardWithStats(t *testing.T) {
	t.Parallel()

	config := Config{
		Shards:             10,
		LifeWindow:         10 * time.Second,
		MaxEntriesInWindow: 1000,
		MaxEntrySize:       500,
		StatsEnabled:       true,
		Hasher:             newDefaultHasher(),
	}

	shard := initNewShard(config, func(wrappedEntry []byte, reason RemoveReason) {}, &systemClock{})

	if shard.hashmap == nil {
		t.Fatal("expected hashmap to be allocated, got nil")
	}
	if shard.hashmapStats == nil {
		t.Fatal("expected hashmapStats to be allocated when stats are enabled, got nil")
	}

	shard.hit(456)
	shard.hitWithoutLock(456)
	assertEqual(t, int64(2), shard.getStats().Hits)

	metadata := shard.getKeyMetadata(456)
	assertEqual(t, uint32(2), metadata.RequestCount)

	metadataWithLock := shard.getKeyMetadataWithLock(456)
	assertEqual(t, uint32(2), metadataWithLock.RequestCount)
}

func TestShardResetRespectsStats(t *testing.T) {
	t.Parallel()

	configWithoutStats := Config{
		Shards:             1,
		LifeWindow:         10 * time.Second,
		MaxEntriesInWindow: 100,
		MaxEntrySize:       256,
		StatsEnabled:       false,
		Hasher:             newDefaultHasher(),
	}

	shard := initNewShard(configWithoutStats, func(wrappedEntry []byte, reason RemoveReason) {}, &systemClock{})
	if shard.hashmapStats != nil {
		t.Fatal("expected hashmapStats to be nil before reset")
	}
	shard.reset(configWithoutStats)
	if shard.hashmapStats != nil {
		t.Fatal("expected hashmapStats to be nil after reset when stats are disabled")
	}

	configWithStats := configWithoutStats
	configWithStats.StatsEnabled = true

	shardStats := initNewShard(configWithStats, func(wrappedEntry []byte, reason RemoveReason) {}, &systemClock{})
	if shardStats.hashmapStats == nil {
		t.Fatal("expected hashmapStats to be non-nil before reset")
	}
	shardStats.reset(configWithStats)
	if shardStats.hashmapStats == nil {
		t.Fatal("expected hashmapStats to be non-nil after reset when stats are enabled")
	}
}

func TestInitNewShardConfiguredCapacity(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		maxEntriesInWindow int
		shards             int
		expectedShardSize  int
	}{
		{
			name:               "configured entries above minimum",
			maxEntriesInWindow: 2000,
			shards:             4,
			expectedShardSize:  500,
		},
		{
			name:               "configured entries below minimum",
			maxEntriesInWindow: 8,
			shards:             4,
			expectedShardSize:  minimumEntriesInShard,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Shards:             tc.shards,
				LifeWindow:         time.Minute,
				MaxEntriesInWindow: tc.maxEntriesInWindow,
				MaxEntrySize:       128,
				StatsEnabled:       true,
				Hasher:             newDefaultHasher(),
			}

			assertEqual(t, tc.expectedShardSize, cfg.initialShardSize())

			cache, err := New(context.Background(), cfg)
			noError(t, err)
			defer cache.Close()

			for i := 0; i < tc.shards; i++ {
				if cache.shards[i].hashmap == nil {
					t.Fatalf("shard %d hashmap should be initialized", i)
				}
				if cache.shards[i].hashmapStats == nil {
					t.Fatalf("shard %d hashmapStats should be initialized", i)
				}
			}
		})
	}
}
