package queue

import (
	"encoding/binary"
	"log"
	"time"
)

const (
	// Number of bytes used to keep information about entry size
	headerEntrySize = 4
	// Minimum empty blob size in bytes. Empty blob fills space between tail and head in additional memory allocation.
	// It keeps entries indexes unchanged
	minimumEmptyBlobSize = 1
)

// BytesQueue is a non-thread safe queue type of fifo based on bytes array.
// For every push operation index of entry is returned. It can be used to read the entry later
type BytesQueue struct {
	array        []byte
	capacity     int
	head         int
	tail         int
	count        int
	headerBuffer []byte
	verbose      bool
}

type queueError struct {
	message string
}

// NewBytesQueue initialize new bytes queue.
// Initial capacity is used in bytes array allocation
// When verbose flag is set then information about memory allocation are printed
func NewBytesQueue(initialCapacity int, verbose bool) *BytesQueue {
	return &BytesQueue{
		array:        make([]byte, initialCapacity),
		capacity:     initialCapacity,
		headerBuffer: make([]byte, headerEntrySize),
		tail:         0,
		head:         0,
		verbose:      verbose,
	}
}

// Push copies entry at the end of queue and moves tail pointer. Allocates more space if needed.
// Returns index for pushed data
func (q *BytesQueue) Push(data []byte) int {
	dataLen := len(data)
	if q.availableSpace() < dataLen+headerEntrySize {
		q.allocateAdditionalMemory(dataLen)
	}
	index := q.tail
	q.push(data, dataLen)
	return index
}

func (q *BytesQueue) allocateAdditionalMemory(minimum int) {
	start := time.Now()
	if q.capacity < minimum {
		q.capacity += minimum
	}
	q.capacity = q.capacity * 2
	oldArray := q.array
	length := q.length()
	q.array = make([]byte, q.capacity)

	if q.tail >= q.head {
		copy(q.array, oldArray[q.head:q.tail])
	} else {
		part := copy(q.array, oldArray[q.head:])
		copy(q.array[part:], oldArray[:q.tail])
	}
	q.head = 0
	q.tail = length

	if q.verbose {
		log.Printf("Allocated new queue in %s; Capacity: %d \n", time.Since(start), q.capacity)
	}
}

func (q *BytesQueue) push(data []byte, len int) {
	binary.LittleEndian.PutUint32(q.headerBuffer, uint32(len))
	q.copy(q.headerBuffer, headerEntrySize)

	q.copy(data, len)

	q.count++
}

func (q *BytesQueue) copy(data []byte, len int) {
	part := copy(q.array[q.tail:], data[:len])
	if part < len {
		copy(q.array, data[part:len])
	}
	q.tail = (q.tail + len) % q.capacity
}

// Pop reads the oldest entry from queue and moves head pointer to the next one
func (q *BytesQueue) Pop() ([]byte, error) {
	if q.count == 0 {
		return nil, &queueError{"Empty queue"}
	}

	data, size := q.peek(q.head)

	q.head += headerEntrySize + size
	q.count--

	return data, nil
}

// Peek reads the oldest entry from list without moving head pointer
func (q *BytesQueue) Peek() ([]byte, error) {
	if q.count == 0 {
		return nil, &queueError{"Empty queue"}
	}

	data, _ := q.peek(q.head)

	return data, nil
}

// Get reads entry from index
func (q *BytesQueue) Get(index int) ([]byte, error) {
	if index < 0 {
		return nil, &queueError{"Index must be grater than zero. Invalid index."}
	}

	data, _ := q.peek(index)
	return data, nil
}

// Capacity returns number of allocated bytes for queue
func (q *BytesQueue) Capacity() int {
	return q.capacity
}

// Len returns number of entries kept in queue
func (q *BytesQueue) Len() int {
	return q.count
}

// Error returns error message
func (e *queueError) Error() string {
	return e.message
}

func (q *BytesQueue) peek(index int) ([]byte, int) {
	blockSize := int(binary.LittleEndian.Uint32(q.getBytes(index, headerEntrySize)))
	return q.getBytes(index+headerEntrySize, blockSize), blockSize
}

func (q *BytesQueue) getBytes(index, len int) []byte {
	ret := make([]byte, len)
	if index+len > q.capacity {
		part := copy(ret, q.array[index:])
		copy(ret[part:], q.array[:len-part])
	} else {
		copy(ret, q.array[index:index+len])
	}
	return ret
}

func (q *BytesQueue) length() int {
	return (q.tail - q.head + q.capacity) % q.capacity
}

func (q *BytesQueue) availableSpace() int {
	return q.capacity - q.length() - minimumEmptyBlobSize
}
