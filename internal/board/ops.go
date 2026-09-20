package board

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Report is one issue as both the board and the issue describe it, which is
// what deciding whether to claim it takes.
type Report struct {
	Issue      int      `json:"issue"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	State      string   `json:"state"`
	Status     Status   `json:"status"`
	Option     string   `json:"option"`
	Group      string   `json:"group,omitempty"`
	Assignees  []string `json:"assignees"`
	SubIssues  []Ref    `json:"subIssues"`
	BlockedBy  []Ref    `json:"blockedBy"`
	Blocking   []Ref    `json:"blocking"`
	Claimable  bool     `json:"claimable"`
	Unclaimed  string   `json:"unclaimed,omitempty"`
	Transition []Status `json:"transitions"`
}

// Claim is what a claim did. Won says whether this run holds the issue;
// Changed lists the writes that went out, including the ones a lost claim
// takes back.
type Claim struct {
	Issue     int      `json:"issue"`
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Login     string   `json:"login"`
	From      Status   `json:"from"`
	Status    Status   `json:"status"`
	Assignees []string `json:"assignees"`
	Won       bool     `json:"won"`
	Reason    string   `json:"reason,omitempty"`
	Changed   []string `json:"changed"`
}

// Move is one transition. Status is what the board read back afterwards,
// which is not always what the move asked for.
type Move struct {
	Issue      int      `json:"issue"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	From       Status   `json:"from"`
	To         Status   `json:"to"`
	Status     Status   `json:"status"`
	Landed     bool     `json:"landed"`
	Reason     string   `json:"reason,omitempty"`
	Changed    []string `json:"changed"`
	Unassigned []string `json:"unassigned,omitempty"`
}

