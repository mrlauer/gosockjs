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

// Session.
type Session struct {
}

// Router handles all the SockJS requests.
type Router struct {
	WebsocketEnabled bool
	r                *mux.Router
	handler          Handler
	baseUrl          string

	// Sessions
	sessions    map[string]*Session
	sessionLock sync.RWMutex
}

func (r *Router) GetSession(sessionId string) *Session {
	r.sessionLock.RLock()
	defer r.sessionLock.RUnlock()
	return r.sessions[sessionId]
}

func (r *Router) GetOrAddSession(sessionId string) *Session {
	s := r.GetSession(sessionId)
	if s == nil {
		r.sessionLock.Lock()
		defer r.sessionLock.Unlock()
		s = new(Session)
	}
	return s
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.r.ServeHTTP(w, req)
}

func (r *Router) infoMethod(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("content-type", "application/json; charset=UTF-8")

	// no caching
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")

	// cors
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Response status
	if req.Method == "OPTIONS" {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("Access-Control-Max-Age", "31536000")
		w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET")
		exp := time.Now().Add(time.Hour * 24 * 365).UTC().Format(http.TimeFormat)
		w.Header().Set("Expires", exp)

		origin := req.Header.Get("origin")
		if origin == "" || origin == "null" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)

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

func NewRouter(baseUrl string, h Handler) (*Router, error) {
	r := new(Router)

	// Properties
	r.WebsocketEnabled = true
	r.handler = h

	// Routing
	r.r = mux.NewRouter()
	sub := r.r.PathPrefix(baseUrl).Subrouter()
	//	  ss := r.r.PathPrefix(baseUrl + "/{server}/{session}").Subrouter()
	sub.HandleFunc("/info", infoFunc(r)).Methods("GET", "OPTIONS")

	// Websockets. We don't worry about sessions.
	sub.HandleFunc("/websocket", r.WrapHandler(rawWebsocketHandler)).Methods("GET")
	ss := sub.PathPrefix("/{serverid}/{sessionid}").Subrouter()
	ss.HandleFunc("/websocket", r.WrapHandler(websocketHandler)).Methods("GET")

	return r, nil
}

func Install(baseUrl string, h Handler) (*Router, error) {
	r, err := NewRouter(baseUrl, h)
	if err != nil {
		return nil, err
	}
	http.Handle(baseUrl+"/", r)
	return r, nil
}
