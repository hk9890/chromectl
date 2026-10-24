# Overview

## Start here

The design set under [design/](design/) owns why the system has this shape.
[design/architecture.md](design/architecture.md) states the constraint every
structural choice follows from: only the extension can open a connection, so
the CLI talks to the host Chrome already started, never to Chrome.

```
chromectl list ──unix socket──▶ chromectl host ◀──stdin/stdout──▶ extension ──▶ chrome.tabs
                $XDG_RUNTIME_DIR/chromectl.sock   native messaging
```

## Layout

| Path | Holds |
|---|---|
| `embed.go` | Embeds `extension/` into the binary, for `install` to write out. |
| `cmd/chromectl/` | The CLI, the `install` and `version` commands, and the `host` entry. |
| `internal/proto/` | The one message shape both sides speak, plus the extension id and host name. |
| `internal/native/` | Chrome's framing: 4-byte native-endian length, then JSON. |
| `internal/hostd/` | The host: numbers each request, relays it, returns the reply to the socket connection that asked. |
| `internal/client/` | The CLI's side of the socket, and where the socket lives. |
| `internal/build/` | The version `mise run build` and `.goreleaser.yaml` stamp in. |
| `extension/` | The MV3 extension. Its `key` fixes the id. |
| `scripts/fakechrome.py` | A scripted Chrome on the host's pipes, for driving headless. |

## Wire format

[design/architecture.md](design/architecture.md) owns the two protocols.
The fields are in `internal/proto/proto.go`; the extension reads them by the
same JSON names.

## Finding things

```bash
rg -n 'async \w+\(' extension/background.js    # every command the extension implements
rg -n 'case "' cmd/chromectl/main.go             # every CLI subcommand
rg -n 'json:"' internal/proto/proto.go           # every field on the wire
```

## Outside this repository

- Native messaging: <https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging>
- Service worker lifetime: <https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle>
- revier (<https://github.com/hk9890/revier>) is the first consumer; it execs
  `chromectl` from an adapter the way it execs `wctl`.
