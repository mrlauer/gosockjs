package gosockjs

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Some utilities
func bodyString(r *http.Response) (string, error) {
	b, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	return string(b), err
}

func bodyJSONMap(r *http.Response) (map[string]interface{}, error) {
	dec := json.NewDecoder(r.Body)
	var result map[string]interface{}
	err := dec.Decode(&result)
	if err != nil {
		return make(map[string]interface{}), err
	}
	return result, nil
}

func bodyJSON(r *http.Response, result interface{}) error {
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&result)
	return err
}

type ServerWithRouter struct {
	*httptest.Server
	Router *Router
}

// Servers
func startTestServer(baseUrl string, h Handler) ServerWithRouter {
	r, err := NewRouter(baseUrl, h)
	if err != nil {
		panic(err)
	}
	server := httptest.NewServer(r)
	return ServerWithRouter{server, r}
}

func startEchoServer() (ServerWithRouter, string) {
	echo := func(c *Conn) {
		io.Copy(c, c)
	}
	server := startTestServer("/echo", echo)
	return server, server.URL + "/echo"
}

func startEchoTwiceServer() (ServerWithRouter, string) {
	twice := func(c *Conn) {
		buffer := make([]byte, 4096)
		for {
			n, err := c.Read(buffer)
			if n > 0 {
				c.Write(buffer[:n])
				c.Write(buffer[:n])
			}
			if err != nil {
				return
			}
		}
	}
	server := startTestServer("/twice", twice)
	return server, server.URL + "/twice"
}

func TestBaseUrl(t *testing.T) {
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

func TestInfo(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	infoUrl := baseUrl + "/info"
	r, err := http.Get(infoUrl)
	if err != nil {
		t.Errorf("Error %v getting %s", err, infoUrl)
	}
	if r.StatusCode != http.StatusOK {
		t.Errorf("%s had status %v", infoUrl, r.StatusCode)
	}
	b, err := bodyJSONMap(r)
	if err != nil {
		t.Errorf("error %v getting body JSON", err)
	}
	if !reflect.DeepEqual(b["origins"], []interface{}{"*:*"}) {
		t.Errorf("origins is %v (%T), not [\"*:*\"]", b["origins"], b["origins"])
	}
	entropyIntf := b["entropy"]
	if entropyIntf == nil {
		t.Errorf("no entropy")
	}
	if _, ok := entropyIntf.(float64); !ok {
		t.Errorf("entropy is a %T, not a number", entropyIntf)
	}
}

func TestEntropy(t *testing.T) {
	server, baseUrl := startEchoServer()
	defer server.Close()

	infoUrl := baseUrl + "/info"
	entropies := make(map[int64]bool)
	for i := 0; i < 5; i++ {
		r, _ := http.Get(infoUrl)
		var result struct{ Entropy int64 }
		err := bodyJSON(r, &result)
		if err != nil {
			t.Errorf("Could not get body json for entropy")
		}
		entropy := result.Entropy
		if entropy == 0 {
			t.Errorf("0 entropy")
		}
		_, ok := entropies[entropy]
		if ok {
			t.Errorf("Entropy %v was repeated", entropy)
		}
		entropies[entropy] = true
	}
}
