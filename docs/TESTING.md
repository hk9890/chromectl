# Testing

## Layers

| Layer | Needs | Location | Command |
|---|---|---|---|
| Framing | nothing | `internal/native/*_test.go` | `mise run test` |
| Host against a fake extension | nothing | `internal/hostd/hostd_test.go` | `mise run test` |
| `install` against temporary directories | nothing | `cmd/chromectl/install_test.go` | `mise run test` |
| The built binary on real pipes | Python 3 | `scripts/fakechrome.py` | `mise run drive` |
| Real Chrome | the user's browser and display | none automated | by hand, [RUNNING.md](RUNNING.md) |

The fake extension in `hostd_test.go` plays Chrome's side of the pipe: it reads
framed requests and answers with the same `seq`. It is the pattern for any new
command: one test per command, asserting what the extension received and what
the client got back.

Start every test that calls `install` with `isolate(t)` from `install_test.go`.
It points `HOME` and the XDG directories at temporary directories, so the test
cannot overwrite the manifest of the user's real Chrome.

## Gate

`mise run quality` is the bar for "green"; a plain `go test` is not. Its steps
are the `depends` of `[tasks.quality]` in `.mise.toml`.

CI runs the same checks on every push in `.github/workflows/ci.yml`, plus
`mise run vuln`, which needs the network.

## Before reporting a change as green

- `mise run quality` passes.
- `mise run drive` passes when the change touched `cmd/`, `internal/hostd` or
  `internal/client`; it is the only layer that runs the real binary.
- A change to `extension/background.js` has been loaded into a real Chrome and
  `chromectl list` answered, or the report says it was not. The Go tests cannot
  see the JavaScript.
