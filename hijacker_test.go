package gosockjs

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChunkedWriter(t *testing.T) {
	toWrite := []string{
		"Ohai!",
		"This is\na test",
		"Foo\"bar\"",
	}
	expected := "5\r\nOhai!\r\ne\r\nThis is\na test\r\n8\r\nFoo\"bar\"\r\n0\r\n\r\n"

	buf := bytes.NewBuffer(nil)
	w := &chunkedWriter{buf}
	for _, s := range toWrite {
		io.WriteString(w, s)
	}
	w.end()
	if string(buf.Bytes()) != expected {
		t.Errorf("ChunkedWriter wrote\n%s, not\n%s", buf.Bytes(), expected)
	}
}

func simpleTestHijacker(conn io.WriteCloser, done chan struct{}) {
	io.WriteString(conn, "This is a test")
	conn.Close()
}

func simpleTestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Transfer-encoding", "chunked")
	w.Header().Set("Content-type", "text/plain")
	w.WriteHeader(200)
	hijackAndContinue(w, simpleTestHijacker)
}

func TestHijackerSimple(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(simpleTestHandler))
	defer server.Close()
	url := server.URL

	resp, err := http.Get(url)
	if err != nil {
		t.Errorf("Error in Get %v: %v", url, err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("Status was %d\n", resp.StatusCode)
	}
	result, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("ReadAll returned error %v", err)
	}
	if string(result) != "This is a test" {
		t.Errorf("Body was %s", result)
	}

}
