# Documenting

The standard is the `instruction-writing:writing-project-docs` skill. Below is
this repository's delta, which wins where the two disagree.

## Two trees, two owners

| Tree | Owns | Register |
|---|---|---|
| `docs/*.md` | How to operate on this repository now | Command register: imperative, one instruction per line |
| `docs/design/` | What the system is intended to be, and why | Prose. A design doc argues; rationale is its content, not padding. |

`docs/design/README.md` states which design document owns what. A rejected
alternative is recorded in `design/architecture.md` where it would otherwise
be proposed again. There is no decision log until the first decision is
superseded; when that happens, create `docs/design/decisions.md` and move the
rejected alternatives into it.

## Deliberately absent

Create these only when there is something to record. Each would cost a load
and return nothing:

| File | Create when |
|---|---|
| `docs/MONITORING.md` | the host writes a log file; today its stderr is Chrome's, see [RUNNING.md](RUNNING.md) |
| `CONTRIBUTING.md` | someone other than the author builds from source |
| `docs/design/decisions.md` | a design decision is superseded |

## README scope

`README.md` says what chromectl is, how to install it, and how to use it.
Building, testing, driving and the wire format are `docs/`, not README.
