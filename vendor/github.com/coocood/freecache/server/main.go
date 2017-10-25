//A basic freecache server supports redis protocol
package main

import (
	"bufio"
	"bytes"
	"errors"
	"github.com/coocood/freecache"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"runtime/debug"
	"strconv"
	"time"
)

var (
	protocolErr       = errors.New("protocol error")
	CRLF              = []byte("\r\n")
	PING              = []byte("ping")
	DBSIZE            = []byte("dbsize")
	ERROR_UNSUPPORTED = []byte("-ERR unsupported command\r\n")
	OK                = []byte("+OK\r\n")
	PONG              = []byte("+PONG\r\n")
	GET               = []byte("get")
	SET               = []byte("set")
	SETEX             = []byte("setex")
	DEL               = []byte("del")
	NIL               = []byte("$-1\r\n")
	CZERO             = []byte(":0\r\n")
	CONE              = []byte(":1\r\n")
	BulkSign          = []byte("$")
)

type Request struct {
	args [][]byte
	buf  *bytes.Buffer
}

func (req *Request) Reset() {
	req.args = req.args[:0]
	req.buf.Reset()
}

type operation struct {
	req       Request
	replyChan chan *bytes.Buffer
}

type Session struct {
	server    *Server
	conn      net.Conn
	addr      string
	reader    *bufio.Reader
	replyChan chan *bytes.Buffer
}

type Server struct {
	cache *freecache.Cache
}

func NewServer(cacheSize int) (server *Server) {
	server = new(Server)
	server.cache = freecache.NewCache(cacheSize)
	return
}

func (server *Server) Start(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println(err)
		return err
	}
	defer l.Close()
	log.Println("Listening on port", addr)
	for {
		tcpListener := l.(*net.TCPListener)
		tcpListener.SetDeadline(time.Now().Add(time.Second))
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}

		session := new(Session)
		session.conn = conn
		session.replyChan = make(chan *bytes.Buffer, 100)
		session.addr = conn.RemoteAddr().String()
		session.server = server
		session.reader = bufio.NewReader(conn)
		go session.readLoop()
		go session.writeLoop()
	}
}

func copyN(buffer *bytes.Buffer, r *bufio.Reader, n int64) (err error) {
	if n <= 512 {
		var buf [512]byte
		_, err = r.Read(buf[:n])
		if err != nil {
			return
		}
		buffer.Write(buf[:n])
	} else {
		_, err = io.CopyN(buffer, r, n)
	}
	return
}

func (server *Server) ReadClient(r *bufio.Reader, req *Request) (err error) {
	line, err := readLine(r)
	if err != nil {
		return
	}
	if len(line) == 0 || line[0] != '*' {
		err = protocolErr
		return
	}
	argc, err := btoi(line[1:])
	if err != nil {
		return
	}
	if argc <= 0 || argc > 4 {
		err = protocolErr
		return
	}
	var argStarts [4]int
	var argEnds [4]int
	req.buf.Write(line)
	req.buf.Write(CRLF)
	cursor := len(line) + 2
	for i := 0; i < argc; i++ {
		line, err = readLine(r)
		if err != nil {
			return
		}
		if len(line) == 0 || line[0] != '$' {
			err = protocolErr
			return
		}
		var argLen int
		argLen, err = btoi(line[1:])
		if err != nil {
			return
		}
		if argLen < 0 || argLen > 512*1024*1024 {
			err = protocolErr
			return
		}
		req.buf.Write(line)
		req.buf.Write(CRLF)
		cursor += len(line) + 2
		err = copyN(req.buf, r, int64(argLen)+2)
		if err != nil {
			return
		}
		argStarts[i] = cursor
		argEnds[i] = cursor + argLen
		cursor += argLen + 2
	}
	data := req.buf.Bytes()
	for i := 0; i < argc; i++ {
		req.args = append(req.args, data[argStarts[i]:argEnds[i]])
	}
	lower(req.args[0])
	return
}

func (down *Session) readLoop() {
	var req = new(Request)
	req.buf = new(bytes.Buffer)
	for {
		req.Reset()
		err := down.server.ReadClient(down.reader, req)
		if err != nil {
			close(down.replyChan)
			return
		}
		reply := new(bytes.Buffer)
		if len(req.args) == 4 && bytes.Equal(req.args[0], SETEX) {
			expire, err := btoi(req.args[2])
			if err != nil {
				reply.Write(ERROR_UNSUPPORTED)
			} else {
				down.server.cache.Set(req.args[1], req.args[3], expire)
				reply.Write(OK)
			}
		} else if len(req.args) == 3 && bytes.Equal(req.args[0], SET) {
			down.server.cache.Set(req.args[1], req.args[2], 0)
			reply.Write(OK)
		} else if len(req.args) == 2 {
			if bytes.Equal(req.args[0], GET) {
				value, err := down.server.cache.Get(req.args[1])
				if err != nil {
					reply.Write(NIL)
				} else {
					bukLen := strconv.Itoa(len(value))
					reply.Write(BulkSign)
					reply.WriteString(bukLen)
					reply.Write(CRLF)
					reply.Write(value)
					reply.Write(CRLF)
				}
			} else if bytes.Equal(req.args[0], DEL) {
				if down.server.cache.Del(req.args[1]) {
					reply.Write(CONE)
				} else {
					reply.Write(CZERO)
				}
			}
		} else if len(req.args) == 1 {
			if bytes.Equal(req.args[0], PING) {
				reply.Write(PONG)
			} else if bytes.Equal(req.args[0], DBSIZE) {
				entryCount := down.server.cache.EntryCount()
				reply.WriteString(":")
				reply.WriteString(strconv.Itoa(int(entryCount)))
				reply.Write(CRLF)
			} else {
				reply.Write(ERROR_UNSUPPORTED)
			}
		}
		down.replyChan <- reply
	}
}

func (down *Session) writeLoop() {
	var buffer = bytes.NewBuffer(nil)
	var replies = make([]*bytes.Buffer, 1)
	for {
		buffer.Reset()
		select {
		case reply, ok := <-down.replyChan:
			if !ok {
				down.conn.Close()
				return
			}
			replies = replies[:1]
			replies[0] = reply
			queueLen := len(down.replyChan)
			for i := 0; i < queueLen; i++ {
				reply = <-down.replyChan
				replies = append(replies, reply)
			}
			for _, reply := range replies {
				if reply == nil {
					buffer.Write(NIL)
					continue
				}
				buffer.Write(reply.Bytes())
			}
			_, err := down.conn.Write(buffer.Bytes())
			if err != nil {
				down.conn.Close()
				return
			}
		}
	}
}

func readLine(r *bufio.Reader) ([]byte, error) {
	p, err := r.ReadSlice('\n')
	if err != nil {
		return nil, err
	}
	i := len(p) - 2
	if i < 0 || p[i] != '\r' {
		return nil, protocolErr
	}
	return p[:i], nil
}

func btoi(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	i := 0
	sign := 1
	if data[0] == '-' {
		i++
		sign *= -1
	}
	if i >= len(data) {
		return 0, protocolErr
	}
	var l int
	for ; i < len(data); i++ {
		c := data[i]
		if c < '0' || c > '9' {
			return 0, protocolErr
		}
		l = l*10 + int(c-'0')
	}
	return sign * l, nil
}

func lower(data []byte) {
	for i := 0; i < len(data); i++ {
		if data[i] >= 'A' && data[i] <= 'Z' {
			data[i] += 'a' - 'A'
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU() - 1)
	server := NewServer(256 * 1024 * 1024)
	debug.SetGCPercent(10)
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	server.Start(":7788")
}
