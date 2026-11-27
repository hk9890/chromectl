# Running

Two substrates. The first is headless and safe; the second is the user's live
browser.

To drive chromectl beyond `mise run drive`, call the Skill tool with `run` for
the generic flow. This file is the local delta, and wins where the two
disagree.

## Headless: the fake Chrome

```bash
mise run drive
```

`scripts/fakechrome.py` starts `./bin/chromectl` on pipes as Chrome does,
plays the extension, and runs its `cases` through the CLI. It prints every
request the host forwarded and every CLI result, and exits 1 on the first
result that differs. The task gives it a fresh `XDG_RUNTIME_DIR`, so the CLI
does not also reach the hosts of a running Chrome.

To drive a different case, edit `extension()`, `cases` and `REQUESTS` in the
script together.

## Live: the user's Chrome

The sockets in `$XDG_RUNTIME_DIR/chromectl/` belong to the hosts the user's
browsers started, one per profile. A CLI call against them acts on the user's
real tabs. `tab list` and `group list` are harmless; `tab activate`,
`tab open`, `group activate` and `group add` change what the user sees;
`tab close` is destructive. Confirm before any of the last five.

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
