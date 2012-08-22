package gosockjs

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Some utilities
func bodyString(r *http.Response) (string, error) {
	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	return string(b), err
}

// Servers
func startTestServer(baseUrl string, h Handler) *httptest.Server {
	r, err := NewRouter(baseUrl, h)
	if err != nil {
		panic(err)
	}
	server := httptest.NewServer(r)
	return server
}

func startEchoServer() (*httptest.Server, string) {
	echo := func(c *Conn) {
		io.Copy(c, c)
	}
	server := startTestServer("/echo", echo)
	return server, server.URL + "/echo"
}

func TestInfo(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	r, err := http.Get(baseUrl + "/")
	if err != nil {
		t.Errorf("Error %v getting %s/", err, baseUrl)
	}
	if r.StatusCode != http.StatusOK {
		t.Errorf("%s/ had status %v", baseUrl, r.StatusCode)
	}
	if h := r.Header.Get("content-type"); h != "text/plain; charset=UTF-8" {
		t.Errorf("%s/ content-type is %s", baseUrl, h)
	}
	if b, _ := bodyString(r); b != "Welcome to SockJS!\n" {
		t.Errorf("%s/ body is %s", baseUrl, b)
	}
}
