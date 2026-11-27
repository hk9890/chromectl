# AGENTS.md — chromectl routing

## Repository purpose

chromectl controls Chrome tabs and tab groups from the command line: list,
activate, open and close a tab; list, activate and add to a group. Chrome
shows its tabs only to an extension, so the product is an MV3 extension plus a
Go binary that Chrome starts as its native messaging host; every CLI call is a
one-shot client of that host's Unix socket. Go, no dependencies outside the
standard library.

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
The design set states what each command, field and process is for, and what
the product leaves out.

### Changing a decided design

**MUST read the records in [docs/decisions/](docs/decisions/) that bear on the
change before editing `cmd/`, `internal/`, `extension/`, `embed.go` or
`.goreleaser.yaml`.** Each holds a decision in force, and a change against one is the user's call — name
the record and ask.

### Testing and verification

**MUST read [docs/TESTING.md](docs/TESTING.md) before writing a test, before
your first `go test`, `mise run test*`, `mise run quality` or `mise run drive`,
and before reporting a change as green.** The layer a test belongs in decides
whether it can run without a Chrome.

### Reviewing changes

**MUST read [docs/REVIEWING.md](docs/REVIEWING.md) before your first
`gh pr view`, `gh pr diff` or `git diff` of a change under review, and before
calling a review skill.** It carries the rules a finished change is held to:
what happens to Chrome's error text on the way to the caller, and what the one
sentinel error must say.

### Cutting a release

**MUST read [docs/RELEASING.md](docs/RELEASING.md) before creating a tag,
dispatching or editing `release.yml`, or editing `CHANGELOG.md` or
`.goreleaser.yaml`.**
The dispatched ref must be the tagged commit, and the live Chrome check comes
before the tag.

### Running chromectl by hand

**MUST read [docs/RUNNING.md](docs/RUNNING.md) before running anything that
starts the host, installs the manifest, or touches the user's Chrome.** One
substrate runs headless; the other is the user's live browser.

### Writing project docs

**MUST read [docs/DOCUMENTING.md](docs/DOCUMENTING.md) AND invoke the
`instruction-writing:writing-project-docs` skill before creating or editing ANY
Markdown file.** Some gaps in the doc set are deliberate.
