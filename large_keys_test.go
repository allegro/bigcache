package bigcache

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLargeKeyRoundTrip(t *testing.T) {
	t.Parallel()

	for _, size := range []int{65517, 65518, 65535} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			cache, err := New(context.Background(), Config{Shards: 1, LifeWindow: time.Hour})
			noError(t, err)
			defer cache.Close()
			key := strings.Repeat("k", size)
			noError(t, cache.Set(key, []byte("value")))
			value, err := cache.Get(key)
			noError(t, err)
			assertEqual(t, []byte("value"), value)

			noError(t, cache.Append(key, []byte(" appended")))
			value, _, err = cache.GetWithInfo(key)
			noError(t, err)
			assertEqual(t, []byte("value appended"), value)

			iterator := cache.Iterator()
			assertEqual(t, true, iterator.SetNext())
			entry, err := iterator.Value()
			noError(t, err)
			assertEqual(t, key, entry.Key())
			assertEqual(t, []byte("value appended"), entry.Value())
		})
	}
}

func TestRejectKeyLongerThanHeaderCanRepresent(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"Set", "Append"} {
		t.Run(operation, func(t *testing.T) {
			cache, err := New(context.Background(), Config{Shards: 1, LifeWindow: time.Hour, Hasher: hashStub(1)})
			noError(t, err)
			defer cache.Close()
			noError(t, cache.Set("existing", []byte("original")))
			key := strings.Repeat("k", 65536)
			if operation == "Set" {
				err = cache.Set(key, []byte("value"))
			} else {
				err = cache.Append(key, []byte("value"))
			}
			if err == nil {
				t.Fatal("expected an error for a key longer than 65535 bytes")
			}
			assertEqual(t, 1, cache.Len())
			value, err := cache.Get("existing")
			noError(t, err)
			assertEqual(t, []byte("original"), value)
		})
	}
}
