package gosockjs

import (
	"code.google.com/p/gorilla/mux"
	"net/http"
)

type xhrConn struct {
	closed bool
	inbox  []string
	outbox []string
}

func (c *xhrConn) Read(data []byte) (int, error) {
	return 0, nil
}

func (c *xhrConn) Write(data []byte) (int, error) {
	return 0, nil
}

func (c *xhrConn) Close(data []byte) error {
	return nil
}

func xhrProlog(w http.ResponseWriter, req *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	if req.Method == "OPTIONS" {
		writeCacheAndExpires(w, req)
		writeOptionsAccess(w, req, "POST")
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	writeCorsHeaders(w, req)
	writeNoCache(w, req)
	return false
}

func xhrHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
	sessionId := mux.Vars(req)["sessionid"]
	// Find the session
	s := r.GetSession(sessionId)
	if s != nil {
		// If there is a pending receiver, return now.
		// If there are any pending messages, send them now.
		// Otherwise, wait until there is one.
		// How long until we give up?
	} else {
		// Create a session
		s = new(session)
		// send an open frame
		w.Header().Set("Content-type", "application/javascript; charset=UTF-8")
		w.Write([]byte("o\n"))
	}
}

func xhrSendHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
}

func xhrStreamHandler(r *Router, w http.ResponseWriter, req *http.Request) {
}
