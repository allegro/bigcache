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
	queue := NewBytesQueue(21, false)

	// when
	queue.Push([]byte("hello1"))
	queue.Push([]byte("hello2"))
	queue.Pop()
	queue.Push([]byte("hello3"))

	// then
	assert.Equal(t, 21, queue.Capacity())
	assert.Equal(t, []byte("hello2"), pop(queue))
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
	queue := NewBytesQueue(21, false)

	// when
	queue.Push([]byte("msg"))    // header + msg = 7bytes
	queue.Push([]byte("hello1")) // 10 bytes
	queue.Pop()                  // space freed, 13 bytes available
	queue.Push([]byte("toobig")) // 10 bytes needed, 13 available but not in one segment, allocate additional memory

	// then
	assert.Equal(t, 42, queue.Capacity())
	assert.Equal(t, []byte("hello1"), pop(queue))
}

func TestAllocateAdditionalSpaceForInsufficientFreeFragmentedSpaceWhereTailIsBeforeHead(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(21, false)

	// when
	queue.Push([]byte("hello1")) // header + msg = 10 bytes
	queue.Push([]byte("msg"))    // 7 bytes
	queue.Pop()                  // 13 bytes available - 10 at the beginning and 3 at the end (right margin)
	queue.Push([]byte("a"))      // 5 bytes used at the beginning, tail pointer is before head pointer
	queue.Push([]byte("ab"))     // 6 bytes needed but no available in one segment, allocate new memory

	// then
	assert.Equal(t, 42, queue.Capacity())
	assert.Equal(t, []byte("msg"), pop(queue))
	assert.Equal(t, []byte("a"), pop(queue))
	assert.Equal(t, []byte("ab"), pop(queue))
}

func TestAllocateAdditionalSpaceForValueBiggerThanInitQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(11, false)

	// when
	queue.Push(make([]byte, 100))

	// then
	assert.Equal(t, make([]byte, 100), pop(queue))
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
	result, err := queue.Get(0)

	// then
	assert.Empty(t, result)
	assert.EqualError(t, err, "Index must be grater than zero. Invalid index.")
}

func pop(queue *BytesQueue) []byte {
	entry, _ := queue.Pop()
	return entry
}
