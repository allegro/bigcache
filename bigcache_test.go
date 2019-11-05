package bigcache

import (
	"bytes"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var sink []byte

func TestParallel(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(5 * time.Second))
	value := []byte("value")
	var wg sync.WaitGroup
	wg.Add(3)
	keys := 1337

	// when
	go func() {
		defer wg.Done()
		for i := 0; i < keys; i++ {
			cache.Set(fmt.Sprintf("key%d", i), value)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < keys; i++ {
			sink, _ = cache.Get(fmt.Sprintf("key%d", i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < keys; i++ {
			cache.Delete(fmt.Sprintf("key%d", i))
		}
	}()

	// then
	wg.Wait()
}

func TestWriteAndGetOnCache(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(5 * time.Second))
	value := []byte("value")

	// when
	cache.Set("key", value)
	cachedValue, err := cache.Get("key")

	// then
	assert.NoError(t, err)
	assert.Equal(t, value, cachedValue)
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

	assert.IsType(t, fnv64a{}, cache.hash)
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

	assert.Nil(t, cache)
	assert.Error(t, error, "Shards number must be power of two")
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
	assert.EqualError(t, err, ErrEntryNotFound.Error())
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
	assert.EqualError(t, err, ErrEntryNotFound.Error())
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
	assert.NoError(t, err, ErrEntryNotFound.Error())
	assert.Equal(t, []byte("value"), value)
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
	assert.EqualError(t, err, ErrEntryNotFound.Error())
	assert.Equal(t, value, []byte(nil))
}

func TestOnRemoveCallback(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemoveExtInvoked := false
	onRemove := func(key string, entry []byte) {
		onRemoveInvoked = true
		assert.Equal(t, "key", key)
		assert.Equal(t, []byte("value"), entry)
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
	assert.True(t, onRemoveInvoked)
	assert.False(t, onRemoveExtInvoked)
}

func TestOnRemoveWithReasonCallback(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemove := func(key string, entry []byte, reason RemoveReason) {
		onRemoveInvoked = true
		assert.Equal(t, "key", key)
		assert.Equal(t, []byte("value"), entry)
		assert.Equal(t, reason, RemoveReason(Expired))
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
	assert.True(t, onRemoveInvoked)
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
	assert.False(t, onRemoveInvoked)

	// and when
	cache.Delete("key2")

	// then
	assert.True(t, onRemoveInvoked)
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
	assert.Equal(t, uint32(100), count)
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
	assert.Equal(t, keys, cache.Len())
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
	assert.Equal(t, keys, cache.Len())
	assert.Equal(t, 81920, cache.Capacity())
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
		assert.Nil(t, err)
		assert.Equal(t, string(value), "value")
	}
	for i := 100; i < 110; i++ {
		_, err := cache.Get(fmt.Sprintf("key%d", i))
		assert.Error(t, err)
	}
	for i := 10; i < 20; i++ {
		err := cache.Delete(fmt.Sprintf("key%d", i))
		assert.Nil(t, err)
	}
	for i := 110; i < 120; i++ {
		err := cache.Delete(fmt.Sprintf("key%d", i))
		assert.Error(t, err)
	}

	// then
	stats := cache.Stats()
	assert.Equal(t, stats.Hits, int64(10))
	assert.Equal(t, stats.Misses, int64(10))
	assert.Equal(t, stats.DelHits, int64(10))
	assert.Equal(t, stats.DelMisses, int64(10))
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
		assert.Nil(t, err)
	}

	// then
	keyMetadata := cache.KeyMetadata("key0")
	assert.Equal(t, uint32(10), keyMetadata.RequestCount)
}

func TestCacheDel(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(DefaultConfig(time.Second))

	// when
	err := cache.Delete("nonExistingKey")

	// then
	assert.Equal(t, err.Error(), ErrEntryNotFound.Error())

	// and when
	cache.Set("existingKey", nil)
	err = cache.Delete("existingKey")
	cachedValue, _ := cache.Get("existingKey")

	// then
	assert.Nil(t, err)
	assert.Len(t, cachedValue, 0)
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
		Verbose:            true,
		Hasher:             newDefaultHasher(),
		HardMaxCacheSize:   1,
		Logger:             DefaultLogger(),
	}
	//c.Hasher = hashStub(5)
	cache, _ := NewBigCache(c)
	var wg sync.WaitGroup
	var ntest = 800000
	wg.Add(1)
	go func() {
		for i := 0; i < ntest; i++ {
			r := uint8(rand.Int())
			key := fmt.Sprintf("thekey%d", r)

			cache.Delete(key)
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		val := make([]byte, 1024)
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
	wg.Add(1)
	go func() {
		val := make([]byte, 1024)
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
	assert.Equal(t, keys, cache.Len())

	// and when
	cache.Reset()

	// then
	assert.Equal(t, 0, cache.Len())

	// and when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assert.Equal(t, keys, cache.Len())
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

	assert.Equal(t, false, iterator.SetNext())
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

	assert.Equal(t, err.Error(), ErrEntryNotFound.Error())
	assert.Equal(t, value, []byte(nil))
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
	assert.Equal(t, []byte("value2"), cachedValue)
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
	assert.EqualError(t, key1Err, ErrEntryNotFound.Error())
	assert.EqualError(t, key2Err, ErrEntryNotFound.Error())
	assert.Equal(t, blob('c', 1024*800), entry3)
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
	assert.Nil(t, key1Err)
	assert.Equal(t, blob('a', 1024*400), value)
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
	assert.EqualError(t, err, "entry is bigger than max shard size")
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
	assert.NoError(t, err)
	assert.Equal(t, []byte("value"), cachedValue)

	// when
	cache.Set("costarring", []byte("value 2"))
	cachedValue, err = cache.Get("costarring")

	// then
	assert.NoError(t, err)
	assert.Equal(t, []byte("value 2"), cachedValue)

	// when
	cachedValue, err = cache.Get("liquid")

	// then
	assert.Error(t, err)
	assert.Nil(t, cachedValue)

	assert.NotEqual(t, "", ml.lastFormat)
	assert.Equal(t, cache.Stats().Collisions, int64(1))
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
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, cachedValue)

	// when
	cache.Set("Sartre", nil)
	cachedValue, err = cache.Get("Sartre")

	// then
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, cachedValue)

	// when
	cache.Set("Nietzsche", []byte(nil))
	cachedValue, err = cache.Get("Nietzsche")

	// then
	assert.NoError(t, err)
	assert.Equal(t, []byte{}, cachedValue)
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
	assert.True(t, endGR >= startGR)
	assert.InDelta(t, endGR, startGR, 25)
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
	assert.Error(t, err)
	assert.Equal(t, resp.EntryStatus, RemoveReason(0))
	assert.Equal(t, cache.Stats().Misses, int64(1))
	assert.Nil(t, value)
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
	assert.Equal(t, []byte(value), data)
	assert.NoError(t, err)
	assert.Equal(t, Response{}, resp)
	clock.set(5)
	data, resp, err = cache.GetWithInfo(key)
	assert.Equal(t, err, ErrEntryIsDead)
	assert.Equal(t, Response{EntryStatus: Expired}, resp)
	assert.Nil(t, data)
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

func (mc *mockedClock) epoch() int64 {
	return mc.value
}

func (mc *mockedClock) set(value int64) {
	mc.value = value
}

func blob(char byte, len int) []byte {
	b := make([]byte, len)
	for index := range b {
		b[index] = char
	}
	return b
}
