package bigcache

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestEntriesIterator(t *testing.T) {
	t.Parallel()

	// given
	keysCount := 1000
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         6 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
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
	assertEqual(t, keysCount, len(keys))
}

func TestEntriesIteratorWithMostShardsEmpty(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             8,
		LifeWindow:         6 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	}, &clock)

	cache.Set("key", []byte("value"))

	// when
	iterator := cache.Iterator()

	// then
	if !iterator.SetNext() {
		t.Errorf("Iterator should contain at least single element")
	}

	current, err := iterator.Value()

	// then
	noError(t, err)
	assertEqual(t, "key", current.Key())
	assertEqual(t, uint64(0x3dc94a19365b10ec), current.Hash())
	assertEqual(t, []byte("value"), current.Value())
	assertEqual(t, uint64(0), current.Timestamp())
}

func TestEntriesIteratorWithConcurrentUpdate(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})

	cache.Set("key", []byte("value"))

	// when
	iterator := cache.Iterator()

	// then
	if !iterator.SetNext() {
		t.Errorf("Iterator should contain at least single element")
	}

	getOldestEntry := func(s *cacheShard) ([]byte, error) {
		s.lock.RLock()
		defer s.lock.RUnlock()
		return s.entries.Peek()
	}

	// Quite ugly but works
	for i := 0; i < cache.config.Shards; i++ {
		if oldestEntry, err := getOldestEntry(cache.shards[i]); err == nil {
			cache.onEvict(oldestEntry, 10, cache.shards[i].removeOldestEntry)
		}
	}

	current, err := iterator.Value()

	// then
	assertEqual(t, ErrCannotRetrieveEntry, err)
	assertEqual(t, "Could not retrieve entry from cache", err.Error())
	assertEqual(t, EntryInfo{}, current)
}

func TestEntriesIteratorWithAllShardsEmpty(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})

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
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})

	// when
	iterator := cache.Iterator()

	// then
	_, err := iterator.Value()
	assertEqual(t, ErrInvalidIteratorState, err)
	assertEqual(t, "Iterator is in invalid state. Use SetNext() to move to next position", err.Error())
}

func TestEntriesIteratorParallelAdd(t *testing.T) {
	bc, err := NewBigCache(DefaultConfig(1 * time.Minute))
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 10000; i++ {
			err := bc.Set(strconv.Itoa(i), []byte("aaaaaaa"))
			if err != nil {
				panic(err)
			}

			runtime.Gosched()
		}
		wg.Done()
	}()

	for i := 0; i < 100; i++ {
		iter := bc.Iterator()
		for iter.SetNext() {
			_, _ = iter.Value()
		}
	}
	wg.Wait()
}
