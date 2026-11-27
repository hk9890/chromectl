# 0012: The CLI reads an unknown command from the extension as an extension to reload, and exits 5

After an upgrade the host Chrome started keeps running the old binary beside the old extension
until the user reloads it, so only the new CLI sees the skew. The CLI turns the extension's reply
`unknown command <x>`, and the host's own stale answer, into "reload it at chrome://extensions",
exit 5: as in wctl, the extension cannot be used, and nothing reached Chrome.

Rejected: a version check in the host, because the old host never sees the new binary.

Rejected: a version field on the wire, because the old extension does not send it.

Rejected: a new exit code 3, because wctl's 3 means "the shell refused".
