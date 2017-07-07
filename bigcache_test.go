package bigcache

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	assert.Equal(t, value, cachedValue)
}

func TestConstructCacheWithDefaultHasher(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, false, nil, 0, nil, false, nil})

	assert.IsType(t, fnv64a{}, cache.hash)
}

func TestWillReturnErrorOnInvalidNumberOfPartitions(t *testing.T) {
	t.Parallel()

	// given
	cache, error := NewBigCache(Config{18, 5 * time.Second, 10, 256, false, nil, 0, nil, false, nil})

	assert.Nil(t, cache)
	assert.Error(t, error, "shards number must be power of two")
}

func TestEntryNotFound(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, false, nil, 0, nil, false, nil})

	// when
	_, err := cache.Get("nonExistingKey")

	// then
	assert.EqualError(t, err, "Entry \"nonExistingKey\" not found")
}

func TestTimingEviction(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{1, time.Second, 1, 256, false, nil, 0, nil, false, nil}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))
	_, err := cache.Get("key")

	// then
	assert.EqualError(t, err, "Entry \"key\" not found")
}

func TestTimingEvictionShouldEvictOnlyFromUpdatedShard(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{4, time.Second, 1, 256, false, nil, 0, nil, false, nil}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value 2"))
	value, err := cache.Get("key")

	// then
	assert.NoError(t, err, "Entry \"key\" not found")
	assert.Equal(t, []byte("value"), value)
}

func TestOnRemoveCallback(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	onRemoveInvoked := false
	onRemove := func(key string, entry []byte) {
		onRemoveInvoked = true
		assert.Equal(t, "key", key)
		assert.Equal(t, []byte("value"), entry)
	}
	cache, _ := newBigCache(Config{1, time.Second, 1, 256, false, nil, 0, onRemove, false, nil}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))

	// then
	assert.True(t, onRemoveInvoked)
}

func TestCacheLen(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{8, time.Second, 1, 256, false, nil, 0, nil, false, nil})
	keys := 1337
	// when

	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	// then
	assert.Equal(t, keys, cache.Len())
}

func TestCacheReset(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{8, time.Second, 1, 256, false, nil, 0, nil, false, nil})
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
	cache, _ := NewBigCache(Config{8, time.Second, 1, 256, false, nil, 0, nil, false, nil})
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
	cache, _ := NewBigCache(Config{8, time.Second, 1, 256, false, nil, 0, nil, false, nil})
	keys := 1337

	// when
	for i := 0; i < keys; i++ {
		cache.Set(fmt.Sprintf("key%d", i), []byte("value"))
	}

	cache.Reset()

	// then
	value, err := cache.Get("key1")

	assert.Equal(t, err.Error(), "Entry \"key1\" not found")
	assert.Equal(t, value, []byte(nil))
}

func TestEntryUpdate(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{1, 6 * time.Second, 1, 256, false, nil, 0, nil, false, nil}, &clock)

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
	cache, _ := NewBigCache(Config{1, 5 * time.Second, 1, 1, false, nil, 1, nil, false, nil})

	// when
	cache.Set("key1", blob('a', 1024*400))
	cache.Set("key2", blob('b', 1024*400))
	cache.Set("key3", blob('c', 1024*800))

	_, key1Err := cache.Get("key1")
	_, key2Err := cache.Get("key2")
	entry3, _ := cache.Get("key3")

	// then
	assert.EqualError(t, key1Err, "Entry \"key1\" not found")
	assert.EqualError(t, key2Err, "Entry \"key2\" not found")
	assert.Equal(t, blob('c', 1024*800), entry3)
}

func TestRetrievingEntryShouldCopy(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{1, 5 * time.Second, 1, 1, false, nil, 1, nil, false, nil})
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
	cache, _ := NewBigCache(Config{1, 5 * time.Second, 1, 1, false, nil, 1, nil, false, nil})

	// when
	err := cache.Set("key1", blob('a', 1024*1025))

	// then
	assert.EqualError(t, err, "Entry is bigger than max shard size.")
}

func TestHashCollision(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, true, hashStub(5), 0, nil, false, nil})

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
}

func TestBigCacheFlushOnEviction(t *testing.T) {
	t.Parallel()

	// TODO (mxplusb): need to figure out why flush isn't writing to disk on eviction.

	// test locally to disk.
	tmpFile, err := os.Create("TestBigCacheFlushOnEviction.json")
	if err != nil {
		t.Fatal(err)
	}

	// new timer so we can wait.
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	// given
	cache, err := NewBigCache(Config{16, 1 * time.Second, 10, 256, true, hashStub(5), 0, nil, true, tmpFile})
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err = cache.Set("testGobWriter", []byte("testData")); err != nil {
		t.Fatal(err)
	}

	// then wait for eviction
	<-timer.C

	// cleanup
	tmpFile.Close()
	if err = os.Remove("TestBigCacheFlushOnEviction.json"); err != nil {
		t.Fatal(err)
	}
}

func TestBigCacheFlush(t *testing.T) {
	t.Parallel()

	// test locally to disk.
	tmpFile, err := os.Create("TestBigCacheFlush.json")
	if err != nil {
		t.Fatal(err)
	}

	// given
	cache, err := NewBigCache(Config{16, 3 * time.Second, 10, 256, true, hashStub(5), 0, nil, false, tmpFile})
	if err != nil {
		t.Fatal(err)
	}

	// when
	if err = cache.Set("testGobWriter", []byte("testData")); err != nil {
		t.Fatal(err)
	}

	// then
	if err = cache.Flush(); err != nil {
		t.Fatal(err)
	}

	// then
	tmpFile.Close()

	// then make sure it was written to disk
	file, _ := os.Open("TestBigCacheFlush.json")
	fInfo, _ := file.Stat()
	if fInfo.Size() == 0 {
		t.Fatal("Cache not written to disk on Flush()")
	}
	file.Close()

	if err = os.Remove("TestBigCacheFlush.json"); err != nil {
		t.Fatal(err)
	}
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
