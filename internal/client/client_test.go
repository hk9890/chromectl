package client

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hk9890/chromectl/internal/proto"
)

// socketDir is a fresh directory short enough for a Unix socket path.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHostsReadsEachSocketName(t *testing.T) {
	dir := socketDir(t)
	touch(t, dir,
		SocketName("0123abcd", "chrome", 10, 100),
		SocketName("", "brave", 20, 200),
		".0123abcd.chrome.10.300.sock", // a host not yet listening
		"chromectl.sock",               // the single socket of earlier releases
		"notes.txt",
	)
	hosts, err := Hosts(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Host{
		{Profile: "0123abcd", Browser: "chrome", BrowserPID: 10, Start: 100, Path: filepath.Join(dir, "0123abcd.chrome.10.100.sock")},
		{Browser: "brave", BrowserPID: 20, Start: 200, Path: filepath.Join(dir, "stale.brave.20.200.sock")},
	}
	if fmt.Sprint(hosts) != fmt.Sprint(want) {
		t.Fatalf("Hosts =\n%+v\nwant\n%+v", hosts, want)
	}
}

// Two hosts of one profile in one browser are a reload; the later one is the
// host.
func TestHostsKeepsTheLaterHostOfAReload(t *testing.T) {
	dir := socketDir(t)
	touch(t, dir, SocketName("0123abcd", "chrome", 10, 900), SocketName("0123abcd", "chrome", 10, 100))
	hosts, _ := Hosts(dir)
	if len(hosts) != 1 || hosts[0].Start != 900 || hosts[0].Conflict != "" {
		t.Fatalf("Hosts = %+v", hosts)
	}
}

// One profile under two browsers is a copied profile directory; its ids
// cannot name one tab.
func TestHostsMarksAProfileInTwoBrowsers(t *testing.T) {
	dir := socketDir(t)
	touch(t, dir, SocketName("0123abcd", "chrome", 10, 100), SocketName("0123abcd", "chromium", 20, 100))
	hosts, _ := Hosts(dir)
	if len(hosts) != 2 {
		t.Fatalf("Hosts = %+v", hosts)
	}
	for _, h := range hosts {
		if !strings.Contains(h.Conflict, "chrome (pid 10) and chromium (pid 20)") {
			t.Errorf("Conflict = %q", h.Conflict)
		}
	}
}

func TestHostsWithoutTheDirectoryIsNone(t *testing.T) {
	hosts, err := Hosts(filepath.Join(socketDir(t), "absent"))
	if err != nil || len(hosts) != 0 {
		t.Fatalf("Hosts = %+v, %v", hosts, err)
	}
}

// A visible socket always had a listener, so a refused one is a dead host's.
func TestCallRemovesARefusedSocket(t *testing.T) {
	path := filepath.Join(socketDir(t), SocketName("0123abcd", "chrome", 10, 100))
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	_ = ln.Close()

	if err := Call(path, proto.Request{Cmd: "list"}, nil); err != ErrNoHost {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the dead socket is still there: %v", err)
	}
}

func TestCallWithoutAHostSaysSo(t *testing.T) {
	if err := Call(filepath.Join(socketDir(t), "absent.sock"), proto.Request{Cmd: "list"}, nil); err != ErrNoHost {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
}

func TestCallGivesUpOnAHostThatDoesNotAnswer(t *testing.T) {
	callTimeout = 50 * time.Millisecond
	t.Cleanup(func() { callTimeout = 10 * time.Second })
	path := filepath.Join(socketDir(t), "s.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			defer func() { _ = conn.Close() }()
			time.Sleep(time.Second)
		}
	}()

	err = Call(path, proto.Request{Cmd: "list"}, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "no reply in 50ms") {
		t.Fatalf("err = %v", err)
	}
}

func TestSocketDirFollowsXDGRuntimeDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4711")
	if dir, err := SocketDir(); err != nil || dir != "/run/user/4711/chromectl" {
		t.Fatalf("SocketDir = %q, %v", dir, err)
	}
}

// Under cron XDG_RUNTIME_DIR is unset; the hosts Chrome started are still in
// the user's runtime directory.
func TestSocketDirWithoutXDGRuntimeDirIsTheUsersRunDir(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	run := fmt.Sprintf("/run/user/%d", os.Getuid())
	dir, err := SocketDir()
	if _, statErr := os.Stat(run); statErr != nil {
		if err == nil {
			t.Fatalf("SocketDir = %q without %s, want an error", dir, run)
		}
		return
	}
	if err != nil || dir != run+"/chromectl" {
		t.Fatalf("SocketDir = %q, %v", dir, err)
	}
}
