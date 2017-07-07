package bigcache

import (
	"encoding/json"
	"fmt"
	"log"
)

const (
	minimumEntriesInShard = 10 // Minimum number of entries in single shard
)

// BigCache is fast, concurrent, evicting cache created to keep big number of entries without impact on performance.
// It keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
// therefore entries (de)serialization in front of the cache will be needed in most use cases.
type BigCache struct {
	Shards       []*cacheShard
	LifeWindow   uint64
	clock        clock
	hash         Hasher
	Config       Config
	ShardMask    uint64
	MaxShardSize uint32
}

// NewBigCache initialize new instance of BigCache
func NewBigCache(config Config) (*BigCache, error) {
	return newBigCache(config, &systemClock{})
}

func newBigCache(config Config, clock clock) (*BigCache, error) {

	if !isPowerOfTwo(config.Shards) {
		return nil, fmt.Errorf("shards number must be power of two")
	}

	if config.Hasher == nil {
		config.Hasher = newDefaultHasher()
	}

	cache := &BigCache{
		Shards:       make([]*cacheShard, config.Shards),
		LifeWindow:   uint64(config.LifeWindow.Seconds()),
		clock:        clock,
		hash:         config.Hasher,
		Config:       config,
		ShardMask:    uint64(config.Shards - 1),
		MaxShardSize: uint32(config.maximumShardSize()),
	}

	var onRemove func(wrappedEntry []byte)
	if config.OnRemove == nil {
		onRemove = cache.notProvidedOnRemove
	} else {
		onRemove = cache.providedOnRemove
	}

	for i := 0; i < config.Shards; i++ {
		cache.Shards[i] = initNewShard(config, onRemove)
	}

	return cache, nil
}

// Get reads entry for the key
func (c *BigCache) Get(key string) ([]byte, error) {
	hashedKey := c.hash.Sum64(key)
	shard := c.getShard(hashedKey)
	shard.lock.RLock()

	itemIndex := shard.Hashmap[hashedKey]

	if itemIndex == 0 {
		shard.lock.RUnlock()
		return nil, notFound(key)
	}

	wrappedEntry, err := shard.Entries.Get(int(itemIndex))
	if err != nil {
		shard.lock.RUnlock()
		return nil, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		if c.Config.Verbose {
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

	if previousIndex := shard.Hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := shard.Entries.Get(int(previousIndex)); err == nil {
			resetKeyFromEntry(previousEntry)
		}
	}

	if oldestEntry, err := shard.Entries.Peek(); err == nil {
		if err = c.onEvict(oldestEntry, currentTimestamp, shard.removeOldestEntry); err != nil {
			return err
		}
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &shard.EntryBuffer)

	for {
		if index, err := shard.Entries.Push(w); err == nil {
			shard.Hashmap[hashedKey] = uint32(index)
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
	for _, shard := range c.Shards {
		shard.reset(c.Config)
	}

	return nil
}

// Len computes number of entries in cache
func (c *BigCache) Len() int {
	var len int

	for _, shard := range c.Shards {
		shard.lock.Lock()
		len += shard.len()
		shard.lock.Unlock()
	}

	return len
}

// Iterator returns iterator function to iterate over EntryInfo's from whole cache.
func (c *BigCache) Iterator() *EntryInfoIterator {
	return newIterator(c)
}

func (c *BigCache) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func() error) error {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp-oldestTimestamp > c.LifeWindow {
		if c.Config.Store {
			if err := c.Flush(); err != nil {
				return err
			}
		}
		evict()
	}
	return nil
}

func (c *BigCache) getShard(hashedKey uint64) (shard *cacheShard) {
	return c.Shards[hashedKey&c.ShardMask]
}

func (c *BigCache) providedOnRemove(wrappedEntry []byte) {
	c.Config.OnRemove(readKeyFromEntry(wrappedEntry), readEntry(wrappedEntry))
}

func (c *BigCache) notProvidedOnRemove(wrappedEntry []byte) {
}

func (c *BigCache) Flush() error {
	encoded, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = c.Config.StoreWriter.Write(encoded)
	if err != nil {
		return err
	}
	return nil
}
