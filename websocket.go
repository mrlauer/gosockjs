package gosockjs

import (
	"code.google.com/p/go.net/websocket"
	"errors"
	"fmt"
	"io"
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

// (Non-raw) websockets; with framing
type wsTransport struct {
	lock sync.RWMutex
	ws   *websocket.Conn
}

func (t *wsTransport) conn() *websocket.Conn {
	t.lock.RLock()
	defer t.lock.RUnlock()
	return t.ws
}

func (t *wsTransport) setConn(conn *websocket.Conn) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.ws = conn
}

func (t *wsTransport) writeFrame(w io.Writer, frame []byte) error {
	_, err := w.Write(frame)
	return err
}

func (t *wsTransport) sendFrame(frame []byte) error {
	ws := t.conn()
	if ws != nil {
		return t.writeFrame(ws, frame)
	}
	return errors.New("No connection")
}

func (t *wsTransport) closeTransport() {
	ws := t.conn()
	if ws != nil {
		ws.Close()
		t.setConn(nil)
	}
}

func (r *Router) makeWSHandler() websocket.Handler {
	h := func(c *websocket.Conn) {
		s := newSession(r)
		trans := &wsTransport{ws: c}
		s.trans = trans
		s.newReceiver()
		s.trans.sendFrame(openFrame())
		// Read from the websocket in a goroutine.
		go func() {
			for {
				var m string
				err := websocket.Message.Receive(c, &m)
				if err == nil {
					err = s.fromClient(message(m))
				}
				if err != nil {
					trans.closeTransport()
					s.Close()
					return
				}
			}
		}()
		// And run the handler
		conn := &Conn{s}
		r.handler(conn)
		/*
			rcimpl := &websocketConn{ws: c}
			conn := &Conn{rcimpl}
			c.Write([]byte("o"))
			r.handler(conn)
		*/
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
	conn := strings.ToLower(req.Header.Get("Connection"))
	// Silly firefox...
	if conn == "keep-alive, upgrade" {
		req.Header.Set("Connection", "Upgrade")
	} else if conn != "upgrade" {
		http.Error(w, `"Connection" must be "Upgrade".`, http.StatusBadRequest)
		return
	}
	h := r.makeWSHandler()
	h.ServeHTTP(w, req)
}
