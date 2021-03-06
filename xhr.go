package gosockjs

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"errors"
	"io"
	"net/http"
	"sync"
)

func xhrProlog(w http.ResponseWriter, req *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	h := req.Header.Get("Access-Control-Request-Headers")
	if h != "" {
		w.Header().Set("Access-Control-Allow-Headers", h)
	}
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
	opts     xhrOptions
	lock     sync.RWMutex
}

func (t *xhrTransport) writeFrame(w io.Writer, frame []byte) error {
	return t.opts.writeFrame(w, frame)
}

func (t *xhrTransport) sendFrame(frame []byte) error {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.receiver != nil {
		return t.receiver.opts.writeFrame(t.receiver, frame)
	}
	return errors.New("No receiver")
}

func (t *xhrTransport) closeTransport() {
	t.lock.RLock()
	defer t.lock.RUnlock()
	if t.receiver != nil {
		t.receiver.Close()
	}
}

func (t *xhrTransport) setReceiver(r *xhrReceiver) error {
	t.lock.Lock()
	if t.receiver != nil {
		defer t.lock.Unlock()
		// Nyet.
		t.writeFrame(r, closeFrame(2010, "Another connection still open"))
		return errors.New("Another connection still open")
	}
	t.receiver = r
	r.t = t
	t.lock.Unlock()
	t.s.newReceiver()
	return nil
}

func (t *xhrTransport) clearReceiver() {
	t.lock.Lock()
	t.receiver = nil
	defer t.lock.Unlock()
	t.s.disconnectReceiver()
}

type xhrReceiver struct {
	t        *xhrTransport
	w        io.WriteCloser
	opts     xhrOptions
	closed   chan bool // When closed the receiver closes this channel.
	nwritten int
	lock     sync.Mutex
}

func (r *xhrReceiver) Write(data []byte) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.closed == nil {
		return 0, errors.New("Closed")
	}
	if len(data) == 0 {
		return 0, nil
	}
	n, err := r.w.Write(data)
	r.nwritten += n
	if r.nwritten > r.opts.maxBytes() {
		r.internalClose()
	}
	return n, err
}

func (r *xhrReceiver) Close() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.internalClose()
}

func (r *xhrReceiver) internalClose() error {
	if r.closed != nil {
		close(r.closed)
		r.closed = nil
		return nil
	}
	return errors.New("Closed")
}

// differentiate between XhrPolling and XhrStreaming
type xhrOptions interface {
	maxBytes() int
	writeFrame(w io.Writer, frame []byte) error
	writePrelude(w io.Writer) error
	streaming() bool
	contentType() string
}

type xhrBaseOptions struct {
}

func (o xhrBaseOptions) writeFrame(w io.Writer, frame []byte) error {
	_, err := w.Write(append(frame, '\n'))
	return err
}

func (o xhrBaseOptions) contentType() string {
	return "application/javascript; charset=UTF-8"
}

type xhrPollingOptions struct {
	xhrBaseOptions
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
	xhrBaseOptions
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

func xhrJsessionid(r *Router, w http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("JSESSIONID")
	if err == nil && c != nil {
		if len(c.Path) == 0 {
			c.Path = "/"
		}
		http.SetCookie(w, c)
	} else if r.CookieNeeded {
		c := &http.Cookie{
			Name:  "JSESSIONID",
			Value: "dummy",
			Path:  "/",
		}
		http.SetCookie(w, c)
	}
}

// BUG(mrlauer): xhr connections cannot be reused.

// The handlers
func xhrHandlerBase(opts xhrOptions, r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
	xhrJsessionid(r, w, req)
	w.Header().Set("Content-type", opts.contentType())
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
		s, _ := r.getOrCreateSession(sessionId)
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
			trans.opts = opts
			s.trans = trans
			trans.s = s
			trans.writeFrame(w, openFrame())
			conn := &Conn{s}
			go r.handler(conn)
			if !opts.streaming() {
				w.Close()
				return
			}
		}
		s.sessionLock.Unlock()
		sessionUnlocked = true
		var leavingVoluntarily bool
		loopDone := make(chan bool)
		recvDone := make(chan bool)
		go func() {
			defer close(loopDone)
			defer w.Close()
			select {
			case <-recvDone:
				leavingVoluntarily = true
				return
			case <-done:
				return
			}
		}()
		err := trans.setReceiver(&xhrReceiver{t: trans, w: w, opts: opts, closed: recvDone})
		if err != nil {
			return
		}
		defer trans.clearReceiver()
		// The session may already have closed from underneath us!
		// If so, die now
		if s.closed {
			return
		}
		<-loopDone
		// If the session isn't closed, and we're not closing voluntarily, then
		// assume the client closed us and close the session.
		if !leavingVoluntarily && !s.closed {
			trans.clearReceiver()
			s.Close()
			r.removeSession(sessionId, s)
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
	s := r.getSession(sessionId)
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
	xhrHandlerBase(xhrPollingOptions{}, r, w, req)
}

func xhrStreamingHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	xhrHandlerBase(xhrStreamingOptions{}, r, w, req)
}
