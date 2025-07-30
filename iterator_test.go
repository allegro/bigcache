package bigcache

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

// contains checks if a string is present in a slice
func contains(slice []string, key string) bool {
	for _, element := range slice {
		if element == key {
			return true
		}
	}
	return false
}

func TestIteratorWithKeyCount(t *testing.T) {
	t.Parallel()

	// given
	keysCount := 1000
	ctx := context.Background()
	cache, _ := New(ctx, DefaultConfig(5*time.Second))
	value := []byte("value")

	for i := 0; i < keysCount; i++ {
		cache.Set(fmt.Sprintf("key%d", i), value)
	}

	// when
	keys := make(map[string]struct{})
	iterator := cache.Iterator()

	for iterator.SetNext() {
		current, err := iterator.Value()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if current.Key() == "" {
			t.Fatalf("Expected value for key")
		}
		keys[current.Key()] = struct{}{}
	}

	// then
	if len(keys) != keysCount {
		t.Errorf("Got %d keys, expected %d keys", len(keys), keysCount)
	}
}

func TestIterateOverEmptyCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), DefaultConfig(5*time.Second))
	keys := []string{"key", "key2", "key3"}
	value := []byte("value")

	// when
	iterator := cache.Iterator()
	iteratorSetup := iterator.SetNext()
	result, err := iterator.Value()

	// then
	if iteratorSetup != false {
		t.Errorf("SetNext() should return false on empty cache")
	}
	if err == nil {
		t.Errorf("Error should not be nil on empty cache")
	}
	if result.Key() != "" {
		t.Errorf("Key should be empty on empty cache")
	}
	if result.Value() != nil {
		t.Errorf("Value should be nil on empty cache")
	}

	// fill the cache
	for _, key := range keys {
		cache.Set(key, value)
	}

	// and remove all
	for _, key := range keys {
		cache.Delete(key)
	}

	// when
	iterator = cache.Iterator()
	iteratorSetup = iterator.SetNext()
	result, err = iterator.Value()

	// then
	if iteratorSetup != false {
		t.Errorf("SetNext() should return false on empty cache")
	}
	if err == nil {
		t.Errorf("Error should not be nil on empty cache")
	}
	if result.Key() != "" {
		t.Errorf("Key should be empty on empty cache")
	}
	if result.Value() != nil {
		t.Errorf("Value should be nil on empty cache")
	}
}

func TestIterateOverResetCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), DefaultConfig(5*time.Second))
	keys := []string{"key", "key2", "key3"}
	value := []byte("value")
	for _, key := range keys {
		cache.Set(key, value)
	}

	// when
	cache.Reset()
	recordFound := 0
	iterator := cache.Iterator()
	for iterator.SetNext() {
		current, err := iterator.Value()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if contains(keys, current.Key()) {
			recordFound++
		}
	}

	// then
	if recordFound != 0 {
		t.Errorf("Records found: %d but expected %d", recordFound, 0)
	}
}

func TestIteratorWithAndWithoutCollisions(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
	})

	// when
	cache.Set("key", []byte("value"))
	cache.Set("key", []byte("value2"))
	cache.Set("key", []byte("value3"))
	count := 0
	keys := make(map[string]struct{})

	// then
	iterator := cache.Iterator()
	for iterator.SetNext() {
		count++
		val, err := iterator.Value()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		keys[val.Key()] = struct{}{}
	}

	if count != 1 {
		t.Errorf("Got %d, expected %d", count, 1)
	}

	// when
	cache.Set("key1", []byte("value"))
	cache.Set("key2", []byte("value2"))
	cache.Set("key3", []byte("value3"))
	keys = make(map[string]struct{})
	count = 0

	// then
	iterator = cache.Iterator()
	for iterator.SetNext() {
		count++
		val, err := iterator.Value()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		keys[val.Key()] = struct{}{}
	}

	if count != 4 {
		t.Errorf("Got %d, expected %d", count, 4)
	}
}

