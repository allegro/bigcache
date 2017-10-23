package bigcache

import (
	"fmt"
	"sync"

	"github.com/allegro/bigcache/queue"
)

type cacheShard struct {
	hashmap     map[uint64]uint32
	entries     queue.BytesQueue
	lock        sync.RWMutex
	entryBuffer []byte
	onRemove    func(wrappedEntry []byte)

	isVerbose  bool
	logger     Logger
	clock      clock
	lifeWindow uint64
}

type onRemoveCallback func(wrappedEntry []byte)

func (s *cacheShard) get(key string, hashedKey uint64) ([]byte, error) {
	s.lock.RLock()
	itemIndex := s.hashmap[hashedKey]

	if itemIndex == 0 {
		s.lock.RUnlock()
		return nil, notFound(key)
	}

	wrappedEntry, err := s.entries.Get(int(itemIndex))
	if err != nil {
		s.lock.RUnlock()
		return nil, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		if s.isVerbose {
			s.logger.Printf("Collision detected. Both %q and %q have the same hash %x", key, entryKey, hashedKey)
		}
		s.lock.RUnlock()
		return nil, notFound(key)
	}
	s.lock.RUnlock()
	return readEntry(wrappedEntry), nil
}

func (s *cacheShard) set(key string, hashedKey uint64, entry []byte) error {
	currentTimestamp := uint64(s.clock.epoch())

	s.lock.Lock()

	if previousIndex := s.hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := s.entries.Get(int(previousIndex)); err == nil {
			resetKeyFromEntry(previousEntry)
		}
	}

	if oldestEntry, err := s.entries.Peek(); err == nil {
		s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry)
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &s.entryBuffer)

	for {
		if index, err := s.entries.Push(w); err == nil {
			s.hashmap[hashedKey] = uint32(index)
			s.lock.Unlock()
			return nil
		} else if s.removeOldestEntry() != nil {
			s.lock.Unlock()
			return fmt.Errorf("Entry is bigger than max shard size.")
		}
	}
}

func (s *cacheShard) del(key string, hashedKey uint64) error {
	s.lock.RLock()
	itemIndex := s.hashmap[hashedKey]

	if itemIndex == 0 {
		s.lock.RUnlock()
		return notFound(key)
	}

	wrappedEntry, err := s.entries.Get(int(itemIndex))
	if err != nil {
		s.lock.RUnlock()
		return err
	}

	resetKeyFromEntry(wrappedEntry)
	delete(s.hashmap, hashedKey)
	s.onRemove(wrappedEntry)
	s.lock.RUnlock()
	return nil
}

func (s *cacheShard) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func() error) bool {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp-oldestTimestamp > s.lifeWindow {
		evict()
		return true
	}
	return false
}

func (s *cacheShard) cleanUp(currentTimestamp uint64) {
	s.lock.Lock()
	for {
		if oldestEntry, err := s.entries.Peek(); err != nil {
			break
		} else if evicted := s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry); !evicted {
			break
		}
	}
	s.lock.Unlock()
}

func (s *cacheShard) getOldestEntry() ([]byte, error) {
	return s.entries.Peek()
}

func (s *cacheShard) getEntry(index int) ([]byte, error) {
	return s.entries.Get(index)
}

func (s *cacheShard) copyKeys() (keys []uint32, next int) {
	keys = make([]uint32, len(s.hashmap))

	s.lock.RLock()

	for _, index := range s.hashmap {
		keys[next] = index
		next++
	}

	s.lock.RUnlock()
	return keys, next
}

func (s *cacheShard) removeOldestEntry() error {
	oldest, err := s.entries.Pop()
	if err == nil {
		hash := readHashFromEntry(oldest)
		delete(s.hashmap, hash)
		s.onRemove(oldest)
		return nil
	}
	return err
}

func (s *cacheShard) reset(config Config) {
	s.lock.Lock()
	s.hashmap = make(map[uint64]uint32, config.initialShardSize())
	s.entryBuffer = make([]byte, config.MaxEntrySize+headersSizeInBytes)
	s.entries.Reset()
	s.lock.Unlock()
}

func (s *cacheShard) len() int {
	s.lock.RLock()
	res := len(s.hashmap)
	s.lock.RUnlock()
	return res
}

func initNewShard(config Config, callback onRemoveCallback, clock clock) *cacheShard {
	return &cacheShard{
		hashmap:     make(map[uint64]uint32, config.initialShardSize()),
		entries:     *queue.NewBytesQueue(config.initialShardSize()*config.MaxEntrySize, config.maximumShardSize(), config.Verbose),
		entryBuffer: make([]byte, config.MaxEntrySize+headersSizeInBytes),
		onRemove:    callback,

		isVerbose:  config.Verbose,
		logger:     newLogger(config.Logger),
		clock:      clock,
		lifeWindow: uint64(config.LifeWindow.Seconds()),
	}
}
