package gosockjs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"sync"
	"time"
)

var JSONError error = errors.New("Broken JSON encoding.")
var EmptyPayload error = errors.New("Payload expected.")

type message string

func (m message) bytes() []byte {
	return ([]byte)(m)
}

func (m message) String() string {
	return string(m)
}

func (m message) MarshalJSON() ([]byte, error) {
	// Escape some characters
	js, err := json.Marshal(string(m))
	if err != nil {
		return js, err
	}
	re := regexp.MustCompile("[\x00-\x1f\u200c-\u200f\u2028-\u202f\u2060-\u206f\ufff0-\uffff]")
	sesc := re.ReplaceAllFunc(js, func(s []byte) []byte {
		return []byte(fmt.Sprintf(`\u%04x`, []rune(string(s))[0]))
	})

	return sesc, nil
}

// Session.
type session struct {
	// Reading, client -> server
	readQueue chan message
	unread    []byte

	// Heartbeat and disconnect timers
	timerLock sync.Mutex
	timer     *time.Timer

	// Writing
	outbox []message

	router      *Router
	sessionId   string
	trans       transport
	readLock    sync.Mutex
	writeLock   sync.Mutex
	sessionLock sync.Mutex

	closed bool
}

// session is an io.ReadWriteCloser
func (s *session) Read(data []byte) (int, error) {
	s.readLock.Lock()
	defer s.readLock.Unlock()
	// TODO: check error?
	n := len(data)
	// If there is anything unread, it's part of a partially read message. Return it.
	nu := len(s.unread)
	if nu > 0 {
		copy(data, s.unread)
		if nu > n {
			s.unread = s.unread[n:]
			return n, nil
		} else {
			s.unread = nil
			return nu, nil
		}
	}

	m, ok := <-s.readQueue
	if !ok {
		// We're closed
		return 0, io.EOF
	}
	mbytes := m.bytes()
	copy(data, mbytes)
	nm := len(mbytes)
	if nm > n {
		s.unread = mbytes[n:]
		return n, nil
	}
	return nm, nil
}

func (s *session) Write(data []byte) (int, error) {
	if s.closed {
		return 0, io.EOF
	}
	err := s.fromServer(message(data))
	if err != nil {
		// Assume nothing was written
		return 0, err
	}
	return len(data), nil
}

func (s *session) Close() error {
	s.sessionLock.Lock()
	defer s.sessionLock.Unlock()
	if !s.closed {
		s.closed = true
		// Tell any waiting receiver
		s.trans.sendFrame(closeFrame(3000, "Go away!"))
		s.trans.closeTransport()
		setTimer(s, nil)
	}
	return nil
}

func newSession(r *Router) *session {
	s := &session{router: r}
	s.readQueue = make(chan message, 1024)
	setDisconnect(s)
	return s
}

// Reading
func (s *session) fromClient(m message) error {
	// A message is either a json-encoded string or
	// an array of json-encoded strings.
	b := []byte(m)
	if len(b) == 0 {
		// Do nothing.
		return nil
	}
	var strings []string
	// Hacky, but easy
	if b[0] == '[' {
		// An array
		err := json.Unmarshal(b, &strings)
		if err != nil {
			return JSONError
		}
	} else {
		var str string
		err := json.Unmarshal(b, &str)
		if err != nil {
			return JSONError
		}
		strings = append(strings, str)
	}
	for _, str := range strings {
		select {
		case s.readQueue <- message(str):
		default:
			return errors.New("Message queue full")
		}
	}
	return nil
}

// Writing
func (s *session) fromServer(m message) error {
	// Add to the queue.
	s.writeLock.Lock()
	s.outbox = append(s.outbox, m)
	s.writeLock.Unlock()

	// Try to send the queue.
	s.tryToFlush()

	// This never returns an error...
	return nil
}

func (s *session) tryToFlush() error {
	s.writeLock.Lock()
	defer s.writeLock.Unlock()
	if len(s.outbox) == 0 {
		return nil
	}
	err := s.trans.sendFrame(messageFrame(s.outbox...))
	if err == nil {
		s.outbox = nil
	}
	return err
}

func setTimer(s *session, t *time.Timer) {
	s.timerLock.Lock()
	defer s.timerLock.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = t
}

func heartbeatFunc(s *session) {
	s.trans.sendFrame(heartbeatFrame())
	setHeartbeat(s)
}

func setHeartbeat(s *session) {
	setTimer(s, time.AfterFunc(s.router.HeartbeatDelay, func() { heartbeatFunc(s) }))
}

func setDisconnect(s *session) {
	setTimer(s, time.AfterFunc(s.router.DisconnectDelay, func() {
		s.router.RemoveSession(s.sessionId, s)
		s.Close()
	}))
}

// Events from the transport.
func (s *session) newReceiver() {
	if s.closed {
		s.trans.sendFrame(closeFrame(3000, "Go away!"))
		return
	}
	s.tryToFlush()
	// Set up a timeout
	setHeartbeat(s)
}

func (s *session) disconnectReceiver() {
	// Set up a timeout
	setDisconnect(s)
}

// Transport. Where a session gets messages from and sends them to.
type transport interface {
	writeFrame(w io.Writer, frame []byte) error
	sendFrame(frame []byte) error
	closeTransport()
}

// Frames.

func openFrame() []byte {
	return []byte("o")
}

func heartbeatFrame() []byte {
	return []byte("h")
}

func closeFrame(code int, msg string) []byte {
	s := []interface{}{code, msg}
	js, err := json.Marshal(s)
	if err != nil {
		log.Println("Error in closeFrame:", err)
	}
	return append([]byte("c"), js...)
}

func messageFrame(msgs ...message) []byte {
	w := bytes.NewBuffer(nil)
	w.WriteString("a")

	enc := json.NewEncoder(w)
	err := enc.Encode(msgs)
	if err != nil {
		log.Println("Error in messageFrame:", err)
	}
	bytes := w.Bytes()
	// JSON encoder adds a newline 
	if bytes[len(bytes)-1] == '\n' {
		bytes = bytes[:len(bytes)-1]
	}
	return bytes
}
