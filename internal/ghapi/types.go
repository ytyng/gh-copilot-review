package ghapi

import "time"

// CopilotLogin is the login to pass to the POST requested_reviewers endpoint
// and the login GitHub reports on reviews[].user. The trailing `[bot]` is
// part of the login string in these contexts.
//
// Confirmed via community report:
// https://github.com/orgs/community/discussions/186152
const CopilotLogin = "copilot-pull-request-reviewer[bot]"

// CopilotRequestedReviewerLogin is the login GitHub returns inside
// pulls/{n}.requested_reviewers for Copilot. It is the human-readable
// "Copilot" rather than the bot slug — confirmed empirically against live
// PRs. Both forms must be checked when detecting the reviewing state,
// because GitHub may normalize this representation in the future.
const CopilotRequestedReviewerLogin = "Copilot"

// CopilotState is the high-level state of Copilot on a PR.
type CopilotState string

const (
	StateNotRequested CopilotState = "not_requested"
	StateReviewing    CopilotState = "reviewing"
	StateCompleted    CopilotState = "completed"
)

// User mirrors the GitHub REST user/bot object (subset we care about).
type User struct {
	Login string `json:"login"`
	Type  string `json:"type"` // "User" or "Bot"
}

// PullRequest is a subset of the pulls endpoint response.
type PullRequest struct {
	Number             int    `json:"number"`
	HTMLURL            string `json:"html_url"`
	RequestedReviewers []User `json:"requested_reviewers"`
}

// Review mirrors an element of the pulls/{n}/reviews response.
type Review struct {
	ID          int64     `json:"id"`
	User        User      `json:"user"`
	State       string    `json:"state"` // COMMENTED / APPROVED / CHANGES_REQUESTED / DISMISSED
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submitted_at"`
	HTMLURL     string    `json:"html_url"`
}

// CopilotStatus is the aggregated view surfaced to the CLI.
type CopilotStatus struct {
	PullRequest  *PullRequest
	State        CopilotState
	LatestReview *Review // non-nil if Copilot has ever reviewed this PR
}
