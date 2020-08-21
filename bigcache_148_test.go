package bigcache

import (
	"bytes"
	"strconv"
	"testing"
	"time"
)

func Test_issue_148(t *testing.T) {
	const n = 2070400
	var message = bytes.Repeat([]byte{0}, 2<<10)
	cache, _ := NewBigCache(Config{
		Shards:             1,
		LifeWindow:         time.Hour,
		MaxEntriesInWindow: 10,
		MaxEntrySize:       len(message),
		HardMaxCacheSize:   2 << 13,
	})
	for i := 0; i < n; i++ {
		err := cache.Set(strconv.Itoa(i), message)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := cache.Set(strconv.Itoa(n), message)
	if err != nil {
		t.Fatal(err)
	}

	cache.Get(strconv.Itoa(n))

	i := 0
	defer func() {
		if r := recover(); r != nil {
			t.Log("Element: ", i)
			t.Fatal(r)
		}
	}()

	for ; i < n; i++ {
		v, err := cache.Get(strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(v, message) {
			t.Fatal("Should be equal", i, v, message)
		}
	}
}
