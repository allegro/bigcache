package bigcache

import (
	"context"
	"fmt"
	"math/rand/v2"
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
	cache, _ := New(context.Background(), Config{
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
	cache, _ := newBigCache(context.Background(), Config{
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
	cache, _ := New(context.Background(), Config{
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
	assertEqual(t, nil, err)
	assertEqual(t, []byte("value"), current.Value())

	next := iterator.SetNext()
	assertEqual(t, false, next)
}

func TestEntriesIteratorWithAllShardsEmpty(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), Config{
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
	cache, _ := New(context.Background(), Config{
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
	bc, err := New(context.Background(), DefaultConfig(1*time.Minute))
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

func TestParallelSetAndIteration(t *testing.T) {
	t.Parallel()

	cache, _ := New(context.Background(), Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 100,
		MaxEntrySize:       256,
		HardMaxCacheSize:   1,
		Verbose:            true,
	})

	entrySize := 1024 * 100
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer func() {
			err := recover()
			// no panic
			assertEqual(t, err, nil)
		}()

		defer wg.Done()

		isTimeout := false

		for !isTimeout {

			select {
			case <-ctx.Done():
				isTimeout = true
			default:
				err := cache.Set(strconv.Itoa(rand.IntN(100)), blob('a', entrySize))
				noError(t, err)
			}
		}
	}()

	go func() {
		defer func() {
			err := recover()
			// no panic
			assertEqual(t, nil, err)
		}()

		defer wg.Done()

		isTimeout := false

		for !isTimeout {

			select {
			case <-ctx.Done():
				isTimeout = true
			default:
				iter := cache.Iterator()
				for iter.SetNext() {
					entry, err := iter.Value()

					// then
					noError(t, err)
					assertEqual(t, entrySize, len(entry.Value()))
				}
			}
		}
	}()

	wg.Wait()
}

func TestIterator_IterativeNonRecursive(t *testing.T) {
	t.Parallel()

	clock := mockedClock{value: 0}
	cache, err := newBigCache(context.Background(), Config{
		Shards:             1,
		LifeWindow:         1 * time.Second,
		CleanWindow:        0,
		MaxEntriesInWindow: 10000,
		MaxEntrySize:       64,
	}, &clock)
	noError(t, err)

	// Insert 2000 keys
	count := 2000
	for i := 0; i < count; i++ {
		err := cache.Set(strconv.Itoa(i), []byte("v"))
		noError(t, err)
	}

	// Creating an iterator: iterator takes snapshot of keys
	iterator := cache.Iterator()

	// Clear shard entries queue to simulate all entries having been evicted/removed
	cache.shards[0].lock.Lock()
	cache.shards[0].entries.Reset()
	cache.shards[0].hashmap = make(map[uint64]uint64)
	cache.shards[0].lock.Unlock()

	// SetNext should iterate iteratively through all 2000 missing keys without stack overflow
	hasElements := iterator.SetNext()
	assertEqual(t, false, hasElements)
}

func TestIterator_ValueConcurrentSafety(t *testing.T) {
	t.Parallel()

	cache, err := New(context.Background(), Config{
		Shards:             4,
		LifeWindow:         10 * time.Minute,
		MaxEntriesInWindow: 1000,
		MaxEntrySize:       64,
	})
	noError(t, err)

	for i := 0; i < 500; i++ {
		cache.Set(fmt.Sprintf("k%d", i), []byte("val"))
	}

	iterator := cache.Iterator()
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			iterator.SetNext()
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			iterator.Value()
		}
	}()

	wg.Wait()
}
