# Changelog

## v0.2.0

**Action required:** after the upgrade, run `chromectl install`, then click
reload on the chromectl card at `chrome://extensions` in every profile that
loads it. Until then, chromectl finds no host, or answers that the extension
is older than chromectl.

**Action required:** a script that reads chromectl output must change:

- Tab, window and group ids are now `<profile>:<number>`, for example
  `3f9a12bc:812`, as `list` prints them. A bare number such as
  `activate 812`, `close 812` or `open <url> 5` is a usage error.
- In `--json` output, `id` and `window` are strings, not numbers.
- `list` has two new columns: `group` after `window`, and `browser` after
  `active`. `list --json` has the same two new fields.
- Exit codes now tell the failures apart: 1 an error from Chrome or
  chromectl, 2 bad usage, 4 no reply, 5 no host. Before, wrong arguments and
  bad flags exited 1.

New:

- chromectl reaches every profile of every browser that loads the extension.
  `list` prints the tabs of all of them. Before, only the profile that started
  last answered. With several profiles, `open`, and `activate-group` when it
  creates a group, need a window id.
- `chromectl groups` prints every open tab group: id, window and title.
- `chromectl activate-group <title> <url> [window]` raises the tab group
  titled `<title>` and activates its tab at `<url>`, else a tab under `<url>`,
  else its last used tab. Without such a group, it opens `<url>` in a new
  group with that title. Two quick calls create one group, not two.
- Each call to Chrome gives up after 5 seconds. `--timeout` changes the
  limit.

Fixed:

- chromectl answers after Chrome restarts, without a reload of the extension.
- chromectl answers after a reload of the extension. Before, it reported no
  host until the next reload.
- A command no longer fails when another command gets a bad answer from
  Chrome. A command in flight when Chrome closes exits 4 (no reply) and no
  longer hangs.
- Id 0 is refused. Before, `activate 0` acted on the current tab.
- `install` no longer leaves the extension missing when the copy fails.
- Without `XDG_RUNTIME_DIR`, for example under cron, chromectl finds the host
  in `/run/user/<uid>`.

## v0.1.0

The first release. chromectl lists, activates, opens and closes Chrome tabs
from the command line, through a small extension that Chrome runs.

- `chromectl list` prints every tab: id, window, active, title and URL,
  tab-separated. `list --json` prints them as a JSON array.
- `chromectl activate <tab-id>` brings the tab to the front and focuses its
  window.
- `chromectl open <url> [window]` opens a new tab and prints its id and window.
  `open --json` prints them as a JSON object.
- `chromectl close <tab-id>` closes the tab.
- `chromectl install` copies chromectl and its extension to
  `~/.local/share/chromectl/` and registers it with every installed Chrome,
  Chrome beta and unstable, Chromium and Brave. Install a browser later, and
  run `chromectl install` again. Load `~/.local/share/chromectl/extension` once
  as an unpacked extension at `chrome://extensions`, with Developer mode on.
- `chromectl version` prints its version, commit and build date.
