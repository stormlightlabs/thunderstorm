package board

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stormlightlabs/thunderstorm/internal/config"
)

// settings are project 13's shape: a shared board, separated by a Track
// field, with the three option names this repository uses.
func settings() config.Board {
	return config.Board{
		Owner:       "stormlightlabs",
		Number:      13,
		Repository:  "stormlightlabs/thunderstorm",
		StatusField: "Status",
		Status:      config.Status{Todo: "Todo", InProgress: "In Progress", Done: "Done"},
		GroupField:  "Track",
		GroupValue:  "Thunderstorm",
	}
}

// board is a GitHub that answers the queries and records the writes, so a
// test can ask what order they went out in.
type board struct {
	mu        sync.Mutex
	calls     []string
	status    string
	assignees []string
	group     string
	// onAssign runs while the assignment is being written, which is where a
	// second run's claim lands.
	onAssign func(*board)
}

// unset is an item on the board that was never given a status, which every
// reader here takes as queued.
const unset = "unset"

var options = map[string]string{"opt-todo": "Todo", "opt-progress": "In Progress", "opt-done": "Done"}

func (b *board) record(call string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, call)
}

func (b *board) serve(t *testing.T) *Client {
	t.Helper()
	if b.status == "" {
		b.status = "Todo"
	}
	if b.group == "" {
		b.group = "Thunderstorm"
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/graphql" {
			b.graphql(t, w, r)
			return
		}
		b.rest(t, w, r)
	}))
	t.Cleanup(server.Close)

	client, err := New(settings(), Options{
		Token:   "test-token",
		REST:    server.URL,
		GraphQL: server.URL + "/graphql",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func (b *board) graphql(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var request struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Fatal(err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	switch {
	case strings.Contains(request.Query, "repositoryOwner"):
		reply(t, w, `{"data":{"repositoryOwner":{"projectV2":{
			"id":"PVT_1","title":"THNDRS","number":13,
			"fields":{"nodes":[
				{"id":"F_status","name":"Status","options":[
					{"id":"opt-todo","name":"Todo"},
					{"id":"opt-progress","name":"In Progress"},
					{"id":"opt-done","name":"Done"}]},
				{"id":"F_track","name":"Track","options":[
					{"id":"track-thunderstorm","name":"Thunderstorm"},
					{"id":"track-other","name":"Elsewhere"}]}]}}}}}`)
	case strings.Contains(request.Query, "viewer"):
		reply(t, w, `{"data":{"viewer":{"login":"claimant"}}}`)
	case strings.Contains(request.Query, "updateProjectV2ItemFieldValue"):
		field, _ := request.Variables["field"].(string)
		option, _ := request.Variables["option"].(string)
		if field == "F_status" {
			b.status = options[option]
			b.calls = append(b.calls, "status="+b.status)
		} else {
			b.calls = append(b.calls, "track="+option)
		}
		reply(t, w, `{"data":{"updateProjectV2ItemFieldValue":{"projectV2Item":{"id":"ITEM_1"}}}}`)
	case strings.Contains(request.Query, "addProjectV2ItemById"):
		b.calls = append(b.calls, "added")
		reply(t, w, `{"data":{"addProjectV2ItemById":{"item":{"id":"ITEM_1"}}}}`)
	case strings.Contains(request.Query, "items(first: 100"):
		b.calls = append(b.calls, "items")
		reply(t, w, fmt.Sprintf(`{"data":{"node":{"items":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[%s,
				{"id":"ITEM_2","fieldValues":{"nodes":[{"name":"Todo","field":{"name":"Status"}},
					{"name":"Thunderstorm","field":{"name":"Track"}}]},
				 "content":{"id":"I_2","number":99,"title":"Somebody else's issue","url":"u",
					"state":"OPEN","repository":{"nameWithOwner":"stormlightlabs/trps"},
					"assignees":{"nodes":[]}}},
				{"id":"ITEM_3","fieldValues":{"nodes":[{"name":"Todo","field":{"name":"Status"}},
					{"name":"Elsewhere","field":{"name":"Track"}}]},
				 "content":{"id":"I_3","number":98,"title":"Another track","url":"u",
					"state":"OPEN","repository":{"nameWithOwner":"stormlightlabs/thunderstorm"},
					"assignees":{"nodes":[]}}},
				{"id":"ITEM_4","fieldValues":{"nodes":[]},
				 "content":{"id":"I_4","number":0,"title":"A draft","url":"",
					"state":"OPEN","repository":{"nameWithOwner":""},"assignees":{"nodes":[]}}}]}}}}`,
			b.item()))
	case strings.Contains(request.Query, "node(id: $item)"):
		b.calls = append(b.calls, "item")
		reply(t, w, fmt.Sprintf(`{"data":{"node":%s}}`, b.item()))
	default:
		t.Fatalf("no answer for query %q", request.Query)
	}
}

// item is issue 15 as the board holds it, rendered from whatever the writes
// have made true so far.
func (b *board) item() string {
	logins := make([]string, 0, len(b.assignees))
	for _, login := range b.assignees {
		logins = append(logins, fmt.Sprintf(`{"login":%q}`, login))
	}
	status := fmt.Sprintf(`{"name":%q,"field":{"name":"Status"}},`, b.status)
	if b.status == unset {
		status = ""
	}
	return fmt.Sprintf(`{"id":"ITEM_1","fieldValues":{"nodes":[
			%s
			{"name":%q,"field":{"name":"Track"}}]},
		"content":{"id":"I_15","number":15,"title":"Move board writes onto Projects V2",
			"url":"https://github.com/stormlightlabs/thunderstorm/issues/15","state":"OPEN",
			"repository":{"nameWithOwner":"stormlightlabs/thunderstorm"},
			"assignees":{"nodes":[%s]}}}`, status, b.group, strings.Join(logins, ","))
}

func (b *board) rest(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()

	switch {
	case strings.HasSuffix(r.URL.Path, "/assignees") && r.Method == "POST":
		b.calls = append(b.calls, "assign")
		if b.onAssign != nil {
			b.onAssign(b)
		}
		if !slices.Contains(b.assignees, "claimant") {
			b.assignees = append(b.assignees, "claimant")
		}
		reply(t, w, b.issue())
	case strings.HasSuffix(r.URL.Path, "/assignees") && r.Method == "DELETE":
		b.calls = append(b.calls, "unassign")
		b.assignees = slices.DeleteFunc(b.assignees, func(login string) bool { return login == "claimant" })
		reply(t, w, b.issue())
	case strings.HasSuffix(r.URL.Path, "/sub_issues"), strings.HasSuffix(r.URL.Path, "/blocked_by"),
		strings.HasSuffix(r.URL.Path, "/blocking"):
		reply(t, w, `[]`)
	case strings.HasSuffix(r.URL.Path, "/issues") && r.Method == "POST":
		b.calls = append(b.calls, "created")
		reply(t, w, b.issue())
	case strings.HasSuffix(r.URL.Path, "/comments"):
		b.calls = append(b.calls, "comment")
		reply(t, w, `{}`)
	case strings.HasSuffix(r.URL.Path, "/issues/15"):
		reply(t, w, b.issue())
	default:
		t.Fatalf("no answer for %s %s", r.Method, r.URL.Path)
	}
}

func (b *board) issue() string {
	logins := make([]string, 0, len(b.assignees))
	for _, login := range b.assignees {
		logins = append(logins, fmt.Sprintf(`{"login":%q}`, login))
	}
	return fmt.Sprintf(`{"id":815,"node_id":"I_15","number":15,"title":"Move board writes onto Projects V2",
		"html_url":"https://github.com/stormlightlabs/thunderstorm/issues/15","state":"open",
		"assignees":[%s]}`, strings.Join(logins, ","))
}

func reply(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
}

func (b *board) written() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var writes []string
	for _, call := range b.calls {
		if call != "items" && call != "item" {
			writes = append(writes, call)
		}
	}
	return writes
}

// A run killed between the two writes has to leave the issue recoverable, and
// only one order does: In Progress with nobody on it returns to the queue,
// where an owned issue reading Todo is taken as free.
func TestAClaimWritesTheStatusBeforeTheAssignment(t *testing.T) {
	fake := &board{}
	claim, err := fake.serve(t).Claim(context.Background(), 15)
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Won {
		t.Fatalf("the claim was not taken: %s", claim.Reason)
	}
	if want := []string{"status=In Progress", "assign"}; !slices.Equal(fake.written(), want) {
		t.Errorf("wrote %v, want %v", fake.written(), want)
	}
	if !slices.Equal(claim.Assignees, []string{"claimant"}) {
		t.Errorf("claimed by %v, want claimant alone", claim.Assignees)
	}
	if len(claim.Changed) != 2 {
		t.Errorf("reported %v, want both writes", claim.Changed)
	}
}

// Two runs claiming at once both find themselves assigned, because the
// endpoint adds rather than replaces. Comparing against the whole list is what
// tells them apart.
func TestALostClaimGivesBackItsOwnAssignmentAndNothingElse(t *testing.T) {
	fake := &board{onAssign: func(b *board) { b.assignees = append(b.assignees, "winner") }}
	claim, err := fake.serve(t).Claim(context.Background(), 15)
	if err != nil {
		t.Fatal(err)
	}
	if claim.Won {
		t.Fatal("a claim with two assignees reported a win")
	}
	if !strings.Contains(claim.Reason, "winner") {
		t.Errorf("the reason does not name who holds it: %q", claim.Reason)
	}
	if want := []string{"status=In Progress", "assign", "unassign"}; !slices.Equal(fake.written(), want) {
		t.Errorf("wrote %v, want %v", fake.written(), want)
	}
	if fake.status != "In Progress" {
		t.Errorf("status is %q: the run that won it owns the status", fake.status)
	}
	if !slices.Equal(fake.assignees, []string{"winner"}) {
		t.Errorf("left %v assigned, want the winner alone", fake.assignees)
	}
}

// An issue somebody already holds is not claimable, and reading that costs
// nothing where writing and taking it back costs two round trips.
func TestAHeldIssueIsNotClaimedAndNothingIsWritten(t *testing.T) {
	fake := &board{status: "In Progress", assignees: []string{"somebody"}}
	claim, err := fake.serve(t).Claim(context.Background(), 15)
	if err != nil {
		t.Fatal(err)
	}
	if claim.Won {
		t.Fatal("an issue somebody else holds was claimed")
	}
	if !strings.Contains(claim.Reason, "somebody") {
		t.Errorf("the reason does not name who holds it: %q", claim.Reason)
	}
	if len(fake.written()) != 0 {
		t.Errorf("wrote %v to an issue it could not claim", fake.written())
	}
}

func TestATransitionTheTableDoesNotListWritesNothing(t *testing.T) {
	fake := &board{}
	move, err := fake.serve(t).Transition(context.Background(), 15, Done, "")
	if err != nil {
		t.Fatal(err)
	}
	if move.Landed {
		t.Fatal("todo reached done, which the table does not list")
	}
	if !strings.Contains(move.Reason, "in-progress") {
		t.Errorf("the refusal does not say what todo does reach: %q", move.Reason)
	}
	if len(fake.written()) != 0 {
		t.Errorf("wrote %v for a refused transition", fake.written())
	}
}

// Giving an issue back is a status and an unassignment: an assignee with Todo
// is a claim that half happened.
func TestGivingAnIssueBackUnassignsAndComments(t *testing.T) {
	fake := &board{status: "In Progress", assignees: []string{"claimant"}}
	move, err := fake.serve(t).Transition(context.Background(), 15, Todo, "waiting on the release")
	if err != nil {
		t.Fatal(err)
	}
	if !move.Landed {
		t.Fatalf("the move did not land: %s", move.Reason)
	}
	if want := []string{"comment", "status=Todo", "unassign"}; !slices.Equal(fake.written(), want) {
		t.Errorf("wrote %v, want %v", fake.written(), want)
	}
	if !slices.Equal(move.Unassigned, []string{"claimant"}) {
		t.Errorf("reported %v unassigned, want claimant", move.Unassigned)
	}
}

// One board carries several repositories and several tracks. A read that
// answered with all of them would have a run claiming somebody else's work.
func TestReadsAreFilteredToOneRepositoryAndOneTrack(t *testing.T) {
	items, err := (&board{}).serve(t).Items(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("read %d items, want issue 15 alone: %+v", len(items), items)
	}
	if items[0].Number != 15 || items[0].Status != Todo {
		t.Errorf("read %+v, want 15 in todo", items[0])
	}
}

// Every reader here treats a missing status as Todo. An item added to the
// board and never given one is queued.
func TestAnItemWithNoStatusIsQueued(t *testing.T) {
	item, err := (&board{status: unset}).serve(t).Find(context.Background(), 15)
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != Todo {
		t.Errorf("an unset status read as %q, want todo", item.Status)
	}
}

func TestAnIssueThatIsNotOnTheBoardSaysSo(t *testing.T) {
	_, err := (&board{}).serve(t).Find(context.Background(), 4242)
	if err == nil {
		t.Fatal("an issue the project does not carry was found")
	}
	if !strings.Contains(err.Error(), "4242") || !strings.Contains(err.Error(), "board add") {
		t.Errorf("the error does not say what to do about it: %v", err)
	}
}

func TestFilingPutsTheIssueOnTheBoardWithATrackAndAStatus(t *testing.T) {
	fake := &board{}
	filed, err := fake.serve(t).File(context.Background(), "New work", "found mid-run", 0)
	if err != nil {
		t.Fatal(err)
	}
	if filed.Issue != 15 {
		t.Errorf("filed issue %d, want the one the API answered with", filed.Issue)
	}
	want := []string{"created", "added", "track=track-thunderstorm", "status=Todo"}
	if !slices.Equal(fake.written(), want) {
		t.Errorf("wrote %v, want %v", fake.written(), want)
	}
}

// A board whose option names differ is the common case, and a missing option
// is a configuration error rather than a write that silently does nothing.
func TestAnOptionTheBoardDoesNotHaveIsNamed(t *testing.T) {
	fake := &board{}
	client := fake.serve(t)
	client.settings.Status.Done = "Shipped"
	client.project = nil

	_, err := client.Items(context.Background())
	if err == nil {
		t.Fatal("a status option the board does not have loaded without complaint")
	}
	if !strings.Contains(err.Error(), "Shipped") || !strings.Contains(err.Error(), "In Progress") {
		t.Errorf("the error names neither the missing option nor the ones that exist: %v", err)
	}
}

func TestATokenIsSentOnEveryCall(t *testing.T) {
	fake := &board{}
	client := fake.serve(t)
	client.token = "wrong"
	if _, err := client.Items(context.Background()); err == nil {
		t.Fatal("a rejected token read the board")
	}
}

func TestARemoteIsReadInBothFormsGitWrites(t *testing.T) {
	for _, remote := range []string{
		"git@github.com:stormlightlabs/thunderstorm.git",
		"https://github.com/stormlightlabs/thunderstorm.git",
		"https://github.com/stormlightlabs/thunderstorm",
		"ssh://git@github.com/stormlightlabs/thunderstorm.git",
	} {
		owner, repo, ok := splitRemote(remote)
		if !ok || owner != "stormlightlabs" || repo != "thunderstorm" {
			t.Errorf("%s read as %s/%s (%t)", remote, owner, repo, ok)
		}
	}
}

func TestAnEnterpriseHostGetsItsOwnEndpoints(t *testing.T) {
	rest, graphql := endpoints("github.example.com")
	if rest != "https://github.example.com/api/v3" || graphql != "https://github.example.com/api/graphql" {
		t.Errorf("enterprise endpoints are %s and %s", rest, graphql)
	}
	if rest, _ := endpoints(""); rest != "https://api.github.com" {
		t.Errorf("github.com REST endpoint is %s", rest)
	}
}