// Filed is a new issue and where it landed on the board.
type Filed struct {
	Issue  int    `json:"issue"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Status Status `json:"status"`
	Parent int    `json:"parent,omitempty"`
	Item   string `json:"item"`
}

// Show reads one issue from both the board and the issue itself.
func (c *Client) Show(ctx context.Context, number int) (Report, error) {
	item, err := c.Find(ctx, number)
	if err != nil {
		return Report{}, err
	}
	subIssues, err := c.SubIssues(ctx, number)
	if err != nil {
		return Report{}, err
	}
	blockedBy, err := c.BlockedBy(ctx, number)
	if err != nil {
		return Report{}, err
	}
	blocking, err := c.Blocking(ctx, number)
	if err != nil {
		return Report{}, err
	}

	report := Report{
		Issue: item.Number, Title: item.Title, URL: item.URL, State: item.State,
		Status: item.Status, Option: item.Option, Group: item.Group,
		Assignees: item.Assignees, SubIssues: subIssues,
		BlockedBy: blockedBy, Blocking: blocking,
		Transition: Next(item.Status),
	}
	report.Claimable, report.Unclaimed = claimable(item, subIssues, blockedBy)
	return report, nil
}

// claimable reports whether nobody holds the issue and nothing it waits for
// is still open. An issue with children is never claimed: its state is
// whatever its children say, and the work is in them.
func claimable(item Item, subIssues, blockedBy []Ref) (bool, string) {
	switch {
	case !strings.EqualFold(item.State, "open"):
		return false, "the issue is closed"
	case len(item.Assignees) > 0:
		return false, fmt.Sprintf("%s holds it", strings.Join(item.Assignees, ", "))
	case item.Status != Todo:
		return false, fmt.Sprintf("the board reads %s", item.Status)
	case len(subIssues) > 0:
		return false, fmt.Sprintf("it has %d sub-issues, and a sub-issue is the unit of work", len(subIssues))
	case open(blockedBy) > 0:
		return false, fmt.Sprintf("%d of its blockers are open", open(blockedBy))
	}
	return true, ""
}

// Claim takes an issue: status first, then the assignment.
//
// The order is what makes an interrupted claim recoverable. A run killed
// between the two leaves In Progress with no assignee, which the stale-claim
// rule returns to the queue. Assigning first would leave an owned issue
// reading Todo, which the next claimant takes as free.
//
// The assignment endpoint adds rather than replaces, so two runs claiming at
// once both find themselves assigned and both would otherwise believe they
// won. The read afterwards compares against the whole list: exactly one login
// and it is this one, or the claim was lost. A lost claim gives back its own
// assignment and nothing else. Status belongs to the run that won it, and
// putting the issue back to Todo would hand work in progress to a third run.
func (c *Client) Claim(ctx context.Context, number int) (Claim, error) {
	item, err := c.Find(ctx, number)
	if err != nil {
		return Claim{}, err
	}
	subIssues, err := c.SubIssues(ctx, number)
	if err != nil {
		return Claim{}, err
	}
	blockedBy, err := c.BlockedBy(ctx, number)
	if err != nil {
		return Claim{}, err
	}

	claim := Claim{
		Issue: item.Number, Title: item.Title, URL: item.URL,
		From: item.Status, Status: item.Status, Assignees: item.Assignees,
		Changed: []string{},
	}
	if ok, why := claimable(item, subIssues, blockedBy); !ok {
		claim.Reason = why
		return claim, nil
	}

	login, err := c.Viewer(ctx)
	if err != nil {
		return Claim{}, err
	}
	claim.Login = login

	if err := c.SetStatus(ctx, item.ID, InProgress); err != nil {
		return Claim{}, err
	}
	claim.Status = InProgress
	claim.Changed = append(claim.Changed, fmt.Sprintf("%s: %s to %s",
		c.settings.StatusField, c.OptionName(item.Status), c.OptionName(InProgress)))

	assigned, err := c.Assign(ctx, number, login)
	if err != nil {
		return claim, err
	}
	claim.Changed = append(claim.Changed, "assigned "+login)
	claim.Assignees = assigned.Logins()

	// The assignment write answers with the issue, and that answer is the
	// read: it is one round trip later than the write and carries the whole
	// list, which is what tells two winners apart.
	if slices.Equal(claim.Assignees, []string{login}) {
		claim.Won = true
		return claim, nil
	}

	claim.Reason = fmt.Sprintf("%s also holds it", strings.Join(without(claim.Assignees, login), ", "))
	released, err := c.Unassign(ctx, number, login)
	if err != nil {
		return claim, err
	}
	claim.Changed = append(claim.Changed, "unassigned "+login)
	claim.Assignees = released.Logins()
	return claim, nil
}

// Transition moves an issue between states the table lists, and gives the
// issue back when it returns to Todo. A comment goes out first when one is
// given: an issue back in Todo with no comment cannot be told from one nobody
// has started, so the next run takes it and meets the same wall.
func (c *Client) Transition(ctx context.Context, number int, to Status, comment string) (Move, error) {
	item, err := c.Find(ctx, number)
	if err != nil {
		return Move{}, err
	}

	move := Move{
		Issue: item.Number, Title: item.Title, URL: item.URL,
		From: item.Status, To: to, Status: item.Status, Changed: []string{},
	}
	switch {
	case item.Status == to:
		move.Landed = true
		move.Reason = fmt.Sprintf("the board already reads %s", c.OptionName(to))
		return move, nil
	case !Allowed(item.Status, to):
		move.Reason = fmt.Sprintf("the table has no %s to %s", item.Status, to)
		if next := Next(item.Status); len(next) > 0 {
			move.Reason += fmt.Sprintf(", only %s", join(next))
		}
		return move, nil
	}

	if comment != "" {
		if err := c.Comment(ctx, number, comment); err != nil {
			return move, err
		}
		move.Changed = append(move.Changed, "commented")
	}

	if err := c.SetStatus(ctx, item.ID, to); err != nil {
		return move, err
	}
	move.Changed = append(move.Changed, fmt.Sprintf("%s: %s to %s",
		c.settings.StatusField, c.OptionName(item.Status), c.OptionName(to)))

	// Going back to Todo is a run giving the issue up, and an assignee with
	// Todo is a claim that half happened. Only this run's own assignment
	// comes off: whoever else is on the issue put themselves there.
	if to == Todo {
		login, err := c.Viewer(ctx)
		if err != nil {
			return move, err
		}
		if slices.Contains(item.Assignees, login) {
			if _, err := c.Unassign(ctx, number, login); err != nil {
				return move, err
			}
			move.Changed = append(move.Changed, "unassigned "+login)
			move.Unassigned = []string{login}
		}
	}

	read, err := c.refresh(ctx, item.ID)
	if err != nil {
		return move, err
	}
	move.Status = read.Status
	move.Landed = read.Status == to
	if !move.Landed {
		move.Reason = fmt.Sprintf("the board reads %s, which somebody else wrote", read.Option)
	}
	return move, nil
}

// File puts new work on the board. Work found mid-run goes in a new issue,
// never into the one being worked.
func (c *Client) File(ctx context.Context, title, body string, parent int) (Filed, error) {
	issue, err := c.Create(ctx, title, body, parent)
	if err != nil {
		return Filed{}, err
	}
	item, err := c.Add(ctx, issue)
	if err != nil {
		return Filed{Issue: issue.Number, Title: issue.Title, URL: issue.URL, Parent: parent}, err
	}
	return Filed{
		Issue: issue.Number, Title: issue.Title, URL: issue.URL,
		Status: item.Status, Parent: parent, Item: item.ID,
	}, nil
}

func without(logins []string, login string) []string {
	var rest []string
	for _, l := range logins {
		if l != login {
			rest = append(rest, l)
		}
	}
	return rest
}

func join(statuses []Status) string {
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		names = append(names, string(status))
	}
	return strings.Join(names, " or ")
}
