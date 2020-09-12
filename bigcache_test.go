package bigcache

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriteAndGetOnCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(5 * time.Second))
	value := []byte("value")

	// when
	cache.Set("key", value)
	cachedValue, err := cache.Get("key")

	// then
	noError(t, err)
	assertEqual(t, value, cachedValue)
}

func TestAppendAndGetOnCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(5 * time.Second))
	key := "key"
	value1 := make([]byte, 50)
	rand.Read(value1)
	value2 := make([]byte, 50)
	rand.Read(value2)
	value3 := make([]byte, 50)
	rand.Read(value3)

	// when
	_, err := cache.Get(key)

	// then
	assertEqual(t, ErrEntryNotFound, err)

	// when
	cache.Append(key, value1)
	cachedValue, err := cache.Get(key)

	// then
	noError(t, err)
	assertEqual(t, value1, cachedValue)

	// when
	cache.Append(key, value2)
	cachedValue, err = cache.Get(key)

	// then
	noError(t, err)
	expectedValue := value1
	expectedValue = append(expectedValue, value2...)
	assertEqual(t, expectedValue, cachedValue)

	// when
	cache.Append(key, value3)
	cachedValue, err = cache.Get(key)

	// then
	noError(t, err)
	expectedValue = value1
	expectedValue = append(expectedValue, value2...)
	expectedValue = append(expectedValue, value3...)
	assertEqual(t, expectedValue, cachedValue)
}

// TestAppendRandomly does simultaneous appends to check for corruption errors.
func TestAppendRandomly(t *testing.T) {
	t.Parallel()

	c := Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		CleanWindow:        1 * time.Second,
		MaxEntriesInWindow: 1000 * 10 * 60,
		MaxEntrySize:       500,
		StatsEnabled:       true,
		Verbose:            true,
		Hasher:             newDefaultHasher(),
		HardMaxCacheSize:   1,
		Logger:             DefaultLogger(),
	}
	cache, err := NewBigCache(c)
	noError(t, err)

	nKeys := 5
	nAppendsPerKey := 2000
	nWorker := 10
	var keys []string
	for i := 0; i < nKeys; i++ {
		for j := 0; j < nAppendsPerKey; j++ {
			keys = append(keys, fmt.Sprintf("key%d", i))
		}
	}
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	jobs := make(chan string, len(keys))
	for _, key := range keys {
		jobs <- key
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < nWorker; i++ {
		wg.Add(1)
		go func() {
			for {
				key, ok := <-jobs
				if !ok {
					break
				}
				cache.Append(key, []byte(key))
			}
			wg.Done()
		}()
	}
	wg.Wait()

	assertEqual(t, nKeys, cache.Len())
	for i := 0; i < nKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := []byte(strings.Repeat(key, nAppendsPerKey))
		cachedValue, err := cache.Get(key)
		noError(t, err)
		assertEqual(t, expectedValue, cachedValue)
	}
}

func TestConstructCacheWithDefaultHasher(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             16,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
	})

	_, ok := cache.hash.(fnv64a)
	assertEqual(t, true, ok)
}

func TestWillReturnErrorOnInvalidNumberOfPartitions(t *testing.T) {
	t.Parallel()

	// given
	cache, error := NewBigCache(Config{
		Shards:             18,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
	})

	assertEqual(t, (*BigCache)(nil), cache)
	assertEqual(t, "Shards number must be power of two", error.Error())
}

func TestEntryNotFound(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             16,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
	})

	// when
	_, err := cache.Get("nonExistingKey")

	// then
	assertEqual(t, ErrEntryNotFound, err)
}

func TestTimingEviction(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))
	_, err := cache.Get("key")

	// then
	assertEqual(t, ErrEntryNotFound, err)
}

func TestTimingEvictionShouldEvictOnlyFromUpdatedShard(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             4,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value 2"))
	value, err := cache.Get("key")

	// then
	noError(t, err)
	assertEqual(t, []byte("value"), value)
}

