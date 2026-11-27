# Documenting

The standard is the `instruction-writing:writing-project-docs` skill. Below is
this repository's delta, which wins where the two disagree.

## Two trees, two owners

| Tree | Owns | Register |
|---|---|---|
| `docs/*.md` | How to operate on this repository now | Command register: imperative, one instruction per line |
| `docs/design/` | What the system is intended to be, and why | Prose. A design doc argues; rationale is its content, not padding. |

`docs/design/README.md` states which design document owns what. Where a record
in `docs/decisions/` holds the reason for a choice, link the record from the
design doc.

## Deliberately absent

Create these only when there is something to record. Each would cost a load
and return nothing:

| File | Create when |
|---|---|
| `docs/MONITORING.md` | the host writes a log file; today its stderr is Chrome's, see [RUNNING.md](RUNNING.md) |
| `CONTRIBUTING.md` | someone other than the author builds from source |
