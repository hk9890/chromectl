# AGENTS.md — chromectl routing

## Repository purpose

chromectl controls Chrome tabs from the command line: list, activate, open,
close. Chrome shows its tabs only to an extension, so the product is an MV3
extension plus a Go binary that Chrome starts as its native messaging host;
every CLI call is a one-shot client of that host's Unix socket. Go, no
dependencies outside the standard library.

## Use-case routing

Every route below is **mandatory, not advisory**. Load the document BEFORE the
first action of that kind — loading it afterwards does not count, and no route
becomes skippable because the change looks small.

### Any change, commit, branch, PR

**MUST read [docs/CHANGE-WORKFLOW.md](docs/CHANGE-WORKFLOW.md) before the first
edit to ANY file in this repository, and before any git command that writes.**
It owns the branch, commit and gate rules, including what a commit message must
leave out.

### Research, planning — and finding anything at all

**MUST read [docs/OVERVIEW.md](docs/OVERVIEW.md) before your first `rg`,
`grep`, `ls` or `Glob` of the source tree, and before writing any plan.** It
carries the one constraint that shapes the whole design: which side may open a
connection. A plan that gets it wrong cannot be built.

### Coding and file changes

**MUST read [docs/CODING.md](docs/CODING.md) before creating or editing ANY file
under `cmd/`, `internal/`, `extension/` or `scripts/`.** Two files must agree
byte for byte on the wire format, and one process must never write to stdout;
neither is visible from the code.

### Design intent, and any change to the model

**MUST read [docs/design/README.md](docs/design/README.md) before adding a
command, a field on the wire, a permission to the extension, or a process.**
The design set carries the rejected alternatives, so a rejected idea is not
re-proposed as new.

### Testing and verification

**MUST read [docs/TESTING.md](docs/TESTING.md) before writing a test, before
your first `go test` or `mise run test*`, and before reporting a change as
green.** The layer a test belongs in decides whether it can run without a
Chrome.

### Running chromectl by hand

**MUST read [docs/RUNNING.md](docs/RUNNING.md) before running anything that
starts the host, installs the manifest, or touches the user's Chrome.** One
substrate runs headless; the other is the user's live browser.

### Writing project docs

**MUST read [docs/DOCUMENTING.md](docs/DOCUMENTING.md) AND invoke the
`instruction-writing:writing-project-docs` skill before creating or editing ANY
Markdown file.** Some gaps in the doc set are deliberate.
