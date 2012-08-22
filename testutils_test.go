package gosockjs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
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
		resp.Body.Close()
	}()

	buffer := bytes.NewBuffer(nil)
	io.Copy(buffer, client.Sniffer)
	expected := strings.Join(expectedLines, "\r\n") + "\r\n\r\n"
	if expected != string(buffer.Bytes()) {
		t.Errorf("Got \n%s\n,not\n%s", buffer.Bytes(), expected)
	}
}

// A reader for chunked transfer encoded bodies
type chunkedReader struct {
	r          io.Reader
	readProlog bool
	bufr       *bufio.Reader
	unread     int
}

// A chunkedReader reads data in the chunked transfer-encoding format.
// It attempts to read one chunk at a time. If data is not big enough for 
// a chunk, the chunk will be split over successive reads. One read will
// never return parts of two chunks.
// This is NOT a bulletproof implemenatation. It just has to be good enough
// for tests.
func (cr *chunkedReader) Read(data []byte) (int, error) {
	BadData := errors.New("Bad chunked transfer-encoding")
	var a, b, c, d byte
	for !cr.readProlog {
		a, b, c = b, c, d
		var err error
		d, err = cr.bufr.ReadByte()
		if err != nil {
			return 0, err
		}
		if a == '\r' && b == '\n' && c == '\r' && d == '\n' {
			cr.readProlog = true
		}
	}
	if cr.unread != 0 {
		readTo := data
		if cr.unread < len(data) {
			readTo = data[:cr.unread]
		}
		n, err := cr.bufr.Read(readTo)
		if n < cr.unread {
			cr.unread -= n
		} else {
			cr.unread = 0
			if _, err := fmt.Fscanf(cr.bufr, "\r\n"); err != nil {
				return n, BadData
			}
		}
		return n, err
	}
	var sz int
	_, err := fmt.Fscanf(cr.bufr, "%x\r\n", &sz)
	if err != nil {
		return 0, BadData
	}
	if sz == 0 {
		return 0, io.EOF
	}
	readTo := data
	if sz <= len(data) {
		readTo = data[:sz]
	}
	n, err := cr.bufr.Read(readTo)
	if n < sz {
		cr.unread = sz - n
		return n, err
	} else {
		if _, err := fmt.Fscanf(cr.bufr, "\r\n"); err != nil {
			return n, BadData
		}
	}
	return n, err
}

func newChunkedReader(r io.Reader) *chunkedReader {
	cr := &chunkedReader{r: r, bufr: bufio.NewReader(r)}
	return cr
}

type TestData struct {
	dataSize int
	results  []string
}

func TestChunkedReader(t *testing.T) {
	data := "Foo:bar\r\n\r\n9\r\nSome data\r\ne\r\n0123456789abcd\r\n0\r\n\r\n"
	testData := []TestData{
		{
			1024,
			[]string{
				"Some data",
				"0123456789abcd",
			},
		},
		{
			5,
			[]string{
				"Some ",
				"data",
				"01234",
				"56789",
				"abcd",
			},
		},
	}
	for _, td := range testData {
		b := make([]byte, td.dataSize)
		cr := newChunkedReader(bytes.NewReader([]byte(data)))
		for _, s := range td.results {
			n, err := cr.Read(b)
			if err != nil || string(b[:n]) != s {
				t.Errorf("(%d) Read %s with error %v; should be %s\n", len(b), b[:n], err, s)
			}
		}
		n, err := cr.Read(b)
		if err != io.EOF {
			t.Errorf("Read %s with error %v\n", b[:n], err)
		}
	}
}

// timeoutReader will return a Timeout error if a read takes too long.
// After that happens, not much is really guaranteed.
var Timeout = errors.New("Timeout")

type timeoutReader struct {
	r       io.Reader
	timeout time.Duration
}

func newTimeoutReader(r io.Reader, timeout time.Duration) *timeoutReader {
	tr := &timeoutReader{r, timeout}
	return tr
}

func (tr *timeoutReader) Read(data []byte) (int, error) {
	dataChan := make(chan bool)
	var timer <-chan time.Time
	buffer := make([]byte, len(data))
	if tr.timeout > 0 {
		timer = time.After(tr.timeout)
	}
	var n int
	var err error
	go func() {
		n, err = tr.r.Read(buffer)
		dataChan <- true
	}()
	select {
	case <-dataChan:
		copy(data, buffer)
		return n, err
	case <-timer:
		return 0, Timeout
	}
	panic("unreachable")
}

func TestTimeoutReader(t *testing.T) {
	r, w := io.Pipe()
	delay1 := time.Microsecond * 500
	delay2 := time.Microsecond * 750
	delay3 := time.Millisecond
	tr := newTimeoutReader(r, delay2)
	delays := []time.Duration{0, delay1, delay3}
	strings := []string{"foo", "bar", "baz"}
	go func() {
		for i, d := range delays {
			time.Sleep(d)
			io.WriteString(w, strings[i])
		}
	}()
	data := make([]byte, 32)
	for i, d := range delays {
		n, err := tr.Read(data)
		sread := string(data[:n])
		if d > delay2 {
			if err != Timeout {
				t.Errorf("Read %d did not time out at %v", i, d)
			}
		} else {
			if err != nil || sread != strings[i] {
				t.Errorf("Read %d returned error %v and string %s", i, err, sread)
			}
		}
	}
}
