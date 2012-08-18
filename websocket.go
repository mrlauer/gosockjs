package gosockjs

import (
	"code.google.com/p/go.net/websocket"
	"net/http"
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

func websocketHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if !r.WebsocketEnabled {
		errStatus(w, http.StatusNotFound)
		return
	}
	// Create a session
	// Create a websocket
	// Handle the websocket
}
