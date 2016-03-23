package bigcache

import "hash/fnv"

type hash interface {
	sum(string) uint64
}

type fnv64a struct {
}

func (f fnv64a) sum(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}
