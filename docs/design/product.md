# Product

## The job

A tab is the unit of work in a browser, and the desktop cannot see one. A
window manager sees Chrome windows; a launcher can hand Chrome a URL and hope;
nothing outside Chrome can ask "is this page open, and where" or bring it to
the front. chromectl makes a tab addressable from outside: find it, raise it,
open it, close it. Everything a run-or-raise keybinding, a project switcher, or
a script needs to treat a page the way it treats an editor window.

A project rarely lives in one tab. Chrome's answer to "these tabs belong
together" is the tab group, so chromectl makes a group addressable the same
way: find it by its title, raise it, or create it.

## Model

Chrome owns tab identity, and chromectl does not pretend otherwise.

- A tab id is Chrome's, plus the profile that owns it, for example
  `3f9a12bc:812`. It is stable while the tab lives and gone after Chrome
  restarts. chromectl never invents a longer-lived handle, because any it
  invented would have to be re-found by URL anyway.
- A group id is the same, and Chrome gives every group a new one when it
  restores a session. A group's title is the one handle that survives a
  restart, and the user sets it, not chromectl. So a group is addressed by its
  title, compared exactly: case-sensitive, nothing trimmed, no wildcards. A
  title is not an invented handle; it is the name the user sees in the tab
  strip.
- Every profile of every browser that loads the extension is reachable at
  once. `list` shows them all; an id names its own profile, so no command
  needs a profile flag.
- Finding a tab by its URL is the caller's job: list, match on the URL, act on
  the id. What counts as the same page (a trailing slash, a query, a fragment,
  a redirect) is a policy, and chromectl holds none. The one place chromectl
  reads a URL to choose is inside a group its title has already found, where
  `activate-group` ranks the group's tabs by the URL it was given. There the
  URL never decides that a page is open.
- A window is addressed only as a place to open into. Raising a tab focuses
  its window as a side effect, because a tab that is active in an unfocused
  window is not raised. On GNOME Wayland that focus does not raise the window:
  Chrome asks the compositor through xdg-activation with no fresh input event,
  and Mutter refuses. The caller raises the window with the window manager.

## Groups

`activate-group <title> <url> [window]` is the run-or-raise of a group, in one
call:

1. Find the open groups whose title equals `<title>`, in every profile.
2. If there are several, take the one that holds the most recently accessed
   tab. That is the group the user worked in last, which a keypress most
   likely means.
3. If one is found: expand it, activate a tab in it, and focus its window. The
   tab is the first of these that exists, the most recently accessed where
   several tie: a tab whose URL equals `url`; a tab whose URL starts with
   `url` followed by `/`, `?` or `#`; any tab of the group. The boundary keeps
   `github.com/o/repo` from ranking `github.com/o/repo-tools` as its page.
4. If none is found: open `url` in a new tab of the window, else of the last
   focused normal window, put the tab in a new group, give the group the
   title, and focus the window. Where no normal window exists, open a new one.
   With several profiles, the window is required, as for `open`, because it
   names the profile.

The rules this settles:

| Case | Behaviour | Why |
|---|---|---|
| Two groups share a title | Step 2 picks one; the other is left alone | Chrome allows duplicates, even in one window. An error would make a keypress fail for a state the user did not notice. |
| The group is collapsed | Expanded before its tab is activated | Activating a tab expands its group in the horizontal tab strip; the explicit expand does not depend on how the strip is laid out. |
| The user renamed the group | The old title no longer finds it; the next call creates a new group under the old title | The title is the handle. A group the user titles by hand is found the same way, which is how a user adopts an existing group. |
| The group was closed and saved | Not found; a new group is created | Chrome's API returns only open groups and has no call to reopen a saved one. Chrome may then hold two saved groups with that title. |
| Two calls race to create the group | The second finds the first's group, where both go to one profile ([how](architecture.md#concurrency-in-the-extension)) | Two presses of a key would otherwise create two groups. |
| The last focused window is an `--app` window | The new group goes into a normal window, named explicitly | `--app` and popup windows cannot hold groups, and Chrome's "current window" can be one. |
| A profile gives no reply | The group is raised where found; where it is found nowhere, none is created | The silent profile may hold it, and creating a second would be the duplicate the title is meant to prevent. A profile that answers with an error, as an extension that predates groups does, holds no group `activate-group` could raise, so it blocks nothing. |

