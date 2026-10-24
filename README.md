# chromectl

Control Chrome tabs from the command line: list, activate, open, close.

Chrome shows its tabs only to an extension. chromectl is a tiny extension with
the `tabs` permission and a Go binary. Chrome starts the binary as the
extension's native messaging host and keeps it alive as long as Chrome runs;
every command is a one-shot client of that host over a Unix socket. There is no
daemon to start or enable.

```
chromectl list                 # id  window  active  title  url
chromectl activate <tab-id>    # activate the tab and focus its window
chromectl open <url> [window]  # open a new tab, in that window or the last focused one
chromectl close <tab-id>
```

`list --json` prints a JSON array of `{id, window, active, title, url}`;
`open --json <url>` prints `{id, window}`. Put `--json` before the arguments.
Give `open` a full URL: `https://www.orf.at`, not `www.orf.at`.

Tab ids are Chrome's own and reset when Chrome restarts. Find a tab by its URL
with `chromectl list`, then act on its id.

## Install

With [mise](https://mise.jdx.dev), which reads the releases page and puts
`chromectl` on your PATH:

```
mise use -g github:hk9890/chromectl
chromectl install
```

Or download `chromectl_<version>_linux_<x64|arm64>.tar.gz` from the
[releases page](https://github.com/hk9890/chromectl/releases) and unpack it
into a directory on your PATH:

```
tar -xzf chromectl_<version>_linux_x64.tar.gz -C ~/.local/bin
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
