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
	buf, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(buf) > MaxToChrome {
		return fmt.Errorf("message of %d bytes exceeds Chrome's limit", len(buf))
	}
	if err := binary.Write(w, binary.NativeEndian, uint32(len(buf))); err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}
