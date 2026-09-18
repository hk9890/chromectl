# chromectl

Control Chrome tabs from the command line: list, activate, open, close.

Chrome does not let an outside process see its tabs. An extension can, so
chromectl is two halves: a tiny extension with the `tabs` permission, and a Go
binary. Chrome starts `chromectl host` for the extension through native
messaging and keeps it alive as long as Chrome runs. Every other subcommand is
a one-shot client of that host over a Unix socket. There is no daemon to start
or enable.

```
chromectl list                 # id  window  active  title  url
chromectl activate <tab-id>    # activate the tab and focus its window
chromectl open <url> [window]  # open a new tab, in that window or the last focused one
chromectl close <tab-id>
```

Tab ids are Chrome's own and reset when Chrome restarts. Find a tab by its URL
with `chromectl list`, then act on its id.

## Install

Requires Go 1.27, or [mise](https://mise.jdx.dev), which installs it.

```
mise run install     # builds ./bin/chromectl and writes the native messaging manifest
```

Then load the `extension/` directory as an unpacked extension at
`chrome://extensions` with Developer mode on. Its id is
`mpnpeobidlmllklmdpnkhhhblkbhpmcp`, fixed by the `key` in its manifest, so the
native messaging manifest already allows it.

`chromectl install` writes `com.hk9890.chromectl.json` into
`NativeMessagingHosts/` of every Chrome-family config directory it finds under
`~/.config`: Chrome, Chrome beta and unstable, Chromium, Brave. Rerun it after
moving the binary; the manifest holds its absolute path.

Once the extension is loaded, `chromectl list` works. If it says no host is
listening, open the extension's service worker console at
`chrome://extensions` for the reason.

## How it works

```
chromectl list ──unix socket──▶ chromectl host ◀──stdin/stdout──▶ extension ──▶ chrome.tabs
                 $XDG_RUNTIME_DIR/chromectl.sock   native messaging
```

- `internal/native` frames messages as Chrome expects: 4-byte length, then JSON.
- `internal/hostd` is the host: it numbers each request, writes it to Chrome,
  and returns the reply with that number to the socket connection that asked.
- `internal/client` is the socket side the CLI uses.
- `extension/background.js` runs the command against the tabs API and echoes
  the number back.

The host exits when Chrome closes the pipe and removes its socket on the way
out. The extension reconnects after two seconds if the host goes away.

## Test

```
mise run test
```

The tests run the host against a fake extension on in-memory pipes. No Chrome
is needed.
