package main

import (
	"fmt"
	"github.com/mrlauer/gosockjs"
	"io"
	"log"
	"net/http"
	"time"
)

func echo(c *gosockjs.Conn) {
	io.Copy(c, c)
}

func closeSock(c *gosockjs.Conn) {
	// Wait a bit to give tests time to catch up!
	time.Sleep(time.Millisecond * 10)
	c.Close()
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
	http.ListenAndServe(":8081", nil)
}
