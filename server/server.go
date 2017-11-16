package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/allegro/bigcache"
)

const (
	// base HTTP paths.
	apiVersion  = "v1"
	apiBasePath = "/api/" + apiVersion + "/"

	// path to cache.
	cachePath = apiBasePath + "cache/"
)

var (
	port    int
	logfile string

	// cache-specific settings.
	cache  *bigcache.BigCache
	config = bigcache.Config{}
)

func init() {
	flag.BoolVar(&config.Verbose, "v", false, "Verbose logging.")
	flag.IntVar(&config.Shards, "shards", 1024, "Number of shards for the cache.")
	flag.IntVar(&config.MaxEntriesInWindow, "maxInWindow", 1000*10*60, "Used only in initial memory allocation.")
	flag.DurationVar(&config.LifeWindow, "lifetime", 100000*100000*60, "Lifetime of each cache object.")
	flag.IntVar(&config.HardMaxCacheSize, "max", 8192, "Maximum amount of data in the cache in MB.")
	flag.IntVar(&config.MaxEntrySize, "maxShardEntrySize", 500, "The maximum size of each object stored in a shard. Used only in initial memory allocation.")
	flag.IntVar(&port, "port", 9090, "The port to listen on.")
	flag.StringVar(&logfile, "logfile", "", "Location of the logfile.")
}

func main() {
	flag.Parse()

	var logger *log.Logger

	if logfile == "" {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		f, err := os.OpenFile(logfile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		logger = log.New(f, "", log.LstdFlags)
	}

	var err error
	cache, err = bigcache.NewBigCache(config)
	if err != nil {
		logger.Fatal(err)
	}

	logger.Print("cache initialised.")

	// let the middleware log.
	http.Handle(cachePath, serviceLoader(cacheIndexHandler(), requestMetrics(logger)))

	logger.Printf("starting server on :%d", port)

	strPort := ":" + strconv.Itoa(port)
	log.Fatal("ListenAndServe: ", http.ListenAndServe(strPort, nil))
}

// our base middleware implementation.
type service func(http.Handler) http.Handler

// chain load middleware services.
func serviceLoader(h http.Handler, svcs ...service) http.Handler {
	for _, svc := range svcs {
		h = svc(h)
	}
	return h
}

func cacheIndexHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getCacheHandler(w, r)
		case http.MethodPut:
			putCacheHandler(w, r)
		}
	})
}

// middleware for request length metrics.
func requestMetrics(l *log.Logger) service {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			h.ServeHTTP(w, r)
			l.Printf("request took %vns.", time.Now().Sub(start).Nanoseconds())
		})
	}
}

// handles get requests.
func getCacheHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Path[len(cachePath):]
	if target == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("can't get a key if there is no key."))
		log.Print("empty request.")
		return
	}
	entry, err := cache.Get(target)
	if err != nil {
		errMsg := (err).Error()
		if strings.Contains(errMsg, "not found") {
			log.Print(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(entry)
}

func putCacheHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Path[len(cachePath):]
	if target == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("can't put a key if there is no key."))
		log.Print("empty request.")
		return
	}

	entry, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := cache.Set(target, []byte(entry)); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("stored \"%s\" in cache.", target)
	w.WriteHeader(http.StatusCreated)
}
