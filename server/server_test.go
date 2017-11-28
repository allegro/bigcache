package main

import (
	"bytes"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/allegro/bigcache"
)

const (
	testBaseString = "http://bigcache.org"
)

func testCacheSetup() {
	cache, _ = bigcache.NewBigCache(bigcache.Config{
		Shards:             1024,
		LifeWindow:         10 * time.Minute,
		MaxEntriesInWindow: 1000 * 10 * 60,
		MaxEntrySize:       500,
		Verbose:            true,
		HardMaxCacheSize:   8192,
		OnRemove:           nil,
	})
}

func TestMain(m *testing.M) {
	testCacheSetup()
	m.Run()
}

func TestGetWithNoKey(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", testBaseString+"/api/v1/cache/", nil)
	rr := httptest.NewRecorder()

	getCacheHandler(rr, req)
	resp := rr.Result()

	if resp.StatusCode != 400 {
		t.Errorf("want: 400; got: %d", resp.StatusCode)
	}
}

func TestGetWithMissingKey(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", testBaseString+"/api/v1/cache/doesNotExist", nil)
	rr := httptest.NewRecorder()

	getCacheHandler(rr, req)
	resp := rr.Result()

	if resp.StatusCode != 404 {
		t.Errorf("want: 404; got: %d", resp.StatusCode)
	}
}

func TestGetKey(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", testBaseString+"/api/v1/cache/getKey", nil)
	rr := httptest.NewRecorder()

	// set something.
	cache.Set("getKey", []byte("123"))

	getCacheHandler(rr, req)
	resp := rr.Result()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("cannot deserialise test response: %s", err)
	}

	if string(body) != "123" {
		t.Errorf("want: 123; got: %s.\n\tcan't get existing key getKey.", string(body))
	}
}

func TestPutKey(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("PUT", testBaseString+"/api/v1/cache/putKey", bytes.NewBuffer([]byte("123")))
	rr := httptest.NewRecorder()

	putCacheHandler(rr, req)

	testPutKeyResult, err := cache.Get("putKey")
	if err != nil {
		t.Errorf("error returning cache entry: %s", err)
	}

	if string(testPutKeyResult) != "123" {
		t.Errorf("want: 123; got: %s.\n\tcan't get PUT key putKey.", string(testPutKeyResult))
	}
}

func TestPutEmptyKey(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("PUT", testBaseString+"/api/v1/cache/", bytes.NewBuffer([]byte("123")))
	rr := httptest.NewRecorder()

	putCacheHandler(rr, req)
	resp := rr.Result()

	if resp.StatusCode != 400 {
		t.Errorf("want: 400; got: %d.\n\tempty key insertion should return with 400", resp.StatusCode)
	}
}
