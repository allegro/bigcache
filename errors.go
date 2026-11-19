package bigcache

import "errors"

var (
	// ErrEntryNotFound is an error type struct which is returned when entry was not found for provided key
	ErrEntryNotFound = errors.New("Entry not found") //nolint:staticcheck // keep for backward compatibility

	// ErrEntryTooBig is returned when an entry cannot fit in a shard queue.
	ErrEntryTooBig = errors.New("entry is bigger than max shard size")
)