Creating a titled group is three Chrome calls: create the tab, group it, set
the title. If the last fails, the untitled group stays and Chrome's error is
the caller's. No call creates a titled group at once.

When `activate-group` creates the group, the tab it prints has not loaded. Its
title is the URL Chrome shows while loading, or empty, and the window's title
changes once the page sets its own. A caller that raises the window by its
title waits for that: it reads the tab's title again with `list` until the
window manager finds a window with it, within its own bound. When the group was
found, the title is the page's and stays.

## Surface

Six commands act on Chrome: `list`, `groups`, `activate`, `activate-group`,
`open`, `close`. Beside them, `install` sets chromectl up and `version` prints
the release it is. Output is for machines first: one item per line,
tab-separated, ids in the leading columns, so a caller reads it with `cut` or a
split and never parses prose.

| Command | Prints, tab-separated | `--json` |
|---|---|---|
| `list` | `id window group active browser title url` per tab | array of `{id, window, group, active, browser, profile, title, url}` |
| `groups` | `id window title` per open group | array of `{id, window, title}` |
| `open <url> [window]` | `id window` | `{id, window}` |
| `activate-group <title> <url> [window]` | `id window created title` | `{id, window, created, title}` |
| `activate <tab-id>`, `close <tab-id>` | nothing | not taken |

`group` is the id of the tab's group, `-` in tab-separated output and `""` in
JSON for a tab in no group, and for every tab of an extension older than
groups. `activate-group` prints the tab it activated. `created` says whether it
made the group, and `title` is the tab's title, which is the text a normal
Chrome window shows before its own suffix: what chromectl can give a caller
that raises the window by its title.

`--json` exists for a caller that cannot trust tab-separated text: a page or
group title may itself contain a tab. Tab-separated stays the default so a
shell pipe needs no `jq`. Errors stay plain text on stderr in both modes. This
follows wctl, whose listing commands take `--json` and whose action commands do
not.

`open`, and `activate-group` where it creates a group, work without a window id
only while one profile runs; with several, the caller names the window, and so
the profile.

### Deadline

Every call to a host gives up after 5 seconds, or after the duration
`--timeout` names in Go's syntax (`2s`, `500ms`). The flag goes with the
command's other flags, before its arguments:
`chromectl activate-group --timeout 2s <title> <url>`. A duration of zero or
less is bad usage. A tab call that is answered at all is answered in
milliseconds; the default leaves room for a Chrome that is restoring a session,
and a keybinding that wants less passes less. The deadline covers a call from
its connect to the last byte of the reply. `list` and `groups` ask every host
at once, so they take one deadline; `activate-group` asks every host, then one,
so it takes two at most. Without a deadline, an extension that never answers
holds the caller forever, and every caller wraps chromectl in `timeout`.

A deadline that passes after the request reached Chrome does not undo it. The
tab may still open or close, so the message says the outcome is unknown.

### Exit codes

| Code | Meaning | What the caller can do |
|---|---|---|
| 0 | Done | |
| 1 | A reply with an error: Chrome's text verbatim, the extension's own, or a host's that cannot serve or never sent the request on. Also any failure of chromectl's own that is not usage, such as `install` | Show the text |
| 2 | Bad usage: an unknown command or flag, a missing or extra argument, an id not of the form `list` prints, an empty title or url, a duration of zero or less, or no window id where several profiles run | Fix the call |
| 4 | No reply: the deadline passed, or the host closed the connection without one, as when Chrome goes away mid-call. The command may or may not have taken effect. | Retry a read; check before repeating a write |
| 5 | No host is listening, none for the profile the id names, or no runtime directory a host could listen in. Nothing reached Chrome. | Fall back, or start Chrome |

Where `list`, `groups` or `activate-group` print what answered while a host did
not, they exit with the code of the first host that failed.

4 and 5 mean what they mean in wctl, "timed out" and "not available", so a
caller that runs both tools reads them the same way. 1 and 2 do not: wctl's 2
is "not found", chromectl's is usage, and a caller branches on those per tool.

