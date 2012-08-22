package gosockjs

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
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

func TestXhrPollingSimple(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	// Hack up server/session
	turl := baseUrl + "/123/456"

	http.DefaultTransport = &http.Transport{DisableKeepAlives: true}
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
		t.Errorf(`Body was %q, not a%s\n`, payload)
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
