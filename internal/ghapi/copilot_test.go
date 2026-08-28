package ghapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

// roundTripFunc lets a test serve canned REST responses without a network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func newTestClient(t *testing.T, handler roundTripFunc) *api.RESTClient {
	t.Helper()
	c, err := api.NewRESTClient(api.ClientOptions{
		Host:      "github.com",
		AuthToken: "test-token",
		Transport: handler,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

var testRepo = repository.Repository{Owner: "ytyng", Name: "example"}

func TestIsCopilotRequested(t *testing.T) {
	tests := []struct {
		name      string
		reviewers []User
		want      bool
	}{
		{"empty", nil, false},
		{
			"bot slug",
			[]User{{Login: CopilotLogin, Type: "Bot"}},
			true,
		},
		{
			// requested_reviewers に出るのは今のところ短い "Copilot"
			"short form with Bot type",
			[]User{{Login: "Copilot", Type: "Bot"}},
			true,
		},
		{
			// "Copilot" は人間のユーザー名としても実在しうるので Type を見る
			"short form with User type is not copilot",
			[]User{{Login: "Copilot", Type: "User"}},
			false,
		},
		{
			"case sensitive",
			[]User{{Login: "copilot", Type: "Bot"}},
			false,
		},
		{
			"other reviewers only",
			[]User{{Login: "ytyng", Type: "User"}},
			false,
		},
		{
			"copilot among others",
			[]User{{Login: "ytyng", Type: "User"}, {Login: CopilotLogin, Type: "Bot"}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCopilotRequested(tt.reviewers); got != tt.want {
				t.Errorf("isCopilotRequested() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLatestCopilotReview(t *testing.T) {
	at := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("parse time: %v", err)
		}
		return ts
	}

	t.Run("no reviews", func(t *testing.T) {
		if got := latestCopilotReview(nil); got != nil {
			t.Errorf("want nil, got %+v", got)
		}
	})

	t.Run("ignores other reviewers", func(t *testing.T) {
		reviews := []Review{
			{ID: 1, User: User{Login: "ytyng"}, SubmittedAt: at("2026-08-28T10:00:00Z")},
		}
		if got := latestCopilotReview(reviews); got != nil {
			t.Errorf("want nil, got %+v", got)
		}
	})

	t.Run("picks the newest copilot review", func(t *testing.T) {
		reviews := []Review{
			{ID: 1, User: User{Login: CopilotLogin}, SubmittedAt: at("2026-08-28T10:00:00Z")},
			{ID: 2, User: User{Login: "ytyng"}, SubmittedAt: at("2026-08-28T12:00:00Z")},
			{ID: 3, User: User{Login: CopilotLogin}, SubmittedAt: at("2026-08-28T11:00:00Z")},
		}

		got := latestCopilotReview(reviews)

		if got == nil || got.ID != 3 {
			t.Fatalf("want review 3, got %+v", got)
		}
	})

	t.Run("order in the response does not matter", func(t *testing.T) {
		reviews := []Review{
			{ID: 1, User: User{Login: CopilotLogin}, SubmittedAt: at("2026-08-28T13:00:00Z")},
			{ID: 2, User: User{Login: CopilotLogin}, SubmittedAt: at("2026-08-28T09:00:00Z")},
		}

		got := latestCopilotReview(reviews)

		if got == nil || got.ID != 1 {
			t.Fatalf("want review 1, got %+v", got)
		}
	})
}

func TestGetCopilotStatus(t *testing.T) {
	reviewedAt := "2026-08-28T10:00:00Z"

	tests := []struct {
		name          string
		requested     string
		reviews       string
		wantState     CopilotState
		wantHasReview bool
	}{
		{
			name:      "not requested and never reviewed",
			requested: `[]`,
			reviews:   `[]`,
			wantState: StateNotRequested,
		},
		{
			name:      "requested",
			requested: `[{"login":"Copilot","type":"Bot"}]`,
			reviews:   `[]`,
			wantState: StateReviewing,
		},
		{
			name:      "reviewed and no longer requested",
			requested: `[]`,
			reviews: fmt.Sprintf(
				`[{"id":1,"user":{"login":%q,"type":"Bot"},"submitted_at":%q}]`,
				CopilotLogin, reviewedAt),
			wantState:     StateCompleted,
			wantHasReview: true,
		},
		{
			// レビュー済みでも再依頼中なら reviewing。
			// 前回のレビューは LatestReview として返る (再レビューの検知に使う)
			name:      "reviewed and requested again",
			requested: `[{"login":"Copilot","type":"Bot"}]`,
			reviews: fmt.Sprintf(
				`[{"id":1,"user":{"login":%q,"type":"Bot"},"submitted_at":%q}]`,
				CopilotLogin, reviewedAt),
			wantState:     StateReviewing,
			wantHasReview: true,
		},
		{
			name:      "reviews from humans only",
			requested: `[]`,
			reviews:   `[{"id":1,"user":{"login":"ytyng","type":"User"},"submitted_at":"2026-08-28T10:00:00Z"}]`,
			wantState: StateNotRequested,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/pulls/12"):
					return jsonResponse(200, fmt.Sprintf(
						`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12","requested_reviewers":%s}`,
						tt.requested)), nil
				case strings.HasSuffix(r.URL.Path, "/pulls/12/reviews"):
					return jsonResponse(200, tt.reviews), nil
				}
				return nil, fmt.Errorf("unexpected path: %s", r.URL.Path)
			})

			status, err := GetCopilotStatus(client, testRepo, 12)
			if err != nil {
				t.Fatalf("GetCopilotStatus: %v", err)
			}
			if status.State != tt.wantState {
				t.Errorf("state = %q, want %q", status.State, tt.wantState)
			}
			if (status.LatestReview != nil) != tt.wantHasReview {
				t.Errorf("LatestReview = %+v, wantHasReview=%v",
					status.LatestReview, tt.wantHasReview)
			}
			if status.PullRequest.Number != 12 {
				t.Errorf("PR number = %d, want 12", status.PullRequest.Number)
			}
		})
	}
}

func TestListReviewsPaginates(t *testing.T) {
	// 1 ページ 100 件。ちょうど埋まっている間は次のページを取りに行き、
	// 短いページが返ったところで止まる。
	full := make([]Review, 100)
	for i := range full {
		full[i] = Review{ID: int64(i + 1), User: User{Login: "ytyng"}}
	}
	fullJSON, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var pages []string
	client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/reviews") {
			page := r.URL.Query().Get("page")
			pages = append(pages, page)
			if page == "1" {
				return jsonResponse(200, string(fullJSON)), nil
			}
			return jsonResponse(200,
				fmt.Sprintf(`[{"id":999,"user":{"login":%q,"type":"Bot"},"submitted_at":"2026-08-28T10:00:00Z"}]`,
					CopilotLogin)), nil
		}
		return jsonResponse(200,
			`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12","requested_reviewers":[]}`), nil
	})

	status, err := GetCopilotStatus(client, testRepo, 12)
	if err != nil {
		t.Fatalf("GetCopilotStatus: %v", err)
	}

	if len(pages) != 2 {
		t.Fatalf("fetched pages = %v, want 2 pages", pages)
	}
	if status.LatestReview == nil || status.LatestReview.ID != 999 {
		t.Fatalf("latest review = %+v, want the one from page 2", status.LatestReview)
	}
}

