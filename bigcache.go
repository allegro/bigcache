package bigcache

import (
	"fmt"
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
	shards       []*cacheShard
	lifeWindow   uint64
	clock        clock
	hash         Hasher
	config       Config
	shardMask    uint64
	maxShardSize uint32
}

type cacheShard struct {
	hashmap     map[uint64]uint32
	entries     queue.BytesQueue
	lock        sync.RWMutex
	entryBuffer []byte
	onRemove    func(wrappedEntry []byte)
}

// EntryInfo holds informations about entry in the cache
type EntryInfo struct {
	entry []byte
}

// Key returns entry's underlying key
func (e EntryInfo) Key() string {
	return readKeyFromEntry(e.entry)
}

// Hash returns entry's hash value
func (e EntryInfo) Hash() uint64 {
	return readHashFromEntry(e.entry)
}

// Timestamp returns entry's timestamp (time of insertion)
func (e EntryInfo) Timestamp() uint64 {
	return readTimestampFromEntry(e.entry)
}

// Value returns entry's underlying value
func (e EntryInfo) Value() []byte {
	return readEntry(e.entry)
}

// EntryInfoIterator allows to iterate over entries in the cache
type EntryInfoIterator struct {
	sync.Mutex
	cache        *BigCache
	currentShard int
	currentIndex int
	elements     []uint32
	valid        bool
}

func copyCurrentShardMap(shard *cacheShard) []uint32 {
	shard.lock.RLock()
	defer shard.lock.RUnlock()

	var elements []uint32

	for _, index := range shard.hashmap {
		elements = append(elements, index)
	}

	return elements
}

// SetNext moves to next element and returns true if it exists.
func (it *EntryInfoIterator) SetNext() bool {
	it.Lock()
	defer it.Unlock()

	it.valid = false
	it.currentIndex++

	if len(it.elements) > it.currentIndex {
		it.valid = true
		return true
	}

	// Last shard - no more entries
	if it.currentShard == it.cache.config.Shards-1 {
		return false
	}

	for i := it.currentShard + 1; i < it.cache.config.Shards; i++ {
		it.currentShard = i
		it.currentIndex = 0
		it.elements = copyCurrentShardMap(it.cache.shards[i])

		// Non empty shard - stick with it
		if len(it.elements) > 0 {
			it.valid = true
			return true
		}
	}

	return false
}

func newIterator(cache *BigCache) *EntryInfoIterator {
	return &EntryInfoIterator{
		cache:        cache,
		currentShard: 0,
		currentIndex: -1,
		elements:     copyCurrentShardMap(cache.shards[0]),
	}
}

// Value returns current value from the iterator
func (it *EntryInfoIterator) Value() (EntryInfo, error) {
	it.Lock()
	defer it.Unlock()

	if !it.valid {
		return EntryInfo{}, fmt.Errorf("Iterator is in invalid state. Use HasNext() to determine if there is next element.")
	}

	current := it.elements[it.currentIndex]

	var entry []byte
	var err error

	if entry, err = it.cache.shards[it.currentShard].entries.Get(int(current)); err != nil {
		return EntryInfo{}, fmt.Errorf("Could not retrieve entry from cache")
	}

	var dst = make([]byte, len(entry))
	copy(dst, entry)

	return EntryInfo{
		entry: dst,
	}, nil
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

	maxShardSize := 0
	if config.HardMaxCacheSize > 0 {
		maxShardSize = convertMBToBytes(config.HardMaxCacheSize) / config.Shards
	}

	cache := &BigCache{
		shards:       make([]*cacheShard, config.Shards),
		lifeWindow:   uint64(config.LifeWindow.Seconds()),
		clock:        clock,
		hash:         config.Hasher,
		config:       config,
		shardMask:    uint64(config.Shards - 1),
		maxShardSize: uint32(maxShardSize),
	}

	var onRemove func(wrappedEntry []byte)
	if config.OnRemove == nil {
		onRemove = cache.notProvidedOnRemove
	} else {
		onRemove = cache.providedOnRemove
	}

	initShardSize := max(config.MaxEntriesInWindow/config.Shards, minimumEntriesInShard)
	for i := 0; i < config.Shards; i++ {
		cache.shards[i] = &cacheShard{
			hashmap:     make(map[uint64]uint32, initShardSize),
			entries:     *queue.NewBytesQueue(initShardSize*config.MaxEntrySize, maxShardSize, config.Verbose),
			entryBuffer: make([]byte, config.MaxEntrySize+headersSizeInBytes),
			onRemove:    onRemove,
		}
	}

	return cache, nil
}

func isPowerOfTwo(number int) bool {
	return (number & (number - 1)) == 0
}

// Get reads entry for the key
func (c *BigCache) Get(key string) ([]byte, error) {
	hashedKey := c.hash.Sum64(key)
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
func (c *BigCache) Set(key string, entry []byte) error {
	hashedKey := c.hash.Sum64(key)
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
		c.onEvict(oldestEntry, currentTimestamp, shard.removeOldestEntry)
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &shard.entryBuffer)

	for {
		if index, err := shard.entries.Push(w); err == nil {
			shard.hashmap[hashedKey] = uint32(index)
			return nil
		} else if shard.removeOldestEntry() != nil {
			return fmt.Errorf("Entry is bigger than max shard size.")
		}
	}
}

// Iterator returns iterator function to iterate over EntryInfo's from whole cache.
func (c *BigCache) Iterator() *EntryInfoIterator {
	return newIterator(c)
}

func (c *BigCache) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func() error) {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp-oldestTimestamp > c.lifeWindow {
		evict()
	}
}

func (s *cacheShard) removeOldestEntry() error {
	oldest, err := s.entries.Pop()
	if err == nil {
		hash := readHashFromEntry(oldest)
		delete(s.hashmap, hash)
		s.onRemove(oldest)
		return nil
	}
	return err
}

func (c *BigCache) getShard(hashedKey uint64) (shard *cacheShard) {
	return c.shards[hashedKey&c.shardMask]
}

func (c *BigCache) providedOnRemove(wrappedEntry []byte) {
	c.config.OnRemove(readKeyFromEntry(wrappedEntry), readEntry(wrappedEntry))
}

func (c *BigCache) notProvidedOnRemove(wrappedEntry []byte) {
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func convertMBToBytes(value int) int {
	return value * 1024 * 1024
}
