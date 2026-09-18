// Package client is the CLI's side of the socket: one request, one reply.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/hk9890/chromectl/internal/proto"
)

// ErrNoHost means no chromectl host is listening: Chrome is not running, or
// the extension is not loaded, or `chromectl install` was never run.
var ErrNoHost = errors.New("no chromectl host is listening: is Chrome running with the chromectl extension loaded, and was `chromectl install` run?")

// SocketPath is where the host listens for this user.
func SocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "chromectl.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("chromectl-%d.sock", os.Getuid()))
}

// Call sends req to the host at socketPath and decodes the result into out,
// which may be nil when the caller only needs success.
func Call(socketPath string, req proto.Request, out any) error {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return ErrNoHost
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	var reply proto.Reply
	if err := json.NewDecoder(conn).Decode(&reply); err != nil {
		return fmt.Errorf("read reply: %w", err)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	if out != nil && len(reply.Result) > 0 {
		return json.Unmarshal(reply.Result, out)
	}
	return nil
}
