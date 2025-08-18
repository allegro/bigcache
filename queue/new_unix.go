//go:build aix || darwin || dragonfly || freebsd || openbsd || solaris || zos || linux || netbsd
// +build aix darwin dragonfly freebsd openbsd solaris zos linux netbsd

// build tags are sync from:
// golang.org/x/sys@v0.21.0/unix/mmap_nomremap.go
// golang.org/x/sys@v0.21.0/unix/mmap_mremap.go

package queue

import (
	"encoding/binary"

	"golang.org/x/sys/unix"
)

func pageUpper(in uintptr) uintptr {
	const pageMask = ((1 << 12) - 1)
	return (in + pageMask) &^ (pageMask)
}

// NewBytesQueue initialize new bytes queue.
// capacity is used in bytes array allocation
// When verbose flag is set then information about memory allocation are printed
func NewBytesQueue(capacity int, maxCapacity int, verbose bool) *BytesQueue {
	var array []byte
	if maxCapacity != 0 {
		var err error
		if maxCapacity >= 1<<12 {
			array, err = unix.Mmap(
				0,
				0,
				int(pageUpper(uintptr(maxCapacity))),
				unix.PROT_READ|unix.PROT_WRITE,
				unix.MAP_ANON|unix.MAP_PRIVATE,
			)
			if err != nil {
				panic(err)
			}
		} else {
			array = make([]byte, maxCapacity)
		}
		capacity = maxCapacity
	} else {
		array = make([]byte, capacity)
	}

	return &BytesQueue{
		array:        array,
		capacity:     capacity,
		maxCapacity:  maxCapacity,
		headerBuffer: make([]byte, binary.MaxVarintLen32),
		tail:         leftMarginIndex,
		head:         leftMarginIndex,
		rightMargin:  leftMarginIndex,
		verbose:      verbose,
	}
}
