// Package gitlab is the library behind the lab command line:
// the HTTP client, request shaping, and the typed data models for the
// GitLab REST API v4 (gitlab.com/api/v4).
//
// The public API requires no key for public projects and users. The Client
// sets a polite User-Agent, paces requests to 1100ms apart (60 req/min limit),
// and retries transient failures (429 and 5xx) with a capped backoff.
package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"strconv"
	"sync"
	"time"
)

// Host is the GitLab API host.
const Host = "gitlab.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns sensible defaults for the GitLab API.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://gitlab.com/api/v4",
		UserAgent: "Mozilla/5.0 (compatible; gitlab-cli/dev; +https://github.com/tamnd/gitlab-cli)",
		Rate:      1100 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to the GitLab API over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Search searches public projects by query string.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Project, error) {
	if limit <= 0 {
		limit = 10
	}
	q := neturl.Values{}
	q.Set("search", query)
	q.Set("order_by", "stars_count")
	q.Set("sort", "desc")
	q.Set("per_page", strconv.Itoa(limit))
	u := c.cfg.BaseURL + "/projects?" + q.Encode()
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var projects []Project
	if err := json.Unmarshal(body, &projects); err != nil {
		return nil, fmt.Errorf("decode projects: %w", err)
	}
	return projects, nil
}

// Project returns detail for one project by path (e.g. "gitlab-org/gitlab") or numeric id.
func (c *Client) Project(ctx context.Context, path string) (*Project, error) {
	encoded := neturl.PathEscape(path)
	u := c.cfg.BaseURL + "/projects/" + encoded
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var p Project
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}
	return &p, nil
}

// Commits returns recent commits for a project path.
func (c *Client) Commits(ctx context.Context, path string, limit int) ([]Commit, error) {
	if limit <= 0 {
		limit = 10
	}
	encoded := neturl.PathEscape(path)
	q := neturl.Values{}
	q.Set("per_page", strconv.Itoa(limit))
	u := c.cfg.BaseURL + "/projects/" + encoded + "/repository/commits?" + q.Encode()
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	if err := json.Unmarshal(body, &commits); err != nil {
		return nil, fmt.Errorf("decode commits: %w", err)
	}
	return commits, nil
}

// Groups searches groups by name.
func (c *Client) Groups(ctx context.Context, name string, limit int) ([]Group, error) {
	if limit <= 0 {
		limit = 10
	}
	q := neturl.Values{}
	q.Set("search", name)
	q.Set("per_page", strconv.Itoa(limit))
	u := c.cfg.BaseURL + "/groups?" + q.Encode()
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var groups []Group
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, fmt.Errorf("decode groups: %w", err)
	}
	return groups, nil
}

// User finds a user by username. Returns the first match or an error if none found.
func (c *Client) User(ctx context.Context, username string) (*User, error) {
	q := neturl.Values{}
	q.Set("username", username)
	u := c.cfg.BaseURL + "/users?" + q.Encode()
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var users []User
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("decode users: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("user %q not found", username)
	}
	return &users[0], nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	return b, err != nil, err
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}

// --- record types ---

// Project is one GitLab project.
type Project struct {
	ID            int    `json:"id"`
	Path          string `json:"path_with_namespace"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Stars         int    `json:"star_count"`
	Forks         int    `json:"forks_count"`
	Visibility    string `json:"visibility"`
	URL           string `json:"web_url"`
	LastActivity  string `json:"last_activity_at,omitempty"`
	OpenIssues    int    `json:"open_issues_count,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
}

// Commit is one repository commit.
type Commit struct {
	ID          string `json:"id"`
	ShortID     string `json:"short_id"`
	Title       string `json:"title"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email,omitempty"`
	CreatedAt   string `json:"created_at"`
	Message     string `json:"message,omitempty"`
	WebURL      string `json:"web_url,omitempty"`
}

// Group is one GitLab group or organization.
type Group struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Path        string `json:"full_path"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
	Stars       int    `json:"star_count,omitempty"`
	WebURL      string `json:"web_url"`
}

// User is one GitLab user.
type User struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	State     string `json:"state,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	WebURL    string `json:"web_url"`
	Bio       string `json:"bio,omitempty"`
	Location  string `json:"location,omitempty"`
}
