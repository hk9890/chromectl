# Architecture

## The constraint

An outside process cannot talk to Chrome. Only an extension can, and only
outward: it calls `connectNative`, Chrome starts the program that the native
messaging manifest names, and the two share a pipe. Nothing lets a process
open that pipe from the other end. So chromectl's command line never connects
to Chrome. It connects to the host that Chrome already started, and the host
relays.

Every structural choice below follows from this one fact.

## Three processes

```
chromectl list ──unix socket──▶ chromectl host ◀──stdin/stdout──▶ extension ──▶ chrome.tabs
                $XDG_RUNTIME_DIR/chromectl.sock   native messaging
```

| Process | Started by | Lives until | Holds |
|---|---|---|---|
| Extension service worker | Chrome, on browser start | Chrome exits. An open native port keeps an MV3 worker alive, so no keepalive tick is needed. | The one port to the host |
| `chromectl host` | Chrome, when the worker connects | Chrome closes the pipe. Exits and leaves its socket file: a reload starts the next host first, and the path may already be that host's. | The socket listener, the pending map |
| `chromectl <cmd>` | The user or a caller | One reply | Nothing |

The host is a child of Chrome in the same sense as a renderer: the user never
starts or stops it. If it dies, the worker reconnects after two seconds and
Chrome starts a fresh one.

## Two protocols

**Host to extension** is Chrome's native messaging: a 4-byte native-endian
length, then JSON, at most 1 MB from the host. Every message from the host is a
request `{seq, cmd, id?, url?, window?}`; every reply is `{seq, result}` or
`{seq, error}`. `seq` is the host's correlation number and `id` is a tab id;
they are separate fields because the extension reads the request's fields as
the arguments of the tabs API call, and a tab id named anything but `id` would
be a translation layer for no gain.

**CLI to host** is one JSON line each way over a Unix socket, then close. A
request-reply socket rather than a stream, because a CLI call is one question
and the socket's lifetime is the simplest possible timeout. The host assigns
`seq`; the CLI never sees it.

## Concurrency in the host

- One goroutine reads replies from Chrome and delivers each to the channel
  registered under its `seq`. A reply for an unknown `seq` is logged and
  dropped. A reply that does not decode fails only the call it answers.
- Every socket connection is its own goroutine: decode, register, write to
  Chrome under one mutex, wait on its channel, encode.
- When the pipe closes, every pending channel receives "chrome went away",
  the listener closes, and `Run` returns once each accepted connection has
  its reply. The process exits with `Run`, so a reply not yet written then
  would be lost.

## Rejected alternatives

Each of these would otherwise be proposed again.

**The WebSocket inversion.** The extension connects to a localhost server that
the first CLI call starts. It matches the wish for "nothing runs until someone
needs it", but an MV3 worker is asleep between events and its alarms cannot
fire more often than every 30 seconds, so a cold call waits up to 30 seconds
for the extension to notice the server. A run-or-raise keypress cannot carry
that hole. Native messaging has no hole, because Chrome starts the host when
the browser starts.

**The DevTools protocol.** `--remote-debugging-port` can list and activate
tabs without an extension. Since Chrome 136 the flag is ignored on the default
user data directory, so it works only for a separate profile with its own
logins, which is not the user's daily browser.

**brotab as a dependency.** A working extension plus native host with the same
shape. Rejected because its extension is Manifest V2, which Chrome is retiring,
and asks for `<all_urls>` and script execution for features chromectl does not
want to carry. The cost of owning sixty lines of extension is lower than the
cost of depending on those.

**The manifest names the running binary.** No copy and no embedded extension:
the manifest points at whatever binary ran `install`, and the user loads
`extension/` from where it lies. A release installed by mise lives in a
directory named after its version, so after an upgrade the manifest would keep
starting the old version, and an extension loaded from it would vanish with the
old version's directory. `install` copies both to one fixed directory instead.

## Layout

| Path | Holds |
|---|---|
| `extension/` | The MV3 extension. `manifest.json` carries the public key that fixes the id; `background.js` is the command table and the port. |
| `embed.go` | The extension, embedded so a released binary can install it. |
| `cmd/chromectl/` | The CLI, `install`, `version`, and the `host` entry. |
| `internal/proto/` | The one message shape both protocols carry, and the two constants the manifest and the extension must agree on. |
| `internal/native/` | Chrome's framing. |
| `internal/hostd/` | The relay. |
| `internal/client/` | The CLI's side of the socket, and where the socket lives. |
| `internal/build/` | The version the build stamps in. |

## Consumers

A consumer execs the binary and parses its lines. It never imports the
packages, so chromectl can change its internals without a release of the
consumer, and the consumer can pin a chromectl version the way it pins any
tool. revier's adapter for it has the same shape as its adapter for wctl.
