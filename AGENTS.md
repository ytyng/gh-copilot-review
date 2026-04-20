# AGENTS.md

Project context for AI coding assistants (Claude, Copilot, Cursor, etc.).
`CLAUDE.md` is a symlink to this file.

## What this project is

A `gh` CLI extension (written in Go) that wraps GitHub Copilot code review:

- `gh copilot-review request <pr>` — add Copilot as a reviewer
- `gh copilot-review wait <pr>`    — poll until Copilot posts a new review
- `gh copilot-review status <pr>`  — one-shot state snapshot (`--json` supported)

Public behavior, flags, and exit codes are documented in `README.md`.

## Directory layout

```
main.go                       # entry point — calls cmd.Run(os.Args[1:])
cmd/
  root.go                     # subcommand dispatch + ExitError type
  request.go                  # `request` implementation
  wait.go                     # `wait` implementation (exit 124 on timeout)
  status.go                   # `status` implementation (--json)
internal/
  ghapi/
    types.go                  # CopilotLogin, CopilotState, PR/Review types
    copilot.go                # REST client, repo resolution, state classification
  render/
    progress.go               # TTY-aware single-line progress output
.github/workflows/release.yml # cli/gh-extension-precompile@v2 on tag push
```

## Dependencies

| Package                              | Purpose                                            |
| ------------------------------------ | -------------------------------------------------- |
| `github.com/cli/go-gh/v2`            | REST/GraphQL client that reuses `gh` authentication |
| `github.com/mattn/go-isatty`         | TTY detection for progress output                   |
| standard library `flag`              | Subcommand parsing (cobra was considered; see below) |

**No cobra.** Earlier drafts considered `cobra` for subcommands, but with
only three flat commands `flag` is shorter and adds no new dependency.
Each subcommand exposes a `RunXxx(args []string) error` function that could
be adapted to a cobra handler trivially if needed later.

**Why this Go extension exists at all**: the reference implementation in
GitHub blog / community discussions uses `gh api` shell commands. This
extension gives the same behaviour a stable CLI surface, richer exit codes
(notably `124` on timeout), and repeat-request detection in `wait`.

## Build, test, run

```sh
# Build binary in place
go build -o gh-copilot-review ./

# Install the locally-built extension into gh
gh extension install .

# Or, without installing, invoke the binary directly
./gh-copilot-review status 42

# Check formatting and static issues
gofmt -l .
go vet ./...
```

There is no unit-test suite yet (spec explicitly deferred it to a later
iteration). When adding tests, prefer table-driven tests in `_test.go`
files next to the code they cover; do **not** stub `go-gh`'s REST client
behind an interface prematurely — wait until there is a second consumer.

## Release / deploy

Releases are fully automated. Push an annotated tag matching `v*`:

```sh
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

`.github/workflows/release.yml` then:

1. Checks out the tag
2. Invokes `cli/gh-extension-precompile@v2` with `go_version_file: go.mod`
3. Builds pre-compiled binaries for every platform `gh` supports
4. Attaches them to a GitHub release and generates attestations

End users then install with `gh extension install ytyng/gh-copilot-review`
and `gh extension upgrade gh-copilot-review` picks up new tags.

## Non-obvious knowledge (don't rediscover)

1. **Copilot's REST login** is `copilot-pull-request-reviewer[bot]` —
   the literal `[bot]` suffix is part of the string the API returns *and*
   the value to pass to `reviewers[]`. Do not drop it.
2. **Copilot never submits `APPROVED` or `CHANGES_REQUESTED`**. Its
   reviews are always `state: COMMENTED`. Completion cannot be inferred
   from `state` alone — check `requested_reviewers` for the in-progress
   signal, and compare `submitted_at` against a baseline captured at
   `wait` start to distinguish new reviews from historical ones.
3. **Repeat review cycles on the same PR are normal.** Users run
   `request` → `wait` multiple times per PR. `wait` uses the
   `LatestReview.SubmittedAt` of the initial poll as the baseline; any
   Copilot review with a strictly greater `submitted_at` is treated as
   new. Do not break this invariant.
4. **Token sensitivity.** The `requested_reviewers` endpoint with a Bot
   login works with a user PAT per community reports; `GITHUB_TOKEN` or
   some GitHub App tokens may reject it. If `request` ever fails with
   422 for unknown reasons, suspect token scope first.
5. **Prefer reading go-gh source over docs.** Once the module is in the
   local Go module cache (`$(go env GOMODCACHE)/github.com/cli/go-gh/v2@*`),
   grep it directly for signatures instead of fetching online docs. The
   entry points this project uses are `api.DefaultRESTClient`,
   `repository.Parse`, and `repository.Current`.

## Working procedures (repeat these)

- **New feature**: make a branch, write code, run `gofmt -l . && go vet ./...`,
  run a manual smoke test against a real PR, commit.
- **Release**: bump tag, push tag. No manual binary uploads. Verify the
  GitHub Actions run succeeds and the release has assets for
  `darwin-amd64`, `darwin-arm64`, `linux-amd64`, `linux-arm64`, `windows-amd64`.
- **Adding dependencies**: keep the dependency list minimal. Anything that
  duplicates `go-gh` functionality (e.g., another HTTP client, another repo
  resolver) should be rejected.
- **Docs**: any new subcommand or flag must be reflected in both
  `README.md` (user-facing) and this file (if there is non-obvious
  behaviour worth preserving for future AI-assisted work).

## Development environment notes

The default macOS Go TLS stack on this machine has a known issue where
`api.github.com` cert verification fails with
`tls: failed to verify certificate: x509: OSStatus -26276` while `curl`
succeeds. This blocks `go get`, `go mod tidy`, and any runtime GitHub
API call made from a Go binary. Workarounds that have been used in this
repo:

- Build using the system module cache as a file proxy:
  `GOPROXY="file:///Users/ytyng/go/pkg/mod/cache/download" go build ./`
- Keep new dependencies to modules already cached locally to avoid
  triggering a download.

Running the extension against the real GitHub API requires fixing the OS
trust store (Keychain Access → System Roots → verify trust settings for
the relevant root CAs).

## References for new contributors (including AI)

- `go-gh` README and `example_gh_test.go` in the upstream repo
- GitHub changelog: Copilot code review via CLI —
  <https://github.blog/changelog/2026-03-11-request-copilot-code-review-from-github-cli/>
- Community discussion on bot login name —
  <https://github.com/orgs/community/discussions/186152>
- Community discussion on Copilot review state —
  <https://github.com/orgs/community/discussions/171743>

Go 1.26 is relatively new; if you hit language/stdlib questions, prefer
reading the standard library source over guessing. For third-party
libraries (`go-gh`, `cobra` if introduced later, etc.), consult the
`context7` MCP server for current documentation rather than relying on
training-data recall.
