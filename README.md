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
		MaxEntrySize: 500,                  // max entry size in bytes
		Verbose: true,                      // prints information about additional memory allocation
	}

cache := bigcache.NewBigCache(config)

cache.Set("my-unique-key", []byte("value"))

if entry, err := cache.Get("my-unique-key"); err == nil {
    fmt.Println(string(entry))
}
```

## How it works

BigCache relays on optimization presented in 1.5 version of Go ([issue-9477](https://github.com/golang/go/issues/9477)).
This optimization states that if map without pointers in keys and values is used then GC will omit it’s content.
Therefore BigCache uses `map[uint64]uint32` where keys are hashed and values are offsets of entries.

Entries are kept in bytes array, to omit GC again. Bytes array size can grow to gigabytes without
impact on performance because GC will only see single pointer to it.

## License

BigCache is released under the Apache 2.0 license (see [LICENSE](LICENSE))
