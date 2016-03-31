package bigcache

import (
	"hash/fnv"
)

// Hasher is responsible for generating unsigned, 64 bit hash of provided string. Hasher should minimize collisions
// (generating same hash for different strings) and while performance is also important fast functions are preferable (i.e.
// you can use FarmHash family).
type Hasher interface {
	Sum64(string) uint64
}

func newDefaultHasher() Hasher {
	return fnv64a{}
}

type fnv64a struct {
}

func (f fnv64a) Sum64(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}
