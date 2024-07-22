package bigcache

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/allegro/bigcache/v3/queue"
)

type onRemoveCallback func(wrappedEntry []byte, reason RemoveReason)

// Metadata contains information of a specific entry
type Metadata struct {
	RequestCount uint32
}

type cacheShard struct {
	hashmap     map[uint64]uint64
	entries     queue.BytesQueue
	lock        sync.RWMutex
	entryBuffer []byte
	onRemove    onRemoveCallback

	isVerbose    bool
	statsEnabled bool
	logger       Logger
	clock        clock
	lifeWindow   uint64

	hashmapStats map[uint64]uint32
	stats        Stats
	cleanEnabled bool
}

func (s *cacheShard) getWithInfo(key string, hashedKey uint64) (entry []byte, resp Response, err error) {
	currentTime := uint64(s.clock.Epoch())
	entry, resp, err = s.getWithInfoByLock(key, hashedKey, currentTime)
	if err != nil {
		return entry, resp, err
	}
	s.hit(hashedKey)
	return entry, resp, nil
}

func (s *cacheShard) getWithInfoByLock(key string, hashedKey uint64,
	currentTime uint64) (entry []byte, resp Response, err error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	wrappedEntry, err := s.getWrappedEntry(hashedKey)
	if err != nil {
		return nil, resp, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		s.collision()
		if s.isVerbose {
			s.logger.Printf("Collision detected. Both %q and %q have the same hash %x", key, entryKey, hashedKey)
		}
		return nil, resp, ErrEntryNotFound
	}

	entry = readEntry(wrappedEntry)
	if s.isExpired(wrappedEntry, currentTime) {
		resp.EntryStatus = Expired
	}
	return entry, resp, nil
}

func (s *cacheShard) get(key string, hashedKey uint64) ([]byte, error) {
	entry, err := s.getByLock(key, hashedKey)
	if err != nil {
		return entry, err
	}
	s.hit(hashedKey)
	return entry, nil
}

func (s *cacheShard) getByLock(key string, hashedKey uint64) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	wrappedEntry, err := s.getWrappedEntry(hashedKey)
	if err != nil {
		return nil, err
	}
	if entryKey := readKeyFromEntry(wrappedEntry); key != entryKey {
		s.collision()
		if s.isVerbose {
			s.logger.Printf("Collision detected. Both %q and %q have the same hash %x", key, entryKey, hashedKey)
		}
		return nil, ErrEntryNotFound
	}
	entry := readEntry(wrappedEntry)
	return entry, nil
}

func (s *cacheShard) getWrappedEntry(hashedKey uint64) ([]byte, error) {
	itemIndex := s.hashmap[hashedKey]

	if itemIndex == 0 {
		s.miss()
		return nil, ErrEntryNotFound
	}

	wrappedEntry, err := s.entries.Get(int(itemIndex))
	if err != nil {
		s.miss()
		return nil, err
	}

	return wrappedEntry, err
}

func (s *cacheShard) getValidWrapEntry(key string, hashedKey uint64) ([]byte, error) {
	wrappedEntry, err := s.getWrappedEntry(hashedKey)
	if err != nil {
		return nil, err
	}

	if !compareKeyFromEntry(wrappedEntry, key) {
		s.collision()
		if s.isVerbose {
			s.logger.Printf("Collision detected. Both %q and %q have the same hash %x", key,
				readKeyFromEntry(wrappedEntry), hashedKey)
		}

		return nil, ErrEntryNotFound
	}
	s.hitWithoutLock(hashedKey)

	return wrappedEntry, nil
}

func (s *cacheShard) set(key string, hashedKey uint64, entry []byte) error {
	currentTimestamp := uint64(s.clock.Epoch())

	s.lock.Lock()
	defer s.lock.Unlock()
	if previousIndex := s.hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := s.entries.Get(int(previousIndex)); err == nil {
			resetHashFromEntry(previousEntry)
			//remove hashkey
			delete(s.hashmap, hashedKey)
		}
	}

	if !s.cleanEnabled {
		if oldestEntry, err := s.entries.Peek(); err == nil {
			s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry)
		}
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &s.entryBuffer)

	for {
		if index, err := s.entries.Push(w); err == nil {
			s.hashmap[hashedKey] = uint64(index)
			return nil
		}
		if s.removeOldestEntry(NoSpace) != nil {
			return errors.New("entry is bigger than max shard size")
		}
	}
}

