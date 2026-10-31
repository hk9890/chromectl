package hostd_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/hostd"
	"github.com/hk9890/chromectl/internal/native"
	"github.com/hk9890/chromectl/internal/proto"
)

// fakeExtension plays Chrome's side of the pipe: it answers every request it
// reads with a reply carrying the same seq.
func fakeExtension(t *testing.T, answer func(proto.Request) proto.Reply) (socket string, done func()) {
	t.Helper()
	socket = filepath.Join(t.TempDir(), "s.sock")
	return socket, fakeExtensionAt(t, socket, answer)
}

// fakeExtensionAt starts a host on socket, which may already hold another
// host's socket, and returns once this host accepts on it.
func fakeExtensionAt(t *testing.T, socket string, answer func(proto.Request) proto.Reply) (done func()) {
	t.Helper()
	return rawExtensionAt(t, socket, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		r := answer(req)
		r.Seq = req.Seq
		return native.Write(toHost, r)
	})
}

// rawExtensionAt is fakeExtensionAt for an extension that sees each request
// as the host framed it and writes its own reply to the host's stdin, or none.
func rawExtensionAt(t *testing.T, socket string, reply func(raw json.RawMessage, req proto.Request, toHost io.Writer) error) (done func()) {
	t.Helper()
	toHost, fromExt := io.Pipe()
	fromHost, toExt := io.Pipe()
	before, _ := os.Stat(socket)

	ran := make(chan error, 1)
	go func() { ran <- hostd.Run(toHost, toExt, socket) }()
	go func() {
		for {
			var raw json.RawMessage
			if err := native.Read(fromHost, &raw); err != nil {
				return
			}
			var req proto.Request
			_ = json.Unmarshal(raw, &req)
			if err := reply(raw, req, fromExt); err != nil {
				return
			}
		}
	}()
	for start := time.Now(); !listening(socket, before); time.Sleep(time.Millisecond) {
		if time.Since(start) > 2*time.Second {
			t.Fatal("host never bound its socket")
		}
	}
	// Chrome closes both pipes at once.
	return func() {
		_ = fromExt.Close()
		_ = toExt.Close()
		if err := <-ran; err != nil {
			t.Errorf("Run returned %v after Chrome closed the pipe", err)
		}
	}
}

