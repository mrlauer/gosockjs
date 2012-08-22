package gosockjs

import (
	"net/http"
	"strings"
	"testing"
)

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
}