func (s *cacheShard) addNewWithoutLock(key string, hashedKey uint64, entry []byte) error {
	currentTimestamp := uint64(s.clock.Epoch())

	if !s.cleanEnabled {
		if oldestEntry, err := s.entries.Peek(); err == nil {
			s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry)
		}
	}

	w := wrapEntry(currentTimestamp, hashedKey, key, entry, &s.entryBuffer)

	for {
		if index, err := s.entries.Push(w); err == nil {
			s.hashmap[hashedKey] = uint64(index)
			return nil
		}
		if s.removeOldestEntry(NoSpace) != nil {
			return errors.New("entry is bigger than max shard size")
		}
	}
}

func (s *cacheShard) setWrappedEntryWithoutLock(currentTimestamp uint64, w []byte, hashedKey uint64) error {
	if previousIndex := s.hashmap[hashedKey]; previousIndex != 0 {
		if previousEntry, err := s.entries.Get(int(previousIndex)); err == nil {
			resetHashFromEntry(previousEntry)
		}
	}

	if !s.cleanEnabled {
		if oldestEntry, err := s.entries.Peek(); err == nil {
			s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry)
		}
	}

	for {
		if index, err := s.entries.Push(w); err == nil {
			s.hashmap[hashedKey] = uint64(index)
			return nil
		}
		if s.removeOldestEntry(NoSpace) != nil {
			return errors.New("entry is bigger than max shard size")
		}
	}
}

func (s *cacheShard) append(key string, hashedKey uint64, entry []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	wrappedEntry, err := s.getValidWrapEntry(key, hashedKey)

	if err == ErrEntryNotFound {
		err = s.addNewWithoutLock(key, hashedKey, entry)
		return err
	}
	if err != nil {
		return err
	}

	currentTimestamp := uint64(s.clock.Epoch())

	w := appendToWrappedEntry(currentTimestamp, wrappedEntry, entry, &s.entryBuffer)

	err = s.setWrappedEntryWithoutLock(currentTimestamp, w, hashedKey)

	return err
}

func (s *cacheShard) preDel(hashedKey uint64) error {
	s.lock.RLock()
	defer s.lock.RUnlock()

	itemIndex := s.hashmap[hashedKey]

	if itemIndex == 0 {
		s.delmiss()
		return ErrEntryNotFound
	}

	if err := s.entries.CheckGet(int(itemIndex)); err != nil {
		s.delmiss()
		return err
	}

	return nil
}

func (s *cacheShard) del(hashedKey uint64) error {
	// Optimistic pre-check using only readlock

	if err := s.preDel(hashedKey); err != nil {
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	{
		// After obtaining the writelock, we need to read the same again,
		// since the data delivered earlier may be stale now
		itemIndex := s.hashmap[hashedKey]

		if itemIndex == 0 {
			s.delmiss()
			return ErrEntryNotFound
		}

		wrappedEntry, err := s.entries.Get(int(itemIndex))
		if err != nil {
			s.delmiss()
			return err
		}

		delete(s.hashmap, hashedKey)
		s.onRemove(wrappedEntry, Deleted)
		if s.statsEnabled {
			delete(s.hashmapStats, hashedKey)
		}
		resetHashFromEntry(wrappedEntry)
	}

	s.delhit()
	return nil
}

func (s *cacheShard) onEvict(oldestEntry []byte, currentTimestamp uint64, evict func(reason RemoveReason) error) bool {
	if s.isExpired(oldestEntry, currentTimestamp) {
		evict(Expired)
		return true
	}
	return false
}

func (s *cacheShard) isExpired(oldestEntry []byte, currentTimestamp uint64) bool {
	oldestTimestamp := readTimestampFromEntry(oldestEntry)
	if currentTimestamp <= oldestTimestamp { // if currentTimestamp < oldestTimestamp, the result will out of uint64 limits;
		return false
	}
	return currentTimestamp-oldestTimestamp > s.lifeWindow
}

func (s *cacheShard) cleanUp(currentTimestamp uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for {
		if oldestEntry, err := s.entries.Peek(); err != nil {
			break
		} else if evicted := s.onEvict(oldestEntry, currentTimestamp, s.removeOldestEntry); !evicted {
			break
		}
	}
}

func (s *cacheShard) getEntry(hashedKey uint64) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	entry, err := s.getWrappedEntry(hashedKey)
	// copy entry
	newEntry := make([]byte, len(entry))
	copy(newEntry, entry)

	return newEntry, err
}

