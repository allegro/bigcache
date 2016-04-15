package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushAndPop(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(10, true)
	entry := []byte("hello")

	// when
	queue.Push(entry)

	// then
	assert.Equal(t, entry, pop(queue))
}

func TestPeek(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, false)
	entry := []byte("hello")
	queue.Push(entry)

	// when
	read, _ := queue.Peek()

	// then
	assert.Equal(t, pop(queue), read)
	assert.Equal(t, entry, read)
}

func TestReuseAvailableSpace(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, false)

	// when
	queue.Push(blob('a', 70))
	queue.Push(blob('b', 20))
	queue.Pop()
	queue.Push(blob('c', 20))

	// then
	assert.Equal(t, 100, queue.Capacity())
	assert.Equal(t, blob('b', 20), pop(queue))
}

func TestAllocateAdditionalSpace(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(11, false)

	// when
	queue.Push([]byte("hello1"))
	queue.Push([]byte("hello2"))

	// then
	assert.Equal(t, 22, queue.Capacity())
}

func TestAllocateAdditionalSpaceForInsufficientFreeFragmentedSpaceWhereHeadIsBeforeTail(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(25, false)

	// when
	queue.Push(blob('a', 3)) // header + entry + left margin = 7 bytes
	queue.Push(blob('b', 6)) // additional 10 bytes
	queue.Pop()              // space freed, 7 bytes available at the beginning
	queue.Push(blob('c', 6)) // 10 bytes needed, 14 available but not in one segment, allocate additional memory

	// then
	assert.Equal(t, 25, queue.Capacity())
	assert.Equal(t, blob('b', 6), pop(queue))
	assert.Equal(t, blob('c', 6), pop(queue))
}

func TestUnchangedEntriesIndexesAfterAdditionalMemoryAllocationWhereHeadIsBeforeTail(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(25, false)

	// when
	queue.Push(blob('a', 3))                // header + entry + left margin = 7 bytes
	index := queue.Push(blob('b', 6))       // additional 10 bytes
	queue.Pop()                             // space freed, 7 bytes available at the beginning
	newestIndex := queue.Push(blob('c', 6)) // 10 bytes needed, 14 available but not in one segment, allocate additional memory

	// then
	assert.Equal(t, 25, queue.Capacity())
	assert.Equal(t, blob('b', 6), get(queue, index))
	assert.Equal(t, blob('c', 6), get(queue, newestIndex))
}

func TestAllocateAdditionalSpaceForInsufficientFreeFragmentedSpaceWhereTailIsBeforeHead(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, false)

	// when
	queue.Push(blob('a', 70)) // header + entry + left margin = 74 bytes
	queue.Push(blob('b', 10)) // 74 + 10 + 4 = 88 bytes
	queue.Pop()               // space freed at the beginning
	queue.Push(blob('c', 30)) // 34 bytes used at the beginning, tail pointer is before head pointer
	queue.Push(blob('d', 50)) // 54 bytes needed but no available in one segment, allocate new memory

	// then
	assert.Equal(t, 200, queue.Capacity())
	assert.Equal(t, blob('b', 10), pop(queue))
	// empty blob fills space between tail and head,
	// created when additional memory was allocated,
	// it keeps current entries indexes unchanged
	assert.Equal(t, blob('c', 30), pop(queue))
	assert.Equal(t, blob('d', 50), pop(queue))
	//assert.Equal(t, blob('d', 40), pop(queue))
}

func TestUnchangedEntriesIndexesAfterAdditionalMemoryAllocationWhereTailIsBeforeHead(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, false)

	// when
	queue.Push(blob('a', 70))                // header + entry = 74 bytes
	index := queue.Push(blob('b', 10))       // 74 + 10 + 4 = 88 bytes
	queue.Pop()                              // space freed at the beginning
	queue.Push(blob('c', 30))                // 34 bytes used at the beginning, tail pointer is before head pointer
	newestIndex := queue.Push(blob('d', 40)) // 44 bytes needed but no available in one segment, allocate new memory

	// then
	assert.Equal(t, 100, queue.Capacity())
	assert.Equal(t, blob('b', 10), get(queue, index))
	assert.Equal(t, blob('d', 40), get(queue, newestIndex))
}

func TestAllocateAdditionalSpaceForValueBiggerThanInitQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(11, false)

	// when
	queue.Push(blob('a', 100))

	// then
	assert.Equal(t, blob('a', 100), pop(queue))
	assert.Equal(t, 222, queue.Capacity())
}

func TestAllocateAdditionalSpaceForValueBiggerThanQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(21, false)

	// when
	queue.Push(make([]byte, 2))
	queue.Push(make([]byte, 2))
	queue.Push(make([]byte, 100))

	// then
	queue.Pop()
	queue.Pop()
	assert.Equal(t, make([]byte, 100), pop(queue))
	assert.Equal(t, 242, queue.Capacity())
}

func TestPopWholeQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(13, false)

	// when
	queue.Push([]byte("a"))
	queue.Push([]byte("b"))
	queue.Pop()
	queue.Pop()
	queue.Push([]byte("c"))

	// then
	assert.Equal(t, 13, queue.Capacity())
	assert.Equal(t, []byte("c"), pop(queue))
}

func TestGetEntryFromIndex(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(20, false)

	// when
	queue.Push([]byte("a"))
	index := queue.Push([]byte("b"))
	queue.Push([]byte("c"))
	result, _ := queue.Get(index)

	// then
	assert.Equal(t, []byte("b"), result)
}

func TestGetEntryFromInvalidIndex(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(13, false)

	// when
	result, err := queue.Get(-1)

	// then
	assert.Empty(t, result)
	assert.EqualError(t, err, "Index must be equal or grater than zero. Invalid index.")
}

func pop(queue *BytesQueue) []byte {
	entry, _ := queue.Pop()
	return entry
}

func get(queue *BytesQueue, index int) []byte {
	entry, _ := queue.Get(index)
	return entry
}

func blob(char byte, len int) []byte {
	b := make([]byte, len)
	for index := range b {
		b[index] = char
	}
	return b
}
