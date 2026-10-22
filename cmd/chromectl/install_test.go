package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hk9890/chromectl/internal/proto"
)

func TestInstallCopiesBinaryAndExtensionAndPointsManifestAtTheCopy(t *testing.T) {
	data, cfg := t.TempDir(), t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.Mkdir(filepath.Join(cfg, "google-chrome"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := install(); err != nil {
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
	raw, err := os.ReadFile(filepath.Join(cfg, "google-chrome", "NativeMessagingHosts", proto.HostName+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Path string }
	if err := json.Unmarshal(raw, &m); err != nil || m.Path != host {
		t.Errorf("manifest path = %q, want %q (%v)", m.Path, host, err)
	}
}

func TestInstallRunTwiceReplacesTheCopy(t *testing.T) {
	data, cfg := t.TempDir(), t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.Mkdir(filepath.Join(cfg, "chromium"), 0o755); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := install(); err != nil {
			t.Fatal(err)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(data, "chromectl", ".chromectl-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestInstallIgnoresARelativeXDGDataHome(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.Mkdir(filepath.Join(cfg, "google-chrome"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := install(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg, "google-chrome", "NativeMessagingHosts", proto.HostName+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct{ Path string }
	want := filepath.Join(home, ".local", "share", "chromectl", "chromectl")
	if err := json.Unmarshal(raw, &m); err != nil || m.Path != want {
		t.Errorf("manifest path = %q, want %q (%v)", m.Path, want, err)
	}
}
