package bigcache

import (
	"errors"
	"sync"
)

// EntryInfo holds information about entry in the cache
type EntryInfo struct {
	timestamp uint64
	hash      uint64
	key       string
	value     []byte
}

// Timestamp returns when entry was last accessed
func (e EntryInfo) Timestamp() uint64 {
	return e.timestamp
}

// Hash returns entry hash
func (e EntryInfo) Hash() uint64 {
	return e.hash
}

// Key returns entry's key
func (e EntryInfo) Key() string {
	return e.key
}

// Value returns entry's value
func (e EntryInfo) Value() []byte {
	return e.value
}

// ErrInvalidIteratorState is reported when iterator is in invalid state
var ErrInvalidIteratorState = errors.New("Iterator is in invalid state. Use SetNext() to move to next position")

// EntryInfoIterator allows to iterate over entries in the cache
type EntryInfoIterator struct {
	mutex         sync.Mutex
	cache         *BigCache
	currentShard  int
	currentIndex  int
	elements      []uint64
	elementsCount int
	valid         bool
}

// SetNext moves to next element and returns true if it exists.
func (it *EntryInfoIterator) SetNext() bool {
	it.mutex.Lock()
	defer it.mutex.Unlock()

	it.valid = false

	for it.currentShard < len(it.cache.shards) {
		if it.currentIndex == 0 {
			it.elements, it.elementsCount = it.cache.shards[it.currentShard].copyHashedKeys()
		}

		if it.currentIndex < it.elementsCount {
			it.valid = true
			it.currentIndex++
			return true
		}

		it.currentIndex = 0
		it.currentShard++
	}

	return false
}

// Value returns current value from the iterator
func (it *EntryInfoIterator) Value() (EntryInfo, error) {
	it.mutex.Lock()
	defer it.mutex.Unlock()

	if !it.valid {
		return EntryInfo{}, ErrInvalidIteratorState
	}

	if it.currentIndex <= 0 || it.currentIndex > it.elementsCount {
		return EntryInfo{}, ErrInvalidIteratorState
	}

	wrappedEntry, err := it.cache.shards[it.currentShard].getEntry(it.elements[it.currentIndex-1])
	if err != nil {
		return EntryInfo{}, err
	}

	key := readKeyFromEntry(wrappedEntry)
	timestamp := readTimestampFromEntry(wrappedEntry)
	hash := readHashFromEntry(wrappedEntry)

	return EntryInfo{
		timestamp: timestamp,
		hash:      hash,
		key:       key,
		value:     readEntry(wrappedEntry),
	}, nil
}

func newIterator(cache *BigCache) *EntryInfoIterator {
	return &EntryInfoIterator{
		cache:        cache,
		currentShard: 0,
		currentIndex: 0,
		valid:        false,
	}
}
