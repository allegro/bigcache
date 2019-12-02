package bigcache

import (
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	// given
	now := uint64(time.Now().Unix())
	hash := uint64(42)
	key := "key"
	data := []byte("data")
	buffer := make([]byte, 100)

	// when
	wrapped := wrapEntry(now, hash, key, data, &buffer)

	// then
	assertEqual(t, key, readKeyFromEntry(wrapped))
	assertEqual(t, hash, readHashFromEntry(wrapped))
	assertEqual(t, now, readTimestampFromEntry(wrapped))
	assertEqual(t, data, readEntry(wrapped))
	assertEqual(t, 100, len(buffer))
}

func TestAllocateBiggerBuffer(t *testing.T) {
	//given
	now := uint64(time.Now().Unix())
	hash := uint64(42)
	key := "1"
	data := []byte("2")
	buffer := make([]byte, 1)

	// when
	wrapped := wrapEntry(now, hash, key, data, &buffer)

	// then
	assertEqual(t, key, readKeyFromEntry(wrapped))
	assertEqual(t, hash, readHashFromEntry(wrapped))
	assertEqual(t, now, readTimestampFromEntry(wrapped))
	assertEqual(t, data, readEntry(wrapped))
	assertEqual(t, 2+headersSizeInBytes, len(buffer))
}
