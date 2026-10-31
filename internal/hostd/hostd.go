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

// Run serves socketPath until fromChrome reaches EOF. It replaces whatever
// socket is at the path on start and leaves its own in place on exit: a
// reload of the extension starts the next host, which binds the path anew,
// before Chrome closes this host's pipe, and a dead socket refuses the CLI
// as an absent one does. Run returns once every connection it accepted has
// had its reply.
func Run(fromChrome io.Reader, toChrome io.Writer, socketPath string) error {
	_ = os.Remove(socketPath)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return err
	}
	ln.SetUnlinkOnClose(false)
	defer func() { _ = ln.Close() }()

	h := &host{toChrome: toChrome, pending: map[uint64]chan proto.Reply{}, conns: map[net.Conn]bool{}}

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
		h.track(conn)
		go h.serve(conn)
	}
	h.stopReading()
	h.served.Wait()
	if e := <-readErr; !errors.Is(e, io.EOF) {
		return e
	}
	return nil
}

type host struct {
	toChrome io.Writer
	writeMu  sync.Mutex
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]chan proto.Reply
	gone     error
	conns    map[net.Conn]bool
	served   sync.WaitGroup
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
			reply.Result = nil
			reply.Error = "bad reply from the extension: " + err.Error()
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
	for seq, ch := range h.pending {
		ch <- goneReply(seq, err)
		delete(h.pending, seq)
	}
}

func goneReply(seq uint64, err error) proto.Reply {
	return proto.Reply{Seq: seq, Error: "chrome went away: " + err.Error()}
}

func (h *host) track(conn net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[conn] = true
	h.served.Add(1)
}

// stopReading ends the wait of every connection that has not sent its
// request yet, so a silent client cannot keep the host alive after Chrome
// is gone. A reply already on its way is still written.
func (h *host) stopReading() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.conns {
		_ = conn.SetReadDeadline(time.Now())
	}
}

func (h *host) serve(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		h.served.Done()
	}()
	var req proto.Request
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(proto.Reply{Error: "bad request: " + err.Error()})
		return
	}
	_ = json.NewEncoder(conn).Encode(h.ask(req))
}

func (h *host) ask(req proto.Request) proto.Reply {
	ch := make(chan proto.Reply, 1)
	h.mu.Lock()
	h.seq++
	req.Seq = h.seq
	if gone := h.gone; gone != nil {
		h.mu.Unlock()
		return goneReply(req.Seq, gone)
	}
	h.pending[req.Seq] = ch
	h.mu.Unlock()

	h.writeMu.Lock()
	err := native.Write(h.toChrome, req)
	h.writeMu.Unlock()
	if err != nil {
		h.mu.Lock()
		delete(h.pending, req.Seq)
		h.mu.Unlock()
		return proto.Reply{Seq: req.Seq, Error: fmt.Sprintf("send to chrome: %v", err)}
	}
	return <-ch
}
