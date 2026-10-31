package hostd_test

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/hostd"
	"github.com/hk9890/chromectl/internal/native"
	"github.com/hk9890/chromectl/internal/proto"
)

const profile = "0123abcd"

// starts gives each host a later start than the one before, as two host
// processes have.
var starts atomic.Int64

// socketDir is a fresh directory short enough that a host's socket name fits
// a Unix socket path; t.TempDir's is not.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// fakeExtension plays Chrome's side of the pipe: it answers identify with
// profile and every other request with a reply carrying the same seq.
func fakeExtension(t *testing.T, answer func(proto.Request) proto.Reply) (socket string, done func()) {
	t.Helper()
	return fakeExtensionIn(t, socketDir(t), 1, answer)
}

// fakeExtensionIn starts a host for profile in dir, under the browser pid
// browserPID, and returns once its socket is in place.
func fakeExtensionIn(t *testing.T, dir string, browserPID int, answer func(proto.Request) proto.Reply) (socket string, done func()) {
	t.Helper()
	return rawExtensionIn(t, dir, browserPID, identifying(profile, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		r := answer(req)
		r.Seq = req.Seq
		return native.Write(toHost, r)
	}))
}

// identifying answers identify with id and hands every other request to
// reply.
func identifying(id string, reply func(json.RawMessage, proto.Request, io.Writer) error) func(json.RawMessage, proto.Request, io.Writer) error {
	return func(raw json.RawMessage, req proto.Request, toHost io.Writer) error {
		if req.Cmd == "identify" {
			return native.Write(toHost, map[string]any{"seq": req.Seq, "result": proto.Identity{Profile: id}})
		}
		return reply(raw, req, toHost)
	}
}

// rawExtensionIn starts a host in dir whose extension sees each request as
// the host framed it, identify included, and writes its own reply to the
// host's stdin, or none. It returns the socket the host renamed into place.
func rawExtensionIn(t *testing.T, dir string, browserPID int, reply func(raw json.RawMessage, req proto.Request, toHost io.Writer) error) (socket string, done func()) {
	t.Helper()
	toHost, fromExt := io.Pipe()
	fromHost, toExt := io.Pipe()
	before := sockets(dir)

	ran := make(chan error, 1)
	go func() {
		ran <- hostd.Run(toHost, toExt, dir, hostd.Self{Browser: "chrome", BrowserPID: browserPID, Start: starts.Add(1)})
	}()
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
	// Chrome closes both pipes at once.
	done = func() {
		_ = fromExt.Close()
		_ = toExt.Close()
		if err := <-ran; err != nil {
			t.Errorf("Run returned %v after Chrome closed the pipe", err)
		}
	}
	for start := time.Now(); ; time.Sleep(time.Millisecond) {
		for name := range sockets(dir) {
			if !before[name] {
				return filepath.Join(dir, name), done
			}
		}
		if time.Since(start) > 2*time.Second {
			t.Fatal("host never put its socket in place")
		}
	}
}

// sockets returns the visible socket names in dir.
func sockets(dir string) map[string]bool {
	names := map[string]bool{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sock") && !strings.HasPrefix(e.Name(), ".") {
			names[e.Name()] = true
		}
	}
	return names
}

// recordingExtension answers every request with result and hands the frame
// the host sent, decoded by the JSON names the extension reads, to sent.
func recordingExtension(t *testing.T, result any) (socket string, sent <-chan map[string]any, done func()) {
	t.Helper()
	frames := make(chan map[string]any, 1)
	socket, done = rawExtensionIn(t, socketDir(t), 1, identifying(profile, func(raw json.RawMessage, req proto.Request, toHost io.Writer) error {
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		frames <- fields
		return native.Write(toHost, map[string]any{"seq": req.Seq, "result": result})
	}))
	return socket, frames, done
}

