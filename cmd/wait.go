package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ytyng/gh-copilot-review/internal/ghapi"
	"github.com/ytyng/gh-copilot-review/internal/render"
)

const waitUsage = `gh copilot-review wait <pr>

Wait until Copilot posts a new review on the given pull request. The command
snapshots the latest Copilot review timestamp on entry and returns as soon as
a review with a *newer* submitted_at appears. This makes repeated wait calls
on the same PR work across multiple review cycles.

Exit codes:
  0    A new Copilot review was detected. The review body is printed to stdout.
  1    An error occurred.
  124  The --timeout elapsed without detecting a new review.

Flags:
  --repo owner/name   Target repository (default: current git remote)
  --interval 10s      Polling interval (any duration accepted by time.ParseDuration)
  --timeout  10m      Give up after this duration
`

// RunWait implements the `wait` subcommand.
func RunWait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, waitUsage) }
	var (
		repoFlag string
		interval time.Duration
		timeout  time.Duration
	)
	fs.StringVar(&repoFlag, "repo", "", "Target repository as owner/name")
	fs.DurationVar(&interval, "interval", 10*time.Second, "Polling interval")
	fs.DurationVar(&timeout, "timeout", 10*time.Minute, "Timeout")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return &ExitError{Code: 0}
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("wait: expected 1 positional argument <pr>, got %d", fs.NArg())
	}
	pr, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("parse PR number: %w", err)
	}
	if interval <= 0 {
		return fmt.Errorf("--interval must be positive, got %s", interval)
	}
	if timeout <= 0 {
		return fmt.Errorf("--timeout must be positive, got %s", timeout)
	}

	repo, err := ghapi.ResolveRepo(repoFlag)
	if err != nil {
		return fmt.Errorf("resolve repository: %w", err)
	}
	client, err := ghapi.NewRESTClient()
	if err != nil {
		return err
	}

	// Baseline: timestamp of the most recent Copilot review *before* we start
	// waiting. Anything strictly later than this is considered "new".
	initial, err := ghapi.GetCopilotStatus(client, repo, pr)
	if err != nil {
		return err
	}
	var baseline time.Time
	if initial.LatestReview != nil {
		baseline = initial.LatestReview.SubmittedAt
	}

	progress := render.NewProgress()
	defer progress.Done()
	progress.Update(string(initial.State))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return &ExitError{Code: 124, Err: fmt.Errorf("timed out after %s waiting for Copilot review on %s/%s#%d", timeout, repo.Owner, repo.Name, pr)}
		case <-ticker.C:
		}

		status, err := ghapi.GetCopilotStatus(client, repo, pr)
		if err != nil {
			// Transient API errors shouldn't kill the wait. Clear the
			// progress line before printing so the error appears on its own
			// line on both TTY and non-TTY output, then redisplay progress.
			progress.Done()
			fmt.Fprintf(os.Stderr, "(poll error, retrying): %v\n", err)
			progress.Update("error")
			continue
		}
		progress.Update(string(status.State))

		// Require StateCompleted (not just a fresh SubmittedAt) to guard
		// against a transient window where `reviews` has a new entry but
		// `requested_reviewers` hasn't been cleared yet.
		if status.State == ghapi.StateCompleted &&
			status.LatestReview != nil &&
			status.LatestReview.SubmittedAt.After(baseline) {
			// Clear the progress line explicitly so the review output
			// doesn't get concatenated onto it on a TTY. The deferred
			// Done() still runs as a safety net.
			progress.Done()
			fmt.Fprintf(os.Stderr, "review: %s\n", status.LatestReview.HTMLURL)
			fmt.Println(status.LatestReview.Body)
			return nil
		}
	}
}
