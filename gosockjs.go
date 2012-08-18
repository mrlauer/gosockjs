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
	"time"
)

// Conn is a SockJS connection. It is a ReadWriteCloser
type Conn struct {
}

func (c *Conn) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (c *Conn) Write(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (c *Conn) Close() error {
	return nil
}

// Handler is an interface to a SockJS connection.
type Handler func(*Conn)

// Router handles all the SockJS requests.
type Router struct {
	WebsocketEnabled bool
	r                *mux.Router
	handler          Handler
	baseUrl          string
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

func infoFunc(r *Router) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		r.infoMethod(w, req)
	}
}

func NewRouter(baseUrl string, h Handler) (*Router, error) {
	r := new(Router)

	// Properties
	r.WebsocketEnabled = true

	// Routing
	r.r = mux.NewRouter()
	sub := r.r.PathPrefix(baseUrl).Subrouter()
	//	  ss := r.r.PathPrefix(baseUrl + "/{server}/{session}").Subrouter()
	sub.HandleFunc("/info", infoFunc(r)).Methods("GET", "OPTIONS")
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
