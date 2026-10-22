# Product

## The job

A tab is the unit of work in a browser, and the desktop cannot see one. A
window manager sees Chrome windows; a launcher can hand Chrome a URL and hope;
nothing outside Chrome can ask "is this page open, and where" or bring it to
the front. chromectl makes a tab addressable from outside: find it, raise it,
open it, close it. Everything a run-or-raise keybinding, a project switcher, or
a script needs to treat a page the way it treats an editor window.

## Model

Chrome owns tab identity, and chromectl does not pretend otherwise.

- A tab id is stable while the tab lives and gone after Chrome restarts.
  chromectl exposes it as it is. It never invents a longer-lived handle,
  because any it invented would have to be re-found by URL anyway.
- Finding a tab again is the caller's job: list, match on the URL, act on the
  id. The one consumer that exists, revier, already owns a matcher, and a
  second matcher inside chromectl would be a second opinion about what a match
  means.
- A window is addressed only as a place to open into. Raising a tab focuses
  its window as a side effect, because a tab that is active in an unfocused
  window is not raised.

## Surface

Four commands act on tabs: `list`, `activate`, `open`, `close`. Beside them,
`install` sets chromectl up and `version` prints the release it is. Output is for machines
first: one tab per line, tab-separated, ids in the leading columns, so a
caller reads it with `cut` or a split and never parses prose. An error from
Chrome reaches the caller verbatim, with exit code 1; chromectl adds nothing
Chrome did not say.

`--json` on `list` and `open` prints the same fields as JSON, for a caller
that cannot trust tab-separated text: a page title may itself contain a tab.
`list` prints an array of `{id, window, active, title, url}`, `open` an object
`{id, window}`. `activate` and `close` print nothing on success, so they take
no `--json`; errors stay plain text on stderr in both modes. Tab-separated stays
the default so a shell pipe needs no `jq`. This follows wctl, whose listing
commands take `--json` and whose action commands do not.

## Install

One binary, one unpacked extension, one manifest write. There is no daemon the
user starts, enables, or notices: Chrome starts the host when the extension
connects and stops it when Chrome exits.

The binary carries the extension inside it, and `chromectl install` writes both
to `~/.local/share/chromectl/`: a copy of the running binary, and the
extension beside it. The manifest names that copy, and the user loads the
extension from there. A release installed by mise lives in a directory named
after its version, so a manifest that named the running binary would keep
starting the old version after an upgrade, and an extension loaded from it
would vanish with the old version's directory. One fixed directory makes an
upgrade `mise upgrade`, `chromectl install`, and a reload of the extension's
card; nothing is loaded again from a new path.

The extension id is fixed by a public key in its manifest. Without that, an
unpacked extension's id would depend on the directory it was loaded from, and
the native messaging manifest, which must name the id it allows, could only be
written after the user loaded the extension and read the id back. With it,
`chromectl install` can write the manifest first and the extension works the
moment it is loaded.

## Scope

| In | Why |
|---|---|
| Chrome, Chromium, Brave, on Linux | One native messaging manifest format, one config root, one socket convention |
| The `tabs` API | Everything the job needs, and the narrowest permission that provides it |

**Out, deliberately**

| Left out | Why |
|---|---|
| Reading page content, running scripts, screenshots | A different permission set (`<all_urls>`, `scripting`) and a different trust level. The extension asks for what the job needs and nothing that would let the host read a page. |
| Bookmarks, history, cookies, downloads | Not tabs. Each is a product of its own. |
| Firefox | Decided out. Its native messaging and extension format are close enough that it would be tempting, and the second browser would double every manual test with no consumer asking for it. |
| A URL matcher | The caller owns matching. See Model. |
| Any window management beyond focus | The window manager owns windows. wctl exists for that. |

## Non-goals

chromectl is not a browser automation tool. It does not drive pages, fill
forms, or wait for loads. Anything that needs to see inside a page belongs to
the DevTools protocol and a test profile, not to a user's daily browser.

## What degrades, and how far

| Condition | Behaviour |
|---|---|
| Chrome is not running | The socket is absent. The CLI names the three causes and exits 1. |
| The extension is disabled or not loaded | Same as above; the host was never started. |
| The host crashed | The extension reconnects after two seconds. A call in that window fails cleanly. |
| Chrome exits mid-call | The pending call gets "chrome went away" and the host exits, removing its socket. |
| A stale socket from a killed host | The next host removes it on start. |
