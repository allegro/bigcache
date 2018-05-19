package bigcache

import (
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	pb "github.com/allegro/bigcache/pb"
	qpb "github.com/allegro/bigcache/queue/pb"
	"github.com/magiconair/properties/assert"
)

func TestBigCacheExport(t *testing.T) {
	cache, _ := NewBigCache(DefaultConfig(5 * time.Second))
	hasher := sha256.New()

	firstShard := pb.CacheShard{
		Hashmap: cache.shards[0].hashmap,
		Entries: &qpb.BytesQueue{
			Array:           cache.shards[0].entries.Array,
			Queuecapacity:   uint64(cache.shards[0].entries.QueueCapacity),
			MaxCapacity:     uint64(cache.shards[0].entries.MaxCapacity),
			Head:            uint64(cache.shards[0].entries.Head),
			Tail:            uint64(cache.shards[0].entries.Tail),
			Count:           uint64(cache.shards[0].entries.Count),
			RightMargin:     uint64(cache.shards[0].entries.RightMargin),
			HeaderBuffer:    cache.shards[0].entries.HeaderBuffer,
			Verbose:         cache.shards[0].entries.Verbose,
			InitialCapacity: uint64(cache.shards[0].entries.InitialCapacity),
		},
		Entrybuffer: cache.shards[0].entryBuffer,
		IsVerbose:   cache.shards[0].isVerbose,
		LifeWindow:  cache.shards[0].lifeWindow,
		Stats: &pb.Stats{
			Hits:       cache.shards[0].stats.Hits,
			Misses:     cache.shards[0].stats.Misses,
			DelHits:    cache.shards[0].stats.DelHits,
			DelMisses:  cache.shards[0].stats.DelMisses,
			Collisions: cache.shards[0].stats.Collisions,
		},
	}

	shardData, err := firstShard.Marshal()
	if err != nil {
		t.Error(err)
	}

	testData := shardData
	testChecksum := fmt.Sprintf("%x", hasher.Sum(shardData))
	hasher.Reset()

	for i := 0; i < 10; i++ {
		cache.Set(fmt.Sprintf("%d", i), []byte("value"))
	}

	target, err := cache.Export()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, testChecksum, target.DataChecksum[0])
	assert.Equal(t, testData, target.Data[0])
}