// listening reports whether a socket other than before is at the path and
// accepts. The path appears at bind, before listen, so a file alone is not
// a host that answers.
func listening(socket string, before os.FileInfo) bool {
	now, err := os.Stat(socket)
	if err != nil || before != nil && os.SameFile(before, now) {
		return false
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func TestListReachesTheExtensionAndComesBack(t *testing.T) {
	socket, done := fakeExtension(t, func(req proto.Request) proto.Reply {
		if req.Cmd != "list" {
			t.Errorf("cmd = %q", req.Cmd)
		}
		res, _ := json.Marshal([]proto.Tab{{ID: 4, Window: 1, Active: true, Title: "PRs", URL: "https://github.com/pulls"}})
		return proto.Reply{Result: res}
	})
	defer done()

	var tabs []proto.Tab
	if err := client.Call(socket, proto.Request{Cmd: "list"}, &tabs); err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 || tabs[0].URL != "https://github.com/pulls" || !tabs[0].Active {
		t.Fatalf("tabs = %+v", tabs)
	}
}

// recordingExtension answers every request with result and hands the frame
// the host sent, decoded by the JSON names the extension reads, to sent.
func recordingExtension(t *testing.T, result any) (socket string, sent <-chan map[string]any, done func()) {
	t.Helper()
	socket = filepath.Join(t.TempDir(), "s.sock")
	frames := make(chan map[string]any, 1)
	done = rawExtensionAt(t, socket, func(raw json.RawMessage, req proto.Request, toHost io.Writer) error {
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		frames <- fields
		return native.Write(toHost, map[string]any{"seq": req.Seq, "result": result})
	})
	return socket, frames, done
}

func TestOpenSendsUrlAndWindow(t *testing.T) {
	socket, sent, done := recordingExtension(t, map[string]int{"id": 5, "window": 2})
	defer done()

	var opened proto.Tab
	if err := client.Call(socket, proto.Request{Cmd: "open", URL: "https://example.com", Window: 2}, &opened); err != nil {
		t.Fatal(err)
	}
	if got := <-sent; got["cmd"] != "open" || got["url"] != "https://example.com" || got["window"] != float64(2) {
		t.Fatalf("the extension received %v", got)
	}
	if opened.ID != 5 || opened.Window != 2 {
		t.Fatalf("opened = %+v", opened)
	}
}

func TestCloseSendsTheTabId(t *testing.T) {
	socket, sent, done := recordingExtension(t, map[string]any{})
	defer done()

	if err := client.Call(socket, proto.Request{Cmd: "close", Tab: 7}, nil); err != nil {
		t.Fatal(err)
	}
	if got := <-sent; got["cmd"] != "close" || got["id"] != float64(7) {
		t.Fatalf("the extension received %v", got)
	}
}

// Chrome reads a missing tab id as the current tab, so a 0 from any socket
// client must reach the extension as a 0.
func TestATabIdOfZeroIsSent(t *testing.T) {
	socket, sent, done := recordingExtension(t, map[string]any{})
	defer done()

	_ = client.Call(socket, proto.Request{Cmd: "activate"}, nil)
	if got := <-sent; got["id"] != float64(0) {
		t.Fatalf("the extension received %v", got)
	}
}

func TestTheExtensionsErrorIsTheCallersError(t *testing.T) {
	socket, done := fakeExtension(t, func(req proto.Request) proto.Reply {
		return proto.Reply{Error: "No tab with id: 99."}
	})
	defer done()

	err := client.Call(socket, proto.Request{Cmd: "activate", Tab: 99}, nil)
	if err == nil || err.Error() != "No tab with id: 99." {
		t.Fatalf("err = %v", err)
	}
}

// One reply the host cannot decode fails its own call, not the host.
func TestABadReplyFailsOnlyItsCall(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s.sock")
	done := rawExtensionAt(t, socket, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		switch req.Tab {
		case 1:
			return native.Write(toHost, map[string]any{"seq": req.Seq, "error": 5})
		case 2:
			return native.Write(toHost, map[string]any{"seq": fmt.Sprint(req.Seq), "result": 1})
		}
		return native.Write(toHost, map[string]any{"seq": req.Seq, "result": map[string]int{"id": req.Tab}})
	})
	defer done()

	// Tab 1's reply names its seq; tab 2's seq does not decode, so the host
	// cannot tell which call it answers and fails every pending one.
	for _, tab := range []int{1, 2} {
		err := client.Call(socket, proto.Request{Cmd: "activate", Tab: tab}, nil)
		if err == nil || !strings.HasPrefix(err.Error(), "bad reply from the extension: ") {
			t.Fatalf("the bad reply for tab %d gave %v", tab, err)
		}
	}
	if err := client.Call(socket, proto.Request{Cmd: "activate", Tab: 3}, nil); err != nil {
		t.Fatalf("the next call gave %v", err)
	}
}

// A caller that never reads its reply cannot keep the host alive once
// Chrome is gone.
func TestACallerThatDoesNotReadCannotHoldTheHost(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s.sock")
	answered := make(chan struct{})
	done := rawExtensionAt(t, socket, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		defer close(answered)
		// Larger than a socket buffer, so the host's write blocks. native.Write
		// refuses it: the limit is for the host's direction only.
		frame, _ := json.Marshal(map[string]any{"seq": req.Seq, "result": strings.Repeat("x", 4<<20)})
		if err := binary.Write(toHost, binary.NativeEndian, uint32(len(frame))); err != nil {
			return err
		}
		_, err := toHost.Write(frame)
		return err
	})

	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := json.NewEncoder(conn).Encode(proto.Request{Cmd: "list"}); err != nil {
		t.Fatal(err)
	}
	<-answered
	exited := make(chan struct{})
	go func() { done(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return while a caller held its reply unread")
	}
}

// A call that is pending when Chrome closes the pipe gets its answer before
// Run returns, and so before the host process exits.
func TestAPendingCallHearsThatChromeWentAway(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s.sock")
	asked := make(chan struct{})
	done := rawExtensionAt(t, socket, func(json.RawMessage, proto.Request, io.Writer) error {
		close(asked)
		return nil
	})

	errs := make(chan error, 1)
	go func() { errs <- client.Call(socket, proto.Request{Cmd: "list"}, nil) }()
	<-asked
	done()
	if err := <-errs; err == nil || err.Error() != "chrome went away: EOF" {
		t.Fatalf("err = %v", err)
	}
}

func TestConcurrentCallsAreCorrelatedBySeq(t *testing.T) {
	socket, done := fakeExtension(t, func(req proto.Request) proto.Reply {
		res, _ := json.Marshal(proto.Tab{ID: req.Tab})
		return proto.Reply{Result: res}
	})
	defer done()

	errs := make(chan error, 20)
	for i := 1; i <= 20; i++ {
		go func(id int) {
			var got proto.Tab
			err := client.Call(socket, proto.Request{Cmd: "activate", Tab: id}, &got)
			if err == nil && got.ID != id {
				err = fmt.Errorf("asked for tab %d, got %d", id, got.ID)
			}
			errs <- err
		}(i)
	}
	for i := 0; i < 20; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func TestWithoutAHostTheClientSaysSo(t *testing.T) {
	err := client.Call(filepath.Join(t.TempDir(), "absent.sock"), proto.Request{Cmd: "list"}, nil)
	if err != client.ErrNoHost {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
}

// A reload of the extension starts the new host before Chrome closes the old
// host's pipe; the old host must leave the new host's socket in place.
func TestAnOldHostExitingKeepsTheNewHostsSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s.sock")
	oldDone := fakeExtensionAt(t, socket, func(proto.Request) proto.Reply { return proto.Reply{Error: "old host"} })
	newDone := fakeExtensionAt(t, socket, func(proto.Request) proto.Reply { return proto.Reply{Error: "new host"} })
	oldDone()

	if err := client.Call(socket, proto.Request{Cmd: "list"}, nil); err == nil || err.Error() != "new host" {
		t.Fatalf("after the old host exited, Call = %v, want the new host's answer", err)
	}
	newDone()
	if err := client.Call(socket, proto.Request{Cmd: "list"}, nil); err != client.ErrNoHost {
		t.Fatalf("after every host exited, Call = %v, want ErrNoHost", err)
	}
}
