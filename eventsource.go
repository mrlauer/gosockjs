package gosockjs

import (
	"fmt"
	"io"
	"net/http"
)

type eventsourceOptions struct {
}

func (o eventsourceOptions) writeFrame(w io.Writer, frame []byte) error {
	_, err := fmt.Fprintf(w, "data: %s\r\n\r\n", frame)
	return err
}

func (o eventsourceOptions) contentType() string {
	return "text/event-stream; charset=UTF-8"
}

func (o eventsourceOptions) maxBytes() int {
	return 4096
}

func (o eventsourceOptions) writePrelude(w io.Writer) error {
	_, err := io.WriteString(w, "\r\n")
	return err
}

func (o eventsourceOptions) streaming() bool {
	return true
}

func eventsourceHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	xhrHandlerBase(eventsourceOptions{}, r, w, req)
}
