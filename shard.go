package bigcache

import (
	"sync"

	"github.com/allegro/bigcache/queue"
)

type cacheShard struct {
	Hashmap     map[uint64]uint32 `json:"hashmap"`
	Entries     queue.BytesQueue  `json:"entries"`
	lock        sync.RWMutex
	EntryBuffer []byte `json:"entry_buffer"`
	onRemove    func(wrappedEntry []byte)
}

type onRemoveCallback func(wrappedEntry []byte)

func (s *cacheShard) removeOldestEntry() error {
	oldest, err := s.Entries.Pop()
	if err == nil {
		hash := readHashFromEntry(oldest)
		delete(s.Hashmap, hash)
		s.onRemove(oldest)
		return nil
	}
	return err
}

func (s *cacheShard) reset(config Config) {
	s.lock.Lock()

	s.Hashmap = make(map[uint64]uint32, config.initialShardSize())
	s.EntryBuffer = make([]byte, config.MaxEntrySize+headersSizeInBytes)
	s.Entries.Reset()

	s.lock.Unlock()
}

func (s *cacheShard) len() int {
	return len(s.Hashmap)
}

func initNewShard(config Config, callback onRemoveCallback) *cacheShard {
	return &cacheShard{
		Hashmap:     make(map[uint64]uint32, config.initialShardSize()),
		Entries:     *queue.NewBytesQueue(config.initialShardSize()*config.MaxEntrySize, config.maximumShardSize(), config.Verbose),
		EntryBuffer: make([]byte, config.MaxEntrySize+headersSizeInBytes),
		onRemove:    callback,
	}
}