func TestEntriesIterator(t *testing.T) {
	t.Parallel()

	clock := newMockedClock(10)
	cache, _ := newBigCache(context.Background(), DefaultConfig(5*time.Second), clock)

	// Add more entries than iterator entries buffer size
	for i := 0; i < 200; i++ {
		cache.Set(strconv.Itoa(i), []byte{})
	}

	// when
	iterator := cache.Iterator()

	// then
	entries := 0
	for iterator.SetNext() {
		iterator.Value()
		entries++
	}

	if entries != 200 {
		t.Errorf("Got %d entries, expected 200", entries)
	}
}

func TestIterateOverSmallerCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), DefaultConfig(5*time.Second))
	keys := []string{"key", "key2", "key3"}
	values := [][]byte{[]byte("value"), []byte("value2"), []byte("value3")}

	for i := 0; i < len(keys); i++ {
		cache.Set(keys[i], values[i])
	}

	// when
	cache.Reset()
	recordFound := 0
	iterator := cache.Iterator()
	for iterator.SetNext() {
		current, err := iterator.Value()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if contains(keys, current.Key()) {
			recordFound++
		}
	}

	// then
	if recordFound != 0 {
		t.Fatalf("Expected to find %d elements but found %d", 0, recordFound)
	}
}

func TestGetEntryFromIterator(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := New(context.Background(), DefaultConfig(5*time.Second))
	cache.Set("key", []byte("value"))

	// when
	iterator := cache.Iterator()
	iterator.SetNext()
	entry, err := iterator.Value()

	// then
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	value, err := cache.Get("key")

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !compareByteSlices(value, entry.Value()) {
		t.Fatalf("Expected %s but got %s", string(value), string(entry.Value()))
	}
}

func TestEntriesIterator_WithContext(t *testing.T) {
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
	if keysCount != len(keys) {
		t.Errorf("Expected %d keys, but got %d", keysCount, len(keys))
	}
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
	assertNoError(t, err)
	if current.Key() != "key" {
		t.Errorf("Expected key %v, got %v", "key", current.Key())
	}
	if current.Hash() != uint64(0x3dc94a19365b10ec) {
		t.Errorf("Expected hash %v, got %v", uint64(0x3dc94a19365b10ec), current.Hash())
	}
	if !bytes.Equal([]byte("value"), current.Value()) {
		t.Errorf("Expected value %v, got %v", []byte("value"), current.Value())
	}
	if current.Timestamp() != uint64(0) {
		t.Errorf("Expected timestamp %v, got %v", uint64(0), current.Timestamp())
	}
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
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}
	if !bytes.Equal([]byte("value"), current.Value()) {
		t.Errorf("Expected value %v, got %v", []byte("value"), current.Value())
	}

	next := iterator.SetNext()
	if next {
		t.Errorf("Expected false, got %v", next)
	}
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
	assertEqualValues(t, ErrInvalidIteratorState, err)
	assertEqualValues(t, "Iterator is in invalid state. Use SetNext() to move to next position", err.Error())
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
			err := bc.Set(fmt.Sprintf("%d", i), []byte("aaaaaaa"))
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

	rand.Seed(0)

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
			assertEqualValues(t, err, nil)
		}()

		defer wg.Done()

		isTimeout := false

		for {
			if isTimeout {
				break
			}
			select {
			case <-ctx.Done():
				isTimeout = true
			default:
				err := cache.Set(strconv.Itoa(rand.Intn(100)), blob('a', entrySize))
				assertNoError(t, err)
			}
		}
	}()

	go func() {
		defer func() {
			err := recover()
			// no panic
			assertEqualValues(t, nil, err)
		}()

		defer wg.Done()

		isTimeout := false

		for {
			if isTimeout {
				break
			}
			select {
			case <-ctx.Done():
				isTimeout = true
			default:
				iter := cache.Iterator()
				for iter.SetNext() {
					_, _ = iter.Value()
				}
			}
		}
	}()

	wg.Wait()
}
