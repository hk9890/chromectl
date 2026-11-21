# 0004: A host asks the extension for its profile before it listens, and its socket name carries the answer

One read of the socket directory finds every host, with no registry to keep in step.

Rejected: a socket named for the pid only, with the CLI asking each host for its profile, because
every command for one tab then asks every host, and one hung host holds them all.
