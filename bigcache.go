package bigcache

import (
	"log"
	"sync"

	"github.com/allegro/bigcache/queue"
)

const (
	minimumEntriesInShard = 10 // Minimum number of entries in single shard
)

// BigCache is fast, concurrent, evicting cache created to keep big number of entries without impact on performance.
// It keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
// therefore entries (de)serialization in front of the cache will be needed in most use cases.
type BigCache struct {
	shards     []*cacheShard
	lifeWindow uint64
	clock      clock
	hash       hash
	config     Config
}

type cacheShard struct {
	hashmap     map[uint64]uint32
	entries     queue.BytesQueue
	lock        sync.RWMutex
	entryBuffer []byte
}

// NewBigCache initialize new instance of BigCache
func NewBigCache(config Config) *BigCache {
	return newBigCache(config, &systemClock{})
}

func newBigCache(config Config, clock clock) *BigCache {
	cache := &BigCache{
		shards:     make([]*cacheShard, config.Shards),
		lifeWindow: uint64(config.LifeWindow.Seconds()),
		clock:      clock,
		hash:       fnv64a{},
		config:     config,
	}

	shardSize := max(config.MaxEntriesInWindow/config.Shards, minimumEntriesInShard)
	for i := 0; i < config.Shards; i++ {
		cache.shards[i] = &cacheShard{
			hashmap:     make(map[uint64]uint32, shardSize),
			entries:     *queue.NewBytesQueue(shardSize*config.MaxEntrySize, config.Verbose),
			entryBuffer: make([]byte, config.MaxEntrySize+headersSizeInBytes),
		}
	}
	return cache
}

// Get reads entry for the key
func (c *BigCache) Get(key string) ([]byte, error) {
	hashedKey := c.hash.sum(key)
	shard := c.getShard(hashedKey)
	shard.lock.RLock()
	defer shard.lock.RUnlock()

	itemIndex := shard.hashmap[hashedKey]

	if itemIndex == 0 {
		return nil, notFound(key)
	}

	wrappedEntry, err := shard.entries.Get(int(itemIndex))
	if err != nil {
		return nil, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		if c.config.Verbose {
			log.Printf("Collision detected. Both %q and %q have the same hash %x", key, entryKey, hashedKey)
		}
		return nil, notFound(key)
	}
	return readEntry(wrappedEntry), nil
}

// Set saves entry under the key
func (c *BigCache) Set(key string, entry []byte) {
	hashedKey := c.hash.sum(key)
	shard := c.getShard(hashedKey)
	shard.lock.Lock()
	defer shard.lock.Unlock()

	currentTimestamp := uint64(c.clock.epoch())

	if previousIndex := shard.hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := shard.entries.Get(int(previousIndex)); err == nil {
			resetKeyFromEntry(previousEntry)
		}
	}

	if oldestEntry, err := shard.entries.Peek(); err == nil {
		c.onEvict(oldestEntry, currentTimestamp, func() {
			shard.entries.Pop()
			hash := readHashFromEntry(oldestEntry)
			delete(shard.hashmap, hash)
		})
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &shard.entryBuffer)
	index := shard.entries.Push(w)
	shard.hashmap[hashedKey] = uint32(index)
}

func (c *BigCache) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func()) {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp-oldestTimestamp > c.lifeWindow {
		evict()
	}
}

func (c *BigCache) getShard(hashedKey uint64) (shard *cacheShard) {
	shardKey := hashedKey % uint64(len(c.shards))
	return c.shards[shardKey]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