func TestTheSocketNamesTheProfileAndTheBrowser(t *testing.T) {
	dir := socketDir(t)
	socket, done := fakeExtensionIn(t, dir, 4242, func(proto.Request) proto.Reply { return proto.Reply{} })
	defer done()

	hosts, err := client.Hosts(dir)
	if err != nil || len(hosts) != 1 {
		t.Fatalf("Hosts = %+v, %v", hosts, err)
	}
	if h := hosts[0]; h.Profile != profile || h.Browser != "chrome" || h.BrowserPID != 4242 || h.Path != socket {
		t.Fatalf("host = %+v, socket %s", h, socket)
	}
}

// The socket appears only once the host knows its profile, so the CLI never
// sees a host it cannot name.
func TestTheHostListensOnlyAfterIdentify(t *testing.T) {
	dir := socketDir(t)
	answer := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		if n := len(sockets(dir)); n != 0 {
			t.Errorf("%d sockets before identify was answered", n)
		}
		close(answer)
	}()
	_, done := rawExtensionIn(t, dir, 1, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		if req.Cmd == "identify" {
			<-answer
			return native.Write(toHost, map[string]any{"seq": req.Seq, "result": proto.Identity{Profile: profile}})
		}
		return nil
	})
	done()
}

// An extension older than the binary does not know identify. The host stays
// up, so the extension does not restart it every two seconds, and tells
// every caller what to do.
func TestAnExtensionWithoutIdentifyGetsAStaleHost(t *testing.T) {
	dir := socketDir(t)
	socket, done := rawExtensionIn(t, dir, 1, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		return native.Write(toHost, map[string]any{"seq": req.Seq, "error": "unknown command " + req.Cmd})
	})
	defer done()

	if !strings.HasPrefix(filepath.Base(socket), "stale.chrome.") {
		t.Fatalf("socket = %s", socket)
	}
	if err := client.Call(socket, proto.Request{Cmd: "list"}, nil); err == nil || err.Error() != proto.StaleExtension {
		t.Fatalf("err = %v", err)
	}
}

// An extension that knows identify but cannot answer it, as when IndexedDB
// fails, is not out of date; the host passes the extension's reason on.
func TestAFailedIdentifyGivesItsReason(t *testing.T) {
	socket, done := rawExtensionIn(t, socketDir(t), 1, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		return native.Write(toHost, map[string]any{"seq": req.Seq, "error": "UnknownError: Internal error opening backing store"})
	})
	defer done()

	err := client.Call(socket, proto.Request{Cmd: "list"}, nil)
	if err == nil || err.Error() != "the chromectl extension could not give its profile id: UnknownError: Internal error opening backing store" {
		t.Fatalf("err = %v", err)
	}
}

func TestStartTimeIsThisProcesssStart(t *testing.T) {
	first, err := hostd.StartTime()
	if err != nil || first <= 0 {
		t.Fatalf("StartTime = %d, %v", first, err)
	}
	if again, _ := hostd.StartTime(); again != first {
		t.Fatalf("StartTime changed from %d to %d", first, again)
	}
}

func TestTheHostRemovesItsSocketWhenChromeCloses(t *testing.T) {
	dir := socketDir(t)
	_, done := fakeExtensionIn(t, dir, 1, func(proto.Request) proto.Reply { return proto.Reply{} })
	done()
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("left in the socket directory: %v", entries)
	}
}

