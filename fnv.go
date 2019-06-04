package bigcache

import(
	"github.com/cespare/xxhash"
)
// newDefaultHasher returns a new 64-bit xxhash (previously fnv64 was used).
// Keeping old fnv.go and fnv64a struct{} to minimize code refactoring.

func newDefaultHasher() Hasher {
	return fnv64a{}
}

type fnv64a struct{}

// Sum64 gets the string and returns its uint64 hash value.
func (f fnv64a) Sum64(key string) uint64 {
	hash := xxhash.Sum64([]byte(key))
	return hash
}
