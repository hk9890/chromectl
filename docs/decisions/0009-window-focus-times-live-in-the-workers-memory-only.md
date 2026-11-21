# 0009: The extension keeps the window focus times in the worker's memory only

The extension's service worker holds an open native messaging port, which keeps it alive as long as
Chrome. So the times that `tab list` prints as `windowFocusedAt` are lost only when the extension
reloads or Chrome restarts. After a restart the windows that the times named are gone. After a
reload a window that does not have the focus is unknown until it gets it.

Rejected: `chrome.storage.session`, because it needs the `storage` permission.

Rejected: IndexedDB, because it survives a restart of Chrome, which numbers its windows anew, so
old times would attach to new windows unless the extension cleared them at each start.
