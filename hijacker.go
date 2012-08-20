package gosockjs

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Hijack an http connection to keep it open til the client closes it.

// ChunkedWriter writes in the format acceptable for chunked transfer encoding.
type chunkedWriter struct {
	w io.Writer
}

func (w *chunkedWriter) Write(data []byte) (int, error) {
	n := len(data)
	nwritten, err := fmt.Fprintf(w.w, "%x\r\n%s\r\n", n, data)
	if err != nil {
		// A reasonable guess as to how much was written
		nheader := len(fmt.Sprintf("%x\r\n", n))
		ntoreturn := nwritten - nheader
		if ntoreturn < 0 {
			ntoreturn = 0
		} else if ntoreturn > n {
			ntoreturn = n
		}
		return nwritten, err
	}
	return n, nil
}

// Write the last chunk
func (w *chunkedWriter) end() {
	// And a trailer
	fmt.Fprint(w.w, "0\r\n\r\n")
}

func (w *chunkedWriter) Close() error {
	w.end()
	c, ok := w.w.(io.WriteCloser)
	if ok {
		return c.Close()
	}
	return nil
}

// Hijack a connection, passing control to the given function.
// This should happen after writing headers, and the content type should be set!
// If the client closes the connection the function's argument will be closed.
func hijackAndContinue(w http.ResponseWriter, handler func(conn io.WriteCloser, done chan struct{})) error {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return errors.New("ResponseWriter not a hijacker")
	}

	conn, rwc, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	rwc.Flush()
	done := make(chan struct{})
	// Listen for EOF (or any other error, actually).
	go func() {
		buffer := make([]byte, 256)
		for {
			_, err := conn.Read(buffer)
			if err != nil {
				close(done)
				return
			}
		}
	}()
	go handler(&chunkedWriter{conn}, done)
	return nil
}