func TestCleanShouldEvictAll(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             4,
		LifeWindow:         time.Second,
		CleanWindow:        time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})

	// when
	cache.Set("key", []byte("value"))
	<-time.After(3 * time.Second)
	value, err := cache.Get("key")

	// then
	assertEqual(t, ErrEntryNotFound, err)
	assertEqual(t, value, []byte(nil))
}

func TestOnRemoveCallback(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemoveExtInvoked := false
	onRemove := func(key string, entry []byte) {
		onRemoveInvoked = true
		assertEqual(t, "key", key)
		assertEqual(t, []byte("value"), entry)
	}
	onRemoveExt := func(key string, entry []byte, reason RemoveReason) {
		onRemoveExtInvoked = true
	}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
		OnRemove:           onRemove,
		OnRemoveWithReason: onRemoveExt,
	}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))

	// then
	assertEqual(t, true, onRemoveInvoked)
	assertEqual(t, false, onRemoveExtInvoked)
}

func TestOnRemoveWithReasonCallback(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemove := func(key string, entry []byte, reason RemoveReason) {
		onRemoveInvoked = true
		assertEqual(t, "key", key)
		assertEqual(t, []byte("value"), entry)
		assertEqual(t, reason, RemoveReason(Expired))
	}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
		OnRemoveWithReason: onRemove,
	}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))

	// then
	assertEqual(t, true, onRemoveInvoked)
}

func TestOnRemoveFilter(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemove := func(key string, entry []byte, reason RemoveReason) {
		onRemoveInvoked = true
	}
	c := Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
		OnRemoveWithReason: onRemove,
	}.OnRemoveFilterSet(Deleted, NoSpace)

	cache, _ := newBigCache(c, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))

	// then
	assertEqual(t, false, onRemoveInvoked)

	// and when
	cache.Delete("key2")

	// then
	assertEqual(t, true, onRemoveInvoked)
}

func TestOnRemoveFilterExpired(t *testing.T) {
	// t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveDeleted, onRemoveExpired := false, false
	var err error
	onRemove := func(key string, entry []byte, reason RemoveReason) {
		switch reason {

		case Deleted:
			onRemoveDeleted = true
		case Expired:
			onRemoveExpired = true

		}
	}
	c := Config{
		Shards:             1,
		LifeWindow:         3 * time.Second,
		CleanWindow:        0,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
		OnRemoveWithReason: onRemove,
	}

	cache, err := newBigCache(c, &clock)
	assertEqual(t, err, nil)

	// case 1: key is deleted AFTER expire
	// when
	onRemoveDeleted, onRemoveExpired = false, false
	clock.set(0)

	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.cleanUp(uint64(clock.Epoch()))

	err = cache.Delete("key")

	// then
	assertEqual(t, err, ErrEntryNotFound)
	assertEqual(t, false, onRemoveDeleted)
	assertEqual(t, true, onRemoveExpired)

	// case 1: key is deleted BEFORE expire
	// when
	onRemoveDeleted, onRemoveExpired = false, false
	clock.set(0)

	cache.Set("key2", []byte("value2"))
	err = cache.Delete("key2")
	clock.set(5)
	cache.cleanUp(uint64(clock.Epoch()))
	// then

	assertEqual(t, err, nil)
	assertEqual(t, true, onRemoveDeleted)
	assertEqual(t, false, onRemoveExpired)
}

func TestOnRemoveGetEntryStats(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	count := uint32(0)
	onRemove := func(key string, entry []byte, keyMetadata Metadata) {
		count = keyMetadata.RequestCount
	}
	c := Config{
		Shards:               1,
		LifeWindow:           time.Second,
		MaxEntriesInWindow:   1,
		MaxEntrySize:         256,
		OnRemoveWithMetadata: onRemove,
		StatsEnabled:         true,
	}.OnRemoveFilterSet(Deleted, NoSpace)

	cache, _ := newBigCache(c, &clock)

	// when
	cache.Set("key", []byte("value"))

	for i := 0; i < 100; i++ {
		cache.Get("key")
	}

	cache.Delete("key")

	// then
	assertEqual(t, uint32(100), count)
}

