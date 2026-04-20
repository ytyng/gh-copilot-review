package cmd

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/ytyng/gh-copilot-review/internal/ghapi"
)

const requestUsage = `gh copilot-review request <pr>

Request Copilot code review on the given pull request number.

Flags:
  --repo owner/name   Target repository (default: current git remote)
`

// RunRequest implements the `request` subcommand.
func RunRequest(args []string) error {
	fs := flag.NewFlagSet("request", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, requestUsage) }
	var repoFlag string
	fs.StringVar(&repoFlag, "repo", "", "Target repository as owner/name")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return &ExitError{Code: 0}
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("request: expected 1 positional argument <pr>, got %d", fs.NArg())
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

	already, prURL, err := ghapi.RequestCopilotReview(client, repo, pr)
	if err != nil {
		return err
	}
	if already {
		fmt.Fprintf(os.Stderr, "copilot is already requested on %s\n", prURL)
		fmt.Println(prURL)
		return nil
	}
	fmt.Println(prURL)
	return nil
}
