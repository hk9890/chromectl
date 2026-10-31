// Package client is the CLI's side of the sockets: where the hosts are, who
// each one is, and one request and one reply to one of them.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hk9890/chromectl/internal/proto"
)

// ErrNoHost means no chromectl host is listening: Chrome is not running, or
// the extension is not loaded, or `chromectl install` was never run.
var ErrNoHost = errors.New("no chromectl host is listening: is Chrome running with the chromectl extension loaded, and was `chromectl install` run?")

// Timeout is how long a call waits, from its connect to the last byte of
// its reply. The CLI's --timeout sets it.
var Timeout = 5 * time.Second

// NoReplyError is a call that got no reply: the timeout passed, or the host
// closed the connection, as when Chrome went away. Chrome may or may not
// carry the command out. It unwraps to os.ErrDeadlineExceeded or io.EOF.
type NoReplyError struct {
	msg string
	err error
}

func (e *NoReplyError) Error() string { return e.msg }

func (e *NoReplyError) Unwrap() error { return e.err }

// SocketDir is the directory every host's socket is in. Without
// XDG_RUNTIME_DIR, as under cron, it is /run/user/<uid>, the directory a
// desktop session sets the variable to, so the CLI finds the hosts Chrome
// started.
func SocketDir() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "chromectl"), nil
	}
	dir := fmt.Sprintf("/run/user/%d", os.Getuid())
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
		return "", fmt.Errorf("XDG_RUNTIME_DIR is not set, and %s is not a directory of this user", dir)
	}
	return filepath.Join(dir, "chromectl"), nil
}

// Host is one running host, as its socket name tells it.
type Host struct {
	// Profile is the profile's 8 hex digits, or "" for a host whose
	// extension is older than its binary.
	Profile    string
	Browser    string
	BrowserPID int
	Start      int64
	Path       string
	// Conflict is set when the same profile runs under two browsers, so its
	// ids cannot name one tab.
	Conflict string
}

// SocketName is the name a host listens at. The CLI reads every field back
// from it, so a host is known without a call.
func SocketName(profile, browser string, browserPID int, start int64) string {
	if profile == "" {
		profile = "stale"
	}
	return fmt.Sprintf("%s.%s.%d.%d.sock", profile, browser, browserPID, start)
}

var socketName = regexp.MustCompile(`^(` + proto.ProfileID + `|stale)\.([^.]+)\.([0-9]+)\.([0-9]+)\.sock$`)

// Hosts lists the hosts in dir. It first removes every socket whose host is
// dead, so a socket a killed host left cannot pass for a reload or a copied
// profile. Of two live hosts of one profile in one browser, the old one of a
// reload, it keeps the later. A file it cannot parse is not a host's.
func Hosts(dir string) ([]Host, error) {
	hosts, err := parseHosts(dir)
	var live []Host
	for _, h := range hosts {
		if !dead(h.Path) {
			live = append(live, h)
		}
	}
	return newestPerBrowser(live), err
}

// dead reports whether the socket at path refuses a connection, and removes
// it if so. A host renames its socket into place only once it listens, so a
// refused socket is a dead host's, never a starting one's.
func dead(path string) bool {
	conn, err := net.Dial("unix", path)
	if err == nil {
		_ = conn.Close()
		return false
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		_ = os.Remove(path)
		return true
	}
	return false
}

func parseHosts(dir string) ([]Host, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var hosts []Host
	for _, e := range entries {
		m := socketName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		h := Host{Browser: m[2], Path: filepath.Join(dir, e.Name())}
		if m[1] != "stale" {
			h.Profile = m[1]
		}
		h.BrowserPID, _ = strconv.Atoi(m[3])
		h.Start, _ = strconv.ParseInt(m[4], 10, 64)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func newestPerBrowser(hosts []Host) []Host {
	type key struct {
		profile string
		pid     int
	}
	newest := map[key]int{}
	var kept []Host
	for _, h := range hosts {
		k := key{h.Profile, h.BrowserPID}
		if h.Profile == "" {
			kept = append(kept, h)
			continue
		}
		if i, ok := newest[k]; ok {
			if h.Start > kept[i].Start {
				kept[i] = h
			}
			continue
		}
		newest[k] = len(kept)
		kept = append(kept, h)
	}
	browsers := map[string][]string{}
	for _, h := range kept {
		if h.Profile != "" {
			browsers[h.Profile] = append(browsers[h.Profile], fmt.Sprintf("%s (pid %d)", h.Browser, h.BrowserPID))
		}
	}
	for i, h := range kept {
		if b := browsers[h.Profile]; len(b) > 1 {
			kept[i].Conflict = fmt.Sprintf("profile %s runs in %s, a copied profile: clear the chromectl extension's site data in one of them", h.Profile, strings.Join(b, " and "))
		}
	}
	return kept
}

// Call sends req to the host at socketPath and decodes the result into out,
// which may be nil when the caller only needs success. A socket that refuses
// the connection belongs to a host that is dead: Call removes it and returns
// ErrNoHost.
func Call(socketPath string, req proto.Request, out any) error {
	deadline := time.Now().Add(Timeout)
	conn, err := (&net.Dialer{Deadline: deadline}).Dial("unix", socketPath)
	if err != nil && dead(socketPath) {
		return ErrNoHost
	}
	if err != nil {
		return fmt.Errorf("connect to the chromectl host: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(deadline)
	step := "send request"
	err = json.NewEncoder(conn).Encode(req)
	var reply proto.Reply
	if err == nil {
		step = "read reply"
		err = json.NewDecoder(conn).Decode(&reply)
	}
	switch {
	case errors.Is(err, os.ErrDeadlineExceeded):
		return &NoReplyError{fmt.Sprintf("no reply in %s; Chrome may still carry the command out", Timeout), os.ErrDeadlineExceeded}
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, syscall.EPIPE), errors.Is(err, syscall.ECONNRESET):
		return &NoReplyError{"no reply: the chromectl host closed the connection, as when Chrome goes away; Chrome may or may not have carried the command out", io.EOF}
	case err != nil:
		return fmt.Errorf("%s: %w", step, err)
	}
	if reply.Error != "" {
		return errors.New(reply.Error)
	}
	if out != nil && len(reply.Result) > 0 {
		return json.Unmarshal(reply.Result, out)
	}
	return nil
}
