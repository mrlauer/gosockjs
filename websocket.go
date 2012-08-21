package gosockjs

import (
	"code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

func errStatus(w http.ResponseWriter, s int) {
	http.Error(w, http.StatusText(s), s)
}

// Raw websockets -- no framing
type rawWebsocketConn struct {
	ws *websocket.Conn
}

func (c *rawWebsocketConn) Read(data []byte) (int, error) {
	return c.ws.Read(data)
}

func (c *rawWebsocketConn) Write(data []byte) (int, error) {
	return c.ws.Write(data)
}

func (c *rawWebsocketConn) Close() error {
	return c.ws.Close()
}

func (r *Router) makeRawWSHandler() websocket.Handler {
	h := func(c *websocket.Conn) {
		rcimpl := &rawWebsocketConn{ws: c}
		conn := &Conn{rcimpl}
		r.handler(conn)
	}
	return websocket.Handler(h)
}

func rawWebsocketHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if !r.WebsocketEnabled {
		errStatus(w, http.StatusNotFound)
		return
	}
	h := r.makeRawWSHandler()
	h.ServeHTTP(w, req)
}

type websocketConn struct {
	ws        *websocket.Conn
	buffer    []string
	unread    string
	readLock  sync.Mutex
	writeLock sync.Mutex
}

func (c *websocketConn) readBufferedData(data []byte) (int, error) {
	n := len(data)
	var tocopy string
	if len(c.unread) > 0 {
		tocopy = c.unread
	} else {
		for i, s := range c.buffer {
			if len(s) > 0 {
				tocopy = s
				c.buffer = c.buffer[i+1:]
				break
			}
		}
	}
	ntocopy := len(tocopy)
	ncopied := 0
	if ntocopy > 0 {
		copy(data, tocopy)
		if ntocopy > n {
			c.unread = tocopy[n:]
		} else {
			ncopied = ntocopy
			c.unread = ""
		}
	}
	return ncopied, nil
}

func (c *websocketConn) Read(data []byte) (int, error) {
	c.readLock.Lock()
	defer c.readLock.Unlock()
	n, err := c.readBufferedData(data)
	if n > 0 {
		return n, err
	}
	// We should get ONLY lists of strings in json form.
	for {
		var frames interface{}
		var message string
		err := websocket.Message.Receive(c.ws, &message)
		if len(message) == 0 {
			continue
		}
		err = json.Unmarshal([]byte(message), &frames)
		if err != nil {
			// Close immediately
			c.ws.Close()
			return 0, err
		}
		switch f := frames.(type) {
		case string:
			c.buffer = []string{f}
		case []interface{}:
			c.buffer = nil
			for _, s := range f {
				str, ok := s.(string)
				if !ok {
					c.ws.Close()
					return 0, errors.New("Invalid message")
				}
				c.buffer = append(c.buffer, str)
			}
		default:
			continue
		}
		n, err := c.readBufferedData(data)
		if n > 0 {
			return n, err
		}
	}
	panic("unreachable")
}

func (c *websocketConn) Write(data []byte) (int, error) {
	// TODO: abstract the frame away
	toEncode := []string{string(data)}
	json, err := json.Marshal(toEncode)
	if err != nil {
		return 0, err
	}
	toWrite := append([]byte("a"), json...)
	c.writeLock.Lock()
	defer c.writeLock.Unlock()
	return c.ws.Write(toWrite)
}

func (c *websocketConn) Close() error {
	c.ws.Write([]byte(`c[3000,"Go away!"]`))
	return c.ws.Close()
}

func (r *Router) makeWSHandler() websocket.Handler {
	h := func(c *websocket.Conn) {
		rcimpl := &websocketConn{ws: c}
		conn := &Conn{rcimpl}
		c.Write([]byte("o"))
		r.handler(conn)
	}
	return websocket.Handler(h)
}

func websocketHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if !r.WebsocketEnabled {
		errStatus(w, http.StatusNotFound)
		return
	}
	// Some checks
	if req.Method != "GET" {
		// This is gross. To avoid putting extra headers in, we'll hijack the connection!
		rwc, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic("Hijack failed: " + err.Error())
		}
		defer rwc.Close()
		code := http.StatusMethodNotAllowed
		fmt.Fprintf(buf, "HTTP/1.1 %d %s\r\n", code, http.StatusText(code))
		fmt.Fprint(buf, "Content-Length: 0\r\n")
		fmt.Fprint(buf, "Allow: GET\r\n")
		fmt.Fprint(buf, "\r\n")
		buf.Flush()
		return
	}
	// I think there is a bug in SockJS. Hybi v13 wants "Origin", not "Sec-WebSocket-Origin"
	if req.Header.Get("Sec-WebSocket-Version") == "13" && req.Header.Get("Origin") == "" {
		req.Header.Set("Origin", req.Header.Get("Sec-WebSocket-Origin"))
	}
	if strings.ToLower(req.Header.Get("Upgrade")) != "websocket" {
		http.Error(w, `Can "Upgrade" only to "WebSocket".`, http.StatusBadRequest)
		return
	}
	if strings.ToLower(req.Header.Get("Connection")) != "upgrade" {
		http.Error(w, `"Connection" must be "Upgrade".`, http.StatusBadRequest)
		return
	}
	h := r.makeWSHandler()
	h.ServeHTTP(w, req)
}
