# Overview

## Start here

[design/architecture.md](design/architecture.md) states the constraint every
structural choice follows from: only the extension can open a connection, so
the CLI talks to the host Chrome already started, never to Chrome. Its
[process diagram](design/architecture.md#three-processes) shows the three
processes and what connects them.

## Layout

Each Go package states what it does in its package comment.

| Path | Holds |
|---|---|
| `embed.go` | The extension, embedded into the binary. |
| `cmd/chromectl/` | The CLI, the `install` and `version` commands, and the `host` entry. |
| `internal/proto/` | The message shape both protocols carry. |
| `internal/native/` | Chrome's framing. |
| `internal/hostd/` | The host. |
| `internal/client/` | The CLI's side of the sockets. |
| `internal/build/` | The version the build stamps in. |
| `extension/` | The MV3 extension. |
| `scripts/fakechrome.py` | A scripted Chrome on the host's pipes, for `mise run drive`. |

## Wire format

[design/architecture.md](design/architecture.md) owns the two protocols.
The fields are in `internal/proto/proto.go`; the extension reads them by the
same JSON names.

## Finding things

```bash
rg -n 'async "?[\w-]+"?\(' extension/background.js    # every command the extension implements
rg -n 'case (act == )?"' cmd/chromectl/main.go        # every CLI subcommand
rg -n 'json:"' internal/proto/proto.go                # every field on the wire
```

## Outside this repository

- Native messaging: <https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging>
- Service worker lifetime: <https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle>
- revier (<https://github.com/hk9890/revier>) is the first consumer; it execs
  `chromectl` from an adapter the way it execs `wctl`
  (<https://github.com/carlo9890/gnome-window-control>).
