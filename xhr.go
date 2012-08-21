package gosockjs

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"errors"
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

func (t *xhrTransport) writeFrame(w io.Writer, frame []byte) error {
	_, err := w.Write(append(frame, '\n'))
	return err
}

func (t *xhrTransport) sendFrame(frame []byte) error {
	if t.receiver != nil {
		return t.writeFrame(t.receiver, frame)
	}
	return errors.New("No receiver")
}

func (t *xhrTransport) closeTransport() {
	if t.receiver != nil {
		t.receiver.Close()
	}
}

func (t *xhrTransport) setReceiver(r *xhrReceiver) error {
	if t.receiver != nil {
		// Nyet.
		t.writeFrame(r, closeFrame(2010, "Another connection still open"))
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
	w         io.WriteCloser
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

func (r *xhrReceiver) Close() error {
	return r.w.Close()
}

// differentiate between XhrPolling and XhrStreaming
type xhrOptions interface {
	maxBytes() int
	writePrelude(w io.Writer) error
	streaming() bool
}

type xhrPollingOptions struct {
	r *Router
}

func (o xhrPollingOptions) maxBytes() int {
	return 1
}

func (o xhrPollingOptions) writePrelude(io.Writer) error {
	return nil
}

func (o xhrPollingOptions) streaming() bool {
	return false
}

type xhrStreamingOptions struct {
	r *Router
}

func (o xhrStreamingOptions) maxBytes() int {
	return 4096 // r.maxBytes
}

func (o xhrStreamingOptions) writePrelude(w io.Writer) error {
	prelude := make([]byte, 2049)
	for i, _ := range prelude {
		prelude[i] = 'h'
	}
	prelude[2048] = '\n'
	_, err := w.Write(prelude)
	return err
}

func (o xhrStreamingOptions) streaming() bool {
	return true
}

// The handlers
func xhrHandlerBase(opts xhrOptions, r *Router, w http.ResponseWriter, req *http.Request) {
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

	w.WriteHeader(http.StatusOK)
	opts.writePrelude(w)
	hijackAndContinue(w, func(w io.WriteCloser, done chan struct{}) {
		defer w.Close()
		var trans *xhrTransport
		// Find the session
		s, _ := r.GetOrCreateSession(sessionId)
		s.sessionLock.Lock()
		// TODO: encapsulate this logic
		var sessionUnlocked bool
		defer func() {
			if !sessionUnlocked {
				s.sessionLock.Unlock()
			}
		}()
		if s.trans != nil {
			if s.closed {
				s.trans.writeFrame(w, closeFrame(3000, "Go away!"))
				return
			}
			var ok bool
			trans, ok = s.trans.(*xhrTransport)
			if !ok {
				s.trans.writeFrame(w, closeFrame(1001, "Another kind of connection is using this session"))
				return
			}
		} else {
			trans = new(xhrTransport)
			s.trans = trans
			trans.s = s
			trans.writeFrame(w, openFrame())
			conn := &Conn{s}
			go r.handler(conn)
			if !opts.streaming() {
				return
			}
		}
		s.sessionLock.Unlock()
		sessionUnlocked = true
		byteCount := make(chan int)
		var leavingVoluntarily bool
		go func() {
			nwritten := 0
			for {
				select {
				case nb := <-byteCount:
					nwritten += nb
					if nwritten >= opts.maxBytes() {
						leavingVoluntarily = true
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
			return
		}
		defer trans.clearReceiver()
		// The session may already have closed from underneath us!
		// If so, die now
		if s.closed {
			return
		}
		<-done
		// If the session isn't closed, and we're not closing voluntarily, then
		// assume the client closed us and close the session.
		if !leavingVoluntarily && !s.closed {
			trans.clearReceiver()
			s.Close()
			r.RemoveSession(sessionId, s)
		}
	})
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
	xhrHandlerBase(xhrPollingOptions{r}, r, w, req)
}

func xhrStreamingHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	xhrHandlerBase(xhrStreamingOptions{r}, r, w, req)
}