## Install

One binary, one unpacked extension, one manifest write. There is no daemon the
user starts, enables, or notices: Chrome starts the host when the extension
connects and stops it when Chrome exits.

The binary carries the extension inside it, and `chromectl install` writes both
to `~/.local/share/chromectl/`: a copy of the running binary, and the
extension beside it. The manifest names that copy, and the user loads the
extension from there. One fixed directory makes an upgrade `mise upgrade`,
`chromectl install`, and a reload of the extension's card; nothing is loaded
again from a new path. Why the manifest does not name the running binary is in
[decisions.md](decisions.md).

The extension id is fixed by a public key in its manifest. Without that, an
unpacked extension's id would depend on the directory it was loaded from, and
the native messaging manifest, which must name the id it allows, could only be
written after the user loaded the extension and read the id back. With it,
`chromectl install` can write the manifest first and the extension works the
moment it is loaded.

A new permission costs the user nothing beyond that reload. Chrome disables an
extension whose update adds a permission with a warning until the user accepts
it, but only for an extension from the Web Store; an unpacked extension is a
trusted location, and the reload grants the permission silently. Until the
reload, the old extension runs beside the new binary; what that costs is in
[What degrades](#what-degrades-and-how-far). Should chromectl ever ship through
the Web Store, adding a permission becomes a prompt the user must accept.

## Scope

| In | Why |
|---|---|
| Chrome, Chromium, Brave, on Linux | One native messaging manifest format, one config root, one socket convention |
| The `tabs` and `tabGroups` APIs | Everything the job needs, and the narrowest permissions that provide it. `tabGroups` warns "View and manage your tab groups": it reads and changes the strip, not a page. A group's id needs no permission, but its title and its collapsed state do. |

**Out, deliberately**

| Left out | Why |
|---|---|
| Reading page content, running scripts, screenshots | A different permission set (`<all_urls>`, `scripting`) and a different trust level. The extension asks for what the job needs and nothing that would let the host read a page. |
| Bookmarks, history, cookies, downloads | Not tabs. Each is a product of its own. |
| Firefox | Decided out. Its native messaging and extension format are close enough that it would be tempting, and the second browser would double every manual test with no consumer asking for it. |
| A URL matcher | Matching a page is a policy. See Model. |
| Any window management beyond focus | The window manager owns windows. wctl exists for that. |
| Mapping a Chrome window to the window manager's window | No Chrome API exposes the OS window, and every Chrome window shares one process. The caller joins on the title [Surface](#surface) gives it. That join cannot find a window the user named, which shows the name alone, and cannot tell apart two windows whose active tabs share a title. |
| Group colour, adding a tab to a group, moving or ungrouping | No consumer. Each is one Chrome call away when one asks. |

## Non-goals

chromectl is not a browser automation tool. It does not drive pages, fill
forms, or wait for loads. Anything that needs to see inside a page belongs to
the DevTools protocol and a test profile, not to a user's daily browser.

## What degrades, and how far

| Condition | Behaviour |
|---|---|
| Chrome is not running | No host listens. The CLI names the three causes and exits 5. |
| The extension is disabled or not loaded | Same as above; the host was never started. |
| The host crashed | The extension reconnects after two seconds. A call in that window exits 5. |
| The extension does not answer | The call exits 4 at the deadline. |
| Chrome exits mid-call | The host closes the connection without a reply, and the call exits 4. |
| The host dies mid-call | The call exits 4. |
| The extension is reloaded | The CLI uses the new host. |
| One profile's host does not answer in time | `list` prints the other profiles' tabs, warns, and exits 4. |
| The binary was upgraded, the extension not yet reloaded | Every call to that profile answers "reload the chromectl extension", exit 1. An extension that predates groups answers `groups` and `activate-group` with "unknown command", and `list` shows its tabs in no group. |
| A profile was copied into a second browser | The CLI refuses that profile's ids and names both browsers. |
| A profile was copied inside one browser | Known limit: the CLI reads it as a reload and shows only one of the two. |
| No runtime directory, as in a session without logind | Known limit: the host cannot start, and the CLI reports no host. |
