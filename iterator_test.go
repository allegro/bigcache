package bigcache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEntriesIterator(t *testing.T) {
	t.Parallel()

	// given
	keysCount := 1000
	cache, _ := NewBigCache(Config{8, 6 * time.Second, 1, 256, false, nil, 0, nil, false, nil})
	value := []byte("value")

	for i := 0; i < keysCount; i++ {
		cache.Set(fmt.Sprintf("key%d", i), value)
	}

	// when
	keys := make(map[string]struct{})
	iterator := cache.Iterator()

	for iterator.SetNext() {
		current, err := iterator.Value()

		if err == nil {
			keys[current.Key()] = struct{}{}
		}
	}

	// then
	assert.Equal(t, keysCount, len(keys))
}

func TestEntriesIteratorWithMostShardsEmpty(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{8, 6 * time.Second, 1, 256, false, nil, 0, nil, false, nil}, &clock)

	cache.Set("key", []byte("value"))

	// when
	iterator := cache.Iterator()

	// then
	if !iterator.SetNext() {
		t.Errorf("Iterator should contain at least single element")
	}

	current, err := iterator.Value()

	// then
	assert.Nil(t, err)
	assert.Equal(t, "key", current.Key())
	assert.Equal(t, uint64(0x3dc94a19365b10ec), current.Hash())
	assert.Equal(t, []byte("value"), current.Value())
	assert.Equal(t, uint64(0), current.Timestamp())
}

func TestEntriesIteratorWithConcurrentUpdate(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{1, time.Second, 1, 256, false, nil, 0, nil, false, nil})

	cache.Set("key", []byte("value"))

	// when
	iterator := cache.Iterator()

	// then
	if !iterator.SetNext() {
		t.Errorf("Iterator should contain at least single element")
	}

	// Quite ugly but works
	for i := 0; i < cache.Config.Shards; i++ {
		if oldestEntry, err := cache.Shards[i].Entries.Peek(); err == nil {
			if err = cache.onEvict(oldestEntry, 10, cache.Shards[i].removeOldestEntry); err != nil {
				t.Error(err)
			}
		}
	}

	current, err := iterator.Value()

	// then
	assert.Equal(t, ErrCannotRetrieveEntry, err)
	assert.Equal(t, "Could not retrieve entry from cache", err.Error())
	assert.Equal(t, EntryInfo{}, current)
}

func TestEntriesIteratorWithAllShardsEmpty(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{1, time.Second, 1, 256, false, nil, 0, nil, false, nil})

	// when
	iterator := cache.Iterator()

	// then
	if iterator.SetNext() {
		t.Errorf("Iterator should not contain any elements")
	}
}

func TestEntriesIteratorInInvalidState(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{1, time.Second, 1, 256, false, nil, 0, nil, false, nil})

	// when
	iterator := cache.Iterator()

	// then
	_, err := iterator.Value()
	assert.Equal(t, ErrInvalidIteratorState, err)
	assert.Equal(t, "Iterator is in invalid state. Use SetNext() to move to next position", err.Error())
}
