package main

import (
	"bytes"
	"encoding/json"
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

// starts gives each host a later start than the one before, as two host
// processes have.
var starts atomic.Int64

// runtimeDir points XDG_RUNTIME_DIR at a fresh directory short enough for a
// host's socket path, and returns the socket directory in it.
func runtimeDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("XDG_RUNTIME_DIR", dir)
	return filepath.Join(dir, "chromectl")
}

// startHost runs a host for profile whose extension holds tabs, answers
// every other command with the tab or window it was given, and sends each
// request to got.
func startHost(t *testing.T, profile, browser string, browserPID int, tabs []proto.Tab, got chan<- proto.Request) {
	t.Helper()
	sockets := runtimeSockets()
	toHost, fromExt := io.Pipe()
	fromHost, toExt := io.Pipe()
	ran := make(chan error, 1)
	go func() {
		ran <- hostd.Run(toHost, toExt, sockets, hostd.Self{Browser: browser, BrowserPID: browserPID, Start: starts.Add(1)})
	}()
	go func() {
		for {
			var req proto.Request
			if err := native.Read(fromHost, &req); err != nil {
				return
			}
			var result any
			switch {
			case req.Cmd == "identify" && profile == "":
				// An extension older than the binary.
				_ = native.Write(fromExt, map[string]any{"seq": req.Seq, "error": "unknown command identify"})
				continue
			case req.Cmd == "identify":
				result = proto.Identity{Profile: profile}
			case req.Cmd == "list":
				result = tabs
			default:
				got <- req
				result = proto.Tab{ID: 900 + req.Tab, Window: max(req.Window, 1)}
			}
			if err := native.Write(fromExt, map[string]any{"seq": req.Seq, "result": result}); err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = fromExt.Close()
		_ = toExt.Close()
		<-ran
	})
	for start := time.Now(); ; time.Sleep(time.Millisecond) {
		hs, _ := client.Hosts(sockets)
		for _, h := range hs {
			if h.Profile == profile && h.BrowserPID == browserPID {
				return
			}
		}
		if time.Since(start) > 2*time.Second {
			t.Fatal("host never put its socket in place")
		}
	}
}

func runtimeSockets() string {
	dir, _ := client.SocketDir()
	return dir
}

func TestParseIDReadsTheFormListPrints(t *testing.T) {
	if profile, id, err := parseID("tab", "3f9a12bc:812"); err != nil || profile != "3f9a12bc" || id != 812 {
		t.Fatalf("parseID = %q, %d, %v", profile, id, err)
	}
	for _, arg := range []string{"812", "3f9a12bc:0", "3f9a:812", "3F9A12BC:812", "3f9a12bc:", "3f9a12bc:-1", "3f9a12bc:8x"} {
		if _, _, err := parseID("tab", arg); err == nil || err.Error() != `tab id "`+arg+`" is not of the form <profile>:<number> that list prints` {
			t.Errorf("parseID(%q): err = %v", arg, err)
		}
	}
}

func TestListShowsEveryProfile(t *testing.T) {
	runtimeDir(t)
	startHost(t, "aaaa0001", "chrome", 1, []proto.Tab{{ID: 4, Window: 1, Active: true, Title: "PRs", URL: "https://github.com/pulls"}}, nil)
	startHost(t, "bbbb0002", "brave", 2, []proto.Tab{{ID: 4, Window: 3, Title: "News", URL: "https://orf.at"}}, nil)

	var out bytes.Buffer
	if err := list(&out, nil); err != nil {
		t.Fatal(err)
	}
	want := "aaaa0001:4\taaaa0001:1\t*\tchrome\tPRs\thttps://github.com/pulls\n" +
		"bbbb0002:4\tbbbb0002:3\t-\tbrave\tNews\thttps://orf.at\n"
	if out.String() != want {
		t.Fatalf("list =\n%s\nwant\n%s", out.String(), want)
	}

	out.Reset()
	if err := list(&out, []string{"--json"}); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("list --json = %s, %v", out.String(), err)
	}
	if r := rows[1]; r["id"] != "bbbb0002:4" || r["window"] != "bbbb0002:3" || r["browser"] != "brave" || r["profile"] != "bbbb0002" {
		t.Fatalf("row = %v", r)
	}
}

