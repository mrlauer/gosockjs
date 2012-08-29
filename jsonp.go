package gosockjs

import (
	"bytes"
	"code.google.com/p/gorilla/mux"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type jsonpOptions struct {
	callback string
}

func (o jsonpOptions) writeFrame(w io.Writer, frame []byte) error {
	js, err := json.Marshal(string(frame))
	if err != nil {
		return err
	}
	munged := fmt.Sprintf("%s(%s);\r\n", o.callback, js)
	_, err = io.WriteString(w, munged)
	return err
}

func (o jsonpOptions) contentType() string {
	return "application/javascript; charset=UTF-8"
}

func (o jsonpOptions) maxBytes() int {
	return 1
}

func (o jsonpOptions) writePrelude(io.Writer) error {
	return nil
}

func (o jsonpOptions) streaming() bool {
	return false
}

func extractSendContent(req *http.Request) (string, error) {
	// What are the options? Is this it?
	ctype := req.Header.Get("Content-Type")
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, req.Body)
	req.Body.Close()
	switch ctype {
	case "application/x-www-form-urlencoded":
		values, err := url.ParseQuery(string(buf.Bytes()))
		if err != nil {
			return "", errors.New("Could not parse query")
		}
		return values.Get("d"), nil
	case "text/plain":
		return string(buf.Bytes()), nil
	}
	return "", errors.New("Unrecognized content type")
}

func jsonpSendHandler(r *Router, w http.ResponseWriter, req *http.Request) {
	if xhrProlog(w, req) {
		return
	}
	w.Header().Set("Content-type", "text/plain; charset=UTF-8")
	sessionId := mux.Vars(req)["sessionid"]
	// Find the session
	s := r.getSession(sessionId)
	if s == nil {
		http.NotFoundHandler().ServeHTTP(w, req)
		return
	}
	xhrJsessionid(r, w, req)

	payload, err := extractSendContent(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(payload) == 0 {
		http.Error(w, EmptyPayload.Error(), http.StatusInternalServerError)
		return
	}
	err = s.fromClient(message(payload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	io.WriteString(w, "ok")
}

func jsonpHandler(r *Router, w http.ResponseWriter, req *http.Request) {
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
	xhrHandlerBase(jsonpOptions{callback}, r, w, req)
}
