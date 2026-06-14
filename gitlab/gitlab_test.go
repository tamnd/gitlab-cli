package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fakeProjectsJSON = `[{"id":278964,"name":"GitLab","path_with_namespace":"gitlab-org/gitlab","description":"GitLab is an open source end-to-end software development platform.","star_count":22000,"forks_count":7000,"visibility":"public","web_url":"https://gitlab.com/gitlab-org/gitlab","default_branch":"master"}]`

const fakeProjectJSON = `{"id":278964,"name":"GitLab","path_with_namespace":"gitlab-org/gitlab","description":"GitLab is an open source end-to-end software development platform.","star_count":22000,"forks_count":7000,"visibility":"public","web_url":"https://gitlab.com/gitlab-org/gitlab","default_branch":"master","open_issues_count":30000}`

const fakeCommitsJSON = `[{"id":"abc123def456","short_id":"abc123","title":"Fix: resolve merge conflict","author_name":"Jane Smith","author_email":"jane@example.com","created_at":"2024-01-01T10:00:00.000+00:00","web_url":"https://gitlab.com/gitlab-org/gitlab/-/commit/abc123def456"}]`

const fakeGroupsJSON = `[{"id":9970,"name":"GitLab.org","full_path":"gitlab-org","description":"Open source software to collaborate on code","visibility":"public","web_url":"https://gitlab.com/groups/gitlab-org"}]`

const fakeUsersJSON = `[{"id":1234567,"username":"torvalds","name":"Linus Torvalds","state":"active","web_url":"https://gitlab.com/torvalds","bio":"","location":"Portland, OR"}]`

func newTestClient(ts *httptest.Server) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return NewClient(cfg)
}

func TestSearchParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeProjectsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	projects, err := c.Search(context.Background(), "gitlab", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projects))
	}
	if projects[0].Name != "GitLab" {
		t.Errorf("projects[0].Name = %q, want GitLab", projects[0].Name)
	}
	if projects[0].Stars != 22000 {
		t.Errorf("projects[0].Stars = %d, want 22000", projects[0].Stars)
	}
}

func TestSearchSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeProjectsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent header not sent")
	}
}

func TestProjectParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify path encoding: namespace%2Fproject in raw URI
		if !strings.Contains(r.RequestURI, "%2F") {
			t.Errorf("RequestURI %q missing URL-encoded slash", r.RequestURI)
		}
		_, _ = fmt.Fprint(w, fakeProjectJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	p, err := c.Project(context.Background(), "gitlab-org/gitlab")
	if err != nil {
		t.Fatal(err)
	}
	if p.Path != "gitlab-org/gitlab" {
		t.Errorf("p.Path = %q, want gitlab-org/gitlab", p.Path)
	}
	if p.OpenIssues != 30000 {
		t.Errorf("p.OpenIssues = %d, want 30000", p.OpenIssues)
	}
}

func TestCommitsParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeCommitsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	commits, err := c.Commits(context.Background(), "gitlab-org/gitlab", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if commits[0].ShortID != "abc123" {
		t.Errorf("commits[0].ShortID = %q, want abc123", commits[0].ShortID)
	}
	if commits[0].AuthorName != "Jane Smith" {
		t.Errorf("commits[0].AuthorName = %q, want Jane Smith", commits[0].AuthorName)
	}
}

func TestGroupsParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeGroupsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	groups, err := c.Groups(context.Background(), "gitlab-org", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	if groups[0].Path != "gitlab-org" {
		t.Errorf("groups[0].Path = %q, want gitlab-org", groups[0].Path)
	}
}

func TestUserParses(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeUsersJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	u, err := c.User(context.Background(), "torvalds")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "torvalds" {
		t.Errorf("u.Username = %q, want torvalds", u.Username)
	}
	if u.Name != "Linus Torvalds" {
		t.Errorf("u.Name = %q, want Linus Torvalds", u.Name)
	}
}

func TestUserNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "[]")
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.User(context.Background(), "nobody-xyzzy-404")
	if err == nil {
		t.Error("expected error for empty user list")
	}
}

func TestSearchRetriesOn429(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = fmt.Fprint(w, fakeProjectsJSON)
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	_, err := c.Search(context.Background(), "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