func TestCacheLen(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assertEqual(t, keys, cache.Len())
}

func TestCacheCapacity(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assertEqual(t, keys, cache.Len())
	assertEqual(t, 40960, cache.Capacity())
}

func TestCacheInitialCapacity(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 2 * 1024,
		HardMaxCacheSize:   1,
		MaxEntrySize:       1024,
	})

	assertEqual(t, 0, cache.Len())
	assertEqual(t, 1024*1024, cache.Capacity())

	keys := 1024 * 1024

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assertEqual(t, true, cache.Len() < keys)
	assertEqual(t, 1024*1024, cache.Capacity())
}

func TestRemoveEntriesWhenShardIsFull(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         100 * time.Second,
		MaxEntriesInWindow: 100,
		MaxEntrySize:       256,
		HardMaxCacheSize:   1,
	})

	value := blob('a', 1024*300)

	// when
	cache.Set("key", value)
	cache.Set("key", value)
	cache.Set("key", value)
	cache.Set("key", value)
	cache.Set("key", value)
	cachedValue, err := cache.Get("key")

	// then
	noError(t, err)
	assertEqual(t, value, cachedValue)
}

func TestCacheStats(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})

	// when
	for i := 0; i < 100; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	for i := 0; i < 10; i++ {
		value, err := cache.Get(fmt.Sprintf("key%d", i))
		noError(t, err)
		assertEqual(t, string(value), "value")
	}
	for i := 100; i < 110; i++ {
		_, err := cache.Get(fmt.Sprintf("key%d", i))
		assertEqual(t, ErrEntryNotFound, err)
	}
	for i := 10; i < 20; i++ {
		err := cache.Delete(fmt.Sprintf("key%d", i))
		noError(t, err)
	}
	for i := 110; i < 120; i++ {
		err := cache.Delete(fmt.Sprintf("key%d", i))
		assertEqual(t, ErrEntryNotFound, err)
	}

	// then
	stats := cache.Stats()
	assertEqual(t, stats.Hits, int64(10))
	assertEqual(t, stats.Misses, int64(10))
	assertEqual(t, stats.DelHits, int64(10))
	assertEqual(t, stats.DelMisses, int64(10))
}
func TestCacheEntryStats(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
		StatsEnabled:       true,
	})

	cache.Set("key0", []byte("value"))

	for i := 0; i < 10; i++ {
		_, err := cache.Get("key0")
		noError(t, err)
	}

	// then
	keyMetadata := cache.KeyMetadata("key0")
	assertEqual(t, uint32(10), keyMetadata.RequestCount)
}

func TestCacheDel(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(time.Second))

	// when
	err := cache.Delete("nonExistingKey")

	// then
	assertEqual(t, err, ErrEntryNotFound)

	// and when
	cache.Set("existingKey", nil)
	err = cache.Delete("existingKey")
	cachedValue, _ := cache.Get("existingKey")

	// then
	noError(t, err)
	assertEqual(t, 0, len(cachedValue))
}

