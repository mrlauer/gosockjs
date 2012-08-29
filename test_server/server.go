package main

import (
	"fmt"
	"github.com/mrlauer/gosockjs"
	"io"
	"log"
	"net/http"
	"path"
)

func echo(c *gosockjs.Conn) {
	io.Copy(c, c)
}

func closeSock(c *gosockjs.Conn) {
	c.Close()
}

type NoRedirectServer struct {
	*http.ServeMux
}

// Stolen from http package
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)
	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}
	return np
}

func (m *NoRedirectServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// To get the sockjs-protocol tests to work, barf if the path is not already clean.
	if req.URL.Path != cleanPath(req.URL.Path) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	http.DefaultServeMux.ServeHTTP(w, req)
}

func main() {
	gosockjs.Install("/echo", echo)
	dwe, err := gosockjs.Install("/disabled_websocket_echo", echo)
	if err != nil {
		log.Println(err)
	}
	dwe.WebsocketEnabled = false
	cne, err := gosockjs.Install("/cookie_needed_echo", echo)
	if err != nil {
		log.Println(err)
	}
	cne.CookieNeeded = true
	gosockjs.Install("/close", closeSock)
	fmt.Println("Listening on port 8081")
	http.ListenAndServe("127.0.0.1:8081", new(NoRedirectServer))
}