func TestActivateGoesToTheProfileItNames(t *testing.T) {
	runtimeDir(t)
	a, b := make(chan proto.Request, 1), make(chan proto.Request, 1)
	startHost(t, "aaaa0001", "chrome", 1, nil, a)
	startHost(t, "bbbb0002", "chrome", 1, nil, b)

	if err := tabCommand("activate", []string{"bbbb0002:4"}); err != nil {
		t.Fatal(err)
	}
	if req := <-b; req.Cmd != "activate" || req.Tab != 4 {
		t.Fatalf("profile bbbb0002 got %+v", req)
	}
	if len(a) != 0 {
		t.Fatalf("profile aaaa0001 got %+v", <-a)
	}
	if err := tabCommand("close", []string{"cccc0003:4"}); err == nil || !strings.Contains(err.Error(), "no chromectl host for profile cccc0003") {
		t.Fatalf("close in a profile that does not run: err = %v", err)
	}
}

func TestOpenWithOneProfileNeedsNoWindow(t *testing.T) {
	runtimeDir(t)
	got := make(chan proto.Request, 1)
	startHost(t, "aaaa0001", "chrome", 1, nil, got)

	var out bytes.Buffer
	if err := open(&out, []string{"https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if req := <-got; req.URL != "https://example.com" || req.Window != 0 {
		t.Fatalf("the extension got %+v", req)
	}
	if out.String() != "aaaa0001:900\taaaa0001:1\n" {
		t.Fatalf("open printed %q", out.String())
	}
}

func TestOpenWithSeveralProfilesNeedsAWindow(t *testing.T) {
	runtimeDir(t)
	got := make(chan proto.Request, 1)
	startHost(t, "aaaa0001", "chrome", 1, []proto.Tab{{ID: 4, Window: 1}}, nil)
	startHost(t, "bbbb0002", "brave", 2, []proto.Tab{{ID: 5, Window: 3}}, got)

	startHost(t, "cccc0003", "chrome", 1, nil, nil)
	err := open(io.Discard, []string{"https://example.com"})
	want := "several Chrome profiles run, so open needs a window id, one of aaaa0001:1 (chrome), bbbb0002:3 (brave)" +
		"; without a window, and so out of reach of open: cccc0003 (chrome)"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v", err)
	}
	var out bytes.Buffer
	if err := open(&out, []string{"--json", "https://example.com", "bbbb0002:3"}); err != nil {
		t.Fatal(err)
	}
	if req := <-got; req.Window != 3 {
		t.Fatalf("the extension got %+v", req)
	}
	if out.String() != `{"id":"bbbb0002:900","window":"bbbb0002:3"}`+"\n" {
		t.Fatalf("open --json printed %q", out.String())
	}
}

// A socket that refuses the connection is a dead host's: list skips it and
// removes it.
func TestListSkipsADeadHost(t *testing.T) {
	sockets := runtimeDir(t)
	startHost(t, "aaaa0001", "chrome", 1, []proto.Tab{{ID: 4, Window: 1}}, nil)
	dead := filepath.Join(sockets, client.SocketName("bbbb0002", "chrome", 1, 1))
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: dead, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	_ = ln.Close()

	var out bytes.Buffer
	if err := list(&out, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "aaaa0001:4\t") || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("list = %q", out.String())
	}
	if _, err := os.Stat(dead); !os.IsNotExist(err) {
		t.Fatalf("the dead socket is still there: %v", err)
	}
}

// A host whose extension is older than the binary cannot list; list prints
// the others and says the list is incomplete.
func TestListWithAStaleHostIsIncomplete(t *testing.T) {
	runtimeDir(t)
	startHost(t, "aaaa0001", "chrome", 1, []proto.Tab{{ID: 4, Window: 1}}, nil)
	startHost(t, "", "brave", 2, nil, nil)

	var out bytes.Buffer
	err := list(&out, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "the list is incomplete") {
		t.Fatalf("err = %v", err)
	}
	if !strings.HasPrefix(out.String(), "aaaa0001:4\t") {
		t.Fatalf("list = %q", out.String())
	}
	// The stale host cannot say its profile, so it may be the one asked for.
	if err := tabCommand("activate", []string{"cccc0003:4"}); err == nil || err.Error() != proto.StaleExtension {
		t.Fatalf("activate in a profile that has no current host: err = %v", err)
	}
}

func TestListWithoutAnyHostSaysSo(t *testing.T) {
	runtimeDir(t)
	if err := list(io.Discard, nil); err != client.ErrNoHost {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
}
