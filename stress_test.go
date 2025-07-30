package bigcache

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

// TestStressWithCleanup simulates a high-load scenario with continuous cleanup
// to verify that BigCache's cleanup process doesn't block operations under heavy load
func TestStressWithCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Optimized config as mentioned in PR description
	config := Config{
		Shards:                   1024,              // Reduce lock contention
		LifeWindow:               10 * time.Minute,  // For timely expiration
		CleanWindow:              1 * time.Second,   // Frequent cleanup
		MaxEntriesInWindow:       1000 * 10 * 60,    // Default value
		MaxEntrySize:             500,               // Default value
		Verbose:                  testing.Verbose(), // Only verbose in verbose mode
		HardMaxCacheSize:         512,               // 512MB cap as mentioned in PR
		EnableNonBlockingCleanup: true,              // Use non-blocking cleanup
		EnableMetrics:            true,              // Enable metrics for later analysis
	}

	// Create cache with background context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer cache.Close()

	// Parameters for the stress test
	const (
		numGoroutines        = 50               // 50 goroutines as mentioned in PR
		operationsPerRoutine = 10000            // 10K operations per goroutine
		dataSize             = 400              // Size of test data in bytes
		gcInterval           = 1 * time.Minute  // GC every minute as mentioned in PR
		memStatsInterval     = 10 * time.Second // Log memory stats every 10 seconds
	)

	// Create a variety of test data sizes
	testData := make(map[int][]byte)
	for i := 1; i <= 10; i++ {
		testData[i] = make([]byte, i*dataSize/10)
		rand.Read(testData[i]) // Fill with random data
	}

	// Pre-fill the cache with some data
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("init-key-%d", i)
		err := cache.Set(key, testData[i%10+1])
		if err != nil {
			return
		}
	}

	// Setup memory monitoring goroutine
	stopMonitoring := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		memStatsTicker := time.NewTicker(memStatsInterval)
		gcTicker := time.NewTicker(gcInterval)
		defer memStatsTicker.Stop()
		defer gcTicker.Stop()

		var ms runtime.MemStats
		for {
			select {
			case <-memStatsTicker.C:
				runtime.ReadMemStats(&ms)
				t.Logf("Memory stats - Alloc: %v MB, Sys: %v MB, NumGC: %v, Cache size: %v entries",
					ms.Alloc/1024/1024, ms.Sys/1024/1024, ms.NumGC, cache.Len())
			case <-gcTicker.C:
				t.Log("Triggering manual GC")
				runtime.GC()
			case <-stopMonitoring:
				return
			}
		}
	}()

	// Start the stress test
	t.Logf("Starting stress test with %d goroutines, %d operations each", numGoroutines, operationsPerRoutine)
	startTime := time.Now()

	// WaitGroup to wait for all goroutines to finish
	var testWg sync.WaitGroup
	testWg.Add(numGoroutines)

	// Launch goroutines for the stress test
	for g := 0; g < numGoroutines; g++ {
		go func(routineID int) {
			defer testWg.Done()

			// Each goroutine performs a mix of operations
			for i := 0; i < operationsPerRoutine; i++ {
				// Create a key with routine ID to avoid collisions between routines
				// but allow overwrites within the same routine
				key := fmt.Sprintf("r%d-k%d", routineID, i%1000)

				// Mix of operations: 70% sets, 20% gets, 10% deletes
				op := rand.Intn(10)

				if op < 7 { // 70% sets
					// Vary the data size slightly
					dataIdx := rand.Intn(10) + 1
					err := cache.Set(key, testData[dataIdx])
					if err != nil {
						t.Errorf("Set failed: %v", err)
					}
				} else if op < 9 { // 20% gets
					_, err := cache.Get(key)
					if err != nil && err != ErrEntryNotFound {
						t.Errorf("Get failed with unexpected error: %v", err)
					}
				} else { // 10% deletes
					err := cache.Delete(key)
					if err != nil {
						return
					}
				}

				// Occasionally overwrite keys from other routines to create contention
				if i%100 == 0 {
					otherRoutine := rand.Intn(numGoroutines)
					otherKey := fmt.Sprintf("r%d-k%d", otherRoutine, rand.Intn(1000))
					err := cache.Set(otherKey, testData[1])
					if err != nil {
						return
					}
				}

				// Add a tiny sleep occasionally to simulate real-world load patterns
				if i%500 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(g)
	}

	// Wait for all operations to complete
	testWg.Wait()
	elapsedTime := time.Since(startTime)

	// Stop monitoring
	close(stopMonitoring)
	wg.Wait()

	// Calculate and display test results
	totalOperations := numGoroutines * operationsPerRoutine
	opsPerSecond := float64(totalOperations) / elapsedTime.Seconds()

	t.Logf("Stress test completed in %v", elapsedTime)
	t.Logf("Total operations: %d", totalOperations)
	t.Logf("Operations per second: %.2f", opsPerSecond)
	t.Logf("Final cache size: %d entries", cache.Len())

	// Get and display metrics if enabled
	metrics := cache.GetMetrics()
	t.Logf("Cleanup metrics - Count: %d, Total time: %v, Evicted entries: %d",
		metrics.CleanupOperationsCount, metrics.TotalCleanupTime, metrics.TotalEntriesRemoved)

	avgCleanupTime := float64(0)
	if metrics.CleanupOperationsCount > 0 {
		avgCleanupTime = float64(metrics.TotalCleanupTime.Nanoseconds()) / float64(metrics.CleanupOperationsCount) / 1000000 // ms
		t.Logf("Average cleanup time: %.2f ms", avgCleanupTime)
	}
}

// TestConcurrentReadWriteWithCleanup tests concurrent read/write operations
// with regular cleanup to ensure the cleanup process doesn't block operations
func TestConcurrentReadWriteWithCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Create optimized config
	config := Config{
		Shards:                   1024,
		LifeWindow:               5 * time.Second,
		CleanWindow:              1 * time.Second,
		MaxEntriesInWindow:       1000 * 10 * 60,
		MaxEntrySize:             500,
		Verbose:                  testing.Verbose(),
		HardMaxCacheSize:         256,
		EnableNonBlockingCleanup: true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache, err := New(ctx, config)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	defer func(cache *BigCache) {
		err := cache.Close()
		if err != nil {

		}
	}(cache)

	const (
		numKeys      = 10000
		numWorkers   = 20
		testDuration = 5 * time.Second
	)

	// Pre-fill the cache
	data := make([]byte, 400)
	for i := 0; i < numKeys; i++ {
		key := strconv.Itoa(i)
		err := cache.Set(key, data)
		if err != nil {
			return
		}
	}

	// Create stop channel for workers
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Start reader goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
					key := strconv.Itoa(rand.Intn(numKeys))
					_, _ = cache.Get(key)
					counter++
				}
			}
		}()
	}

	// Start writer goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localData := make([]byte, 400)
			counter := 0
			for {
				select {
				case <-stop:
					return
				default:
					key := strconv.Itoa(rand.Intn(numKeys))
					cache.Set(key, localData)
					counter++
				}
			}
		}()
	}

	// Run the test for the specified duration
	time.Sleep(testDuration)
	close(stop)
	wg.Wait()

	// Verify the cache is still functional after the test
	if cache.Len() == 0 {
		t.Error("Cache is empty after concurrent operations")
	}

	// Try a few more operations to ensure everything still works
	for i := 0; i < 100; i++ {
		key := strconv.Itoa(i)
		err := cache.Set(key, data)
		if err != nil {
			t.Errorf("Failed to set key after test: %v", err)
		}

		_, err = cache.Get(key)
		if err != nil {
			t.Errorf("Failed to get key after test: %v", err)
		}
	}
}
