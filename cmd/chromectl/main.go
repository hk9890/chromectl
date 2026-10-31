// chromectl controls Chrome tabs from the command line through a small
// extension. Chrome starts `chromectl host` for the extension; every other
// subcommand talks to that host over a Unix socket.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/hk9890/chromectl"
	"github.com/hk9890/chromectl/internal/build"
	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/hostd"
	"github.com/hk9890/chromectl/internal/proto"
)

const usage = `usage: chromectl <command> [args]

  list [--json]                every tab of every profile: id, window, group,
                               active, browser, title, url (tab-separated, or a
                               JSON array)
  groups [--json]              every open tab group: id, window, title
  activate <tab-id>            activate the tab and focus its window
  activate-group [--json] <title> <url> [window]
                               raise the tab group titled title: activate its tab
                               at url, else under url, else its last used tab.
                               Without such a group, open url in a new group with
                               that title, in the given window or the last
                               focused one. Prints the tab's id, its window,
                               whether the group was created, and the tab's title
  open [--json] <url> [window] open url in a new tab, in the given window or the
                               last focused one; prints its id and window
  close <tab-id>               close the tab
  install                      copy this binary and the extension to ~/.local/share/chromectl
                               and register that copy as the native messaging host
  version                      print the version
  host                         the native messaging host; Chrome starts this, not you

Tab, window and group ids are <profile>:<number>, as list prints them. With
several profiles, open, and activate-group where it creates a group, need a
window id.

Every command that calls Chrome takes --timeout <duration> (default 5s) before
its arguments. Exit codes: 1 an error from Chrome or chromectl, 2 bad usage,
4 no reply (the command may or may not have taken effect), 5 no host.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	// Chrome starts the native host as `chromectl <origin> --parent-window=0`;
	// the manifest names a binary and cannot add a `host` argument.
	if strings.HasPrefix(cmd, "chrome-extension://") {
		cmd = "host"
	}
	var err error
	switch cmd {
	case "list":
		err = list(os.Stdout, args)
	case "activate":
		err = tabCommand("activate", args)
	case "close":
		err = tabCommand("close", args)
	case "groups":
		err = groups(os.Stdout, args)
	case "activate-group":
		err = activateGroup(os.Stdout, args)
	case "open":
		err = open(os.Stdout, args)
	case "install":
		err = install(args)
	case "host":
		err = host()
	case "version", "--version":
		fmt.Printf("chromectl %s (%s, %s)\n", build.Version, build.Commit, build.Date)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "chromectl: unknown command %q\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "chromectl:", err)
		os.Exit(exitCode(err))
	}
}

// host runs the native messaging host for the browser that started it, its
// parent.
func host() error {
	// Without this, a write to Chrome's closed stdout kills the process
	// before Run can tell its callers that Chrome went away.
	signal.Ignore(syscall.SIGPIPE)
	dir, err := client.SocketDir()
	if err != nil {
		return err
	}
	start, err := hostd.StartTime()
	if err != nil {
		return err
	}
	browser := os.Getppid()
	return hostd.Run(os.Stdin, os.Stdout, dir, hostd.Self{Browser: hostd.Browser(browser), BrowserPID: browser, Start: start})
}

// install copies this binary and the extension to one directory whose path
// no upgrade changes, then writes the native messaging manifest into every
// Chrome-family config directory that exists, pointing at that copy.
func install(args []string) error {
	if len(args) != 0 {
		return badUsage("install takes no arguments")
	}
	data, err := xdgDir("XDG_DATA_HOME", ".local/share")
	if err != nil {
		return err
	}
	cfg, err := xdgDir("XDG_CONFIG_HOME", ".config")
	if err != nil {
		return err
	}
	var browserDirs []string
	for _, browser := range []string{"google-chrome", "google-chrome-beta", "google-chrome-unstable", "chromium", "BraveSoftware/Brave-Browser"} {
		dir := filepath.Join(cfg, browser)
		if _, err := os.Stat(dir); err == nil {
			browserDirs = append(browserDirs, dir)
		}
	}
	if len(browserDirs) == 0 {
		return fmt.Errorf("no Chrome-family config directory found under %s", cfg)
	}

	dir := filepath.Join(data, "chromectl")
	host := filepath.Join(dir, "chromectl")
	if err := copySelf(host); err != nil {
		return err
	}
	extDir := filepath.Join(dir, "extension")
	if err := writeExtension(extDir); err != nil {
		return err
	}

	manifest, _ := json.MarshalIndent(map[string]any{
		"name":            proto.HostName,
		"description":     "chromectl native messaging host",
		"path":            host,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + proto.ExtensionID + "/"},
	}, "", "  ")
	var written []string
	for _, browserDir := range browserDirs {
		path := filepath.Join(browserDir, "NativeMessagingHosts", proto.HostName+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, manifest, 0o644); err != nil {
			return err
		}
		written = append(written, path)
	}
	fmt.Printf("installed %s and %s\nwrote:\n  %s\n\n"+
		"first install: load %s as an unpacked extension at chrome://extensions (id %s)\n"+
		"upgrade: click reload on the chromectl card at chrome://extensions\n",
		host, extDir, strings.Join(written, "\n  "), extDir, proto.ExtensionID)
	return nil
}

// xdgDir reads an XDG base directory variable, or returns fallback under the
// home directory. The spec makes a relative value invalid, and Chrome refuses
// a relative host path in the manifest.
func xdgDir(name, fallback string) (string, error) {
	if dir := os.Getenv(name); filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, fallback), nil
}

// copySelf puts a copy of the running binary at dst. It renames a temporary
// file over dst, because a host Chrome runs from dst cannot be written in place.
func copySelf(dst string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".chromectl-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// writeExtension replaces dir with the embedded extension, so no file a
// previous version had is left behind. It writes a sibling directory first
// and renames it into place, because Chrome loads the extension from dir and
// must not find it half written. Between the two renames dir is absent for
// an instant; where the second rename fails, the old copy goes back.
func writeExtension(dir string) error {
	ext, err := fs.Sub(chromectl.Extension, "extension")
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".extension-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := os.CopyFS(tmp, ext); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	old := tmp + ".old"
	if err := os.Rename(dir, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.Rename(old, dir)
		return err
	}
	// The new copy is in place; a leftover old copy is not worth failing
	// install before it writes the manifests.
	_ = os.RemoveAll(old)
	return nil
}
