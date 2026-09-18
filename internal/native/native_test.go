package native

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, map[string]any{"id": 7, "cmd": "list"}); err != nil {
		t.Fatal(err)
	}
	if n := binary.NativeEndian.Uint32(buf.Bytes()[:4]); int(n) != buf.Len()-4 {
		t.Fatalf("length prefix %d, body %d", n, buf.Len()-4)
	}
	var got struct {
		ID  int
		Cmd string
	}
	if err := Read(&buf, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 7 || got.Cmd != "list" {
		t.Fatalf("got %+v", got)
	}
}

func TestWriteRefusesOversize(t *testing.T) {
	err := Write(&bytes.Buffer{}, strings.Repeat("x", MaxToChrome))
	if err == nil {
		t.Fatal("want an error for a message over the limit")
	}
}
