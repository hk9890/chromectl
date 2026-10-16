// chromectl controls Chrome tabs from the command line through a small
// extension. Chrome starts `chromectl host` for the extension; every other
// subcommand talks to that host over a Unix socket.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/hostd"
	"github.com/hk9890/chromectl/internal/proto"
)

const usage = `usage: chromectl <command> [args]

  list                 every tab: id, window, active, title, url (tab-separated)
  activate <tab-id>    activate the tab and focus its window
  open <url> [window]  open url in a new tab, in the given window or the last focused one
  close <tab-id>       close the tab
  install              register this binary as the extension's native messaging host
  host                 the native messaging host; Chrome starts this, not you
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
		err = list()
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

func list() error {
	var tabs []proto.Tab
	if err := client.Call(client.SocketPath(), proto.Request{Cmd: "list"}, &tabs); err != nil {
		return err
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
	fmt.Printf("%d\t%d\n", opened.ID, opened.Window)
	return nil
}

// install writes the native messaging manifest into every Chrome-family
// config directory that exists, pointing at this binary.
func install() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	manifest, _ := json.MarshalIndent(map[string]any{
		"name":            proto.HostName,
		"description":     "chromectl native messaging host",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + proto.ExtensionID + "/"},
	}, "", "  ")

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	var written []string
	for _, browser := range []string{"google-chrome", "google-chrome-beta", "google-chrome-unstable", "chromium", "BraveSoftware/Brave-Browser"} {
		dir := filepath.Join(cfg, browser)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		path := filepath.Join(dir, "NativeMessagingHosts", proto.HostName+".json")
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
	fmt.Printf("wrote:\n  %s\n\nnow load %s as an unpacked extension at chrome://extensions (id %s)\n",
		strings.Join(written, "\n  "), extensionDir(), proto.ExtensionID)
	return nil
}

func extensionDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "extension/"
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "extension")
}
