package gosockjs

import (
	"bytes"
	"strconv"
	"testing"
)

func TestFrames(t *testing.T) {
	h := heartbeatFrame()
	if string(h) != "h" {
		t.Errorf("Heartbeat frame was %s", h)
	}
	o := openFrame()
	if string(o) != "o" {
		t.Errorf("Open frame was %s", o)
	}
	c := closeFrame(1234, "Go away!")
	if string(c) != `c[1234,"Go away!"]` {
		t.Errorf("Close frame was %s", c)
	}
	msgStrings := []string{
		"Ohai!",
		"a",
		"How are you??",
		`Quoted "string"`,
	}
	var msgs []message
	buf := bytes.NewBuffer(nil)
	buf.WriteString("a")
	buf.WriteString("[")
	for i, str := range msgStrings {
		msgs = append(msgs, message(str))
		if i != 0 {
			buf.WriteString(",")
		}
		buf.WriteString(strconv.Quote(str))
	}
	buf.WriteString("]")
	a := messageFrame(msgs...)
	if !bytes.Equal(a, buf.Bytes()) {
		t.Errorf("message frame was\n%s, not\n%s", a, buf.Bytes())
	}
}
