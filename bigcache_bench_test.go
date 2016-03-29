package bigcache

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

var message = []byte(`Lorem ipsum dolor sit amet, consectetuer adipiscing elit. Aenean commodo ligula eget dolor. Aenean massa.
Cum sociis natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Donec quam felis, ultricies nec, pellentesque eu, pretium quis,.`)

func BenchmarkWriteToCacheWith1Shard(b *testing.B) {
	writeToCache(b, 1, 100*time.Second, b.N)
}

func BenchmarkWriteToCacheWith512Shards(b *testing.B) {
	writeToCache(b, 512, 100*time.Second, b.N)
}

func BenchmarkWriteToCacheWith1024Shards(b *testing.B) {
	writeToCache(b, 1024, 100*time.Second, b.N)
}

func BenchmarkWriteToCacheWith8192Shards(b *testing.B) {
	writeToCache(b, 8192, 100*time.Second, b.N)
}

func BenchmarkWriteToCacheWith1024ShardsAndSmallShardInitSize(b *testing.B) {
	writeToCache(b, 1024, 100*time.Second, 100)
}

func BenchmarkReadFromCacheWith1024Shards(b *testing.B) {
	readFromCache(b, 1024)
}

func BenchmarkReadFromCacheWith8192Shards(b *testing.B) {
	readFromCache(b, 8192)
}

func writeToCache(b *testing.B, shards int, lifeWindow time.Duration, requestsInLifeWindow int) {
	cache, _ := NewBigCache(Config{shards, lifeWindow, max(requestsInLifeWindow, 100), 500, false, nil})
	rand.Seed(time.Now().Unix())

	b.RunParallel(func(pb *testing.PB) {
		id := rand.Int()
		counter := 0
		for pb.Next() {
			cache.Set(fmt.Sprintf("key-%d-%d", id, counter), message)
			counter = counter + 1
		}
	})
}

func readFromCache(b *testing.B, shards int) {
	cache, _ := NewBigCache(Config{8192, 1000 * time.Second, max(b.N, 100), 500, false, nil})
	for i := 0; i < b.N; i++ {
		cache.Set(strconv.Itoa(i), message)
	}
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get(strconv.Itoa(rand.Intn(b.N)))
		}
	})
}
