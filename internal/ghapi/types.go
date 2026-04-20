package ghapi

import "time"

// CopilotLogin is the GitHub `login` value reported by the REST API for the
// Copilot PR reviewer bot. The trailing `[bot]` is part of the login string
// for this bot user and must be included when calling the
// `requested_reviewers` endpoint.
//
// Confirmed via community report:
// https://github.com/orgs/community/discussions/186152
const CopilotLogin = "copilot-pull-request-reviewer[bot]"

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
