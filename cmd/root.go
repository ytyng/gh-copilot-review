package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const rootUsage = `gh copilot-review - manage GitHub Copilot code review from the CLI.

Usage:
  gh copilot-review <command> [arguments]

Commands:
  request  Request Copilot code review for a pull request.
  wait     Wait until Copilot posts a review.
  status   Show the current Copilot review state for a pull request.

Run "gh copilot-review <command> -h" for command-specific flags.
`

// Run dispatches subcommands. It returns an exit code.
func Run(args []string) int {
	if err := dispatch(args); err != nil {
		return reportError(os.Stderr, err)
	}
	return 0
}

// reportError prints err and maps it to a process exit code. Split out from
// Run so the mapping can be tested without dispatching a subcommand (every
// subcommand talks to the GitHub API).
func reportError(w io.Writer, err error) int {
	var exit *ExitError
	if errors.As(err, &exit) {
		if exit.Err != nil {
			fmt.Fprintln(w, "gh-copilot-review:", exit.Err)
		}
		return exit.Code
	}
	fmt.Fprintln(w, "gh-copilot-review:", err)
	return 1
}

func dispatch(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, rootUsage)
		return errors.New("no command specified")
	}
	switch args[0] {
	case "request":
		return RunRequest(args[1:])
	case "wait":
		return RunWait(args[1:])
	case "status":
		return RunStatus(args[1:])
	case "-h", "--help", "help":
		fmt.Fprint(os.Stdout, rootUsage)
		return nil
	default:
		fmt.Fprint(os.Stderr, rootUsage)
		return fmt.Errorf("unknown command: %q", args[0])
	}
}

// ExitError carries an explicit process exit code. Used by `wait` to report
// a timeout via exit code 124 without tripping the default "error → 1" path.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }
