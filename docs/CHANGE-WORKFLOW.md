# Change workflow

## Branch and PR

Every change lands through a branch and a PR against `main`. `main` itself
takes no direct commit.

```bash
git switch -c <type>/<what>   # feat/, fix/, docs/, chore/
mise run quality
# commit, per Commits below
git push -u origin HEAD
gh pr create --fill
```

Call the Skill tool with `worktree-flow:worktree-review` and the PR number
before the merge; it reviews against [REVIEWING.md](REVIEWING.md).
Squash-merge; delete the branch after.

## Commits

Call the Skill tool with `commit-commands:commit` for the standard flow.

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
