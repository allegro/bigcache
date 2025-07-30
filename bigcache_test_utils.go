package bigcache

import (
	"fmt"
	"math/rand"
	"time"
)

// blob is a helper function that creates a byte slice of a specified size
// filled with the specified character
func blob(char byte, size int) []byte {
	bytes := make([]byte, size)
	for i := 0; i < size; i++ {
		bytes[i] = char
	}
	return bytes
}

// mockedClock is used in testing to simulate time
type mockedClock struct {
	value int64
}

func (mc *mockedClock) Epoch() int64 {
	return mc.value
}

func (mc *mockedClock) Set(value int64) {
	mc.value = value
}

// newMockedClock creates a new mockedClock with initial time value
func newMockedClock(value int64) *mockedClock {
	return &mockedClock{value: value}
}

// Function to generate a slice of unique test data of specified size
func generateBlobs(numEntries int) [][]byte {
	blobs := make([][]byte, 0, numEntries)
	for i := 0; i < numEntries; i++ {
		blobs = append(blobs, []byte(fmt.Sprintf("key-%d", i)))
	}
	return blobs
}

// initialize random seed
func init() {
	rand.Seed(time.Now().UnixNano())
}
