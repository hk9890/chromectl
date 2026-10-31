// Package hostd is the process Chrome starts for the extension. It relays
// requests from a Unix socket to the extension over stdin/stdout and returns
// each reply to the connection that asked. It lives as long as Chrome keeps
// the pipe open.
package hostd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/native"
	"github.com/hk9890/chromectl/internal/proto"
)

// lastReplyWindow is how long a caller has to read its reply once Chrome is
// gone, so a caller that never reads cannot keep the host alive.
const lastReplyWindow = time.Second

var profileID = regexp.MustCompile(`^[0-9a-f]{8}$`)

// Run serves Chrome's pipe until fromChrome reaches EOF. It first asks the
// extension for its profile id, then listens in dir at the name that tells
// the CLI who it is, client.SocketName. An extension that cannot answer is
// older than this binary; the host then listens as a stale host and answers
// every call with proto.StaleExtension, because exiting would only make the
// extension start it again. Run removes its socket when Chrome closes the
// pipe, and returns once every call that reached Chrome has written its
// reply, so the process does not exit under it.
func Run(fromChrome io.Reader, toChrome io.Writer, dir, browser string, browserPID int) error {
	h := &host{toChrome: toChrome, pending: map[uint64]chan proto.Reply{}, inFlight: map[net.Conn]struct{}{}}

	readErr := make(chan error, 1)
	readDone := make(chan struct{})
	go func() {
		readErr <- h.readReplies(fromChrome)
		close(readDone)
	}()

	var id proto.Identity
	reply, _ := h.ask(nil, proto.Request{Cmd: "identify"})
	h.mu.Lock()
	gone := h.gone
	h.mu.Unlock()
	if gone != nil {
		return chromeEnded(<-readErr)
	}
	if reply.Error == "" {
		_ = json.Unmarshal(reply.Result, &id)
	}
	if !profileID.MatchString(id.Profile) {
		log.Printf("identify answered %q %s: %s", id.Profile, reply.Error, proto.StaleExtension)
		h.stale = true
		id.Profile = ""
	}

	path, ln, err := listen(dir, client.SocketName(id.Profile, browser, browserPID, time.Now().UnixNano()))
	if err != nil {
		return err
	}
	go func() {
		<-readDone
		_ = os.Remove(path)
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil {
			log.Printf("accept: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go h.serve(conn)
	}
	h.limitLastReplies()
	h.inFlightDone.Wait()
	return chromeEnded(<-readErr)
}

func chromeEnded(readErr error) error {
	if errors.Is(readErr, io.EOF) {
		return nil
	}
	return readErr
}

// listen binds and listens at a hidden name in dir, then renames the socket
// to name. A name the CLI sees therefore always has a listener behind it, and
// a refused connection means a dead host.
func listen(dir, name string) (string, *net.UnixListener, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	path, tmp := filepath.Join(dir, name), filepath.Join(dir, "."+name)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: tmp, Net: "unix"})
	if err != nil {
		return "", nil, err
	}
	ln.SetUnlinkOnClose(false)
	if err := os.Rename(tmp, path); err != nil {
		_ = ln.Close()
		_ = os.Remove(tmp)
		return "", nil, err
	}
	return path, ln, nil
}

// Browser names the browser whose process is pid, from the directory of its
// executable: Chrome's channels and Brave's share an executable name and
// differ by directory.
func Browser(pid int) string {
	exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return "unknown"
	}
	return browserName(exe)
}

func browserName(exe string) string {
	exe = strings.TrimSuffix(exe, " (deleted)")
	dir := filepath.Base(filepath.Dir(exe))
	var name string
	switch {
	case dir == "chrome" || strings.HasPrefix(dir, "chrome-") || strings.HasPrefix(dir, "brave"):
		name = dir
	case strings.Contains(exe, "chromium"):
		name = "chromium"
	default:
		name = filepath.Base(exe)
	}
	// A dot separates the fields of the socket name.
	return strings.ReplaceAll(name, ".", "_")
}

