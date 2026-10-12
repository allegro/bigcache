package bigcache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestOnRemoveReinsertDoesNotStopLaterCallbacks(t *testing.T) {
	clock := mockedClock{value: 0}
	var cache *BigCache
	var mu sync.Mutex
	var calls []string
	cache, err := newBigCache(context.Background(), Config{
		Shards:             1,
		LifeWindow:         time.Second,
		MaxEntriesInWindow: 1,
		MaxEntrySize:       256,
		OnRemoveWithReason: func(key string, entry []byte, reason RemoveReason) {
			mu.Lock()
			calls = append(calls, key)
			mu.Unlock()
			if key == "a" {
				if err := cache.Set("a", append([]byte(nil), entry...)); err != nil {
					t.Errorf("reinsert: %v", err)
				}
			}
		},
	}, &clock)
	if err != nil {
		t.Fatal(err)
	}

	if err = cache.Set("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	clock.set(5)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := cache.Set("b", []byte("2")); err != nil {
			t.Errorf("set b: %v", err)
		}
		clock.set(10)
		if err := cache.Set("c", []byte("3")); err != nil {
			t.Errorf("set c: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("OnRemove reinsert blocked later eviction")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) < 2 {
		t.Fatalf("callbacks = %v, want a later eviction after the reinsert", calls)
	}
}
