package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/hk9890/chromectl/internal/proto"
)

// isolate points HOME and the XDG directories at fresh temporary directories,
// with a google-chrome config directory, so install cannot reach the user's
// real Chrome.
func isolate(t *testing.T) (home, data, cfg string) {
	t.Helper()
	home, data, cfg = t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.Mkdir(filepath.Join(cfg, "google-chrome"), 0o755); err != nil {
		t.Fatal(err)
	}
	return home, data, cfg
}

// manifestPath returns the host path that the google-chrome manifest under cfg names.
func manifestPath(t *testing.T, cfg string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(cfg, "google-chrome", "NativeMessagingHosts", proto.HostName+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Path string }
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m.Path
}

func TestInstallCopiesBinaryAndExtensionAndPointsManifestAtTheCopy(t *testing.T) {
	_, data, cfg := isolate(t)

	if err := install(nil); err != nil {
		t.Fatal(err)
	}

	host := filepath.Join(data, "chromectl", "chromectl")
	if fi, err := os.Stat(host); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("no executable copy at %s: %v", host, err)
	}
	for _, name := range []string{"manifest.json", "background.js"} {
		want, err := os.ReadFile(filepath.Join("..", "..", "extension", name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(data, "chromectl", "extension", name))
		if err != nil || string(got) != string(want) {
			t.Errorf("extension/%s was not written as embedded: %v", name, err)
		}
	}
	if got := manifestPath(t, cfg); got != host {
		t.Errorf("manifest path = %q, want %q", got, host)
	}
}

func TestInstallRunTwiceReplacesTheCopy(t *testing.T) {
	_, data, _ := isolate(t)
	if err := install(nil); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(data, "chromectl")
	host, stale := filepath.Join(dir, "chromectl"), filepath.Join(dir, "extension", "stale.js")
	for _, f := range []string{host, stale} {
		if err := os.WriteFile(f, []byte("stale"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := install(nil); err != nil {
		t.Fatal(err)
	}

	if b, err := os.ReadFile(host); err != nil || string(b) == "stale" {
		t.Errorf("the second install did not replace %s: %v", host, err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file the embedded extension does not have survived the second install: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestInstallWithoutAChromeConfigDirectoryWritesNothing(t *testing.T) {
	_, data, cfg := isolate(t)
	if err := os.Remove(filepath.Join(cfg, "google-chrome")); err != nil {
		t.Fatal(err)
	}

	if err := install(nil); err == nil {
		t.Fatal("install succeeded without a Chrome-family config directory")
	}

	if _, err := os.Stat(filepath.Join(data, "chromectl")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("install wrote into %s before it failed: %v", data, err)
	}
}

func TestInstallIgnoresARelativeXDGDataHome(t *testing.T) {
	home, _, cfg := isolate(t)
	t.Chdir(t.TempDir())
	t.Setenv("XDG_DATA_HOME", "relative")

	if err := install(nil); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, ".local", "share", "chromectl", "chromectl")
	if got := manifestPath(t, cfg); got != want {
		t.Errorf("manifest path = %q, want %q", got, want)
	}
}
