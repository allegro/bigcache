package bigcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriteAndGetOnCache(t *testing.T) {
	t.Parallel()

	// given
	cache := NewBigCache(Config{10, 5 * time.Second, 10, 256, false})
	value := []byte("value")

	// when
	cache.Set("key", value)
	cachedValue, err := cache.Get("key")

	// then
	assert.NoError(t, err)
	assert.Equal(t, value, cachedValue)
}

func TestEntryNotFound(t *testing.T) {
	t.Parallel()

	// given
	cache := NewBigCache(Config{10, 5 * time.Second, 10, 256, false})

	// when
	_, err := cache.Get("nonExistingKey")

	// then
	assert.EqualError(t, err, "Entry \"nonExistingKey\" not found")
}

func TestTimingEviction(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache := newBigCache(Config{1, time.Second, 1, 256, false}, &clock)

	// when
	cache.Set("key", []byte("value"))
	clock.set(5)
	cache.Set("key2", []byte("value2"))
	_, err := cache.Get("key")

	// then
	assert.EqualError(t, err, "Entry \"key\" not found")
}

func TestEntryUpdate(t *testing.T) {
	t.Parallel()

	// given
	clock := mockedClock{value: 0}
	cache := newBigCache(Config{1, 6 * time.Second, 1, 256, false}, &clock)

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

func TestHashCollision(t *testing.T) {
	t.Parallel()

	// given
	cache := NewBigCache(Config{10, 5 * time.Second, 10, 256, true})
	cache.hash = hashStub(5)

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
