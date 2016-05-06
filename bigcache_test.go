package bigcache

import (
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
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, false, nil, 0, nil})

	assert.IsType(t, fnv64a{}, cache.hash)
}

func TestWillReturnErrorOnInvalidNumberOfPartitions(t *testing.T) {
	t.Parallel()

	// given
	cache, error := NewBigCache(Config{18, 5 * time.Second, 10, 256, false, nil, 0, nil})

	assert.Nil(t, cache)
	assert.Error(t, error, "Shards number must be power of two")
}

func TestEntryNotFound(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, false, nil, 0, nil})

	// when
	_, err := cache.Get("nonExistingKey")

	// then
	assert.EqualError(t, err, "Entry \"nonExistingKey\" not found")
}

func TestTimingEviction(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{1, time.Second, 1, 256, false, nil, 0, nil}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))
	_, err := cache.Get("key")

	// then
	assert.EqualError(t, err, "Entry \"key\" not found")
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
	cache, _ := newBigCache(Config{1, time.Second, 1, 256, false, nil, 0, onRemove}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))

	// then
	assert.True(t, onRemoveInvoked)
}

func TestEntryUpdate(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache, _ := newBigCache(Config{1, 6 * time.Second, 1, 256, false, nil, 0, nil}, &clock)

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
	cache, _ := NewBigCache(Config{1, 5 * time.Second, 1, 1, false, nil, 1, nil})

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

func TestEntryBiggerThanMaxShardSizeError(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{1, 5 * time.Second, 1, 1, false, nil, 1, nil})

	// when
	err := cache.Set("key1", blob('a', 1024*1025))

	// then
	assert.EqualError(t, err, "Entry is bigger than max shard size.")
}

func TestHashCollision(t *testing.T) {
	t.Parallel()

	// given
	cache, _ := NewBigCache(Config{16, 5 * time.Second, 10, 256, true, hashStub(5), 0, nil})

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
