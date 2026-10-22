// chromectl controls Chrome tabs from the command line through a small
// extension. Chrome starts `chromectl host` for the extension; every other
// subcommand talks to that host over a Unix socket.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hk9890/chromectl"
	"github.com/hk9890/chromectl/internal/build"
	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/hostd"
	"github.com/hk9890/chromectl/internal/proto"
)

const usage = `usage: chromectl <command> [args]

  list [--json]                every tab: id, window, active, title, url
                               (tab-separated, or a JSON array)
  activate <tab-id>            activate the tab and focus its window
  open [--json] <url> [window] open url in a new tab, in the given window or the
                               last focused one; prints its id and window
  close <tab-id>               close the tab
  install                      copy this binary and the extension to ~/.local/share/chromectl
                               and register that copy as the native messaging host
  version                      print the version
  host                         the native messaging host; Chrome starts this, not you
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
		err = list(args)
	case "activate":
		err = tabCommand("activate", args)
	case "close":
		err = tabCommand("close", args)
	case "open":
		err = open(args)
	case "install":
		err = install()
	case "host":
		err = hostd.Run(os.Stdin, os.Stdout, client.SocketPath())
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
		os.Exit(1)
	}
}

func list(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON array")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	tabs := []proto.Tab{}
	if err := client.Call(client.SocketPath(), proto.Request{Cmd: "list"}, &tabs); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(tabs)
	}
	for _, t := range tabs {
		active := "-"
		if t.Active {
			active = "*"
		}
		fmt.Printf("%d\t%d\t%s\t%s\t%s\n", t.ID, t.Window, active, t.Title, t.URL)
	}
	return nil
}

func tabCommand(cmd string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%s takes exactly one tab id", cmd)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("tab id %q is not a number", args[0])
	}
	return client.Call(client.SocketPath(), proto.Request{Cmd: cmd, Tab: id}, nil)
}

func open(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print a JSON object")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 || fs.NArg() > 2 {
		return fmt.Errorf("open takes a url and an optional window id")
	}
	req := proto.Request{Cmd: "open", URL: fs.Arg(0)}
	if fs.NArg() == 2 {
		w, err := strconv.Atoi(fs.Arg(1))
		if err != nil {
			return fmt.Errorf("window id %q is not a number", fs.Arg(1))
		}
		req.Window = w
	}
	var opened proto.Tab
	if err := client.Call(client.SocketPath(), req, &opened); err != nil {
		return err
	}
	if *asJSON {
		return printJSON(struct {
			ID     int `json:"id"`
			Window int `json:"window"`
		}{opened.ID, opened.Window})
	}
	fmt.Printf("%d\t%d\n", opened.ID, opened.Window)
	return nil
}

// printJSON writes v as one line of JSON. URLs keep their & < > as is.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// install copies this binary and the extension to one directory whose path
// no upgrade changes, then writes the native messaging manifest into every
// Chrome-family config directory that exists, pointing at that copy.
func install() error {
	data, cfg := xdgDir("XDG_DATA_HOME"), xdgDir("XDG_CONFIG_HOME")
	if data == "" || cfg == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if data == "" {
			data = filepath.Join(home, ".local", "share")
		}
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
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
	for _, browser := range []string{"google-chrome", "google-chrome-beta", "google-chrome-unstable", "chromium", "BraveSoftware/Brave-Browser"} {
		browserDir := filepath.Join(cfg, browser)
		if _, err := os.Stat(browserDir); err != nil {
			continue
		}
		path := filepath.Join(browserDir, "NativeMessagingHosts", proto.HostName+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, manifest, 0o644); err != nil {
			return err
		}
		written = append(written, path)
	}
	if len(written) == 0 {
		return fmt.Errorf("no Chrome-family config directory found under %s", cfg)
	}
	fmt.Printf("installed %s and %s\nwrote:\n  %s\n\n"+
		"first install: load %s as an unpacked extension at chrome://extensions (id %s)\n"+
		"upgrade: click reload on the chromectl card at chrome://extensions\n",
		host, extDir, strings.Join(written, "\n  "), extDir, proto.ExtensionID)
	return nil
}

// xdgDir reads an XDG base directory variable. The spec makes a relative
// value invalid, and Chrome refuses a relative host path in the manifest.
func xdgDir(name string) string {
	if dir := os.Getenv(name); filepath.IsAbs(dir) {
		return dir
	}
	return ""
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
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// writeExtension writes the embedded extension into dir.
func writeExtension(dir string) error {
	files, err := fs.Glob(chromectl.Extension, "extension/*")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, name := range files {
		b, err := chromectl.Extension.ReadFile(name)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(name)), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}
