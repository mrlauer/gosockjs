package gosockjs

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"errors"
	"fmt"
	"io"
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

type xhrTransport struct {
	receiver *xhrReceiver
	s        *session
}

func (t *xhrTransport) sendFrame(frame []byte) error {
	if t.receiver != nil {
		_, err := t.receiver.Write(append(frame, '\n'))
		return err
	}
	return errors.New("No receiver")
}

func (t *xhrTransport) closeTransport() {
}

func (t *xhrTransport) setReceiver(r *xhrReceiver) error {
	if t.receiver != nil {
		// Nyet.
		fmt.Fprintf(r, "%s\n", closeFrame(2010, "Another connection still open"))
		return errors.New("Another connection still open")
	}
	t.receiver = r
	r.t = t
	t.s.newReceiver()
	return nil
}

func (t *xhrTransport) clearReceiver() {
	t.receiver = nil
	t.s.disconnectReceiver()
}

type xhrReceiver struct {
	t         *xhrTransport
	w         io.Writer
	byteCount chan int
}

func (r *xhrReceiver) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	n, err := r.w.Write(data)
	if r.byteCount != nil && n > 0 {
		r.byteCount <- n
	}
	return n, err
}

// The handlers
func xhrHandlerBase(maxBytes int, prelude []byte, r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
	w.Header().Set("Content-type", "application/javascript; charset=UTF-8")
	// For CORS, if the server sent Access-Control-Request-Headers, we
	// echo it back.
	acrh := req.Header.Get("Access-Control-Request-Headers")
	if acrh != "" {
		w.Header().Set("Access-Control-Allow-Headers", acrh)
	}
	sessionId := mux.Vars(req)["sessionid"]
	// Find the session
	s, _ := r.GetOrCreateSession(sessionId)
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()

	doHijack := func(trans *xhrTransport) {
		hijackAndContinue(w, func(w io.WriteCloser, done chan struct{}) {
			byteCount := make(chan int)
			go func() {
				nwritten := 0
				for {
					select {
					case nb := <-byteCount:
						nwritten += nb
						if nwritten >= maxBytes {
							w.Close()
							return
						}
					case <-done:
						w.Close()
						return
					}
				}
			}()
			err := trans.setReceiver(&xhrReceiver{trans, w, byteCount})
			if err != nil {
				w.Close()
				return
			}
			defer trans.clearReceiver()
			<-done
		})
	}
	if s.trans != nil {
		trans, ok := s.trans.(*xhrTransport)
		if !ok {
			w.Write(closeFrame(1001, "Another kind of connection is using this session"))
			return
		}
		w.WriteHeader(http.StatusOK)
		doHijack(trans)
	} else {
		trans := new(xhrTransport)
		s.trans = trans
		trans.s = s
		if len(prelude) > 0 {
			w.Write(prelude)
		}
		trans.setReceiver(&xhrReceiver{trans, w, nil})
		trans.sendFrame(openFrame())
		conn := &Conn{s}
		trans.clearReceiver()
		go r.handler(conn)
		if prelude != nil {
			doHijack(trans)
		}
	}
}

func xhrSendHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	sessionId := mux.Vars(req)["sessionid"]
	// Find the session
	s := r.GetSession(sessionId)
	if s == nil {
		http.NotFoundHandler().ServeHTTP(w, req)
		return
	}
	// Synchronization? What if an xhr request is still creating this?
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, req.Body)
	req.Body.Close()
	if buf.Len() == 0 {
		http.Error(w, EmptyPayload.Error(), http.StatusInternalServerError)
		return
	}
	err := s.fromClient(message(buf.Bytes()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-length", "0")
	w.WriteHeader(http.StatusNoContent)
}

func xhrHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	xhrHandlerBase(1, nil, r, w, req)
}

func xhrStreamingHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	prelude := make([]byte, 2049)
	for i, _ := range prelude {
		prelude[i] = 'h'
	}
	prelude[2048] = '\n'
	xhrHandlerBase(4096, prelude, r, w, req)
}
