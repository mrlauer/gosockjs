package gosockjs

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Things clients do
type Poster interface {
	Post(url string, bodyType string, body io.Reader) (r *http.Response, err error)
}

func sendXhr(c Poster, url string, data ...interface{}) (*http.Response, error) {
	r, w := io.Pipe()
	enc := json.NewEncoder(w)
	go func() {
		enc.Encode(data)
		w.Close()
	}()
	return c.Post(url+"/xhr_send", "application/json", r)
}

func xhrMessage(s string) string {
	jsbytes, err := message(s).MarshalJSON()
	if err != nil {
		return ""
	}
	return "a[" + string(jsbytes) + "]\n"
}

func TestXhrPollingSimple(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	// Hack up server/session
	turl := baseUrl + "/123/456"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(turl+"/xhr", "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	b, _ := bodyString(r)
	if b != "o\n" {
		t.Errorf(`initial response body was %q, not "o\n"`, b)
	}

	// Send something
	payload := `["abc"]`
	r, err = c.Post(turl+"/xhr_send", "text/plain", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("Could not post: %v", err)
	}
	if r.StatusCode != 204 {
		t.Errorf("xhr_send returned code %d", r.StatusCode)
	}
	// Make sure it got there
	r, err = c.Post(turl+"/xhr", "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}

	b, _ = bodyString(r)
	if b != "a"+payload+"\n" {
		t.Errorf(`Body was %q, not a%s\n`, b, payload)
	}

	r1, err := c.Post(turl+"/xhr", "", nil)
	// r1 should still be open
	r2, err := c.Post(turl+"/xhr", "", nil)
	// r2 should return a Go Away response.
	s, err := bodyString(r2)
	if err != nil || s != "c[2010,\"Another connection still open\"]\n" {
		t.Errorf("Request that should have returned Closed said %s with error %v", s, err)
	}
	// Now send another message
	msgs := []string{"“Þiß is å mess\u03b1ge‟"}
	sendXhr(c, turl, msgs[0])

	s, err = bodyString(r1)
	expected := "a[\"" + strings.Join(msgs, ",") + "\"]\n"
	if err != nil || s != expected {
		t.Errorf("Received %s (error %v), expected %s", s, err, expected)
	}
}

func TestXhrPollingResponseLimit(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	// Hack up server/session
	turl := baseUrl + "/123/456"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(turl+"/xhr", "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	b, _ := bodyString(r)
	if b != "o\n" {
		t.Errorf(`initial response body was %q, not "o\n"`, b)
	}

	// Open a reading connection
	r1, err := c.Post(turl+"/xhr", "", nil)
	body := r1.Body

	sendXhr(c, turl, "abc")
	s, err := readString(body)
	if err != nil || s != "a[\"abc\"]\n" {
		t.Errorf("First read: %s (error %v)", s, err)
	}
	sendXhr(c, turl, "abc")
	s, err = readString(body)
	if err != io.EOF || s != "" {
		t.Errorf("Second read: %s (error %v)", s, err)
	}
	body.Close()

	sendXhr(c, turl, "def")
	r1, err = c.Post(turl+"/xhr", "", nil)
	body = r1.Body

	sendXhr(c, turl, "ghi")
	s, err = readString(body)
	if err != nil || s != "a[\"abc\",\"def\"]\n" {
		t.Errorf("With data waiting: First read: %s (error %v)", s, err)
	}
	// Read ghi
	r1, err = c.Post(turl+"/xhr", "", nil)
	if s, err := readString(r1.Body); err != nil || s != "a[\"ghi\"]\n" {
		t.Errorf("With data waiting: Second read: %s (error %v)", s, err)
	}
	r1, err = c.Post(turl+"/xhr", "", nil)
	sendXhr(c, turl, "klm")
	sendXhr(c, turl, "nop")
	if s, err := readString(r1.Body); err != nil || s != "a[\"klm\"]\n" {
		t.Errorf("With data waiting: Third read: %s (error %v)", s, err)
	}

}

func TestXhrPollingMultiMessage(t *testing.T) {
	server, baseUrl := startEchoTwiceServer()
	defer server.Close()

	// Hack up server/session
	turl := baseUrl + "/123/456"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(turl+"/xhr", "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	b, _ := bodyString(r)
	if b != "o\n" {
		t.Errorf(`initial response body was %q, not "o\n"`, b)
	}

	// Open a reading connection
	r1, err := c.Post(turl+"/xhr", "", nil)
	body := r1.Body

	sendXhr(c, turl, "abc")
	s, err := readString(body)
	if err != nil || s != "a[\"abc\"]\n" {
		// If this fails, the next read will probably hang, so use Fatalf
		t.Fatalf("First read: %s (error %v)", s, err)
	}
	s, err = readString(body)
	if err != io.EOF {
		t.Fatalf("Read too much from xhr poll: %s with error %v", s, err)
	}
	r1, err = c.Post(turl+"/xhr", "", nil)
	body = r1.Body

	s, err = readString(body)
	if err != nil || s != "a[\"abc\"]\n" {
		t.Errorf("Second read: %s (error %v)", s, err)
	}
}

func readString(r io.Reader) (string, error) {
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if n > 0 {
		return string(buf[:n]), err
	}
	return "", err
}

func TestXhrStreamingSimple(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	// Hack up server/session
	turl := baseUrl + "/123/456"
	surl := turl + "/xhr_streaming"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(surl, "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	body := r.Body
	// Read prelude
	prelude := make([]byte, 2049)
	body.Read(prelude)
	s, err := readString(body)
	if err != nil || s != "o\n" {
		t.Errorf("Initial response was %s with %v", s, err)
	}

	// Send something
	data := []string{"abc", "def"}
	for _, str := range data {
		sendXhr(c, turl, str)
		s, err := readString(body)
		expected := "a[\"" + str + "\"]\n"
		if err != nil || s != expected {
			t.Errorf("xhr_streaming got %s with error %v, expected %s", s, err, expected)
		}
	}
}

func TestXhrHeartbeat(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()
	server.Router.HeartbeatDelay = time.Millisecond * 2

	// Hack up server/session
	turl := baseUrl + "/123/456"
	surl := turl + "/xhr_streaming"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(surl, "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	body := r.Body
	// Read prelude
	prelude := make([]byte, 2049)
	body.Read(prelude)
	s, err := readString(body)
	if err != nil || s != "o\n" {
		t.Errorf("Initial response was %s with %v", s, err)
	}

	nheartbeats := 0
	go func() {
		buffer := make([]byte, 128)
		for {
			n, err := r.Body.Read(buffer)
			if err != nil {
				return
			}
			s := string(buffer[:n])
			if s != "h\n" {
				t.Errorf("Got %q, expecting heartbeat", s)
			}
			nheartbeats++
		}
	}()
	time.Sleep(time.Millisecond * 7)
	c.Conn.Close()
	if nheartbeats != 3 {
		t.Errorf("Got %d heartbeats, expecting 3", nheartbeats)
	}
}

func TestXhrTimeout(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()
	server.Router.DisconnectDelay = time.Millisecond * 5
	start := time.Now()

	// Hack up server/session
	turl := baseUrl + "/123/456"
	surl := turl + "/xhr"

	c := newSniffingClient()
	var r *http.Response
	var err error
	// Open a connection
	r, err = c.Post(surl, "", nil)
	if r.StatusCode != 200 {
		t.Errorf("initial response was %v", r.StatusCode)
	}
	body := r.Body
	// Read prelude
	s, err := readString(body)
	if err != nil || s != "o\n" {
		t.Errorf("Initial response was %s with %v", s, err)
	}

	for i := 0; i < 4; i++ {
		time.Sleep(time.Microsecond * 2000)
		r, err = c.Post(surl, "", nil)
		if err != nil {
			t.Errorf("xhr error %v", err)
			continue
		}
		if r.StatusCode != 200 {
			t.Errorf("xhr message %d has status %d", i, r.StatusCode)
		}
		sendXhr(c, turl, "abc")
		s, err = bodyString(r)
		tm := time.Now()
		if err != nil || s != xhrMessage("abc") {
			t.Errorf("xhr message %d was %s with error %v after %v", i, s, err, tm.Sub(start))
		}
		start = tm
	}

	// Now sleep long enough to time out
	time.Sleep(time.Microsecond * 5100)
	r, err = c.Post(surl, "", nil)
	sendXhr(c, turl, "abc")
	s, err = readString(r.Body)
	if err != nil || s != "o\n" {
		t.Errorf("xhr message was %s with error %v; should have been o, after timeout", s, err)
	}

}
