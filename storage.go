package bigcache

import "io"

// IStorage is used for streaming or storing the cache externally.
type IStorage interface {
	// Prepare is intended to be a pre-export or pre-import state which can be called to prepare the cache and/or
	// storage implementation for saving or loading.
	Prepare() error
	// Save is intended to store or stream the cache in a given implementation.
	Save(io.Writer) (bool, error)
	// Load an external cache from storage or streaming provider.
	Load(io.Reader) (bool, error)
}
