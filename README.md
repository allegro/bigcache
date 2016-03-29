# BigCache [![Build Status](https://travis-ci.org/allegro/bigcache.svg?branch=master)](https://travis-ci.org/allegro/bigcache)[![Coverage Status](https://coveralls.io/repos/github/allegro/bigcache/badge.svg?branch=master)](https://coveralls.io/github/allegro/bigcache?branch=master)[![GoDoc](https://godoc.org/github.com/allegro/bigcache?status.svg)](https://godoc.org/github.com/allegro/bigcache)

Fast, concurrent, evicting in-memory cache written to keep big number of entries without impact on performance.
BigCache keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
therefore entries (de)serialization in front of the cache will be needed in most use cases.

## Usage

### Simple initialization

```go
import "github.com/allegro/bigcache"

cache := bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))

cache.Set("my-unique-key", []byte("value"))

entry, _ := cache.Get("my-unique-key")
fmt.Println(string(entry))
```

### Custom initialization

When cache load can be predicted in advance then it is better to use custom initialization because additional memory
allocation can be avoided in that way.

```go
import "github.com/allegro/bigcache"

config := bigcache.Config{
		Shards: 1000,                       // number of shards
		LifeWindow: 10 * time.Minute,       // time after which entry can be evicted
		MaxEntriesInWindow: 1000 * 10 * 60, // rps * lifeWindow
		MaxEntrySize: 500,                  // max entry size in bytes, used only in initial memory allocation
		Verbose: true,                      // prints information about additional memory allocation
	}

cache := bigcache.NewBigCache(config)

cache.Set("my-unique-key", []byte("value"))

if entry, err := cache.Get("my-unique-key"); err == nil {
	fmt.Println(string(entry))
}
```

## Benchmarks

Three caches were compared: bigcache, [freecache](https://github.com/coocood/freecache) and map.
Benchmark tests were made on MacBook Pro (3 GHz Processor Intel Core i7, 16GB Memory).

### Writes and reads

```
go test -bench=. -benchtime=10s ./...

BenchmarkMapSet-4              	10000000	      1691 ns/op
BenchmarkFreeCacheSet-4        	20000000	      1309 ns/op
BenchmarkBigCacheSet-4         	20000000	      1110 ns/op
BenchmarkMapGet-4              	30000000	       544 ns/op
BenchmarkFreeCacheGet-4        	20000000	      1020 ns/op
BenchmarkBigCacheGet-4         	20000000	       766 ns/op
BenchmarkBigCacheSetParallel-4 	20000000	       563 ns/op
BenchmarkFreeCacheSetParallel-4	30000000	       666 ns/op
BenchmarkBigCacheGetParallel-4 	50000000	       625 ns/op
BenchmarkFreeCacheGetParallel-4	20000000	       696 ns/op
ok  	github.com/allegro/bigcache/caches_bench	470.259s
```

Parallel writes and reads in bigcache and freecache are on very similar level.
In serial tests reads are slightly faster in bigcache than in fastcache.

### GC pause time

```
go run caches_gc_overhead_comparsion.go

Number of entries:  20000000
GC pause for bigcache:  50.8121ms
GC pause for freecache:  29.451837ms
GC pause for map:  11.231013483s
```

Test shows how long are the GC pauses for caches filled with 20mln of entries.
Freecache has the shortest GC pause time, bigcache is slightly slower.
It is clear that both reduce GC overhead in contrast to map
which GC pause time took more than 10 seconds.

## How it works

BigCache relays on optimization presented in 1.5 version of Go ([issue-9477](https://github.com/golang/go/issues/9477)).
This optimization states that if map without pointers in keys and values is used then GC will omit it’s content.
Therefore BigCache uses `map[uint64]uint32` where keys are hashed and values are offsets of entries.

Entries are kept in bytes array, to omit GC again.
Bytes array size can grow to gigabytes without impact on performance
because GC will only see single pointer to it.

## Bigcache vs Freecache
Both caches provide the same core features but they reduce GC overhead in different ways.
Bigcache relays on `map[uint64]uint32`, freecache implements its own mapping built on
slices to reduce number of pointers.

Results from benchmark tests are presented above.
One of the advantage of bigcache over freecache is that you don’t need to know
the size of cache in advance, because when bigcache is full,
it allocates additional memory for new entries instead of
overwriting existing ones as freecache does currently.

## License

BigCache is released under the Apache 2.0 license (see [LICENSE](LICENSE))
