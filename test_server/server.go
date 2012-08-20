package main

import (
	"fmt"
	"github.com/mrlauer/gosockjs"
	"io"
	"log"
	"net/http"
	"strings"
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

func (m *NoRedirectServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.HasSuffix(req.URL.Path, "//") {
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
	gosockjs.Install("/close", closeSock)
	fmt.Println("Listening on port 8081")
	http.ListenAndServe(":8081", new(NoRedirectServer))
}