func (s *cacheShard) copyHashedKeys() (keys []uint64, next int) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	keys = make([]uint64, len(s.hashmap))

	for key := range s.hashmap {
		keys[next] = key
		next++
	}

	return keys, next
}

func (s *cacheShard) removeOldestEntry(reason RemoveReason) error {
	oldest, err := s.entries.Pop()
	if err == nil {
		hash := readHashFromEntry(oldest)
		if hash == 0 {
			// entry has been explicitly deleted with resetHashFromEntry, ignore
			return nil
		}
		delete(s.hashmap, hash)
		s.onRemove(oldest, reason)
		if s.statsEnabled {
			delete(s.hashmapStats, hash)
		}
		return nil
	}
	return err
}

func (s *cacheShard) reset(config Config) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.hashmap = make(map[uint64]uint64, config.initialShardSize())
	s.entryBuffer = make([]byte, config.MaxEntrySize+headersSizeInBytes)
	s.entries.Reset()
}

func (s *cacheShard) resetStats() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.stats = Stats{}
}

func (s *cacheShard) len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	res := len(s.hashmap)
	return res
}

func (s *cacheShard) capacity() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	res := s.entries.Capacity()
	return res
}

func (s *cacheShard) getStats() Stats {
	var stats = Stats{
		Hits:       atomic.LoadInt64(&s.stats.Hits),
		Misses:     atomic.LoadInt64(&s.stats.Misses),
		DelHits:    atomic.LoadInt64(&s.stats.DelHits),
		DelMisses:  atomic.LoadInt64(&s.stats.DelMisses),
		Collisions: atomic.LoadInt64(&s.stats.Collisions),
	}
	return stats
}

func (s *cacheShard) getKeyMetadataWithLock(key uint64) Metadata {
	s.lock.RLock()
	defer s.lock.RUnlock()
	c := s.hashmapStats[key]
	return Metadata{
		RequestCount: c,
	}
}

func (s *cacheShard) getKeyMetadata(key uint64) Metadata {
	return Metadata{
		RequestCount: s.hashmapStats[key],
	}
}

func (s *cacheShard) hit(key uint64) {
	atomic.AddInt64(&s.stats.Hits, 1)
	if s.statsEnabled {
		s.lock.Lock()
		defer s.lock.Unlock()
		s.hashmapStats[key]++
	}
}

func (s *cacheShard) hitWithoutLock(key uint64) {
	atomic.AddInt64(&s.stats.Hits, 1)
	if s.statsEnabled {
		s.hashmapStats[key]++
	}
}

func (s *cacheShard) miss() {
	atomic.AddInt64(&s.stats.Misses, 1)
}

func (s *cacheShard) delhit() {
	atomic.AddInt64(&s.stats.DelHits, 1)
}

func (s *cacheShard) delmiss() {
	atomic.AddInt64(&s.stats.DelMisses, 1)
}

func (s *cacheShard) collision() {
	atomic.AddInt64(&s.stats.Collisions, 1)
}

func initNewShard(config Config, callback onRemoveCallback, clock clock) *cacheShard {
	bytesQueueInitialCapacity := config.initialShardSize() * config.MaxEntrySize
	maximumShardSizeInBytes := config.maximumShardSizeInBytes()
	if maximumShardSizeInBytes > 0 && bytesQueueInitialCapacity > maximumShardSizeInBytes {
		bytesQueueInitialCapacity = maximumShardSizeInBytes
	}
	return &cacheShard{
		hashmap:      make(map[uint64]uint64, config.initialShardSize()),
		hashmapStats: make(map[uint64]uint32, config.initialShardSize()),
		entries:      *queue.NewBytesQueue(bytesQueueInitialCapacity, maximumShardSizeInBytes, config.Verbose),
		entryBuffer:  make([]byte, config.MaxEntrySize+headersSizeInBytes),
		onRemove:     callback,

		isVerbose:    config.Verbose,
		logger:       newLogger(config.Logger),
		clock:        clock,
		lifeWindow:   uint64(config.LifeWindow.Seconds()),
		statsEnabled: config.StatsEnabled,
		cleanEnabled: config.CleanWindow > 0,
	}
}
