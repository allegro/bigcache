CHANGES MADE TO BigCache INCLUDES :
1. changing FNV hash to xxhash and will eventually change to xxh3
2. changing key from string to []byte


# BigCache [![Build Status](https://travis-ci.org/allegro/bigcache.svg?branch=master)](https://travis-ci.org/allegro/bigcache)&nbsp;[![Coverage Status](https://coveralls.io/repos/github/allegro/bigcache/badge.svg?branch=master)](https://coveralls.io/github/allegro/bigcache?branch=master)&nbsp;[![GoDoc](https://godoc.org/github.com/allegro/bigcache?status.svg)](https://godoc.org/github.com/allegro/bigcache)&nbsp;[![Go Report Card](https://goreportcard.com/badge/github.com/allegro/bigcache)](https://goreportcard.com/report/github.com/allegro/bigcache)

Fast, concurrent, evicting in-memory cache written to keep big number of entries without impact on performance.
BigCache keeps entries on heap but omits GC for them. To achieve that operations on bytes arrays take place,
therefore entries (de)serialization in front of the cache will be needed in most use cases.

## Usage

### Simple initialization

```go
import "github.com/allegro/bigcache"

cache, _ := bigcache.NewBigCache(bigcache.DefaultConfig(10 * time.Minute))

cache.Set([]byte("my-unique-key"), []byte("value"))

entry, _ := cache.Get([]byte("my-unique-key"))
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
		// callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: nil,
		// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A constant representing the reason will be passed through.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		// Ignored if OnRemove is specified.
		OnRemoveWithReason: nil,
	}

cache, initErr := bigcache.NewBigCache(config)
if initErr != nil {
	log.Fatal(initErr)
}

cache.Set([]byte("my-unique-key"), []byte("value"))

if entry, err := cache.Get([]byte("my-unique-key")); err == nil {
	fmt.Println(string(entry))
}
```

Test shows how long are the GC pauses for caches filled with 20mln of entries.
Bigcache and freecache have very similar GC pause time.
It is clear that both reduce GC overhead in contrast to map
which GC pause time took more than 10 seconds.

## How it works

BigCache relies on optimization presented in 1.5 version of Go ([issue-9477](https://github.com/golang/go/issues/9477)).
This optimization states that if map without pointers in keys and values is used then GC will omit its content.
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

## HTTP Server

This package also includes an easily deployable HTTP implementation of BigCache, which can be found in the [server](/server) package.

## More

Bigcache genesis is described in allegro.tech blog post: [writing a very fast cache service in Go](http://allegro.tech/2016/03/writing-fast-cache-service-in-go.html)

## License

BigCache is released under the Apache 2.0 license (see [LICENSE](LICENSE))
