# Changelog

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
  `~/.local/share/chromectl/` and registers it with Chrome, Chrome beta and
  unstable, Chromium and Brave. Load `~/.local/share/chromectl/extension` once
  at `chrome://extensions`.
- `chromectl version` prints the installed version.
