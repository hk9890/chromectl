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
chromectl list ──unix socket──▶ chromectl host ◀──stdin/stdout──▶ extension ──▶ chrome.tabs, chrome.tabGroups
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

**Ids.** A tab, window or group id is `<profile>:<chrome id>`. Chrome's numbers are
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

**Fan-out.** `list` and `groups` ask every host at once, and print what
answered, with a warning for each host that did not. `activate-group` asks
every host for its groups, then sends the command to the one profile that
holds the group, or that the window names.

## Two protocols

**Host to extension** is Chrome's native messaging: a 4-byte native-endian
length, then JSON, at most 1 MB from the host. Every message from the host is a
request `{seq, cmd, id, url?, window?, title?}`; every reply is `{seq, result}` or
`{seq, error}`. `seq` is the host's correlation number and `id` is a tab id;
they are separate fields because the extension reads the request's fields as
the arguments of the tabs API call, and a tab id named anything but `id` would
be a translation layer for no gain. `id` is always sent: Chrome reads a
missing tab id as the current tab. `title` is a group's title. It is the one
field the extension does not hand to Chrome as is: `tabGroups.query` reads a
title as a pattern, where `*` and `?` are wildcards, so `activate-group`
queries every group and compares each title itself. The host sends one request
of its own, `identify`, answered with `{profile}`.

**CLI to host** is one JSON line each way over a Unix socket, then close. A
request-reply socket rather than a stream, because a CLI call is one question.
The host assigns `seq`; the CLI never sees it. Where a request may have
reached Chrome and the host has no answer to give, because Chrome went away or
the write to it failed, the host logs the reason and closes the connection
without a reply. The CLI reads a connection closed without a reply as it reads
a deadline that passed: the outcome is unknown. Where the request provably
never reached Chrome, too large for its limit or sent after Chrome went away,
the host replies with the reason instead. Every reply is passed on with its
error text unchanged.

## The deadline

The deadline is the CLI's alone. Each call takes one instant, dials with it,
and sets it on the connection, so the connect, the request and the reply all
fall under it. When it passes, the CLI closes the connection; what it exits
with is [product.md](product.md#exit-codes)'s. The host does not time out
([rejected](decisions.md#rejected-alternatives)). A call the extension never
answers holds one goroutine and one pending entry in the host until Chrome
closes the pipe, and its late reply, written to a closed connection, is
dropped.

The CLI tells the two ways a call ends without a reply by one error type,
`client.NoReplyError`, which unwraps to `os.ErrDeadlineExceeded` or `io.EOF`.
A fan-out leaves out only a dead host, whose socket refuses. A host that
closed the connection is a failure the fan-out reports: the CLI cannot tell a
Chrome that shuts down from a host that crashed mid-call.

## Concurrency in the extension

The extension answers each request as its own async call, so a slow one never
holds the others. `activate-group` is the exception: its calls run one at a
time, chained on one promise. Each reads the groups, then writes in several
calls, and two calls for the same title that overlapped would each see no
group and each create one. The cost: a Chrome call that never settles holds
every later `activate-group` behind it until the worker restarts. Each of those
calls ends at its deadline, so the user sees the failure, and a reload of the
extension clears it. The chain is per profile; two calls the CLI sends to two
profiles cannot race for one group, because the CLI creates a group only in
the profile the window names, or the one that runs.

## Concurrency in the host

- One goroutine reads replies from Chrome and delivers each to the channel
  registered under its `seq`. A reply for an unknown `seq` is logged and
  dropped. A reply that does not decode fails the call its `seq` names; where
  the `seq` does not decode either, it fails every pending call, because no
  one call can be told it was the one.
- Every socket connection is its own goroutine: decode, register, write to
  Chrome under one mutex, wait on its channel, encode.
- When the pipe closes, every pending call is closed without a reply, the
  socket file is removed, the listener closes, and `Run` returns once each
  call that reached Chrome has written its reply, or has had one second to. The process exits with
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
tool. revier is the planned consumer; it decodes wctl's JSON by the fields it
names, and reads chromectl's the same way.
