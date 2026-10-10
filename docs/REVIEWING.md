# Reviewing

Project-specific review rules. The generic review is the `code-review` and
`project-review-*` skills'; this file holds only what they cannot know, and wins
where the two disagree.

## Quality rules

Each rule is a condition to check on the diff, with the correct form.

- **Chrome's error text**: Chrome's error text is the caller's error, unchanged:
  `No tab with id: 99.` reaches the CLI as-is with exit code 1. Flag a wrap or a
  reword of `reply.Error` between the extension and the caller; the correct form is
  `errors.New(reply.Error)`, as in `client.Call`. The `chromectl:` prefix that
  `main` prints before each error is the CLI's own and is no finding.
- **Sentinel errors**: `client.ErrNoHost` is the one sentinel: the socket is absent.
  Flag a second package-level error variable; the correct form is an error made
  where it occurs, as `fmt.Errorf("read reply: %w", err)` in `client.Call`.
- **`ErrNoHost` message**: the message of `client.ErrNoHost` lists the three causes,
  in the order to check them: Chrome is not running, the extension is not loaded,
  `chromectl install` was not run. Flag a message that drops a cause or changes
  the order.
