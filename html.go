package gosockjs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var _htmlFile string = `<!doctype html>
<html><head>
  <meta http-equiv="X-UA-Compatible" content="IE=edge" />
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
</head><body><h2>Don't panic!</h2>
  <script>
    document.domain = document.domain;
    var c = parent.%s;
    c.start();
    function p(d) {c.message(d);};
    window.onload = function() {c.stop();};
  </script>
`

type htmlfileOptions struct {
	callback string
}

func (o htmlfileOptions) writeFrame(w io.Writer, frame []byte) error {
	js, err := json.Marshal(string(frame))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "<script>\np(%s);\n</script>\r\n", js)
	return err
}

func (o htmlfileOptions) contentType() string {
	return "text/html; charset=UTF-8"
}

func (o htmlfileOptions) maxBytes() int {
	return 4096
}

func (o htmlfileOptions) writePrelude(w io.Writer) error {
	prelude := fmt.Sprintf(_htmlFile, o.callback)
	// It must be at least 1024 bytes.
	if len(prelude) < 1024 {
		prelude += strings.Repeat(" ", 1024)
	}
	prelude += "\r\n"
	_, err := io.WriteString(w, prelude)
	return err
}

func (o htmlfileOptions) streaming() bool {
	return true
}

func htmlfileHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		http.Error(w, "Bad query", http.StatusInternalServerError)
		return
	}
	callback := req.Form.Get("c")
	if callback == "" {
		http.Error(w, `"callback" parameter required`, http.StatusInternalServerError)
		return
	}
	xhrHandlerBase(htmlfileOptions{callback: callback}, r, w, req)
}
