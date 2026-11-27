package bigcache

import "errors"

var (
	// ErrEntryNotFound is an error type struct which is returned when entry was not found for provided key
	ErrEntryNotFound = errors.New("Entry not found") //nolint:staticcheck // keep for backward compatibility
	// ErrKeyTooLong is returned when key length exceeds math.MaxUint16 (65535 bytes)
	ErrKeyTooLong = errors.New("Key is too long") //nolint:staticcheck // keep for backward compatibility
)