// A reload starts the new host before Chrome closes the old one's pipe. The
// CLI takes the later one, and the old one removes only its own socket.
func TestAReloadKeepsTheNewHost(t *testing.T) {
	dir := socketDir(t)
	_, oldDone := fakeExtensionIn(t, dir, 7, func(proto.Request) proto.Reply { return proto.Reply{Error: "old host"} })
	newSocket, newDone := fakeExtensionIn(t, dir, 7, func(proto.Request) proto.Reply { return proto.Reply{Error: "new host"} })
	defer newDone()

	hosts, _ := client.Hosts(dir)
	if len(hosts) != 1 || hosts[0].Path != newSocket {
		t.Fatalf("while both run, Hosts = %+v, want only %s", hosts, newSocket)
	}
	oldDone()
	if names := sockets(dir); len(names) != 1 || !names[filepath.Base(newSocket)] {
		t.Fatalf("after the old host exited, sockets = %v", names)
	}
	if err := client.Call(newSocket, proto.Request{Cmd: "list"}, nil); err == nil || err.Error() != "new host" {
		t.Fatalf("Call = %v, want the new host's answer", err)
	}
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

func TestGroupsComesBack(t *testing.T) {
	socket, sent, done := recordingExtension(t, []proto.Group{{ID: 11, Window: 2, Title: "chromectl", LastAccessed: 1700000000000.5}})
	defer done()

	var groups []proto.Group
	if err := client.Call(socket, proto.Request{Cmd: "groups"}, &groups); err != nil {
		t.Fatal(err)
	}
	if got := <-sent; got["cmd"] != "groups" {
		t.Fatalf("the extension received %v", got)
	}
	if len(groups) != 1 || groups[0].Title != "chromectl" || groups[0].LastAccessed != 1700000000000.5 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestActivateGroupSendsTitleUrlAndWindow(t *testing.T) {
	socket, sent, done := recordingExtension(t, proto.Raised{ID: 5, Window: 2, Created: true, Title: "github.com/hk9890/chromectl"})
	defer done()

	var raised proto.Raised
	req := proto.Request{Cmd: "activate-group", Title: "chromectl", URL: "https://github.com/hk9890/chromectl", Window: 2}
	if err := client.Call(socket, req, &raised); err != nil {
		t.Fatal(err)
	}
	if got := <-sent; got["cmd"] != "activate-group" || got["title"] != "chromectl" || got["url"] != "https://github.com/hk9890/chromectl" || got["window"] != float64(2) {
		t.Fatalf("the extension received %v", got)
	}
	if !raised.Created || raised.ID != 5 || raised.Title != "github.com/hk9890/chromectl" {
		t.Fatalf("raised = %+v", raised)
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
	socket, done := rawExtensionIn(t, socketDir(t), 1, identifying(profile, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		switch req.Tab {
		case 1:
			return native.Write(toHost, map[string]any{"seq": req.Seq, "error": 5})
		case 2:
			return native.Write(toHost, map[string]any{"seq": fmt.Sprint(req.Seq), "result": 1})
		}
		return native.Write(toHost, map[string]any{"seq": req.Seq, "result": map[string]int{"id": req.Tab}})
	}))
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
	answered := make(chan struct{})
	socket, done := rawExtensionIn(t, socketDir(t), 1, identifying(profile, func(_ json.RawMessage, req proto.Request, toHost io.Writer) error {
		defer close(answered)
		// Larger than a socket buffer, so the host's write blocks. native.Write
		// refuses it: the limit is for the host's direction only.
		frame, _ := json.Marshal(map[string]any{"seq": req.Seq, "result": strings.Repeat("x", 4<<20)})
		if err := binary.Write(toHost, binary.NativeEndian, uint32(len(frame))); err != nil {
			return err
		}
		_, err := toHost.Write(frame)
		return err
	}))

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

// A call that is pending when Chrome closes the pipe hears so before Run
// returns, and so before the host process exits: the host closes its
// connection without a reply, because Chrome may or may not have acted.
func TestAPendingCallGetsNoReplyWhenChromeGoesAway(t *testing.T) {
	asked := make(chan struct{})
	socket, done := rawExtensionIn(t, socketDir(t), 1, identifying(profile, func(json.RawMessage, proto.Request, io.Writer) error {
		close(asked)
		return nil
	}))

	errs := make(chan error, 1)
	go func() { errs <- client.Call(socket, proto.Request{Cmd: "list"}, nil) }()
	<-asked
	done()
	err := <-errs
	var noReply *client.NoReplyError
	if !errors.As(err, &noReply) || !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want a NoReplyError for a closed connection", err)
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
