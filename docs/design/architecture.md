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

Every profile that loads the extension has its own worker, and every worker
starts its own host. Chrome, Chromium and Brave can run at once, each with
several profiles. So several hosts run at once, and the CLI must know which
one owns a tab.

**The profile id.** No API tells the extension or the host which profile it
serves without a new permission. The extension makes one: 8 random hex digits,
kept in IndexedDB, which every profile has apart and which clearing browsing
data does not touch for an extension. The host asks for it with the request
`identify` before it opens its socket.

**Ids.** A tab or window id is `<profile>:<chrome id>`, for example
`3f9a12bc:812`. Chrome's tab and window ids come from one counter per browser
process, so they are unique across the profiles of one browser, but two
browsers can hand out the same number. The profile part makes the id unique
across all of them, and it is always printed and read in full: a shortened
form would mean one thing while some profiles run and another after one stops.
The host and the extension see only Chrome's number; the CLI adds and strips
the profile.

**The socket name is the host's identity.** Each host listens at
`<dir>/chromectl/<profile>.<browser>.<browser pid>.<start>.sock`, where
`<dir>` is `$XDG_RUNTIME_DIR`, else `/run/user/<uid>` when it exists and
belongs to the user. `<browser>` comes from the directory of the parent
process's executable (`chrome`, `chrome-beta`, `chromium`, `brave`), and
`<start>` is the host's start time in nanoseconds. So one read of the
directory tells the CLI every host, its profile and its browser, and a
command for one tab dials one socket.

- The host binds and listens at a hidden temporary name, then renames the
  socket into place. A visible name always has a listener behind it, so a
  refused connection means the host is dead, and the CLI deletes the file.
- Two names with the same profile and the same browser pid are a reload: the
  old host still runs until Chrome closes its pipe. The CLI uses the later
  start.
- The same profile under two browser pids is a profile directory copied with
  its IndexedDB. The CLI refuses that profile's ids and names both browsers.
- A host whose extension answers `identify` with an error runs an extension
  older than its binary: `chromectl install` replaced the binary, and the host
  died before the user reloaded the extension. It listens at
  `stale.<browser>.<browser pid>.<start>.sock` and answers every call with
  "reload the chromectl extension". Exiting instead would start a loop: the
  worker reconnects every two seconds, and the reason would reach only
  Chrome's stderr.

**Fan-out.** `list` asks every host at once and prints what they answer. A
call that gets no reply in 10 seconds fails; `list` then prints the other
hosts' tabs, a warning per host, and exits 1, so a script can tell the list is
incomplete. A host that answers "chrome went away" is shutting down, and is
skipped like a dead one. The limit is the CLI's alone: a call that timed out
may still be carried out by Chrome.

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

**One socket path for every host.** Each new host removed the socket before it
and took the path. With two profiles, the profile that started last took every
call, and the other host ran on with no way to reach it.

**Plain Chrome ids, found by asking every host.** Ids would stay Chrome's
numbers, and the CLI would find the owner of a tab by listing every host first.
Two browsers can hand out the same number, so an id could name two tabs, and
every call would cost a round trip to every host.

**Sending a command to every host.** The host without the tab answers "No tab
with id". Simple, but on a clash between two browsers it acts in both.

**The shortest unique prefix of the profile id.** Short ids, as git prints
them. Git's rule holds because objects are never removed; hosts come and go.
An id printed while two profiles ran could name a different profile after one
stopped.

**A `--profile` flag on `open`.** A second way to choose the host. The window
id already names its profile, and both need a `list` to learn the value.

**The host learns its profile from the CLI's question.** The socket named only
the pid, and the CLI asked each host who it was. Every command for one tab then
dialled every host, and one hung host held them all.

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
