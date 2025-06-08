package bigcache

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"
)

// BenchmarkCleanupMethods compares the performance of regular and non-blocking cleanup
func BenchmarkCleanupMethods(b *testing.B) {
	benchmarks := []struct {
		name        string
		nonBlocking bool
	}{
		{"SynchronousCleanup", false},
		{"NonBlockingCleanup", true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			config := Config{
				Shards:             1024,
				LifeWindow:         5 * time.Second,
				CleanWindow:        0, // We'll manually trigger cleanup
				MaxEntriesInWindow: 1000 * 10 * 60,
				MaxEntrySize:       500,
				Verbose:            false,
				HardMaxCacheSize:   8,
			}

			cache, _ := New(context.Background(), config)
			defer cache.Close()

			// Prepare test data
			data := make([]byte, 400)

			// Fill the cache with data
			for i := 0; i < 10000; i++ {
				key := strconv.Itoa(i)
				cache.Set(key, data)
			}

			b.ResetTimer()

			// Benchmark concurrent operations while cleanup is happening
			b.RunParallel(func(pb *testing.PB) {
				counter := 0

				for pb.Next() {
					// Every 1000 iterations, trigger the cleanup method we're testing
					if counter%1000 == 0 {
						currentTime := uint64(time.Now().Unix())
						if bm.nonBlocking {
							cache.cleanUpNonBlocking()
						} else {
							cache.cleanUp(currentTime)
						}
					}

					// Perform normal cache operations
					key := strconv.Itoa(counter % 10000)
					cache.Set(key, data)
					cache.Get(key)

					counter++
				}
			})
		})
	}
}

// BenchmarkHeavyLoadWithCleanup simulates a real-world scenario with heavy load
func BenchmarkHeavyLoadWithCleanup(b *testing.B) {
	benchmarks := []struct {
		name        string
		nonBlocking bool
	}{
		{"HeavyLoad_SynchronousCleanup", false},
		{"HeavyLoad_NonBlockingCleanup", true},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			config := Config{
				Shards:             1024,
				LifeWindow:         5 * time.Second,
				CleanWindow:        0, // We'll manually trigger cleanup
				MaxEntriesInWindow: 1000 * 10 * 60,
				MaxEntrySize:       500,
				Verbose:            false,
				HardMaxCacheSize:   8,
			}

			cache, _ := New(context.Background(), config)
			defer cache.Close()

			// Prepare test data
			data := make([]byte, 400)

			// Pre-fill the cache
			for i := 0; i < 5000; i++ {
				key := strconv.Itoa(i)
				cache.Set(key, data)
			}

			b.ResetTimer()

			// Start a separate goroutine to periodically trigger cleanup
			stopCleanup := make(chan struct{})
			var cleanupWg sync.WaitGroup
			cleanupWg.Add(1)

			go func() {
				defer cleanupWg.Done()
				cleanupTicker := time.NewTicker(50 * time.Millisecond)
				defer cleanupTicker.Stop()

				for {
					select {
					case <-cleanupTicker.C:
						currentTime := uint64(time.Now().Unix())
						if bm.nonBlocking {
							cache.cleanUpNonBlocking()
						} else {
							cache.cleanUp(currentTime)
						}
					case <-stopCleanup:
						return
					}
				}
			}()

			// Run the benchmark
			b.RunParallel(func(pb *testing.PB) {
				counter := 0
				localData := make([]byte, 400)

				for pb.Next() {
					// Mix of operations: 80% writes, 20% reads
					key := strconv.Itoa(counter % 10000)

					if counter%5 != 0 {
						cache.Set(key, localData)
					} else {
						cache.Get(key)
					}

					counter++
				}
			})

			b.StopTimer()
			close(stopCleanup)
			cleanupWg.Wait()
		})
	}
}
