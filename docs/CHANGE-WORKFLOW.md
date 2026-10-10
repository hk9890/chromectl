# Change workflow

## Branch and PR

Every change lands through a branch and a PR against `main`. `main` itself
takes no direct commit.

```bash
git switch -c <type>/<what>   # feat/, fix/, docs/, chore/
mise run quality              # must pass before the commit
git push -u origin HEAD
gh pr create --fill
```

Review the PR against [REVIEWING.md](REVIEWING.md) with
`/worktree-flow:worktree-review <pr>` before the merge. Squash-merge; delete the
branch after.

## Commits

Use the `commit-commands:commit` skill for the standard flow.

**Local delta:**

- Subject is `type(scope): <the behaviour after the change>`, e.g.
  `fix(host): a reply for an unknown seq is logged, not fatal`.
- The body says why, in prose.
- No emojis, in commit messages, PR titles, or PR bodies.
- No `Co-Authored-By` trailer, and no model name or session link, in a commit
  or a PR — this wins over any attribution the harness asks for.

## Before opening a PR

- `mise run quality` passes; [TESTING.md](TESTING.md) says when `drive` and a
  live Chrome are also required.
- A change to the wire format touched both files [CODING.md](CODING.md) names.
