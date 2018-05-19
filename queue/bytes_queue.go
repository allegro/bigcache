package queue

import (
	"encoding/binary"
	"log"
	"time"
)

const (
	// Number of bytes used to keep information about entry size
	headerEntrySize = 4
	// Bytes before left margin are not used. Zero index means element does not exist in queue, useful while reading slice from index
	leftMarginIndex = 1
	// Minimum empty blob size in bytes. Empty blob fills space between Tail and Head in additional memory allocation.
	// It keeps entries indexes unchanged
	minimumEmptyBlobSize = 32 + headerEntrySize
)

// BytesQueue is a non-thread safe queue type of fifo based on bytes Array.
// For every push operation index of entry is returned. It can be used to read the entry later
type BytesQueue struct {
	Array           []byte
	QueueCapacity   int
	MaxCapacity     int
	Head            int
	Tail            int
	Count           int
	RightMargin     int
	HeaderBuffer    []byte
	Verbose         bool
	InitialCapacity int
}

type queueError struct {
	message string
}

// NewBytesQueue initialize new bytes queue.
// Initial QueueCapacity is used in bytes Array allocation
// When Verbose flag is set then information about memory allocation are printed
func NewBytesQueue(initialCapacity int, maxCapacity int, verbose bool) *BytesQueue {
	return &BytesQueue{
		Array:           make([]byte, initialCapacity),
		QueueCapacity:   initialCapacity,
		MaxCapacity:     maxCapacity,
		HeaderBuffer:    make([]byte, headerEntrySize),
		Tail:            leftMarginIndex,
		Head:            leftMarginIndex,
		RightMargin:     leftMarginIndex,
		Verbose:         verbose,
		InitialCapacity: initialCapacity,
	}
}

// Reset removes all entries from queue
func (q *BytesQueue) Reset() {
	// Just reset indexes
	q.Tail = leftMarginIndex
	q.Head = leftMarginIndex
	q.RightMargin = leftMarginIndex
	q.Count = 0
}

// Push copies entry at the end of queue and moves Tail pointer. Allocates more space if needed.
// Returns index for pushed data or error if maximum size queue limit is reached.
func (q *BytesQueue) Push(data []byte) (int, error) {
	dataLen := len(data)

	if q.availableSpaceAfterTail() < dataLen+headerEntrySize {
		if q.availableSpaceBeforeHead() >= dataLen+headerEntrySize {
			q.Tail = leftMarginIndex
		} else if q.QueueCapacity+headerEntrySize+dataLen >= q.MaxCapacity && q.MaxCapacity > 0 {
			return -1, &queueError{"Full queue. Maximum size limit reached."}
		} else {
			q.allocateAdditionalMemory(dataLen + headerEntrySize)
		}
	}

	index := q.Tail

	q.push(data, dataLen)

	return index, nil
}

func (q *BytesQueue) allocateAdditionalMemory(minimum int) {
	start := time.Now()
	if q.QueueCapacity < minimum {
		q.QueueCapacity += minimum
	}
	q.QueueCapacity = q.QueueCapacity * 2
	if q.QueueCapacity > q.MaxCapacity && q.MaxCapacity > 0 {
		q.QueueCapacity = q.MaxCapacity
	}

	oldArray := q.Array
	q.Array = make([]byte, q.QueueCapacity)

	if leftMarginIndex != q.RightMargin {
		copy(q.Array, oldArray[:q.RightMargin])

		if q.Tail < q.Head {
			emptyBlobLen := q.Head - q.Tail - headerEntrySize
			q.push(make([]byte, emptyBlobLen), emptyBlobLen)
			q.Head = leftMarginIndex
			q.Tail = q.RightMargin
		}
	}

	if q.Verbose {
		log.Printf("Allocated new queue in %s; QueueCapacity: %d \n", time.Since(start), q.QueueCapacity)
	}
}

func (q *BytesQueue) push(data []byte, len int) {
	binary.LittleEndian.PutUint32(q.HeaderBuffer, uint32(len))
	q.copy(q.HeaderBuffer, headerEntrySize)

	q.copy(data, len)

	if q.Tail > q.Head {
		q.RightMargin = q.Tail
	}

	q.Count++
}

func (q *BytesQueue) copy(data []byte, len int) {
	q.Tail += copy(q.Array[q.Tail:], data[:len])
}

// Pop reads the oldest entry from queue and moves Head pointer to the next one
func (q *BytesQueue) Pop() ([]byte, error) {
	data, size, err := q.peek(q.Head)
	if err != nil {
		return nil, err
	}

	q.Head += headerEntrySize + size
	q.Count--

	if q.Head == q.RightMargin {
		q.Head = leftMarginIndex
		if q.Tail == q.RightMargin {
			q.Tail = leftMarginIndex
		}
		q.RightMargin = q.Tail
	}

	return data, nil
}

// Peek reads the oldest entry from list without moving Head pointer
func (q *BytesQueue) Peek() ([]byte, error) {
	data, _, err := q.peek(q.Head)
	return data, err
}

// Get reads entry from index
func (q *BytesQueue) Get(index int) ([]byte, error) {
	data, _, err := q.peek(index)
	return data, err
}

// Capacity returns number of allocated bytes for queue
func (q *BytesQueue) Capacity() int {
	return q.QueueCapacity
}

// Len returns number of entries kept in queue
func (q *BytesQueue) Len() int {
	return q.Count
}

// Error returns error message
func (e *queueError) Error() string {
	return e.message
}

func (q *BytesQueue) peek(index int) ([]byte, int, error) {

	if q.Count == 0 {
		return nil, 0, &queueError{"Empty queue"}
	}

	if index <= 0 {
		return nil, 0, &queueError{"Index must be grater than zero. Invalid index."}
	}

	if index+headerEntrySize >= len(q.Array) {
		return nil, 0, &queueError{"Index out of range"}
	}

	blockSize := int(binary.LittleEndian.Uint32(q.Array[index : index+headerEntrySize]))
	return q.Array[index+headerEntrySize : index+headerEntrySize+blockSize], blockSize, nil
}

func (q *BytesQueue) availableSpaceAfterTail() int {
	if q.Tail >= q.Head {
		return q.QueueCapacity - q.Tail
	}
	return q.Head - q.Tail - minimumEmptyBlobSize
}

func (q *BytesQueue) availableSpaceBeforeHead() int {
	if q.Tail >= q.Head {
		return q.Head - leftMarginIndex - minimumEmptyBlobSize
	}
	return q.Head - q.Tail - minimumEmptyBlobSize
}
