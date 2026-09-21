package board

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Issue is an issue as REST returns it. Two identifiers matter and they are
// not interchangeable: Number addresses the issue in a path, and ID is what a
// sub-issue or dependency write puts in a body. An issue's number is per
// repository where its ID is global, so a number sent as an ID is some other
// repository's issue.
type Issue struct {
	ID        int64  `json:"id"`
	NodeID    string `json:"node_id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"html_url"`
	State     string `json:"state"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
}

// Logins returns the issue's assignees.
func (i Issue) Logins() []string {
	logins := make([]string, 0, len(i.Assignees))
	for _, assignee := range i.Assignees {
		logins = append(logins, assignee.Login)
	}
	return logins
}

// Ref is one issue in a relation, which is all a caller needs back from a
// sub-issue or dependency read.
type Ref struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	URL    string `json:"url"`
}

func (c *Client) issuesPath(parts ...string) string {
	segments := append([]string{c.rest, "repos", c.owner, c.repo, "issues"}, parts...)
	return strings.Join(segments, "/")
}

// Issue reads one issue.
func (c *Client) Issue(ctx context.Context, number int) (Issue, error) {
	var issue Issue
	err := c.call(ctx, "GET", c.issuesPath(itoa(number)), nil, &issue)
	return issue, err
}

// Viewer is the login the token authenticates as. A claim assigns this and
// then checks the assignment against it, so it has to come from the token
// rather than from an argument.
func (c *Client) Viewer(ctx context.Context) (string, error) {
	var reply struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := c.query(ctx, `query { viewer { login } }`, nil, &reply); err != nil {
		return "", err
	}
	if reply.Viewer.Login == "" {
		return "", fmt.Errorf("the token authenticates as nobody, so there is no login to assign")
	}
	return reply.Viewer.Login, nil
}

// Assign adds a login to an issue, leaving any other assignee alone.
func (c *Client) Assign(ctx context.Context, number int, login string) (Issue, error) {
	var issue Issue
	body := map[string]any{"assignees": []string{login}}
	err := c.call(ctx, "POST", c.issuesPath(itoa(number), "assignees"), body, &issue)
	return issue, err
}

// Unassign removes a login from an issue and leaves the rest of the list
// alone, which is the difference between this endpoint and an issue update
// carrying an assignees array.
func (c *Client) Unassign(ctx context.Context, number int, login string) (Issue, error) {
	var issue Issue
	body := map[string]any{"assignees": []string{login}}
	err := c.call(ctx, "DELETE", c.issuesPath(itoa(number), "assignees"), body, &issue)
	return issue, err
}

// Create files an issue in this repository. A body is whatever the caller
// read from a file: nothing here interprets it.
func (c *Client) Create(ctx context.Context, title, body string, parent int) (Issue, error) {
	payload := map[string]any{"title": title, "body": body}
	var issue Issue
	if err := c.call(ctx, "POST", c.issuesPath(), payload, &issue); err != nil {
		return Issue{}, err
	}
	if parent > 0 {
		if err := c.AddSubIssue(ctx, parent, issue.ID); err != nil {
			return issue, fmt.Errorf("issue %d was filed, and attaching it to %d failed: %w",
				issue.Number, parent, err)
		}
	}
	return issue, nil
}

// Comment adds a comment to an issue. Releasing an issue says what would
// unblock it in the same breath, because an issue back in Todo with no
// comment cannot be told from one nobody has started.
func (c *Client) Comment(ctx context.Context, number int, body string) error {
	return c.call(ctx, "POST", c.issuesPath(itoa(number), "comments"),
		map[string]any{"body": body}, nil)
}

// SubIssues returns the children of an issue.
func (c *Client) SubIssues(ctx context.Context, number int) ([]Ref, error) {
	var issues []Issue
	if err := c.call(ctx, "GET", c.issuesPath(itoa(number), "sub_issues"), nil, &issues); err != nil {
		return nil, err
	}
	return refs(issues), nil
}

// AddSubIssue attaches a child to a parent. The path takes the parent's
// number and the body takes the child's ID.
func (c *Client) AddSubIssue(ctx context.Context, parent int, child int64) error {
	return c.call(ctx, "POST", c.issuesPath(itoa(parent), "sub_issues"),
		map[string]any{"sub_issue_id": child}, nil)
}

// RemoveSubIssue detaches a child from its parent. The endpoint is singular
// where the one that adds is plural.
func (c *Client) RemoveSubIssue(ctx context.Context, parent int, child int64) error {
	return c.call(ctx, "DELETE", c.issuesPath(itoa(parent), "sub_issue"),
		map[string]any{"sub_issue_id": child}, nil)
}

// BlockedBy returns the issues one issue waits for. GitHub refuses to close an
// issue whose blockers are open, so this is also what says whether a closing
// keyword will do anything.
func (c *Client) BlockedBy(ctx context.Context, number int) ([]Ref, error) {
	return c.dependencies(ctx, number, "blocked_by")
}

// Blocking returns the issues waiting for one issue. The relation is stored
// once and projected both ways, so reading it from this end after writing it
// from the other is a check rather than an echo.
func (c *Client) Blocking(ctx context.Context, number int) ([]Ref, error) {
	return c.dependencies(ctx, number, "blocking")
}

func (c *Client) dependencies(ctx context.Context, number int, relation string) ([]Ref, error) {
	var issues []Issue
	endpoint := c.issuesPath(itoa(number), "dependencies", relation)
	if err := c.call(ctx, "GET", endpoint, nil, &issues); err != nil {
		return nil, err
	}
	return refs(issues), nil
}

// Block records that an issue waits for a blocker. The path takes the blocked
// issue's number and the body takes the blocker's ID.
func (c *Client) Block(ctx context.Context, blocked int, blocker int64) error {
	return c.call(ctx, "POST", c.issuesPath(itoa(blocked), "dependencies", "blocked_by"),
		map[string]any{"issue_id": blocker}, nil)
}

// Unblock removes the relation. This one takes the blocker's ID in the path.
func (c *Client) Unblock(ctx context.Context, blocked int, blocker int64) error {
	endpoint := c.issuesPath(itoa(blocked), "dependencies", "blocked_by", itoa64(blocker))
	return c.call(ctx, "DELETE", endpoint, nil, nil)
}

func refs(issues []Issue) []Ref {
	out := make([]Ref, 0, len(issues))
	for _, issue := range issues {
		out = append(out, Ref{Number: issue.Number, Title: issue.Title, State: issue.State, URL: issue.URL})
	}
	return out
}

// open counts the refs that are still open, which is the number that decides
// whether an issue is claimable.
func open(refs []Ref) int {
	count := 0
	for _, ref := range refs {
		if strings.EqualFold(ref.State, "open") {
			count++
		}
	}
	return count
}

func itoa(n int) string     { return url.PathEscape(fmt.Sprint(n)) }
func itoa64(n int64) string { return url.PathEscape(fmt.Sprint(n)) }
