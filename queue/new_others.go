//go:build !aix && !darwin && !dragonfly && !freebsd && !openbsd && !solaris && !zos && !linux && !netbsd
// +build !aix,!darwin,!dragonfly,!freebsd,!openbsd,!solaris,!zos,!linux,!netbsd

package queue

import "encoding/binary"

// NewBytesQueue initialize new bytes queue.
// capacity is used in bytes array allocation
// When verbose flag is set then information about memory allocation are printed
func NewBytesQueue(capacity int, maxCapacity int, verbose bool) *BytesQueue {
	return &BytesQueue{
		array:        make([]byte, capacity),
		capacity:     capacity,
		maxCapacity:  maxCapacity,
		headerBuffer: make([]byte, binary.MaxVarintLen32),
		tail:         leftMarginIndex,
		head:         leftMarginIndex,
		rightMargin:  leftMarginIndex,
		verbose:      verbose,
	}
}