func TestRequestCopilotReview(t *testing.T) {
	t.Run("already requested does not POST", func(t *testing.T) {
		posted := false
		client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodPost {
				posted = true
			}
			return jsonResponse(200,
				`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12","requested_reviewers":[{"login":"Copilot","type":"Bot"}]}`), nil
		})

		already, url, err := RequestCopilotReview(client, testRepo, 12)
		if err != nil {
			t.Fatalf("RequestCopilotReview: %v", err)
		}
		if !already {
			t.Error("already = false, want true")
		}
		if posted {
			t.Error("POST was sent even though copilot is already requested")
		}
		if url != "https://github.com/ytyng/example/pull/12" {
			t.Errorf("url = %q", url)
		}
	})

	t.Run("posts the copilot bot login", func(t *testing.T) {
		var body map[string][]string
		client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodPost {
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					return nil, err
				}
				return jsonResponse(201,
					`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12"}`), nil
			}
			return jsonResponse(200,
				`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12","requested_reviewers":[]}`), nil
		})

		already, url, err := RequestCopilotReview(client, testRepo, 12)
		if err != nil {
			t.Fatalf("RequestCopilotReview: %v", err)
		}
		if already {
			t.Error("already = true, want false")
		}
		if got := body["reviewers"]; len(got) != 1 || got[0] != CopilotLogin {
			t.Errorf("posted reviewers = %v, want [%s]", got, CopilotLogin)
		}
		if url != "https://github.com/ytyng/example/pull/12" {
			t.Errorf("url = %q", url)
		}
	})

	t.Run("falls back to the PR url when the response has none", func(t *testing.T) {
		client := newTestClient(t, func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodPost {
				return jsonResponse(201, `{"number":12}`), nil
			}
			return jsonResponse(200,
				`{"number":12,"html_url":"https://github.com/ytyng/example/pull/12","requested_reviewers":[]}`), nil
		})

		_, url, err := RequestCopilotReview(client, testRepo, 12)
		if err != nil {
			t.Fatalf("RequestCopilotReview: %v", err)
		}
		if url != "https://github.com/ytyng/example/pull/12" {
			t.Errorf("url = %q, want the PR url from the GET", url)
		}
	})
}

func TestResolveRepoWithOverride(t *testing.T) {
	repo, err := ResolveRepo("ytyng/example")
	if err != nil {
		t.Fatalf("ResolveRepo: %v", err)
	}
	if repo.Owner != "ytyng" || repo.Name != "example" {
		t.Errorf("repo = %+v", repo)
	}
}

func TestResolveRepoRejectsGarbage(t *testing.T) {
	if _, err := ResolveRepo("not-a-repo-spec/"); err == nil {
		t.Error("want an error for a malformed --repo value")
	}
}
