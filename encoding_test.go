package bigcache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecode(t *testing.T) {
	// given
	now := uint64(time.Now().Unix())
	hash := uint64(42)
	key := "key"
	data := []byte("data")

	// when
	wrapped := wrapEntry(now, hash, key, data)

	// then
	assert.Equal(t, key, readKeyFromEntry(wrapped))
	assert.Equal(t, hash, readHashFromEntry(wrapped))
	assert.Equal(t, now, readTimestampFromEntry(wrapped))
	assert.Equal(t, data, readEntry(wrapped))
}
