// Package board performs the loop's GitHub Projects operations.
//
// Status lives on a Projects V2 single-select field. That field is GraphQL
// only: the GitHub MCP server does not expose it, and gh project reaches it
// only when the token carries the project scope. Issue dependencies are REST
// and no transport an agent has performs them at all. What was left was a
// skill asking a model to read a value, write it back, and notice afterwards
// that it had lost a race. The operations here do the reading, the writing and
// the noticing, and report what changed.
//
// Nothing about one board is compiled in. The project, the status field, the
// option names and the field that separates one repository's work from
// another's come from config.Board. What stays here is the order a claim
// writes in and which transitions exist.
package board

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/stormlightlabs/thunderstorm/internal/config"
)

// Options are what a caller supplies that is not configuration: the token, the
// endpoints, and the directory whose origin remote names the repository.
type Options struct {
	// Token authenticates every call. Empty resolves it through the
	// environment and then gh.
	Token string
	// REST and GraphQL default to github.com, or to GH_HOST when that is set.
	REST    string
	GraphQL string
	// Dir is the directory whose origin remote names the repository, used
	// only when the configuration does not name one.
	Dir string
	// HTTP defaults to a client with a timeout. A board call is a few
	// requests and none of them should hang a hook.
	HTTP *http.Client
}

// Client reads and writes one project, filtered to one repository.
type Client struct {
	settings config.Board
	owner    string
	repo     string
	token    string
	rest     string
	graphql  string
	http     *http.Client

	// project is looked up once. Every operation needs the project's node ID
	// and the status field's option IDs, and neither changes during a run.
	project *Project
}

// New returns a client for the configured board, or an error naming what the
// configuration left out.
func New(settings config.Board, opts Options) (*Client, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}

	owner, repo, err := repository(settings.Repository, opts.Dir)
	if err != nil {
		return nil, err
	}

	token := opts.Token
	if token == "" {
		if token, err = Token(); err != nil {
			return nil, err
		}
	}

	restAPI, graphQL := endpoints(os.Getenv("GH_HOST"))
	if opts.REST != "" {
		restAPI = opts.REST
	}
	if opts.GraphQL != "" {
		graphQL = opts.GraphQL
	}

	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		settings: settings,
		owner:    owner,
		repo:     repo,
		token:    token,
		rest:     strings.TrimSuffix(restAPI, "/"),
		graphql:  graphQL,
		http:     client,
	}, nil
}

// Repository is the repository every read is filtered to, as owner/name.
func (c *Client) Repository() string { return c.owner + "/" + c.repo }

// Settings are the board settings the client was built from.
func (c *Client) Settings() config.Board { return c.settings }

// Token resolves the token to authenticate with: the environment first, so a
// cloud container carrying GH_TOKEN and no gh binary works, then gh itself.
//
// Asking gh rather than reading its configuration is what keeps keyring
// storage, GH_HOST and enterprise accounts out of this package. gh already
// resolves all three and prints the answer.
func Token() (string, error) {
	for _, name := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value, nil
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if token := strings.TrimSpace(string(out)); err == nil && token != "" {
		return token, nil
	}
	return "", fmt.Errorf("no GitHub token: set GH_TOKEN, or run gh auth login")
}

// endpoints returns the REST and GraphQL URLs for a host. Empty is
// github.com, whose API lives on its own domain; every other host is an
// enterprise server, where both APIs are paths under it.
func endpoints(host string) (rest, graphql string) {
	host = strings.TrimSuffix(strings.TrimSpace(host), "/")
	if host == "" || host == "github.com" {
		return "https://api.github.com", "https://api.github.com/graphql"
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	return host + "/api/v3", host + "/api/graphql"
}

// repository returns the owner and name to filter reads by. The setting wins;
// without one the origin remote of the directory the command runs in answers,
// because that is the repository whose work is being claimed.
func repository(configured, dir string) (owner, repo string, err error) {
	if configured != "" {
		owner, repo, ok := split(configured)
		if !ok {
			return "", "", fmt.Errorf("board.repository is %q, want owner/name", configured)
		}
		return owner, repo, nil
	}

	args := []string{"remote", "get-url", "origin"}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", "", fmt.Errorf("no origin remote to read the repository from: name it as board.repository")
	}
	owner, repo, ok := splitRemote(strings.TrimSpace(string(out)))
	if !ok {
		return "", "", fmt.Errorf("origin remote %q is not a GitHub repository: name one as board.repository",
			strings.TrimSpace(string(out)))
	}
	return owner, repo, nil
}

// splitRemote reads owner and name out of a remote URL, in either of the two
// forms git writes: scp-style ssh, and a URL.
func splitRemote(remote string) (owner, repo string, ok bool) {
	path := remote
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", "", false
		}
		path = u.Path
	} else if _, after, found := strings.Cut(remote, ":"); found {
		path = after
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[len(parts)-2], parts[len(parts)-1], true
}

func split(nameWithOwner string) (owner, repo string, ok bool) {
	owner, repo, ok = strings.Cut(nameWithOwner, "/")
	return owner, repo, ok && owner != "" && repo != "" && !strings.Contains(repo, "/")
}

// call sends one request and decodes the response into out, which may be nil
// when the caller wants only the status.
func (c *Client) call(ctx context.Context, method, endpoint string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		// Without this the dependency endpoints fail 415 with a body that is
		// valid JSON and headers that are otherwise right.
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return apiError(method, endpoint, resp.StatusCode, payload)
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

// apiError turns a failed response into a line an operator can act on. curl
// without -f and a model reading only the exit status share the same failure:
// a write that never landed reported as success.
func apiError(method, endpoint string, status int, payload []byte) error {
	var reply struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(payload, &reply)

	message := reply.Message
	for _, e := range reply.Errors {
		if e.Message != "" {
			message += ": " + e.Message
		} else if e.Code != "" {
			message += ": " + e.Code
		}
	}
	if strings.TrimSpace(message) == "" {
		message = strings.TrimSpace(string(payload))
	}
	if message == "" {
		message = http.StatusText(status)
	}
	return fmt.Errorf("%s %s: %d %s%s", method, path(endpoint), status, message, scopeHint(status, message))
}

// scopeHint names the fix for the one failure a correct call still gets: a
// token without the project scope, which reads as a permission error about
// something the caller can see in the browser.
func scopeHint(status int, message string) string {
	if status != 401 && status != 403 && !strings.Contains(strings.ToLower(message), "scope") {
		return ""
	}
	return " (the project scope is what reads and writes a board: gh auth refresh -s project)"
}

// path shortens an endpoint for an error message. The host is the same for
// every call in a run and the path is what differs.
func path(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Path != "" {
		return u.Path
	}
	return endpoint
}
