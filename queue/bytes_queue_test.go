package queue

import (
	"bytes"
	"fmt"
	"path"
	"reflect"
	"runtime"
	"testing"
)

func TestPushAndPop(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(10, 0, true)
	entry := []byte("hello")

	// when
	_, err := queue.Pop()

	// then
	assertEqual(t, "Empty queue", err.Error())

	// when
	queue.Push(entry)

	// then
	assertEqual(t, entry, pop(queue))
}

func TestLen(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)
	entry := []byte("hello")
	assertEqual(t, 0, queue.Len())

	// when
	queue.Push(entry)

	// then
	assertEqual(t, queue.Len(), 1)
}

func TestPeek(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)
	entry := []byte("hello")

	// when
	read, err := queue.Peek()
	err2 := queue.peekCheckErr(queue.head)
	// then
	assertEqual(t, err, err2)
	assertEqual(t, "Empty queue", err.Error())
	assertEqual(t, 0, len(read))

	// when
	queue.Push(entry)
	read, err = queue.Peek()
	err2 = queue.peekCheckErr(queue.head)

	// then
	assertEqual(t, err, err2)
	noError(t, err)
	assertEqual(t, pop(queue), read)
	assertEqual(t, entry, read)
}

func TestResetFullQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(10, 20, false)

	// when
	queue.Push(blob('a', 3))
	queue.Push(blob('b', 4))

	// when
	assertEqual(t, blob('a', 3), pop(queue)) // space freed at the beginning
	_, err := queue.Push(blob('a', 3))       // will set q.full to true

	// then
	assertEqual(t, err, nil)

	// when
	queue.Reset()
	queue.Push(blob('c', 8)) // should not trigger a re-allocation

	// then
	assertEqual(t, blob('c', 8), pop(queue))
	assertEqual(t, queue.Capacity(), 10)
}

func TestReset(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)
	entry := []byte("hello")

	// when
	queue.Push(entry)
	queue.Push(entry)
	queue.Push(entry)

	queue.Reset()
	read, err := queue.Peek()

	// then
	assertEqual(t, "Empty queue", err.Error())
	assertEqual(t, 0, len(read))

	// when
	queue.Push(entry)
	read, err = queue.Peek()

	// then
	noError(t, err)
	assertEqual(t, pop(queue), read)
	assertEqual(t, entry, read)

	// when
	read, err = queue.Peek()

	// then
	assertEqual(t, "Empty queue", err.Error())
	assertEqual(t, 0, len(read))
}

func TestReuseAvailableSpace(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)

	// when
	queue.Push(blob('a', 70))
	queue.Push(blob('b', 20))
	queue.Pop()
	queue.Push(blob('c', 20))

	// then
	assertEqual(t, 100, queue.Capacity())
	assertEqual(t, blob('b', 20), pop(queue))
}

func TestAllocateAdditionalSpace(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(11, 0, false)

	// when
	queue.Push([]byte("hello1"))
	queue.Push([]byte("hello2"))

	// then
	assertEqual(t, 22, queue.Capacity())
}

func TestAllocateAdditionalSpaceForInsufficientFreeFragmentedSpaceWhereHeadIsBeforeTail(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(25, 0, false)

	// when
	queue.Push(blob('a', 3)) // header + entry + left margin = 5 bytes
	queue.Push(blob('b', 6)) // additional 7 bytes
	queue.Pop()              // space freed, 4 bytes available at the beginning
	queue.Push(blob('c', 6)) // 7 bytes needed, 13 bytes available at the tail

	// then
	assertEqual(t, 25, queue.Capacity())
	assertEqual(t, blob('b', 6), pop(queue))
	assertEqual(t, blob('c', 6), pop(queue))
}

func TestUnchangedEntriesIndexesAfterAdditionalMemoryAllocationWhereHeadIsBeforeTail(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(25, 0, false)

	// when
	queue.Push(blob('a', 3))                   // header + entry + left margin = 5 bytes
	index, _ := queue.Push(blob('b', 6))       // additional 7 bytes
	queue.Pop()                                // space freed, 4 bytes available at the beginning
	newestIndex, _ := queue.Push(blob('c', 6)) // 7 bytes needed, 13 available at the tail

	// then
	assertEqual(t, 25, queue.Capacity())
	assertEqual(t, blob('b', 6), get(queue, index))
	assertEqual(t, blob('c', 6), get(queue, newestIndex))
}

func TestAllocateAdditionalSpaceForInsufficientFreeFragmentedSpaceWhereTailIsBeforeHead(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)

	// when
	queue.Push(blob('a', 70)) // header + entry + left margin = 72 bytes
	queue.Push(blob('b', 10)) // 72 + 10 + 1 = 83 bytes
	queue.Pop()               // space freed at the beginning
	queue.Push(blob('c', 30)) // 31 bytes used at the beginning, tail pointer is before head pointer
	queue.Push(blob('d', 40)) // 41 bytes needed but no available in one segment, allocate new memory

	// then
	assertEqual(t, 200, queue.Capacity())
	assertEqual(t, blob('c', 30), pop(queue))
	// empty blob fills space between tail and head,
	// created when additional memory was allocated,
	// it keeps current entries indexes unchanged
	assertEqual(t, blob(0, 39), pop(queue))
	assertEqual(t, blob('b', 10), pop(queue))
	assertEqual(t, blob('d', 40), pop(queue))
}

