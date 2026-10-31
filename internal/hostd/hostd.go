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
	"sync"
	"time"

	"github.com/hk9890/chromectl/internal/native"
	"github.com/hk9890/chromectl/internal/proto"
)

// lastReplyWindow is how long a caller has to read its reply once Chrome is
// gone, so a caller that never reads cannot keep the host alive.
const lastReplyWindow = time.Second

// Run serves socketPath until fromChrome reaches EOF. It replaces whatever
// socket is at the path on start and leaves its own in place on exit: a
// reload of the extension starts the next host, which binds the path anew,
// before Chrome closes this host's pipe, and a dead socket refuses the CLI
// as an absent one does. Run returns once every call that reached Chrome has
// written its reply, so the process does not exit under it.
func Run(fromChrome io.Reader, toChrome io.Writer, socketPath string) error {
	_ = os.Remove(socketPath)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return err
	}
	ln.SetUnlinkOnClose(false)
	defer func() { _ = ln.Close() }()

	h := &host{toChrome: toChrome, pending: map[uint64]chan proto.Reply{}, inFlight: map[net.Conn]struct{}{}}

	readErr := make(chan error, 1)
	go func() {
		readErr <- h.readReplies(fromChrome)
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
	if e := <-readErr; !errors.Is(e, io.EOF) {
		return e
	}
	return nil
}

type host struct {
	toChrome     io.Writer
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

func goneMsg(err error) string { return "chrome went away: " + err.Error() }

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
// call went in flight, which Run waits for; a call made after Chrome is gone
// does not.
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
	h.inFlight[conn] = struct{}{}
	h.inFlightDone.Add(1)
	h.mu.Unlock()

	h.writeMu.Lock()
	err := native.Write(h.toChrome, req)
	h.writeMu.Unlock()
	if err != nil {
		h.mu.Lock()
		delete(h.pending, req.Seq)
		h.mu.Unlock()
		return proto.Reply{Seq: req.Seq, Error: fmt.Sprintf("send to chrome: %v", err)}, true
	}
	return <-ch, true
}
