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

**A `--profile` flag on `tab open`.** A second way to choose the host. The window
id already names its profile, and both need a `list` to learn the value.

**The host learns its profile from the CLI's question.** The socket named only
the pid, and the CLI asked each host who it was. Every command for one tab then
dialled every host, and one hung host held them all.

**The CLI prints the window manager's title.** chromectl would print
`<active tab title> - Google Chrome`, the title a normal Chrome window shows,
so the caller hands it to wctl as is. Rejected because chromectl would print a
guess as a fact. The suffix is a Chrome UI string no extension can read: it is
" - Chromium" in Chromium and something else in Brave. A window the user named
shows the name alone, and the extension cannot read that name either. The tab
title is what Chrome says, so chromectl prints that, and the caller, which
knows its browser, adds the suffix.

**Open-or-activate by URL prefix.** One call would find a tab whose URL starts
with a prefix, or open the URL. Rejected: it makes chromectl the judge of when
a page is open, which is the matcher [product.md](product.md#model) leaves to
the caller.

**`tab open --group` and a raise-only `group activate`.** `tab open --group
<title> <url>` would open a tab straight into a group, creating it if needed,
and a `group activate <title>` without a URL would raise a group or fail. Rejected:
`group activate <title> <url>` opens into a group when it creates one, and
`group add` puts any tab, a new one included, into a group, so a flag on
`tab open` would be a second way to do the same. A raise that can fail on a
missing group needs a "not found" exit code that nothing else needs.

**A deadline in the host.** The host would answer a pending call with an error
after a fixed time. Rejected: it cannot cancel what the extension has started,
so the call might still act after the caller was told it failed, and it would
be a second deadline the caller cannot set. The CLI's deadline already frees
the caller.

**A flag on the reply for "the host could not get an answer".** The host would
reply `{seq, error, lost: true}` where Chrome went away, so the CLI could tell
it from Chrome's own error and keep the text "chrome went away". Rejected:
closing the connection without a reply says the same thing with no new field,
and the CLI already has to handle a connection that closes early.

**Window focus times in `chrome.storage.session`.** The times `windowFocusedAt`
reports would survive a restart of the worker. Rejected: the storage API needs
the `storage` permission, and the worker lives as long as Chrome
([Three processes](architecture.md#three-processes)), so the times are lost
only when the extension reloads or Chrome restarts. After a restart the
windows they named are gone; after a reload each window is unknown until it
is focused again, which a keypress that follows the user's work soon does.

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
one hung host would hold every call; a call now has a deadline.

These were replaced when chromectl learned tab groups.

**The `tabs` permission alone.** The extension asked for `tabs` and
`nativeMessaging` only. Finding a group by its title is the job a project key
has, and that needs `tabGroups`, so the set grew by one. The rule that a
command needing more is a different product still stands, for the set as it
is now. Why `tabGroups` fits is in [Scope](product.md#scope).

**chromectl never reads a URL to choose a tab.** The caller matched every URL.
Now `group activate` uses the URL it was given to rank the tabs inside the
group that its title found. That is the only exception: a matcher of its own
stays rejected, as "Open-or-activate by URL prefix" above says. The ranking is
in [Groups](product.md#groups), and why the caller still owns matching is in
[Model](product.md#model).

**A fixed 10-second limit per call.** Long enough for a Chrome that restores a
session, too long for a keypress, and no caller could change it, so callers
wrapped chromectl in `timeout`. The deadline is now 5 seconds, and `--timeout`
sets it.

**"chrome went away" as an error reply.** The host answered a call it could
not finish with that text, and the CLI exited 1, as for Chrome's own error. A
caller could not tell "Chrome refused" from "Chrome may have acted" without
reading the text. The host now closes the connection without a reply, and the
CLI exits 4.

**Exit 1 for every failure.** A bad argument, a missing host and Chrome's
refusal all exited 1. A caller that falls back when no host runs could not
tell that from a typo. Each now has its own code.

These were replaced when chromectl learned to add a tab to a group.

**A flat command list.** `list`, `groups`, `activate`, `activate-group`,
`open`, `close`. The tab was the implied object, and the group was named
twice, two ways: as a plural and as a suffix. A user could not guess one
command from another, and adding a tab to a group would have added a third
way: `group`, one letter from `groups`, though one reads and the other
changes the browser. Commands now name the object first, `tab` or `group`,
then the action. The old names were dropped with no alias.

**"No consumer" as a reason to leave a group action out.** Adding a tab to a
group was out until a consumer asked. It is one Chrome call on a permission
chromectl already holds, and it completes the group commands, so it went in
without one.

These were replaced when the consumer moved from groups to matching by URL.

**No window type in `tab list`.** Rejected while the planned consumer,
revier's project key, addressed groups: a group cannot live in an `--app`
window, so nothing would have read the field. The key now lists tabs and
matches their URL, and an `--app` window can hold a tab on the project's URL.
Its window shows no browser suffix, so a caller that raises the window by its
title cannot find it. `tab list` now carries the window's type, so the caller
can skip such a tab. One `windows.getAll` call gives every tab with its
window, so the field costs no second call.
