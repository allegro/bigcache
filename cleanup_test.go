package bigcache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestNonBlockingCleanup verifies that the non-blocking cleanup doesn't interfere with normal operations
func TestNonBlockingCleanup(t *testing.T) {
	t.Parallel()

	// Given
	config := DefaultConfig(time.Second)
	config.CleanWindow = 0 // We'll trigger cleanup manually
	config.EnableNonBlockingCleanup = true
	cache, _ := New(context.Background(), config)
	defer cache.Close()

	// Fill cache with data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%d", i)
		cache.Set(key, []byte("value"))
	}

	// When - simulate concurrent operations during cleanup
	var wg sync.WaitGroup
	wg.Add(2)

	// Start cleanup in one goroutine
	go func() {
		defer wg.Done()
		cache.cleanUpNonBlocking()
	}()

	// Access cache concurrently from another goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("key-%d", i)
			cache.Set(key, []byte("new-value"))

			// Try to get the key we just set
			_, err := cache.Get(key)
			if err != nil && err != ErrEntryNotFound {
				t.Errorf("Unexpected error during cache access: %v", err)
			}
		}
	}()

	// Then - both operations should complete without deadlocking
	wg.Wait()
}
