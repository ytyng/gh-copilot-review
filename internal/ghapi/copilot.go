package ghapi

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

// NewRESTClient returns a REST client using the default gh authentication.
func NewRESTClient() (*api.RESTClient, error) {
	c, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("create REST client: %w", err)
	}
	return c, nil
}

// ResolveRepo honours the --repo override and falls back to the git remote of
// the current directory.
func ResolveRepo(override string) (repository.Repository, error) {
	if override != "" {
		return repository.Parse(override)
	}
	return repository.Current()
}

// fetchPR retrieves only the PR metadata (including requested_reviewers),
// without paginating the full review history.
func fetchPR(c *api.RESTClient, repo repository.Repository, pr int) (*PullRequest, error) {
	var p PullRequest
	path := fmt.Sprintf("repos/%s/%s/pulls/%d", repo.Owner, repo.Name, pr)
	if err := c.Get(path, &p); err != nil {
		return nil, fmt.Errorf("fetch PR %s/%s#%d: %w", repo.Owner, repo.Name, pr, err)
	}
	return &p, nil
}

// GetCopilotStatus fetches the PR and the list of reviews, then classifies
// the Copilot state:
//   - Copilot is in requested_reviewers  → StateReviewing
//   - Copilot has reviewed and is no longer requested → StateCompleted
//   - Otherwise → StateNotRequested
//
// The latest Copilot review (by SubmittedAt) is always returned when present,
// regardless of state. This lets callers detect repeat review cycles by
// comparing timestamps across polls.
func GetCopilotStatus(c *api.RESTClient, repo repository.Repository, pr int) (*CopilotStatus, error) {
	p, err := fetchPR(c, repo, pr)
	if err != nil {
		return nil, err
	}

	reviews, err := listReviews(c, repo, pr)
	if err != nil {
		return nil, err
	}

	status := &CopilotStatus{PullRequest: p, LatestReview: latestCopilotReview(reviews)}
	switch {
	case isCopilotRequested(p.RequestedReviewers):
		status.State = StateReviewing
	case status.LatestReview != nil:
		status.State = StateCompleted
	default:
		status.State = StateNotRequested
	}
	return status, nil
}

// RequestCopilotReview asks GitHub to add Copilot to the PR's requested
// reviewers. Returns already=true only when Copilot is currently in
// requested_reviewers (StateReviewing). When Copilot has reviewed before but
// is no longer requested (StateCompleted), a fresh request is sent — this is
// how repeat review cycles are supported on the same PR.
//
// Uses REST POST /repos/{owner}/{repo}/pulls/{pull_number}/requested_reviewers
// with reviewers=[copilot-pull-request-reviewer[bot]]. Per community report,
// this works only with a user PAT (not GITHUB_TOKEN or some App tokens).
func RequestCopilotReview(c *api.RESTClient, repo repository.Repository, pr int) (already bool, prURL string, err error) {
	// Only the PR metadata is needed here; paginating the full review history
	// would be wasteful on large PRs when we just want to check whether
	// Copilot is already requested.
	p, err := fetchPR(c, repo, pr)
	if err != nil {
		return false, "", err
	}
	if isCopilotRequested(p.RequestedReviewers) {
		return true, p.HTMLURL, nil
	}

	path := fmt.Sprintf("repos/%s/%s/pulls/%d/requested_reviewers", repo.Owner, repo.Name, pr)
	payload, err := json.Marshal(map[string][]string{
		"reviewers": {CopilotLogin},
	})
	if err != nil {
		return false, "", fmt.Errorf("encode request payload: %w", err)
	}

	var resp PullRequest
	if err := c.Post(path, bytes.NewReader(payload), &resp); err != nil {
		return false, "", fmt.Errorf("request copilot reviewer on %s/%s#%d: %w", repo.Owner, repo.Name, pr, err)
	}
	if resp.HTMLURL == "" {
		resp.HTMLURL = p.HTMLURL
	}
	return false, resp.HTMLURL, nil
}

func listReviews(c *api.RESTClient, repo repository.Repository, pr int) ([]Review, error) {
	// PRs with many reviewers can exceed 100 reviews. GitHub's max per_page is
	// 100, so iterate until a short page comes back. maxPages is a safety
	// valve: real PRs never reach this, but it prevents a runaway loop if the
	// API ever returns exactly-full pages indefinitely.
	const (
		perPage  = 100
		maxPages = 100 // up to 10,000 reviews
	)
	var all []Review
	for page := 1; page <= maxPages; page++ {
		path := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews?per_page=%d&page=%d",
			repo.Owner, repo.Name, pr, perPage, page)
		var batch []Review
		if err := c.Get(path, &batch); err != nil {
			return nil, fmt.Errorf("list reviews for %s/%s#%d page=%d: %w",
				repo.Owner, repo.Name, pr, page, err)
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			return all, nil
		}
	}
	return nil, fmt.Errorf("list reviews for %s/%s#%d: exceeded %d-page safety limit",
		repo.Owner, repo.Name, pr, maxPages)
}

func isCopilotRequested(reviewers []User) bool {
	for _, u := range reviewers {
		if u.Login == CopilotLogin {
			return true
		}
	}
	return false
}

func latestCopilotReview(reviews []Review) *Review {
	var latest *Review
	for i := range reviews {
		r := &reviews[i]
		if r.User.Login != CopilotLogin {
			continue
		}
		if latest == nil || r.SubmittedAt.After(latest.SubmittedAt) {
			latest = r
		}
	}
	return latest
}
