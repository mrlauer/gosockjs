package gosockjs

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// A set of utilities for sniffing the raw tcp underneath an http response.
type sniffingConn struct {
	net.Conn
	// Read the underlying stream from here.
	Sniffer   io.Reader
	buffer    *bytes.Buffer
	connError error
	lock      sync.Mutex
	gotData   *sync.Cond
}

func (s *sniffingConn) Read(data []byte) (int, error) {
	n, err := s.Conn.Read(data)
	s.lock.Lock()
	defer s.lock.Unlock()
	if n > 0 {
		s.buffer.Write(data[:n])
	}
	if err != nil {
		s.connError = err
	}
	s.gotData.Signal()
	return n, err
}

// Kind of a cute hack to provide an alternate Read
type sniffingReader sniffingConn

func (sr *sniffingReader) Read(data []byte) (int, error) {
	s := (*sniffingConn)(sr)
	s.lock.Lock()
	defer s.lock.Unlock()
	for s.buffer.Len() == 0 && s.connError == nil {
		s.gotData.Wait()
	}
	if s.buffer.Len() == 0 {
		return 0, s.connError
	}
	n, err := s.buffer.Read(data)
	return n, err
}

func (s *sniffingConn) Close() error {
	s.Conn.Close()
	s.lock.Lock()
	defer s.lock.Unlock()
	// Anything that's not already read, too bad.
	s.connError = io.EOF
	s.gotData.Signal()
	return nil
}

func newSniffingConn(c net.Conn) *sniffingConn {
	s := &sniffingConn{
		Conn:   c,
		buffer: bytes.NewBuffer(nil),
	}
	s.Sniffer = (*sniffingReader)(s)
	s.gotData = sync.NewCond(&s.lock)
	return s
}

type sniffingClient struct {
	http.Client
	Conn    *sniffingConn
	Sniffer io.Reader
}

func newSniffingClient() *sniffingClient {
	client := &sniffingClient{}
	dial := func(netw, addr string) (net.Conn, error) {
		c, err := net.Dial(netw, addr)
		if err != nil {
			return nil, err
		}
		sc := newSniffingConn(c)
		client.Conn = sc
		client.Sniffer = sc.Sniffer
		return sc, nil
	}
	transport := &http.Transport{Dial: dial, DisableKeepAlives: true, DisableCompression: true}
	client.Transport = transport
	return client
}

func TestSniffingClient(t *testing.T) {
	toWrite := []string{
		"This is a test\n",
		"This is another test\n",
	}
	// Simple server that does very little.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/plain;charset=UTF-8")
		for _, s := range toWrite {
			io.WriteString(w, s)
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()
	url := server.URL

	expectedLines := []string{
		"HTTP/1.1 200 OK",
		"Content-Type: text/plain;charset=UTF-8",
		"Date: " + time.Now().UTC().Format(http.TimeFormat),
		"Transfer-Encoding: chunked",
		"",
		"f",
		"This is a test\n",
		"15",
		"This is another test\n",
		"0",
	}

	client := newSniffingClient()
	resp, err := client.Get(url)
	if err != nil {
		t.Errorf("Error getting %v: %v", url, err)
	}
	go func() {
		ioutil.ReadAll(resp.Body)
		client.Conn.Close()
	}()

	buffer := bytes.NewBuffer(nil)
	io.Copy(buffer, client.Sniffer)
	expected := strings.Join(expectedLines, "\r\n") + "\r\n\r\n"
	if expected != string(buffer.Bytes()) {
		t.Errorf("Got \n%s\n,not\n%s", buffer.Bytes(), expected)
	}
}
