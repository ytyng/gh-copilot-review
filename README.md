# gh-copilot-review

A `gh` CLI extension that drives GitHub Copilot code review from the command
line. Request a review, poll until it finishes, and pipe the review body into
other tooling — without clicking through the web UI.

## Install

```sh
gh extension install ytyng/gh-copilot-review
```

Upgrade later with `gh extension upgrade gh-copilot-review`.

## Commands

All commands take a pull request number as their single positional argument.
The target repository defaults to the current git remote and can be
overridden with `--repo owner/name`.

### `gh copilot-review request <pr>`

Adds `copilot-pull-request-reviewer[bot]` to the PR's requested reviewers.
Prints the PR URL to stdout. If Copilot is already requested, the command is
a no-op: the "already requested" notice goes to stderr and the PR URL is
still printed to stdout so pipelines stay uniform.

```sh
gh copilot-review request 42
# → https://github.com/owner/repo/pull/42
```

### `gh copilot-review wait <pr>`

Polls until Copilot posts a *new* review, then prints the review body to
stdout and the review URL to stderr. A "new" review is one whose
`submitted_at` is later than the latest Copilot review observed at the
moment `wait` started — so running `request` followed by `wait` works
repeatedly on the same PR across multiple review cycles.

| Flag         | Default | Description                           |
| ------------ | ------- | ------------------------------------- |
| `--interval` | `10s`   | Poll interval (any `time.Duration`)   |
| `--timeout`  | `10m`   | Give up after this duration           |
| `--repo`     | —       | `owner/name` to override git remote   |

Exit codes:

- `0` — a new review was detected; body printed to stdout
- `1` — an error occurred
- `124` — `--timeout` elapsed without detecting a new review (GNU `timeout`
  convention)

### `gh copilot-review status <pr>`

One-shot snapshot of Copilot's current state on the PR. The state is one of:

- `not_requested` — Copilot has never reviewed and is not currently requested
- `reviewing` — Copilot is in the PR's `requested_reviewers`
- `completed` — Copilot is no longer requested and has at least one review
  on record

```sh
gh copilot-review status 42
# pr: https://github.com/owner/repo/pull/42
# state: completed
# submitted_at: 2026-04-19T12:34:56Z
# review: https://github.com/owner/repo/pull/42#pullrequestreview-xxxxx
```

The human output intentionally omits the review body; pipe through `--json`
(or use `wait`) when you need the text itself.

```json
gh copilot-review status 42 --json
{
  "pr": "https://github.com/owner/repo/pull/42",
  "state": "completed",
  "submitted_at": "2026-04-19T12:34:56Z",
  "review_url": "https://github.com/owner/repo/pull/42#pullrequestreview-xxxxx",
  "body": "..."
}
```

## How it works

The extension talks to GitHub's REST API through [go-gh][go-gh], which reuses
the authentication of the host `gh` binary. Copilot is added to a PR by
`POST /repos/{owner}/{repo}/pulls/{pr}/requested_reviewers` with
`reviewers: ["copilot-pull-request-reviewer[bot]"]`. Review state is derived
from `GET /repos/.../pulls/{pr}` (for `requested_reviewers`) combined with
`GET /repos/.../pulls/{pr}/reviews` (for the Copilot review body and
timestamp). Copilot always submits reviews with `state: COMMENTED`, so state
alone is not enough — presence in `requested_reviewers` is the authoritative
"in progress" signal.

[go-gh]: https://github.com/cli/go-gh

## Requirements

- `gh` CLI authenticated against github.com (`gh auth login`).
- The authenticated user must have permission to request reviewers on the
  target PR. Community reports indicate the `copilot-pull-request-reviewer[bot]`
  reviewers payload works with a user PAT but may not work with
  `GITHUB_TOKEN` or some GitHub App tokens.

## License

MIT (see [LICENSE](LICENSE) if present).
