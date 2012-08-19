package gosockjs

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
)

type message []byte

func (m message) bytes() []byte {
	return ([]byte)(m)
}

func (m message) String() string {
	return string(m)
}

func (m message) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(m))
}

// Session.
type session struct {
	// Reading, client -> server
	readQueue chan message
	unread    []byte

	trans     transport
	readLock  sync.Mutex
	writeLock sync.Mutex
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
	return 0, nil
}

func (s *session) Close() error {
	return nil
}

func newSession() *session {
	s := &session{}
	s.readQueue = make(chan message, 1024)
	return s
}

// Reading
func (s *session) fromTransport(m message) error {
	select {
	case s.readQueue <- m:
	default:
		return errors.New("Message queue full")
	}
	return nil
}

// Writing
func (s *session) fromServer(m message) error {
	// Add to the queue.
	// Try to send the queue.
	return nil
}

func (s *session) tryToFlush() {
}

// Events from the transport.
func (s *session) newReceiver() {
	s.trans.sendFrame(openFrame())
}

func (s *session) disconnectReceiver() {
}

// Transport. Where a session gets messages from and sends them to.
type transport interface {
	sendFrame(frame []byte) error
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
