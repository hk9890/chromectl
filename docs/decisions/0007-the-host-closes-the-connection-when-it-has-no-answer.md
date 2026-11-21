# 0007: The host closes the connection without a reply when a request may have reached Chrome and no answer came

An error reply carries a reason: Chrome's, the extension's, or the host's where the request
provably never reached Chrome. No reply means that Chrome may have acted. The caller tells the two
apart by the exit code, 1 or 4, without reading the text.

Rejected: an error reply with the text "chrome went away" for a call that may have reached Chrome,
because the caller cannot tell it from Chrome's own error without reading the text.

Rejected: a `lost: true` flag on that error reply, because a closed connection says the same with
no new field, and the CLI already handles a connection that closes early.
