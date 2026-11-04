package bigcache

import (
	"context"
	"testing"
	"time"
)

func TestAppendCollisionSurvivesOldEntryExpiration(t *testing.T) {
	t.Parallel()

	clock := mockedClock{value: 100}
	var removed []string
	cache, err := newBigCache(context.Background(), Config{
		Shards:             1,
		LifeWindow:         5 * time.Second,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       64,
		Hasher:             hashStub(1),
		OnRemove: func(key string, _ []byte) {
			removed = append(removed, key)
		},
	}, &clock)
	noError(t, err)
	defer cache.Close()

	noError(t, cache.Set("old", []byte("old value")))
	clock.set(101)
	noError(t, cache.Append("new", []byte("new value")))

	cache.cleanUp(105)

	value, err := cache.Get("new")
	noError(t, err)
	assertEqual(t, []byte("new value"), value)
	assertEqual(t, 1, cache.Len())
	assertEqual(t, 0, len(removed))

	cache.cleanUp(106)
	_, err = cache.Get("new")
	assertEqual(t, ErrEntryNotFound, err)
	assertEqual(t, []string{"new"}, removed)
}
