// Package native frames messages the way Chrome's native messaging expects:
// a 4-byte native-endian length, then that many bytes of JSON.
package native

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MaxToChrome is the largest message Chrome accepts from a host.
const MaxToChrome = 1024 * 1024

func Read(r io.Reader, v any) error {
	var n uint32
	if err := binary.Read(r, binary.NativeEndian, &n); err != nil {
		return err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}

func Write(w io.Writer, v any) error {
	buf, err := Encode(v)
	if err != nil {
		return err
	}
	return writeFrame(w, buf)
}

// Encode marshals v and checks it against Chrome's limit. A caller that must
// know whether a refused message reached Chrome checks with Encode before it
// calls Write.
func Encode(v any) ([]byte, error) {
	buf, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(buf) > MaxToChrome {
		return nil, fmt.Errorf("message of %d bytes exceeds Chrome's limit", len(buf))
	}
	return buf, nil
}

// writeFrame writes buf, as Encode made it, with its length.
func writeFrame(w io.Writer, buf []byte) error {
	if err := binary.Write(w, binary.NativeEndian, uint32(len(buf))); err != nil {
		return err
	}
	_, err := w.Write(buf)
	return err
}
