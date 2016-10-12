# BigCache [![Build Status](https://travis-ci.org/allegro/bigcache.svg?branch=master)](https://travis-ci.org/allegro/bigcache)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/allegro/bigcache/badge.svg?branch=master)](https://coveralls.io/github/allegro/bigcache?branch=master)&nbsp;[![GoDoc](https://godoc.org/github.com/allegro/bigcache?status.svg)](https://godoc.org/github.com/allegro/bigcache)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/allegro/bigcache)](https://goreportcard.com/report/github.com/allegro/bigcache)

Fast, concurrent, evicting in-memory cache written to keep big number of entries without impact on performance.
BigCache keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
therefore entries (de)serialization in front of the cache will be needed in most use cases.

## Usage

### Simple initialization

```go
import "github.com/allegro/bigcache"

cache, _ := bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))

cache.Set("my-unique-key", []byte("value"))

entry, _ := cache.Get("my-unique-key")
fmt.Println(string(entry))
```

### Custom initialization

When cache load can be predicted in advance then it is better to use custom initialization because additional memory
allocation can be avoided in that way.

```go
import (
	"log"

	"github.com/allegro/bigcache"
)

config := bigcache.Config {
		// number of shards (must be a power of 2)
		Shards: 1024,
		// time after which entry can be evicted
		LifeWindow: 10 * time.Minute,
		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: 1000 * 10 * 60,
		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: 500,
		// prints information about additional memory allocation
		Verbose: true,
		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 8192,
		// callback fired when the oldest entry is removed because of its
		// expiration time or no space left for the new entry. Default value is nil which
		// means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: nil,
	}

cache, initErr := bigcache.NewBigCache(config)
if initErr != nil {
	log.Fatal(initErr)
}

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
cd caches_bench; go test -bench=. -benchtime=10s ./... -timeout 30m

BenchmarkMapSet-4              	20000000	      1681 ns/op	     296 B/op	       3 allocs/op
BenchmarkFreeCacheSet-4        	20000000	      1132 ns/op	     349 B/op	       3 allocs/op
BenchmarkBigCacheSet-4         	20000000	       831 ns/op	     305 B/op	       2 allocs/op
BenchmarkMapGet-4              	30000000	       540 ns/op	      24 B/op	       2 allocs/op
BenchmarkFreeCacheGet-4        	20000000	       986 ns/op	     152 B/op	       4 allocs/op
BenchmarkBigCacheGet-4         	20000000	       726 ns/op	      40 B/op	       3 allocs/op
BenchmarkBigCacheSetParallel-4 	20000000	       532 ns/op	     313 B/op	       3 allocs/op
BenchmarkFreeCacheSetParallel-4	20000000	       564 ns/op	     357 B/op	       4 allocs/op
BenchmarkBigCacheGetParallel-4 	50000000	       338 ns/op	      40 B/op	       3 allocs/op
BenchmarkFreeCacheGetParallel-4	30000000	       581 ns/op	     152 B/op	       4 allocs/op
```

Writes and reads in bigcache are faster than in freecache.
Writes to map are the slowest.

### GC pause time

```
cd caches_bench; go run caches_gc_overhead_comparsion.go

Number of entries:  20000000
GC pause for bigcache:  27.81671ms
GC pause for freecache:  30.218371ms
GC pause for map:  11.590772251s
```

Test shows how long are the GC pauses for caches filled with 20mln of entries.
Bigcache and freecache have very similar GC pause time.
It is clear that both reduce GC overhead in contrast to map
which GC pause time took more than 10 seconds.

## How it works

BigCache relies on optimization presented in 1.5 version of Go ([issue-9477](https://github.com/golang/go/issues/9477)).
This optimization states that if map without pointers in keys and values is used then GC will omit it’s content.
Therefore BigCache uses `map[uint64]uint32` where keys are hashed and values are offsets of entries.

Entries are kept in bytes array, to omit GC again.
Bytes array size can grow to gigabytes without impact on performance
because GC will only see single pointer to it.

## Bigcache vs Freecache
Both caches provide the same core features but they reduce GC overhead in different ways.
Bigcache relies on `map[uint64]uint32`, freecache implements its own mapping built on
slices to reduce number of pointers.

Results from benchmark tests are presented above.
One of the advantage of bigcache over freecache is that you don’t need to know
the size of the cache in advance, because when bigcache is full,
it can allocate additional memory for new entries instead of
overwriting existing ones as freecache does currently.
However hard max size in bigcache also can be set, check [HardMaxCacheSize](https://godoc.org/github.com/allegro/bigcache#Config).


## More

Bigcache genesis is described in allegro.tech blog post: [writing a very fast cache service in Go](http://allegro.tech/2016/03/writing-fast-cache-service-in-go.html)

## License

BigCache is released under the Apache 2.0 license (see [LICENSE](LICENSE))
