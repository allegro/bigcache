package bigcache

import (
	"fmt"
	"log"
	"time"
)

const (
	minimumEntriesInShard = 10 // Minimum number of entries in single shard
)

// BigCache is fast, concurrent, evicting cache created to keep big number of entries without impact on performance.
// It keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
// therefore entries (de)serialization in front of the cache will be needed in most use cases.
type BigCache struct {
	shards       []*cacheShard
	lifeWindow   uint64
	clock        clock
	hash         Hasher
	config       Config
	shardMask    uint64
	maxShardSize uint32
}

// NewBigCache initialize new instance of BigCache
func NewBigCache(config Config) (*BigCache, error) {
	return newBigCache(config, &systemClock{})
}

func newBigCache(config Config, clock clock) (*BigCache, error) {

	if !isPowerOfTwo(config.Shards) {
		return nil, fmt.Errorf("Shards number must be power of two")
	}

	if config.Hasher == nil {
		config.Hasher = newDefaultHasher()
	}

	cache := &BigCache{
		shards:       make([]*cacheShard, config.Shards),
		lifeWindow:   uint64(config.LifeWindow.Seconds()),
		clock:        clock,
		hash:         config.Hasher,
		config:       config,
		shardMask:    uint64(config.Shards - 1),
		maxShardSize: uint32(config.maximumShardSize()),
	}

	var onRemove func(wrappedEntry []byte)
	if config.OnRemove == nil {
		onRemove = cache.notProvidedOnRemove
	} else {
		onRemove = cache.providedOnRemove
	}

	for i := 0; i < config.Shards; i++ {
		cache.shards[i] = initNewShard(config, onRemove)
	}

	if config.CleanWindow > 0 {
		go func() {
			for t := range time.Tick(config.CleanWindow) {
				cache.cleanUp(uint64(t.Unix()))
			}
		}()
	}

	return cache, nil
}

// Get reads entry for the key
func (c *BigCache) Get(key string) ([]byte, error) {
	hashedKey := c.hash.Sum64(key)
	shard := c.getShard(hashedKey)
	shard.lock.RLock()

	itemIndex := shard.hashmap[hashedKey]

	if itemIndex == 0 {
		shard.lock.RUnlock()
		return nil, notFound(key)
	}

	wrappedEntry, err := shard.entries.Get(int(itemIndex))
	if err != nil {
		shard.lock.RUnlock()
		return nil, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		if c.config.Verbose {
			log.Printf("Collision detected. Both %q and %q have the same hash %x", key, entryKey, hashedKey)
		}
		shard.lock.RUnlock()
		return nil, notFound(key)
	}
	shard.lock.RUnlock()
	return readEntry(wrappedEntry), nil
}

// Set saves entry under the key
func (c *BigCache) Set(key string, entry []byte) error {
	hashedKey := c.hash.Sum64(key)
	shard := c.getShard(hashedKey)
	shard.lock.Lock()

	currentTimestamp := uint64(c.clock.epoch())

	if previousIndex := shard.hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := shard.entries.Get(int(previousIndex)); err == nil {
			resetKeyFromEntry(previousEntry)
		}
	}

	if oldestEntry, err := shard.entries.Peek(); err == nil {
		c.onEvict(oldestEntry, currentTimestamp, shard.removeOldestEntry)
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &shard.entryBuffer)

	for {
		if index, err := shard.entries.Push(w); err == nil {
			shard.hashmap[hashedKey] = uint32(index)
			shard.lock.Unlock()
			return nil
		} else if shard.removeOldestEntry() != nil {
			shard.lock.Unlock()
			return fmt.Errorf("Entry is bigger than max shard size.")
		}
	}
}

// Reset empties all cache shards
func (c *BigCache) Reset() error {
	for _, shard := range c.shards {
		shard.lock.Lock()
		shard.reset(c.config)
		shard.lock.Unlock()
	}

	return nil
}

// Len computes number of entries in cache
func (c *BigCache) Len() int {
	var len int

	for _, shard := range c.shards {
		shard.lock.RLock()
		len += shard.len()
		shard.lock.RUnlock()
	}

	return len
}

// Iterator returns iterator function to iterate over EntryInfo's from whole cache.
func (c *BigCache) Iterator() *EntryInfoIterator {
	return newIterator(c)
}

func (c *BigCache) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func() error) bool {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp-oldestTimestamp > c.lifeWindow {
		evict()
		return true
	}
	return false
}

func (c *BigCache) cleanUp(currentTimestamp uint64) {
	for _, shard := range c.shards {
		shard.lock.Lock()
		for {
			if oldestEntry, err := shard.entries.Peek(); err != nil {
				break
			} else if evicted := c.onEvict(oldestEntry, currentTimestamp, shard.removeOldestEntry); !evicted {
				break
			}
		}
		shard.lock.Unlock()
	}
}

func (c *BigCache) getShard(hashedKey uint64) (shard *cacheShard) {
	return c.shards[hashedKey&c.shardMask]
}

func (c *BigCache) providedOnRemove(wrappedEntry []byte) {
	c.config.OnRemove(readKeyFromEntry(wrappedEntry), readEntry(wrappedEntry))
}

func (c *BigCache) notProvidedOnRemove(wrappedEntry []byte) {
}
