package bigcache

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestSetEntryThatFitsAfterGrowingToShardLimit(t *testing.T) {
	t.Parallel()

	cache, err := New(context.Background(), Config{
		Shards:             1,
		LifeWindow:         time.Hour,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       64 * 1024,
		HardMaxCacheSize:   1,
	})
	noError(t, err)
	defer cache.Close()

	value := bytes.Repeat([]byte("v"), 700*1024)
	if err := cache.Set("key", value); err != nil {
		t.Fatal(err)
	}
	actual, err := cache.Get("key")
	noError(t, err)
	assertEqual(t, value, actual)
}
