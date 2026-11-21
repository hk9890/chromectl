# 0001: The CLI reaches Chrome only through a native messaging host that Chrome starts

Only an extension can talk to Chrome, and only outward: it calls `connectNative`, and Chrome starts
the host. Chrome starts the host when the browser starts, so no call waits for a process to wake.

Rejected: a localhost WebSocket server that the first CLI call starts, because the extension's MV3
worker sleeps, and its alarms wake it to find the server every 30 seconds at the fastest.

Rejected: the DevTools protocol, because since Chrome 136 `--remote-debugging-port` is ignored on
the default user data directory, which is the user's daily browser.

Rejected: a dependency on brotab, an extension plus native host of the same shape, because its
extension is Manifest V2, which Chrome is retiring, and asks for `<all_urls>` and script execution.
