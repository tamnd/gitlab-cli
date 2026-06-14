package gitlab

import (
	"context"
	"strings"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes gitlab as a kit Domain driver.
//
// A multi-domain host (ant) enables it with a single blank import:
//
//	import _ "github.com/tamnd/gitlab-cli/gitlab"
func init() { kit.Register(Domain{}) }

// Domain is the GitLab driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "gitlab",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "lab",
			Short:  "GitLab project and user data from gitlab.com",
			Long: `lab fetches public GitLab data from the GitLab REST API v4
(gitlab.com/api/v4). No API key required for public resources. Supports
project search, project detail, commit history, group search, and user lookup.`,
			Site: Host,
			Repo: "https://github.com/tamnd/gitlab-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Search public projects",
	}, searchOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "project",
		Group:   "read",
		Single:  true,
		Summary: "Show project detail",
	}, projectOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "commits",
		Group:   "read",
		List:    true,
		Summary: "List recent commits for a project",
	}, commitsOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "groups",
		Group:   "read",
		List:    true,
		Summary: "Search groups",
	}, groupsOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "user",
		Group:   "read",
		Single:  true,
		Summary: "Find a user by username",
	}, userOp)
}

// newClient builds the client from host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type searchInput struct {
	Query  string        `kit:"flag" help:"search query"`
	Limit  int           `kit:"flag,inherit" help:"max results (default 10)"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type projectInput struct {
	Path   string        `kit:"flag" help:"project path (e.g. gitlab-org/gitlab) or numeric id"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type commitsInput struct {
	Path   string        `kit:"flag" help:"project path (e.g. gitlab-org/gitlab)"`
	Limit  int           `kit:"flag,inherit" help:"max results (default 10)"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type groupsInput struct {
	Name   string        `kit:"flag" help:"group name to search for"`
	Limit  int           `kit:"flag,inherit" help:"max results (default 10)"`
	Delay  time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client *Client       `kit:"inject"`
}

type userInput struct {
	Username string        `kit:"flag" help:"GitLab username"`
	Delay    time.Duration `kit:"flag,inherit" help:"minimum spacing between requests"`
	Client   *Client       `kit:"inject"`
}

// --- handlers ---

func searchOp(ctx context.Context, in searchInput, emit func(Project) error) error {
	items, err := in.Client.Search(ctx, in.Query, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func projectOp(ctx context.Context, in projectInput, emit func(*Project) error) error {
	p, err := in.Client.Project(ctx, in.Path)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func commitsOp(ctx context.Context, in commitsInput, emit func(Commit) error) error {
	items, err := in.Client.Commits(ctx, in.Path, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func groupsOp(ctx context.Context, in groupsInput, emit func(Group) error) error {
	items, err := in.Client.Groups(ctx, in.Name, in.Limit)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := emit(item); err != nil {
			return err
		}
	}
	return nil
}

func userOp(ctx context.Context, in userInput, emit func(*User) error) error {
	u, err := in.Client.User(ctx, in.Username)
	if err != nil {
		return mapErr(err)
	}
	return emit(u)
}

// --- Resolver ---

// Classify turns an input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty gitlab reference")
	}
	// Strip https://gitlab.com/ prefix if present.
	pfx := "https://gitlab.com/"
	if strings.HasPrefix(input, pfx) {
		input = strings.TrimPrefix(input, pfx)
	}
	id = strings.Trim(input, "/")
	return "project", id, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "project":
		return "https://gitlab.com/" + strings.Trim(id, "/"), nil
	default:
		return "", errs.Usage("gitlab has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind.
func mapErr(err error) error {
	return err
}