type host struct {
	toChrome     io.Writer
	stale        bool
	writeMu      sync.Mutex
	mu           sync.Mutex
	seq          uint64
	pending      map[uint64]chan proto.Reply
	gone         error
	inFlight     map[net.Conn]struct{}
	inFlightDone sync.WaitGroup
}

func (h *host) readReplies(r io.Reader) error {
	for {
		var reply proto.Reply
		if err := native.Read(r, &reply); err != nil {
			var syntaxErr *json.SyntaxError
			var typeErr *json.UnmarshalTypeError
			if !errors.As(err, &syntaxErr) && !errors.As(err, &typeErr) {
				h.failAll(err)
				return err
			}
			reply = proto.Reply{Seq: reply.Seq, Error: "bad reply from the extension: " + err.Error()}
			// The host numbers calls from 1, so a 0 is a seq that did not decode
			// either, and no one call can be told it was the one.
			if reply.Seq == 0 {
				h.mu.Lock()
				h.failPending(reply.Error)
				h.mu.Unlock()
				continue
			}
		}
		h.mu.Lock()
		ch, ok := h.pending[reply.Seq]
		delete(h.pending, reply.Seq)
		h.mu.Unlock()
		if ok {
			ch <- reply
		} else {
			log.Printf("reply for unknown seq %d: %s", reply.Seq, reply.Error)
		}
	}
}

func (h *host) failAll(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gone = err
	h.failPending(goneMsg(err))
}

// failPending answers every pending call with msg. The caller holds h.mu.
func (h *host) failPending(msg string) {
	for seq, ch := range h.pending {
		ch <- proto.Reply{Seq: seq, Error: msg}
		delete(h.pending, seq)
	}
}

func goneReply(seq uint64, err error) proto.Reply {
	return proto.Reply{Seq: seq, Error: goneMsg(err)}
}

func goneMsg(err error) string { return proto.ChromeGone + ": " + err.Error() }

// limitLastReplies gives each call in flight lastReplyWindow to write its
// reply. Run calls it after Chrome is gone, when no call can start any more.
func (h *host) limitLastReplies() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.inFlight {
		_ = conn.SetWriteDeadline(time.Now().Add(lastReplyWindow))
	}
}

func (h *host) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	var req proto.Request
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(proto.Reply{Error: "bad request: " + err.Error()})
		return
	}
	if h.stale {
		_ = json.NewEncoder(conn).Encode(proto.Reply{Error: proto.StaleExtension})
		return
	}
	reply, inFlight := h.ask(conn, req)
	_ = json.NewEncoder(conn).Encode(reply)
	if inFlight {
		h.mu.Lock()
		delete(h.inFlight, conn)
		h.mu.Unlock()
		h.inFlightDone.Done()
	}
}

// ask relays req to Chrome and waits for its reply. It reports whether the
// caller's conn went in flight, which Run waits for; a call made after
// Chrome is gone does not, nor the host's own call, which has no conn.
func (h *host) ask(conn net.Conn, req proto.Request) (reply proto.Reply, inFlight bool) {
	ch := make(chan proto.Reply, 1)
	h.mu.Lock()
	h.seq++
	req.Seq = h.seq
	if gone := h.gone; gone != nil {
		h.mu.Unlock()
		return goneReply(req.Seq, gone), false
	}
	h.pending[req.Seq] = ch
	inFlight = conn != nil
	if inFlight {
		h.inFlight[conn] = struct{}{}
		h.inFlightDone.Add(1)
	}
	h.mu.Unlock()

	h.writeMu.Lock()
	err := native.Write(h.toChrome, req)
	h.writeMu.Unlock()
	if err != nil {
		h.mu.Lock()
		delete(h.pending, req.Seq)
		h.mu.Unlock()
		return proto.Reply{Seq: req.Seq, Error: fmt.Sprintf("send to chrome: %v", err)}, inFlight
	}
	return <-ch, inFlight
}
