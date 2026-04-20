package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ytyng/gh-copilot-review/internal/ghapi"
)

const statusUsage = `gh copilot-review status <pr>

Show the current Copilot review state for a pull request.

Flags:
  --repo owner/name   Target repository (default: current git remote)
  --json              Emit a JSON object instead of human-readable text
`

type statusJSON struct {
	PR          string  `json:"pr"`
	State       string  `json:"state"`
	SubmittedAt *string `json:"submitted_at,omitempty"`
	ReviewURL   *string `json:"review_url,omitempty"`
	Body        *string `json:"body,omitempty"`
}

// RunStatus implements the `status` subcommand.
func RunStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, statusUsage) }
	var (
		repoFlag string
		asJSON   bool
	)
	fs.StringVar(&repoFlag, "repo", "", "Target repository as owner/name")
	fs.BoolVar(&asJSON, "json", false, "Emit JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return &ExitError{Code: 0}
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("status: expected 1 positional argument <pr>, got %d", fs.NArg())
	}
	pr, err := strconv.Atoi(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("parse PR number: %w", err)
	}

	repo, err := ghapi.ResolveRepo(repoFlag)
	if err != nil {
		return fmt.Errorf("resolve repository: %w", err)
	}
	client, err := ghapi.NewRESTClient()
	if err != nil {
		return err
	}
	status, err := ghapi.GetCopilotStatus(client, repo, pr)
	if err != nil {
		return err
	}

	if asJSON {
		return printStatusJSON(status)
	}
	printStatusHuman(status)
	return nil
}

func printStatusJSON(status *ghapi.CopilotStatus) error {
	out := statusJSON{
		PR:    status.PullRequest.HTMLURL,
		State: string(status.State),
	}
	if status.LatestReview != nil {
		ts := status.LatestReview.SubmittedAt.UTC().Format("2006-01-02T15:04:05Z")
		out.SubmittedAt = &ts
		out.ReviewURL = &status.LatestReview.HTMLURL
		out.Body = &status.LatestReview.Body
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printStatusHuman(status *ghapi.CopilotStatus) {
	fmt.Printf("pr: %s\n", status.PullRequest.HTMLURL)
	fmt.Printf("state: %s\n", status.State)
	if status.LatestReview != nil {
		fmt.Printf("submitted_at: %s\n", status.LatestReview.SubmittedAt.UTC().Format("2006-01-02T15:04:05Z"))
		fmt.Printf("review: %s\n", status.LatestReview.HTMLURL)
	}
}
