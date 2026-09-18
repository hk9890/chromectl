package hostd_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
	toHost, fromExt := io.Pipe()
	fromHost, toExt := io.Pipe()
	socket = filepath.Join(t.TempDir(), "s.sock")

	ran := make(chan error, 1)
	go func() { ran <- hostd.Run(toHost, toExt, socket) }()
	go func() {
		for {
			var req proto.Request
			if err := native.Read(fromHost, &req); err != nil {
				return
			}
			r := answer(req)
			r.Seq = req.Seq
			if err := native.Write(fromExt, r); err != nil {
				return
			}
		}
	}()
	for start := time.Now(); ; time.Sleep(time.Millisecond) {
		if _, err := os.Stat(socket); err == nil {
			break
		}
		if time.Since(start) > 2*time.Second {
			t.Fatal("host never bound its socket")
		}
	}
	return socket, func() {
		fromExt.Close()
		if err := <-ran; err != nil {
			t.Errorf("Run returned %v after Chrome closed the pipe", err)
		}
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
				err = &mismatch{want: id, got: got.ID}
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

type mismatch struct{ want, got int }

func (m *mismatch) Error() string { return "asked for tab " + itoa(m.want) + ", got " + itoa(m.got) }
func itoa(i int) string           { b, _ := json.Marshal(i); return string(b) }

func TestWithoutAHostTheClientSaysSo(t *testing.T) {
	err := client.Call(filepath.Join(t.TempDir(), "absent.sock"), proto.Request{Cmd: "list"}, nil)
	if err != client.ErrNoHost {
		t.Fatalf("err = %v, want ErrNoHost", err)
	}
}
