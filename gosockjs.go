/*
Package gosockjs is an implementation of a SockJS server.

See https://github.com/sockjs .
*/
package gosockjs

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// connImpl is the interface for implementations of Conn
type connImpl interface {
	io.ReadWriteCloser
}

// Conn is a SockJS connection. It is a ReadWriteCloser
type Conn struct {
	connImpl
}

// Handler is an interface to a SockJS connection.
type Handler func(*Conn)

// Router handles all the SockJS requests.
type Router struct {
	WebsocketEnabled bool
	r                *mux.Router
	handler          Handler
	baseUrl          string

	// Sessions
	sessions    map[string]*session
	sessionLock sync.RWMutex
}

func (r *Router) GetSession(sessionId string) *session {
	r.sessionLock.RLock()
	defer r.sessionLock.RUnlock()
	return r.sessions[sessionId]
}

func (r *Router) GetOrAddSession(sessionId string) *session {
	s := r.GetSession(sessionId)
	if s == nil {
		r.sessionLock.Lock()
		defer r.sessionLock.Unlock()
		s = new(session)
	}
	return s
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.r.ServeHTTP(w, req)
}

// Utility methods
func writeNoCache(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
}

func writeCacheAndExpires(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	exp := time.Now().Add(time.Hour * 24 * 365).UTC().Format(http.TimeFormat)
	w.Header().Set("Expires", exp)
}

func writeOptionsAccess(w http.ResponseWriter, req *http.Request, methods ...string) {
	w.Header().Set("Access-Control-Max-Age", "31536000")
	m := "OPTIONS"
	for _, method := range methods {
		m = m + ", " + method
	}
	w.Header().Set("Access-Control-Allow-Methods", m)
	origin := req.Header.Get("origin")
	if origin == "" || origin == "null" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)

}

func writeCorsHeaders(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func (r *Router) infoMethod(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("content-type", "application/json; charset=UTF-8")

	// no caching
	writeNoCache(w, req)

	// cors
	writeCorsHeaders(w, req)

	// Response status
	if req.Method == "OPTIONS" {
		writeCacheAndExpires(w, req)
		writeOptionsAccess(w, req, "GET")

		w.WriteHeader(204)
		return
	}

	data := make(map[string]interface{})
	data["websocket"] = r.WebsocketEnabled
	data["cookie_needed"] = false
	data["origins"] = []string{"*:*"}
	entropy := make([]byte, 4)
	rand.Read(entropy)
	var uent uint32
	binary.Read(bytes.NewReader(entropy), binary.LittleEndian, &uent)
	data["entropy"] = uent
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Print(err)
	}
}

func (r *Router) WrapHandler(f func(r *Router, w http.ResponseWriter, req *http.Request)) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		f(r, w, req)
	}
}

func infoFunc(r *Router) func(w http.ResponseWriter, req *http.Request) {
	return r.WrapHandler((*Router).infoMethod)
}

func greetingHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	body := "Welcome to SockJS!\n"
	w.Write([]byte(body))
}

func notFoundHandler(w http.ResponseWriter, req *http.Request) {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func NewRouter(baseUrl string, h Handler) (*Router, error) {
	r := new(Router)

	// Properties
	r.WebsocketEnabled = true
	r.handler = h

	// Routing
	r.r = mux.NewRouter()
	r.r.StrictSlash(true)
	sub := r.r.PathPrefix(baseUrl).Subrouter()
	sub.StrictSlash(true)
	ss := sub.PathPrefix("/{serverid}/{sessionid}").Subrouter()

	// Greeting, info
	r.r.HandleFunc(baseUrl+"/", r.WrapHandler(greetingHandler)).Methods("GET")
	sub.HandleFunc("/info", infoFunc(r)).Methods("GET", "OPTIONS")

	// Websockets. We don't worry about sessions.
	sub.HandleFunc("/websocket", r.WrapHandler(rawWebsocketHandler)).Methods("GET")
	ss.HandleFunc("/websocket", r.WrapHandler(websocketHandler))

	// XHR
	ss.HandleFunc("/xhr", r.WrapHandler(xhrHandler)).Methods("POST", "OPTIONS")
	ss.HandleFunc("/xhr_send", r.WrapHandler(xhrSendHandler)).Methods("POST", "OPTIONS")

	return r, nil
}

func Install(baseUrl string, h Handler) (*Router, error) {
	r, err := NewRouter(baseUrl, h)
	if err != nil {
		return nil, err
	}
	http.Handle(baseUrl+"/", r)
	http.HandleFunc(baseUrl, r.WrapHandler(greetingHandler))
	return r, nil
}
