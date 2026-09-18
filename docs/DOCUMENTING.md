# Documenting

The standard is the `instruction-writing:writing-project-docs` skill. Below is
this repository's delta, which wins where the two disagree.

## Register

`docs/*.md` is command register: imperative, one instruction per line. The one
paragraph of rationale in the set is the constraint at the top of
[OVERVIEW.md](OVERVIEW.md); it stays because a plan that misses it cannot be
built.

## Deliberately absent

Create these only when there is something to record. Each would cost a load
and return nothing:

| File | Create when |
|---|---|
| `docs/RELEASING.md` | a release exists: a tag, a built artifact, a Web Store listing |
| `docs/MONITORING.md` | the host writes a log file; today its stderr is Chrome's, see [RUNNING.md](RUNNING.md) |
| `docs/REVIEWING.md` | this repository has a review rule the `code-review` skill cannot know |
| `CONTRIBUTING.md` | someone other than the author builds from source |
| `docs/design/` | a decision is superseded; until then the constraint in OVERVIEW.md is the whole design |

## README scope

`README.md` says what chromectl is, how to install it, and how to use it.
Building, testing, driving and the wire format are `docs/`, not README.
