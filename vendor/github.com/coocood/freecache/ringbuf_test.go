package freecache

import (
	"testing"
)

func TestRingBuf(t *testing.T) {
	rb := NewRingBuf(16, 0)
	rb.Write([]byte("fghibbbbccccddde"))
	rb.Write([]byte("fghibbbbc"))
	rb.Resize(16)
	off := rb.Evacuate(9, 3)
	t.Log(string(rb.Dump()))
	if off != rb.End()-3 {
		t.Log(string(rb.Dump()), rb.End())
		t.Fatalf("off got %v", off)
	}
	off = rb.Evacuate(15, 5)
	t.Log(string(rb.Dump()))
	if off != rb.End()-5 {
		t.Fatalf("off got %v", off)
	}
	rb.Resize(64)
	rb.Resize(32)
	data := make([]byte, 5)
	rb.ReadAt(data, off)
	if string(data) != "efghi" {
		t.Fatalf("read at should be efghi, got %v", string(data))
	}

	off = rb.Evacuate(0, 10)
	if off != -1 {
		t.Fatal("evacutate out of range offset should return error")
	}
}
