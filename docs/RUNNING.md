# Running

Two substrates. The first is headless and safe; the second is the user's live
browser.

## Headless: the fake Chrome

```bash
mise run drive
```

`scripts/fakechrome.py` starts `./bin/chromectl` on pipes with the arguments Chrome passes, plays the
extension, and runs `list`, `open` and `activate` through the CLI. It prints
every request the host forwarded and every CLI result, then closes the pipe and
reports the host's exit and that the socket is gone. Edit its `extension()`
function to script a different answer.

To drive by hand instead: start the host with `XDG_RUNTIME_DIR` set to a fresh
directory so it cannot take the socket of a running Chrome's host, and use the
same variable for the CLI.

## Live: the user's Chrome

The socket at `$XDG_RUNTIME_DIR/chromectl.sock` belongs to the host the user's
Chrome started. A CLI call against it acts on the user's real tabs. `list` is
harmless; `activate` and `open` change what the user sees; `close` is
destructive. Confirm before any of the last three.

Run `mise run chromectl -- <args>` to rebuild and call the CLI in one step.

Setup, only when asked:

1. `mise run install` copies `./bin/chromectl` and the extension to
   `~/.local/share/chromectl/` and writes the manifest naming that copy.
2. The user loads `~/.local/share/chromectl/extension` unpacked at
   `chrome://extensions`, once. An agent cannot do this step.

To put a change in front of the live Chrome, rerun step 1, then the user clicks
reload on the chromectl card. Both are needed for any change: Chrome runs the
copy, not `./bin/chromectl`, and the host it started keeps running the old copy
until the reload restarts it.

## Where output lands

- The host's stderr is Chrome's stderr. Start Chrome from a terminal to read
  it. There is no log file.
- The extension's `console.log` is in its service worker console at
  `chrome://extensions`, behind the card's "service worker" link. A host that
  fails to start shows there as the reason, including a manifest that names a
  binary that is not there.
