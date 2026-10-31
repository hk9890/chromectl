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
                $XDG_RUNTIME_DIR/chromectl/       native messaging
                one socket per host
```

| Process | Started by | Lives until | Holds |
|---|---|---|---|
| Extension service worker | Chrome, on profile start: the worker listens for `runtime.onStartup`, and Chrome starts an MV3 worker at browser start only for a worker that does | Chrome exits. An open native port keeps an MV3 worker alive, so no keepalive tick is needed. | The one port to the host, and the profile id |
| `chromectl host` | Chrome, when the worker connects | Chrome closes the pipe. It removes its socket at that moment, then has one second to write the last replies. | The socket listener, the pending map, its profile id |
| `chromectl <cmd>` | The user or a caller | One reply from each host it asks | Nothing |

The host is a child of Chrome in the same sense as a renderer: the user never
starts or stops it. If it dies, the worker reconnects after two seconds and
Chrome starts a fresh one.

## One host per profile

Every profile that loads the extension starts its own host, and Chrome,
Chromium and Brave can run at once. So several hosts run at once, and the CLI
must know which one owns a tab.

**The profile id.** No API names the profile without a new permission, so the
extension makes a random id once and keeps it in IndexedDB: each profile has
its own, and clearing browsing data does not touch it. The host asks for it
with `identify` before it listens.

**Ids.** A tab or window id is `<profile>:<chrome id>`. Chrome's numbers are
unique in one browser process only; the profile part makes them unique across
browsers. An id is always printed and read in full: a shortened form would
change its meaning when a profile starts or stops. Only the CLI adds and
strips the profile.

**The socket name is the host's identity.** Each host has its own socket in
the user's runtime directory, and its name carries the profile, the browser,
the browser's pid and the host's start time. One read of the directory finds
every host, with no registry to keep in step.

- A socket name appears only once a host listens behind it, so a refused
  connection means a dead host. The CLI removes such a socket before it reads
  the names.
- The same profile under one browser pid twice is a reload: the later start
  wins.
- The same profile under two browser pids is a copied profile: the CLI refuses
  its ids.
- A host whose extension cannot give its id stays up as a stale host and
  answers every call with the reason. Exiting would start a reconnect loop,
  and the reason would reach only Chrome's stderr.

**Fan-out.** `list` asks every host at once. Each call has a time limit, which
is the CLI's alone: Chrome may still carry out a call that timed out. `list`
prints what answered, warns for each host that did not, and exits 1.

## Two protocols

**Host to extension** is Chrome's native messaging: a 4-byte native-endian
length, then JSON, at most 1 MB from the host. Every message from the host is a
request `{seq, cmd, id, url?, window?}`; every reply is `{seq, result}` or
`{seq, error}`. `seq` is the host's correlation number and `id` is a tab id;
they are separate fields because the extension reads the request's fields as
the arguments of the tabs API call, and a tab id named anything but `id` would
be a translation layer for no gain. `id` is always sent: Chrome reads a
missing tab id as the current tab. The host sends one request of its own,
`identify`, answered with `{profile}`.

**CLI to host** is one JSON line each way over a Unix socket, then close. A
request-reply socket rather than a stream, because a CLI call is one question.
The host assigns `seq`; the CLI never sees it.

## Concurrency in the host

- One goroutine reads replies from Chrome and delivers each to the channel
  registered under its `seq`. A reply for an unknown `seq` is logged and
  dropped. A reply that does not decode fails the call its `seq` names; where
  the `seq` does not decode either, it fails every pending call, because no
  one call can be told it was the one.
- Every socket connection is its own goroutine: decode, register, write to
  Chrome under one mutex, wait on its channel, encode.
- When the pipe closes, every pending channel receives "chrome went away",
  the socket file is removed, the listener closes, and `Run` returns once each call that reached Chrome
  has written its reply, or has had one second to. The process exits with
  `Run`, so a reply not yet written would be lost, and a caller that never
  reads must not hold the process.

## Rejected alternatives

[decisions.md](decisions.md) holds them, and the designs this one replaced.

## Layout

| Path | Holds |
|---|---|
| `extension/` | The MV3 extension. `manifest.json` carries the public key that fixes the id; `background.js` is the command table and the port. |
| `embed.go` | The extension, embedded so a released binary can install it. |
| `cmd/chromectl/` | The CLI, `install`, `version`, and the `host` entry. |
| `internal/proto/` | The one message shape both protocols carry, and the two constants the manifest and the extension must agree on. |
| `internal/native/` | Chrome's framing. |
| `internal/hostd/` | The relay, and the socket it listens at. |
| `internal/client/` | The CLI's side: finding the hosts, and calling one. |
| `internal/build/` | The version the build stamps in. |

## Consumers

A consumer execs the binary and parses its lines. It never imports the
packages, so chromectl can change its internals without a release of the
consumer, and the consumer can pin a chromectl version the way it pins any
tool. revier's adapter for it has the same shape as its adapter for wctl.
