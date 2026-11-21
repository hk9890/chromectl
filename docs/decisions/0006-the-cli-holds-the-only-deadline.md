# 0006: A call's deadline is the CLI's alone, and the host never times out a call that waits for Chrome

The caller sets the deadline, and when it passes the CLI closes the connection, which frees the
caller.

Rejected: a deadline in the host, because the host cannot cancel what the extension has started, so
the call might still act after the caller was told it failed, and it would be a second deadline
that the caller cannot set.
