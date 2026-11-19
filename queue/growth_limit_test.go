package queue

import (
	"bytes"
	"testing"
)

func TestGrowToMaximumCapacityForFittingEntry(t *testing.T) {
	t.Parallel()

	for _, size := range []int{150, 197} {
		queue := NewBytesQueue(100, 200, false)
		value := bytes.Repeat([]byte("v"), size)
		index, err := queue.Push(value)
		if err != nil {
			t.Fatalf("push %d bytes: %v", size, err)
		}
		actual, err := queue.Get(index)
		assertEqual(t, nil, err)
		assertEqual(t, value, actual)
		assertEqual(t, value, pop(queue))
		assertEqual(t, 200, queue.Capacity())
	}
}

func TestCappedGrowthPreservesExistingEntry(t *testing.T) {
	t.Parallel()

	queue := NewBytesQueue(128, 256, false)
	first := bytes.Repeat([]byte("a"), 50)
	second := bytes.Repeat([]byte("b"), 150)
	firstIndex, err := queue.Push(first)
	assertEqual(t, nil, err)
	_, err = queue.Push(second)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := queue.Get(firstIndex)
	assertEqual(t, nil, err)
	assertEqual(t, first, actual)
	assertEqual(t, first, pop(queue))
	assertEqual(t, second, pop(queue))
}
