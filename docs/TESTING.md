# Testing

## Layers

| Layer | Needs | Location | Command |
|---|---|---|---|
| Framing | nothing | `internal/native/*_test.go` | `mise run test` |
| Host against a fake extension | nothing | `internal/hostd/hostd_test.go` | `mise run test` |
| The built binary on real pipes | Python 3 | `scripts/fakechrome.py` | `mise run drive` |
| Real Chrome | the user's browser and display | none automated | by hand, [RUNNING.md](RUNNING.md) |

The fake extension in `hostd_test.go` plays Chrome's side of the pipe: it reads
framed requests and answers with the same `seq`. It is the pattern for any new
command: one test per command, asserting what the extension received and what
the client got back.

## Gate

`mise run quality` runs `fmt:check`, `vet` and `test:race` three times. It is
the bar for "green"; a plain `go test` is not.

## Before reporting a change as green

- `mise run quality` passes.
- `mise run drive` passes when the change touched `cmd/`, `internal/hostd` or
  `internal/client`; it is the only layer that runs the real binary.
- A change to `extension/background.js` has been loaded into a real Chrome and
  `chromectl list` answered, or the report says it was not. The Go tests cannot
  see the JavaScript.
