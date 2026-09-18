# Overview

## The constraint everything follows from

An outside process cannot talk to Chrome. Only the extension can, and only
outward: it calls `connectNative`, Chrome starts the program the manifest
names, and the two share a pipe. So the CLI never connects to Chrome. It
connects to the host Chrome already started.

```
chromectl list ──unix socket──▶ chromectl host ◀──stdin/stdout──▶ extension ──▶ chrome.tabs
                $XDG_RUNTIME_DIR/chromectl.sock   native messaging
```

- The host lives as long as Chrome keeps the pipe open, and removes its socket
  on the way out. Chrome owns its lifecycle; nobody starts or stops it.
- An open native port keeps the MV3 service worker alive, so the extension
  needs no keepalive.
- The extension reconnects two seconds after the host goes away.

## Layout

| Path | Holds |
|---|---|
| `cmd/chromectl/` | The CLI, the `install` command, and the `host` entry. |
| `internal/proto/` | The one message shape both sides speak, plus the extension id and host name. |
| `internal/native/` | Chrome's framing: 4-byte native-endian length, then JSON. |
| `internal/hostd/` | The host: numbers each request, relays it, returns the reply to the socket connection that asked. |
| `internal/client/` | The CLI's side of the socket, and where the socket lives. |
| `extension/` | The MV3 extension. Its `key` fixes the id. |
| `scripts/fakechrome.py` | A scripted Chrome on the host's pipes, for driving headless. |

## Wire format

- Host to extension: `{seq, cmd, id?, url?, window?}`. Extension to host:
  `{seq, result}` or `{seq, error}`. `seq` correlates; `id` is a tab id.
- CLI to host over the socket: one `Request` as a JSON line; one `Reply` back;
  connection closed. The host fills `seq`.
- Chrome accepts at most 1 MB per message from a host.

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
