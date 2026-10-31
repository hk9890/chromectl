# chromectl

Control Chrome tabs from the command line: list, activate, open, close, and
find or create a tab group by its title.

Chrome shows its tabs only to an extension. chromectl is a tiny extension with
the `tabs` and `tabGroups` permissions and a Go binary. Chrome starts the binary as the
extension's native messaging host and keeps it alive as long as Chrome runs;
every command is a one-shot client of that host over a Unix socket. There is no
daemon to start or enable.

```
chromectl list                 # id  window  group  active  browser  title  url
chromectl groups               # id  window  title
chromectl activate <tab-id>    # activate the tab and focus its window
chromectl activate-group <title> <url> [window]
                               # raise the group with that title, or create it with url
chromectl open <url> [window]  # open a new tab, in that window or the last focused one
chromectl close <tab-id>
```

`list --json` prints a JSON array of
`{id, window, group, active, browser, profile, title, url}`; `groups --json`
`{id, window, title}`; `open --json <url>` prints `{id, window}`, and
`activate-group --json` `{id, window, created, title}`. Put `--json` before the
arguments. Give `open` a full URL: `https://www.orf.at`, not `www.orf.at`.

`activate-group chromectl https://github.com/hk9890/chromectl` finds the open
tab group titled exactly `chromectl`, expands it, and activates its tab at that
URL, else one under it, else the tab you used last. Without such a group, it
opens the URL in a new group with that title. It prints the tab's title, so a
script can raise the window with its window manager; on GNOME Wayland, Chrome
cannot raise its own window.

Every command that talks to Chrome gives up after 5 seconds; `--timeout 2s`
sets another limit. Exit codes: 1 an error from Chrome, 2 bad usage, 4 no reply
in time (the command may still have taken effect), 5 Chrome or the extension
is not running.

A tab, window or group id is `<profile>:<number>`, for example `3f9a12bc:812`: the
browser profile that owns it, and Chrome's own number, which resets when
Chrome restarts. `list` shows the tabs of every profile of every browser that
has the extension loaded. Find a tab by its URL with `chromectl list`, then act
on its id. With more than one profile open, give `open` a window id from
`list`, so it knows which profile to open the tab in; the same holds for
`activate-group` when it creates a group.

## Install

Linux only, x64 and arm64.

With [mise](https://mise.jdx.dev), which reads the releases page and puts
`chromectl` on your PATH:

```
mise use -g github:hk9890/chromectl
chromectl install
```

Or download `chromectl_<version>_linux_<x64|arm64>.tar.gz` from the
[releases page](https://github.com/hk9890/chromectl/releases) and unpack the
binary into a directory on your PATH:

```
tar -xzf chromectl_<version>_linux_x64.tar.gz -C ~/.local/bin chromectl
chromectl install
```

`chromectl install` copies the binary and the extension it carries to
`~/.local/share/chromectl/` and writes the native messaging manifest into
every Chrome-family config directory under `~/.config`: Chrome, Chrome beta
and unstable, Chromium, Brave.

Then load `~/.local/share/chromectl/extension` as an unpacked extension at
`chrome://extensions` with Developer mode on. Its id is
`mpnpeobidlmllklmdpnkhhhblkbhpmcp`, fixed by the `key` in its manifest, so the
manifest already allows it.

`chromectl list` now works. If it says no host is listening, the extension's
service worker console at `chrome://extensions` shows the reason.

To upgrade, run `mise upgrade github:hk9890/chromectl`, or unpack the new
archive the same way, then run `chromectl install` and click reload on the
chromectl card at `chrome://extensions`.

## License

MIT.
