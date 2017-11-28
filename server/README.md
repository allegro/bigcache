# BigCache HTTP Server

This is a basic HTTP server implementation for BigCache. It has a basic RESTful API.

```bash
GET /api/v1/cache/{key}
PUT /api/v1/cache/{key}
```

This server is designed to be consumed as a standalone executable, for things like Cloud Foundry, Heroku, etc.

### Command-line Interface

```powershell
PS C:\go\src\github.com\mxplusb\bigcache\server> .\server.exe -h
Usage of C:\go\src\github.com\mxplusb\bigcache\server\server.exe:
  -lifetime int
        Lifetime, in minutes, of each cache object. (default 10)
  -logfile string
        Location of the logfile.
  -max int
        Maximum amount of objects in the cache. (default 8192)
  -maxInWindow int
        Used only in initial memory allocation. (default 600000)
  -maxShardEntrySize int
        The maximum size of each object stored in a shard. Used only in initial memory allocation. (default 500)
  -port int
        The port to listen on. (default 9090)
  -shards int
        Number of shards for the cache. (default 1024)
  -v    Verbose logging.

```

Example:

```bash
$ curl -v -XPUT localhost:9090/api/v1/cache/example -d "yay!"
*   Trying 127.0.0.1...
* Connected to localhost (127.0.0.1) port 9090 (#0)
> PUT /api/v1/cache/example HTTP/1.1
> Host: localhost:9090
> User-Agent: curl/7.47.0
> Accept: */*
> Content-Length: 4
> Content-Type: application/x-www-form-urlencoded
>
* upload completely sent off: 4 out of 4 bytes
< HTTP/1.1 201 Created
< Date: Fri, 17 Nov 2017 03:50:07 GMT
< Content-Length: 0
< Content-Type: text/plain; charset=utf-8
<
* Connection #0 to host localhost left intact
$ 
$ curl -v -XGET localhost:9090/api/v1/cache/example
Note: Unnecessary use of -X or --request, GET is already inferred.
*   Trying 127.0.0.1...
* Connected to localhost (127.0.0.1) port 9090 (#0)
> GET /api/v1/cache/example HTTP/1.1
> Host: localhost:9090
> User-Agent: curl/7.47.0
> Accept: */*
>
< HTTP/1.1 200 OK
< Date: Fri, 17 Nov 2017 03:50:23 GMT
< Content-Length: 4
< Content-Type: text/plain; charset=utf-8
<
* Connection #0 to host localhost left intact
yay!
```

The server does log basic metrics:

```bash
$ ./server
2017/11/16 22:49:22 cache initialised.
2017/11/16 22:49:22 starting server on :9090
2017/11/16 22:50:07 stored "example" in cache.
2017/11/16 22:50:07 request took 277000ns.
2017/11/16 22:50:23 request took 9000ns.
```

### Acquiring

This is native Go with no external dependencies, so it will compile for all supported Golang platforms. To build:

```bash
go build server.go
```