// TestCacheDelRandomly does simultaneous deletes, puts and gets, to check for corruption errors.
func TestCacheDelRandomly(t *testing.T) {
	t.Parallel()

	c := Config{
		Shards:             1,
		LifeWindow:         time.Second,
		CleanWindow:        0,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       10,
		Verbose:            false,
		Hasher:             newDefaultHasher(),
		HardMaxCacheSize:   1,
		StatsEnabled:       true,
		Logger:             DefaultLogger(),
	}

	cache, _ := NewBigCache(c)
	var wg sync.WaitGroup
	var ntest = 800000
	wg.Add(3)
	go func() {
		for i := 0; i < ntest; i++ {
			r := uint8(rand.Int())
			key := fmt.Sprintf("thekey%d", r)

			cache.Delete(key)
		}
		wg.Done()
	}()
	valueLen := 1024
	go func() {
		val := make([]byte, valueLen)
		for i := 0; i < ntest; i++ {
			r := byte(rand.Int())
			key := fmt.Sprintf("thekey%d", r)

			for j := 0; j < len(val); j++ {
				val[j] = r
			}
			cache.Set(key, val)
		}
		wg.Done()
	}()
	go func() {
		val := make([]byte, valueLen)
		for i := 0; i < ntest; i++ {
			r := byte(rand.Int())
			key := fmt.Sprintf("thekey%d", r)

			for j := 0; j < len(val); j++ {
				val[j] = r
			}
			if got, err := cache.Get(key); err == nil && !bytes.Equal(got, val) {
				t.Errorf("got %s ->\n %x\n expected:\n %x\n ", key, got, val)
			}
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestWriteAndReadParallelSameKeyWithStats(t *testing.T) {
	t.Parallel()

	c := DefaultConfig(0)
	c.StatsEnabled = true

	cache, _ := NewBigCache(c)
	var wg sync.WaitGroup
	ntest := 1000
	n := 10
	wg.Add(n)
	key := "key"
	value := blob('a', 1024)
	for i := 0; i < ntest; i++ {
		assertEqual(t, nil, cache.Set(key, value))
	}
	for j := 0; j < n; j++ {
		go func() {
			for i := 0; i < ntest; i++ {
				v, err := cache.Get(key)
				assertEqual(t, nil, err)
				assertEqual(t, value, v)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	assertEqual(t, Stats{Hits: int64(n * ntest)}, cache.Stats())
	assertEqual(t, ntest*n, int(cache.KeyMetadata(key).RequestCount))
}

func TestCacheReset(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assertEqual(t, keys, cache.Len())

	// and when
	cache.Reset()

	// then
	assertEqual(t, 0, cache.Len())

	// and when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assertEqual(t, keys, cache.Len())
}

func TestIterateOnResetCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}
	cache.Reset()

	// then
	iterator := cache.Iterator()

	assertEqual(t, false, iterator.SetNext())
}

func TestGetOnResetCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             8,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	cache.Reset()

	// then
	value, err := cache.Get("key1")

	assertEqual(t, err, ErrEntryNotFound)
	assertEqual(t, value, []byte(nil))
}

func TestEntryUpdate(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         6 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
	}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key", []byte("value2"))
	clock.set(7)
	cache.Set("key2", []byte("value3"))
	cachedValue, _ := cache.Get("key")

	// then
	assertEqual(t, []byte("value2"), cachedValue)
}

func TestOldestEntryDeletionWhenMaxCacheSizeIsReached(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
	})

	// when
	cache.Set("key1", blob('a', 1024*400))
	cache.Set("key2", blob('b', 1024*400))
	cache.Set("key3", blob('c', 1024*800))

	_, key1Err := cache.Get("key1")
	_, key2Err := cache.Get("key2")
	entry3, _ := cache.Get("key3")

	// then
	assertEqual(t, key1Err, ErrEntryNotFound)
	assertEqual(t, key2Err, ErrEntryNotFound)
	assertEqual(t, blob('c', 1024*800), entry3)
}

func TestRetrievingEntryShouldCopy(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
	})
	cache.Set("key1", blob('a', 1024*400))
	value, key1Err := cache.Get("key1")

	// when
	// override queue
	cache.Set("key2", blob('b', 1024*400))
	cache.Set("key3", blob('c', 1024*400))
	cache.Set("key4", blob('d', 1024*400))
	cache.Set("key5", blob('d', 1024*400))

	// then
	noError(t, key1Err)
	assertEqual(t, blob('a', 1024*400), value)
}

func TestEntryBiggerThanMaxShardSizeError(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
	})

	// when
	err := cache.Set("key1", blob('a', 1024*1025))

	// then
	assertEqual(t, "entry is bigger than max shard size", err.Error())
}

