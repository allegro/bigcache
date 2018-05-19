package bigcache

import (
	"crypto/sha256"
	"fmt"
	"time"

	pb "github.com/allegro/bigcache/pb"
	qpb "github.com/allegro/bigcache/queue/pb"
)

const (
	fileVersion = 1
)

// Layout defines the file structure of the cache on export.
type Layout struct {
	// which version of the layout we are working with.
	Version int `json:"version"`
	// when the file was last written.
	Date string `json:"date"`
	// a checksum of each shard segment to make sure it's not been tampered with.
	DataChecksum map[int64]string `json:"data_checksum"`
	// each shard's data encoded as a gob.
	Data map[int64][]byte `json:"data"`
}

// Exports data in a standardized and easy to understand format.
func (c *BigCache) export() (Layout, error) {
	fl := Layout{
		Data:         make(map[int64][]byte),
		DataChecksum: make(map[int64]string),
	}
	fl.Version = fileVersion
	hasher := sha256.New()

	for idx, shard := range c.shards {
		// prepare the shard for encoding.
		// todo (mxplusb): optimise this a bit. not sure how else to do that.
		p := pb.CacheShard{
			Hashmap: shard.hashmap,
			Entries: &qpb.BytesQueue{
				Array:           shard.entries.Array,
				Queuecapacity:   uint64(shard.entries.QueueCapacity),
				MaxCapacity:     uint64(shard.entries.MaxCapacity),
				Head:            uint64(shard.entries.Head),
				Tail:            uint64(shard.entries.Tail),
				Count:           uint64(shard.entries.Count),
				RightMargin:     uint64(shard.entries.RightMargin),
				HeaderBuffer:    shard.entries.HeaderBuffer,
				Verbose:         shard.entries.Verbose,
				InitialCapacity: uint64(shard.entries.InitialCapacity),
			},
			Entrybuffer: shard.entryBuffer,
			IsVerbose:   shard.isVerbose,
			LifeWindow:  shard.lifeWindow,
			Stats: &pb.Stats{
				Hits:       shard.stats.Hits,
				Misses:     shard.stats.Misses,
				DelHits:    shard.stats.DelHits,
				DelMisses:  shard.stats.DelMisses,
				Collisions: shard.stats.Collisions,
			},
		}
		// encode the shard.
		shardData, err := p.Marshal()
		if err != nil {
			return Layout{}, err
		}
		// get the hash of the encoded shard so we can validate the shard is untouched.
		fl.Data[int64(idx)] = shardData
		fl.DataChecksum[int64(idx)] = fmt.Sprintf("%x", hasher.Sum(shardData))
		hasher.Reset()
	}
	// always use UTC time.
	fl.Date = time.Now().UTC().Format(time.RFC3339)
	return fl, nil
}

// Export will export the data to work with downstream.
func (c *BigCache) Export() (Layout, error) {
	return c.export()
}
