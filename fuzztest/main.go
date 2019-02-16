package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/allegro/bigcache"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func fuzzDeletePutGet(ctx context.Context) {
	c := bigcache.DefaultConfig(time.Second)
	c.Shards = 1
	c.MaxEntriesInWindow = 10
	c.MaxEntriesInWindow = 10
	c.HardMaxCacheSize = 1

	cache, _ := bigcache.NewBigCache(c)
	var wg sync.WaitGroup

	// Deleter
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				r := uint8(rand.Int())
				key := fmt.Sprintf("thekey%d", r)
				cache.Delete(key)
			}
		}
	}()

	// Setter
	wg.Add(1)
	go func() {
		defer wg.Done()
		val := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				r := byte(rand.Int())
				key := fmt.Sprintf("thekey%d", r)

				for j := 0; j < len(val); j++ {
					val[j] = r
				}
				cache.Set(key, val)
			}
		}
	}()

	// Getter
	wg.Add(1)
	go func() {
		defer wg.Done()
		var (
			val    = make([]byte, 1024)
			hits   = uint64(0)
			misses = uint64(0)
		)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				r := byte(rand.Int())
				key := fmt.Sprintf("thekey%d", r)

				for j := 0; j < len(val); j++ {
					val[j] = r
				}
				if got, err := cache.Get(key); err == nil && !bytes.Equal(got, val) {
					errStr := fmt.Sprintf("got %s ->\n %x\n expected:\n %x\n ", key, got, val)
					panic(errStr)
				} else {
					if err == nil {
						hits++
					} else {
						misses++
					}
				}
				if total := hits + misses; total%1000000 == 0 {
					percentage := float64(100) * float64(hits) / float64(total)
					fmt.Printf("Hits %d (%.2f%%) misses %d \n", hits, percentage, misses)
				}
			}
		}
	}()
	wg.Wait()

}
func main() {

	sigs := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Press ctrl-c to exit")
	go fuzzDeletePutGet(ctx)

	<-sigs
	fmt.Println("Exiting...")
	cancel()

}
