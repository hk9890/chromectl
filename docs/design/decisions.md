# Decisions

The alternatives that were weighed and rejected, and the designs this one
replaced. Each of these would otherwise be proposed again. The design that
stands is in [architecture.md](architecture.md) and [product.md](product.md).

## Rejected alternatives

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

## Superseded

These held in v0.1.0 and were replaced when chromectl gave every profile its
own host.

**One socket path for every host.** `$XDG_RUNTIME_DIR/chromectl.sock`. Each new
host removed the socket before it and took the path. With two profiles, the
profile that started last took every call, and the other host ran on with no
way to reach it.

**Tab ids exactly as Chrome gives them.** Right while one host answered every
call. With a host per profile, a bare number does not say which host owns it,
and two browsers can hand out the same one.

**The socket's lifetime as the only timeout.** A call waited as long as its
connection stayed open. With several hosts, `list` waits for all of them, and
one hung host would hold every call; a call now gives up after 10 seconds.
