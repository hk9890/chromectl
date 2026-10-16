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

	"github.com/hk9890/chromectl/internal/native"
	"github.com/hk9890/chromectl/internal/proto"
)

// Run serves socketPath until fromChrome reaches EOF. The socket is removed
// on the way out.
func Run(fromChrome io.Reader, toChrome io.Writer, socketPath string) error {
	_ = os.Remove(socketPath)
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(socketPath) }()
	defer func() { _ = ln.Close() }()

	h := &host{toChrome: toChrome, pending: map[uint64]chan proto.Reply{}}

	readErr := make(chan error, 1)
	go func() {
		readErr <- h.readReplies(fromChrome)
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if e := <-readErr; e != nil && !errors.Is(e, io.EOF) {
				return e
			}
			return nil
		}
		go h.serve(conn)
	}
}

type host struct {
	toChrome io.Writer
	writeMu  sync.Mutex
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]chan proto.Reply
}

func (h *host) readReplies(r io.Reader) error {
	for {
		var reply proto.Reply
		if err := native.Read(r, &reply); err != nil {
			h.failAll(err)
			return err
		}
		h.mu.Lock()
		ch, ok := h.pending[reply.Seq]
		delete(h.pending, reply.Seq)
		h.mu.Unlock()
		if ok {
			ch <- reply
		} else {
			log.Printf("reply for unknown seq %d", reply.Seq)
		}
	}
}

func (h *host) failAll(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for seq, ch := range h.pending {
		ch <- proto.Reply{Seq: seq, Error: "chrome went away: " + err.Error()}
		delete(h.pending, seq)
	}
}

func (h *host) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
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