func TestHashCollision(t *testing.T) {
	t.Parallel()

	ml := &mockedLogger{}
	// given
	cache, _ := NewBigCache(Config{
		Shards:             16,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       256,
		Verbose:            true,
		Hasher:             hashStub(5),
		Logger:             ml,
	})

	// when
	cache.Set("liquid", []byte("value"))
	cachedValue, err := cache.Get("liquid")

	// then
	noError(t, err)
	assertEqual(t, []byte("value"), cachedValue)

	// when
	cache.Set("costarring", []byte("value 2"))
	cachedValue, err = cache.Get("costarring")

	// then
	noError(t, err)
	assertEqual(t, []byte("value 2"), cachedValue)

	// when
	cachedValue, err = cache.Get("liquid")

	// then
	assertEqual(t, ErrEntryNotFound, err)
	assertEqual(t, []byte(nil), cachedValue)

	assertEqual(t, "Collision detected. Both %q and %q have the same hash %x", ml.lastFormat)
	assertEqual(t, cache.Stats().Collisions, int64(1))
}

func TestNilValueCaching(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
	})

	// when
	cache.Set("Kierkegaard", []byte{})
	cachedValue, err := cache.Get("Kierkegaard")

	// then
	noError(t, err)
	assertEqual(t, []byte{}, cachedValue)

	// when
	cache.Set("Sartre", nil)
	cachedValue, err = cache.Get("Sartre")

	// then
	noError(t, err)
	assertEqual(t, []byte{}, cachedValue)

	// when
	cache.Set("Nietzsche", []byte(nil))
	cachedValue, err = cache.Get("Nietzsche")

	// then
	noError(t, err)
	assertEqual(t, []byte{}, cachedValue)
}

func TestClosing(t *testing.T) {
	// given
	config := Config{
		CleanWindow: time.Minute,
	}
	startGR := runtime.NumGoroutine()

	// when
	for i := 0; i < 100; i++ {
		cache, _ := NewBigCache(config)
		cache.Close()
	}

	// wait till all goroutines are stopped.
	time.Sleep(200 * time.Millisecond)

	// then
	endGR := runtime.NumGoroutine()
	assertEqual(t, true, endGR >= startGR)
	assertEqual(t, true, math.Abs(float64(endGR-startGR)) < 25)
}

func TestEntryNotPresent(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
	}, &clock)

	// when
	value, resp, err := cache.GetWithInfo("blah")
	assertEqual(t, ErrEntryNotFound, err)
	assertEqual(t, resp.EntryStatus, RemoveReason(0))
	assertEqual(t, cache.Stats().Misses, int64(1))
	assertEqual(t, []byte(nil), value)
}

func TestBigCache_GetWithInfo(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		CleanWindow:        5 * time.Minute,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       1,
		HardMaxCacheSize:   1,
		Verbose:            true,
	}, &clock)
	key := "deadEntryKey"
	value := "100"
	cache.Set(key, []byte(value))

	// when
	data, resp, err := cache.GetWithInfo(key)

	// then
	assertEqual(t, []byte(value), data)
	noError(t, err)
	assertEqual(t, Response{}, resp)

	// when
	clock.set(5)
	data, resp, err = cache.GetWithInfo(key)

	// then
	assertEqual(t, err, nil)
	assertEqual(t, Response{EntryStatus: Expired}, resp)
	assertEqual(t, []byte(value), data)
}

type mockedLogger struct {
	lastFormat string
	lastArgs   []interface{}
}

func (ml *mockedLogger) Printf(format string, v ...interface{}) {
	ml.lastFormat = format
	ml.lastArgs = v
}

type mockedClock struct {
	value int64
}

func (mc *mockedClock) Epoch() int64 {
	return mc.value
}

func (mc *mockedClock) set(value int64) {
	mc.value = value
}

func blob(char byte, len int) []byte {
	return bytes.Repeat([]byte{char}, len)
}