func TestUnchangedEntriesIndexesAfterAdditionalMemoryAllocationWhereTailIsBeforeHead(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(100, 0, false)

	// when
	queue.Push(blob('a', 70))                   // header + entry + left margin = 72 bytes
	index, _ := queue.Push(blob('b', 10))       // 72 + 10 + 1 = 83 bytes
	queue.Pop()                                 // space freed at the beginning
	queue.Push(blob('c', 30))                   // 31 bytes used at the beginning, tail pointer is before head pointer
	newestIndex, _ := queue.Push(blob('d', 40)) // 41 bytes needed but no available in one segment, allocate new memory

	// then
	assertEqual(t, 200, queue.Capacity())
	assertEqual(t, blob('b', 10), get(queue, index))
	assertEqual(t, blob('d', 40), get(queue, newestIndex))
}

func TestAllocateAdditionalSpaceForValueBiggerThanInitQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(11, 0, false)

	// when
	queue.Push(blob('a', 100))
	// then
	assertEqual(t, blob('a', 100), pop(queue))
	// 224 = (101 + 11) * 2
	assertEqual(t, 224, queue.Capacity())
}

func TestAllocateAdditionalSpaceForValueBiggerThanQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(21, 0, false)

	// when
	queue.Push(make([]byte, 2))
	queue.Push(make([]byte, 2))
	queue.Push(make([]byte, 100))

	// then
	queue.Pop()
	queue.Pop()
	assertEqual(t, make([]byte, 100), pop(queue))
	// 244 = (101 + 21) * 2
	assertEqual(t, 244, queue.Capacity())
}

func TestPopWholeQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(13, 0, false)

	// when
	queue.Push([]byte("a"))
	queue.Push([]byte("b"))
	queue.Pop()
	queue.Pop()
	queue.Push([]byte("c"))

	// then
	assertEqual(t, 13, queue.Capacity())
	assertEqual(t, []byte("c"), pop(queue))
}

func TestGetEntryFromIndex(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(20, 0, false)

	// when
	queue.Push([]byte("a"))
	index, _ := queue.Push([]byte("b"))
	queue.Push([]byte("c"))
	result, _ := queue.Get(index)

	// then
	assertEqual(t, []byte("b"), result)
}

func TestGetEntryFromInvalidIndex(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(1, 0, false)
	queue.Push([]byte("a"))

	// when
	result, err := queue.Get(0)
	err2 := queue.CheckGet(0)

	// then
	assertEqual(t, err, err2)
	assertEqual(t, []byte(nil), result)
	assertEqual(t, "Index must be greater than zero. Invalid index.", err.Error())
}

func TestGetEntryFromIndexOutOfRange(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(1, 0, false)
	queue.Push([]byte("a"))

	// when
	result, err := queue.Get(42)
	err2 := queue.CheckGet(42)

	// then
	assertEqual(t, err, err2)
	assertEqual(t, []byte(nil), result)
	assertEqual(t, "Index out of range", err.Error())
}

func TestGetEntryFromEmptyQueue(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(13, 0, false)

	// when
	result, err := queue.Get(1)
	err2 := queue.CheckGet(1)

	// then
	assertEqual(t, err, err2)
	assertEqual(t, []byte(nil), result)
	assertEqual(t, "Empty queue", err.Error())
}

func TestMaxSizeLimit(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(30, 50, false)

	// when
	queue.Push(blob('a', 25))
	queue.Push(blob('b', 5))
	capacity := queue.Capacity()
	_, err := queue.Push(blob('c', 20))

	// then
	assertEqual(t, 50, capacity)
	assertEqual(t, "Full queue. Maximum size limit reached.", err.Error())
	assertEqual(t, blob('a', 25), pop(queue))
	assertEqual(t, blob('b', 5), pop(queue))
}

func TestPushEntryAfterAllocateAdditionMemory(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(9, 20, true)

	// when
	queue.Push([]byte("aaa"))
	queue.Push([]byte("bb"))
	queue.Pop()

	// allocate more memory
	assertEqual(t, 9, queue.Capacity())
	queue.Push([]byte("c"))
	assertEqual(t, 18, queue.Capacity())

	// push after allocate
	_, err := queue.Push([]byte("d"))
	noError(t, err)
}

func TestPushEntryAfterAllocateAdditionMemoryInFull(t *testing.T) {
	t.Parallel()

	// given
	queue := NewBytesQueue(9, 40, true)

	// when
	queue.Push([]byte("aaa"))
	queue.Push([]byte("bb"))
	_, err := queue.Pop()
	noError(t, err)

	queue.Push([]byte("c"))
	queue.Push([]byte("d"))
	queue.Push([]byte("e"))
	_, err = queue.Pop()
	noError(t, err)
	_, err = queue.Pop()
	noError(t, err)
	queue.Push([]byte("fff"))
	_, err = queue.Pop()
	noError(t, err)
}

func pop(queue *BytesQueue) []byte {
	entry, err := queue.Pop()
	if err != nil {
		panic(err)
	}
	return entry
}

func get(queue *BytesQueue, index int) []byte {
	entry, err := queue.Get(index)
	if err != nil {
		panic(err)
	}
	return entry
}

func blob(char byte, len int) []byte {
	return bytes.Repeat([]byte{char}, len)
}

func assertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	if !objectsAreEqual(expected, actual) {
		_, file, line, _ := runtime.Caller(1)
		file = path.Base(file)
		t.Errorf(fmt.Sprintf("\n%s:%d: Not equal: \n"+
			"expected: %T(%#v)\n"+
			"actual  : %T(%#v)\n",
			file, line, expected, expected, actual, actual), msgAndArgs...)
	}
}

func noError(t *testing.T, e error) {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		file = path.Base(file)
		t.Errorf(fmt.Sprintf("\n%s:%d: Error is not nil: \n"+
			"actual  : %T(%#v)\n", file, line, e, e))
	}
}

func objectsAreEqual(expected, actual interface{}) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}
	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}
