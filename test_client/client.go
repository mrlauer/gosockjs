/*
A very simple client for testing gosockjs.
*/
package main

import (
	"code.google.com/p/gorilla/mux"
	"fmt"
	"github.com/mrlauer/gosockjs"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"runtime"
)

var StaticDir string
var TemplateDir string

func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Could not get file")
	}
	StaticDir = path.Join(path.Dir(file), "static")
	TemplateDir = path.Join(path.Dir(file), "templates")
}

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles(path.Join(TemplateDir, "client.html")))
	err := t.Execute(w, nil)
	if err != nil {
		log.Println(err)
	}
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["Filename"]
	w.Header().Set("Cache-Control", "no-cache")
	filepath := path.Join(StaticDir, filename)
	http.ServeFile(w, r, filepath)
}

func echo(c *gosockjs.Conn) {
	io.Copy(c, c)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/static/{Filename:.*}", staticHandler)
	r.HandleFunc("/", handler)
	http.Handle("/", r)
	gosockjs.Install("/echo", echo)
	fmt.Printf("Listening on port :8082\n")
	http.ListenAndServe(":8082", nil)
}
