# Coding

```bash
mise run build      # ./bin/chromectl
mise run fmt
mise run quality    # the pre-commit gate
```

## Two files must agree

`internal/proto/proto.go` and `extension/background.js` are the two ends of
one wire. A field renamed on one side is a silent failure on the other: the
extension answers `{seq, error: "unknown command"}` or acts on `undefined`.
Change them in the same commit, and add the case to
`internal/hostd/hostd_test.go`.

## The host owns stdout

In `chromectl host`, stdout is Chrome's pipe. One stray byte there desyncs the
framing and Chrome drops the host. Log to stderr, via `log`, and nothing else.
`native.Write` is the only writer.

## Adding a command

1. Add the async function to `commands` in `extension/background.js`.
2. Add the fields it reads to `proto.Request`, flat, with the JSON name the
   extension reads.
3. Add the `case` in `cmd/chromectl/main.go`. Read a tab or window id with
   `parseID`, and route the call to the host it names with `hostFor`.
4. Add a case to `internal/hostd/hostd_test.go` with the fake extension.

The extension keeps the `tabs` and `nativeMessaging` permissions and no other.
A command that needs more is a different product.

## Identity

- The extension id `mpnpeobidlmllklmdpnkhhhblkbhpmcp` is derived from the
  `key` in `extension/manifest.json`. The `key` never changes; every installed
  native messaging manifest allows that id and nothing else.
- Tab and window ids are Chrome's numbers and reset on restart. The CLI prints
  and reads them as `<profile>:<number>`; the host and the extension see only
  the number. Matching a tab by URL is the caller's job.
- A host's identity is its socket name, which `client.Hosts` parses. Change
  the name in `internal/hostd` and `internal/client` together.
