package bigcache

import (
	"testing"
	"time"
)

func TestShardOnEvictWithShortEntries(t *testing.T) {
	t.Parallel()

	clock := &mockedClock{value: 1000}
	cfg := DefaultConfig(10 * time.Second)
	cfg.MaxEntrySize = 100
	shard := initNewShard(cfg, func(wrappedEntry []byte, reason RemoveReason) {}, clock)

	for length := 0; length < headersSizeInBytes; length++ {
		dummy := make([]byte, length)
		evicted := false
		evictFn := func(reason RemoveReason) error {
			evicted = true
			assertEqual(t, Expired, reason)
			return nil
		}

		res := shard.onEvict(dummy, 1000, evictFn)
		assertEqual(t, true, res)
		assertEqual(t, true, evicted)
	}
}

func TestShardCleanUpWithShortEntries(t *testing.T) {
	t.Parallel()

	clock := &mockedClock{value: 1000}
	cfg := DefaultConfig(10 * time.Second)
	cfg.MaxEntrySize = 100
	cfg.CleanWindow = 1 * time.Second
	onRemoveCalled := false
	shard := initNewShard(cfg, func(wrappedEntry []byte, reason RemoveReason) {
		onRemoveCalled = true
	}, clock)

	for length := 0; length < headersSizeInBytes; length++ {
		_, err := shard.entries.Push(make([]byte, length))
		noError(t, err)
	}

	err := shard.set("validKey", 12345, []byte("validValue"))
	noError(t, err)

	shard.cleanUp(1000)
	assertEqual(t, false, onRemoveCalled)

	val, err := shard.get("validKey", 12345)
	noError(t, err)
	assertEqual(t, []byte("validValue"), val)
}

func TestShardSetWithShortEntriesWhenCleanWindowDisabled(t *testing.T) {
	t.Parallel()

	clock := &mockedClock{value: 1000}
	cfg := DefaultConfig(10 * time.Second)
	cfg.MaxEntrySize = 100
	cfg.CleanWindow = 0
	onRemoveCalled := false
	shard := initNewShard(cfg, func(wrappedEntry []byte, reason RemoveReason) {
		onRemoveCalled = true
	}, clock)

	_, err := shard.entries.Push([]byte{})
	noError(t, err)

	err = shard.set("key1", 111, []byte("val1"))
	noError(t, err)
	assertEqual(t, false, onRemoveCalled)

	val, err := shard.get("key1", 111)
	noError(t, err)
	assertEqual(t, []byte("val1"), val)
}

func TestShardSetWithShortEntriesOnNoSpace(t *testing.T) {
	t.Parallel()

	clock := &mockedClock{value: 1000}
	cfg := DefaultConfig(10 * time.Second)
	cfg.MaxEntrySize = 50
	cfg.HardMaxCacheSize = 1
	cfg.Shards = 1
	cfg.MaxEntriesInWindow = 10
	shard := initNewShard(cfg, func(wrappedEntry []byte, reason RemoveReason) {}, clock)

	for length := 0; length < headersSizeInBytes; length++ {
		_, _ = shard.entries.Push(make([]byte, length))
	}

	for i := 0; i < 20; i++ {
		err := shard.set("key", uint64(i), make([]byte, 40))
		if err == nil {
			break
		}
	}
}

func TestShardRemoveOldestEntryWithShortEntry(t *testing.T) {
	t.Parallel()

	clock := &mockedClock{value: 1000}
	cfg := DefaultConfig(10 * time.Second)
	cfg.MaxEntrySize = 100
	onRemoveCalled := false
	shard := initNewShard(cfg, func(wrappedEntry []byte, reason RemoveReason) {
		onRemoveCalled = true
	}, clock)

	for length := 0; length < headersSizeInBytes; length++ {
		_, err := shard.entries.Push(make([]byte, length))
		noError(t, err)

		err = shard.removeOldestEntry(Expired)
		noError(t, err)
		assertEqual(t, false, onRemoveCalled)
	}
}
