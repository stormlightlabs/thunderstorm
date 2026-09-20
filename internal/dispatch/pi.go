package dispatch

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// thinkingLevels are what pi's --thinking takes. A level it does not know is
// rejected here rather than at the far end of a tmux pane, where the only
// evidence would be a session that exited before it started.
var thinkingLevels = []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"}
var multiplexers = []string{"auto", "tmux", "zellij"}

// ThinkingLevels lists the levels for help text and error messages.
func ThinkingLevels() string { return strings.Join(thinkingLevels, ", ") }

// Multiplexers lists the terminal multiplexers dispatch can drive.
func Multiplexers() string { return strings.Join(multiplexers, ", ") }

// Request is one dispatch: a role, the directory it works in, and the model
// it runs.
type Request struct {
	Role        Role
	Worktree    string
	Provider    string
	Model       string
	Thinking    string
	Multiplexer string
	Task        string
	// Out holds the transcript. Empty means a new directory under the system
	// temporary directory, which the Result names.
	Out     string
	Timeout time.Duration
}

// Result is what the next pass reads: the role's own report, and where the
// evidence for it sits.
type Result struct {
	Role string
	// Model is what the transcript says answered, which is not always what
	// the request asked for. models.md wants the run's evidence, not its
	// intent.
	Model       string
	Thinking    string
	Status      int
	Report      string
	Out         string
	Session     string
	Multiplexer string
	// Untranslated names the tools the definition allows that pi has no
	// equivalent for.
	Untranslated []string
}

// unsafeName is everything a tmux session name and a socket name should not
// carry. tmux reads a colon and a full stop as target syntax.
var unsafeName = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// Run starts the role in a multiplexer tab and waits for it to finish.
//
// Inside tmux or Zellij, dispatch adds a tab to the current session so the
// operator can watch and control the worker. Outside either one, auto retains
// the older detached-tmux behavior so the worker can outlive its caller.
func Run(ctx context.Context, req Request) (Result, error) {
	var res Result
	if err := req.validate(); err != nil {
		return res, err
	}

	multiplexer, err := selectMultiplexer(req.Multiplexer)
	if err != nil {
		return res, err
	}
	pi, err := exec.LookPath("pi")
	if err != nil {
		return res, fmt.Errorf("pi is not installed: %w", err)
	}

	out, err := req.outDir()
	if err != nil {
		return res, err
	}
	requestedModel := req.Model
	if req.Provider != "" {
		requestedModel = req.Provider + "/" + requestedModel
	}
	res = Result{
		Role: req.Role.Name, Model: requestedModel, Thinking: req.Thinking,
		Out: out, Multiplexer: multiplexer,
	}
	_, res.Untranslated = PiTools(req.Role.ClaudeTools)

	system := filepath.Join(out, "system.md")
	if err := os.WriteFile(system, []byte(SystemPrompt(req)), 0o644); err != nil {
		return res, err
	}
	if err := os.WriteFile(filepath.Join(out, "task.md"), []byte(req.Task+"\n"), 0o644); err != nil {
		return res, err
	}

	argv := piArgv(pi, req, out, system)
	// The command records what was asked for, the model and the reasoning
	// level, where events.jsonl records what answered.
	if err := os.WriteFile(filepath.Join(out, "command"), []byte(strings.Join(argv, "\n")+"\n"), 0o644); err != nil {
		return res, err
	}
	script := filepath.Join(out, "run.sh")
	if err := os.WriteFile(script, []byte(runScript(argv, out)), 0o755); err != nil {
		return res, err
	}

	name := unsafeName.ReplaceAllString("tstorm-"+filepath.Base(out), "-")
	res.Session = name
	if err := runInMultiplexer(ctx, multiplexer, name, script, out, req); err != nil {
		return res, fmt.Errorf("%w; the transcript so far is %s", err, filepath.Join(out, "events.jsonl"))
	}

	status, err := readStatus(filepath.Join(out, "status"))
	if err != nil {
		return res, fmt.Errorf("the %s session left no exit status in %s, so it died rather than finished: %w",
			req.Role.Name, out, err)
	}
	res.Status = status

	report, model, err := readTranscript(filepath.Join(out, "events.jsonl"))
	if err != nil {
		return res, err
	}
	res.Report = report
	if model != "" {
		res.Model = model
	}
	if status != 0 {
		return res, fmt.Errorf("the %s session exited %d; see %s", req.Role.Name, status, out)
	}
	return res, nil
}

func (r Request) validate() error {
	switch {
	case r.Role.Name == "":
		return errors.New("no role")
	case r.Model == "":
		return errors.New("no model; a session that names none inherits one and records nothing")
	case r.Multiplexer != "" && !slices.Contains(multiplexers, r.Multiplexer):
		return fmt.Errorf("unknown multiplexer %q; try one of %s", r.Multiplexer, Multiplexers())
	case !slices.Contains(thinkingLevels, r.Thinking):
		return fmt.Errorf("pi has no thinking level %q; try one of %s", r.Thinking, ThinkingLevels())
	case strings.TrimSpace(r.Task) == "":
		return errors.New("no task")
	case r.Timeout <= 0:
		return errors.New("no timeout; a pane nothing gives up on holds the run open forever")
	}
	info, err := os.Stat(r.Worktree)
	if err != nil {
		return fmt.Errorf("the %s has nowhere to work: %w", r.Role.Name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", r.Worktree)
	}
	return nil
}

// outDir returns the directory for this run's evidence, refusing one that
// already holds a transcript rather than writing a second run over the first.
func (r Request) outDir() (string, error) {
	if r.Out == "" {
		return os.MkdirTemp("", "tstorm-"+r.Role.Name+"-")
	}
	out, err := filepath.Abs(r.Out)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(out, "events.jsonl")); err == nil {
		return "", fmt.Errorf("%s already holds a transcript; choose another --out", out)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return out, nil
}

// SystemPrompt is the role's definition with what the definition cannot know
// appended: the model, the reasoning level, and the directory this dispatch
// gave it. A review comment's first line has to name the model and the level,
// and on Pi nothing else tells the session either one.
func SystemPrompt(r Request) string {
	var b strings.Builder
	b.WriteString(r.Role.Prompt)
	b.WriteString("\n\n## This dispatch\n\n")
	fmt.Fprintf(&b, "You are the `%s` of a thunderstorm run. You are running on pi as\n", r.Role.Name)
	model := r.Model
	if r.Provider != "" {
		model = r.Provider + "/" + model
	}
	fmt.Fprintf(&b, "`%s` at reasoning level `%s`, in `%s`.\n", model, r.Thinking, r.Worktree)
	b.WriteString("\nName that model and that level where your role says to name them, and\n")
	b.WriteString("stay in that directory: it is yours, and the rest of the checkout is\n")
	b.WriteString("somebody else's.\n\nThis session has no GitHub MCP tools. Reach GitHub through `gh`.\n")
	return b.String()
}

// piArgv is the command the pane runs.
//
// --print and --mode json because nothing is attached to read a TUI and the
// event stream is what the next pass gets parsed for it. --approve because a
// non-interactive pi cannot ask whether to trust the project, and without the
// answer it loads none of the skills the role is told to use.
func piArgv(pi string, r Request, out, system string) []string {
	allow, _ := PiTools(r.Role.ClaudeTools)
	args := []string{
		pi,
		"--mode", "json",
		"--print",
		"--approve",
		"--session-dir", filepath.Join(out, "sessions"),
	}
	if r.Provider != "" {
		args = append(args, "--provider", r.Provider)
	}
	return append(args,
		"--model", r.Model,
		"--thinking", r.Thinking,
		"--tools", strings.Join(allow, ","),
		"--append-system-prompt", system,
		"--", r.Task,
	)
}

// runScript redirects the session's output and records its exit status. A run
// that finished is told from a pane that was killed by whether the status file
// is there at all.
func runScript(argv []string, out string) string {
	quoted := make([]string, 0, len(argv))
	for _, a := range argv {
		quoted = append(quoted, shellQuote(a))
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# Written by tstorm dispatch. One role, one pane, one transcript.\n")
	fmt.Fprintf(&b, "%s >%s 2>%s\n", strings.Join(quoted, " "),
		shellQuote(filepath.Join(out, "events.jsonl")), shellQuote(filepath.Join(out, "pi.err")))
	fmt.Fprintf(&b, "printf '%%s\\n' \"$?\" >%s\n", shellQuote(filepath.Join(out, "status")))
	return b.String()
}

// shellQuote wraps a word so /bin/sh reads it as one argument whatever it
// holds. A single quote is the only character that cannot appear inside the
// quotes, so it is closed, escaped, and reopened.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func selectMultiplexer(requested string) (string, error) {
	if requested == "" {
		requested = "auto"
	}
	if requested != "auto" {
		if requested == "zellij" && os.Getenv("ZELLIJ") == "" {
			return "", errors.New("zellij dispatch needs Pi to be running inside Zellij")
		}
		if _, err := exec.LookPath(requested); err != nil {
			return "", fmt.Errorf("%s is not installed: %w", requested, err)
		}
		return requested, nil
	}
	if os.Getenv("ZELLIJ") != "" {
		if _, err := exec.LookPath("zellij"); err != nil {
			return "", fmt.Errorf("Pi is running inside Zellij, but zellij is not on PATH: %w", err)
		}
		return "zellij", nil
	}
	if os.Getenv("TMUX") != "" {
		if _, err := exec.LookPath("tmux"); err != nil {
			return "", fmt.Errorf("Pi is running inside tmux, but tmux is not on PATH: %w", err)
		}
		return "tmux", nil
	}
	if _, err := exec.LookPath("tmux"); err == nil {
		return "tmux", nil
	}
	return "", errors.New("dispatch needs tmux or Zellij; run Pi inside one of them")
}

func runInMultiplexer(ctx context.Context, multiplexer, name, script, out string, req Request) error {
	if multiplexer == "zellij" {
		return runInZellij(ctx, name, script, out, req)
	}
	return runInTmux(ctx, name, script, req)
}

func runInTmux(ctx context.Context, name, script string, req Request) error {
	tmux, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux is not installed: %w", err)
	}
	if os.Getenv("TMUX") != "" {
		start := exec.CommandContext(ctx, tmux, "new-window", "-d", "-P", "-F", "#{pane_id}",
			"-n", name, "-c", req.Worktree, "/bin/sh "+shellQuote(script))
		output, err := start.CombinedOutput()
		if err != nil {
			return fmt.Errorf("tmux could not open the %s tab: %w: %s",
				req.Role.Name, err, strings.TrimSpace(string(output)))
		}
		pane := strings.TrimSpace(string(output))
		return waitForTmuxPane(ctx, tmux, pane, req.Timeout)
	}

	start := exec.CommandContext(ctx, tmux, "-L", name, "new-session", "-d",
		"-s", name, "-c", req.Worktree, "/bin/sh "+shellQuote(script))
	if output, err := start.CombinedOutput(); err != nil {
		return fmt.Errorf("tmux could not start the %s session: %w: %s",
			req.Role.Name, err, strings.TrimSpace(string(output)))
	}
	socket := socketPath(tmux, name)
	defer removeSocket(socket, name)
	return wait(ctx, tmux, name, req.Timeout)
}

func waitForTmuxPane(ctx context.Context, tmux, pane string, timeout time.Duration) error {
	return poll(ctx, timeout, func() bool {
		return exec.Command(tmux, "display-message", "-p", "-t", pane, "#{pane_id}").Run() != nil
	}, func() { _ = exec.Command(tmux, "kill-pane", "-t", pane).Run() })
}

func runInZellij(ctx context.Context, name, script, out string, req Request) error {
	zellij, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij is not installed: %w", err)
	}
	start := exec.CommandContext(ctx, zellij, "action", "new-tab", "--no-focus", "--close-on-exit",
		"--name", name, "--cwd", req.Worktree, "--", "/bin/sh", script)
	output, err := start.CombinedOutput()
	if err != nil {
		return fmt.Errorf("zellij could not open the %s tab: %w: %s",
			req.Role.Name, err, strings.TrimSpace(string(output)))
	}
	tab := strings.TrimSpace(string(output))
	status := filepath.Join(out, "status")
	return poll(ctx, req.Timeout, func() bool {
		_, err := os.Stat(status)
		return err == nil
	}, func() { _ = exec.Command(zellij, "action", "close-tab-by-id", tab).Run() })
}

func poll(ctx context.Context, timeout time.Duration, done func() bool, cancel func()) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if done() {
			return nil
		}
		if time.Now().After(deadline) {
			cancel()
			return fmt.Errorf("the session was still running after %s and was killed", timeout)
		}
		select {
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// wait blocks until the detached tmux session is gone.
//
// Asking tmux whether the session still exists covers a pane that finished and
// a pane that was killed with one question. tmux wait-for would be woken only
// by a pane that reached the end of its script, so anything killed from
// outside would hold the run until the timeout.
func wait(ctx context.Context, tmux, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := exec.Command(tmux, "-L", name, "has-session", "-t", name).Run(); err != nil {
			return nil
		}
		if time.Now().After(deadline) {
			_ = exec.Command(tmux, "-L", name, "kill-server").Run()
			return fmt.Errorf("the session was still running after %s and was killed", timeout)
		}
		select {
		case <-ctx.Done():
			_ = exec.Command(tmux, "-L", name, "kill-server").Run()
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func socketPath(tmux, name string) string {
	out, err := exec.Command(tmux, "-L", name, "display-message", "-p", "#{socket_path}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// removeSocket uses the path reported by a live server when it has one. A
// short session can exit before socketPath asks, so the fallback finds the
// named socket in tmux's socket directory.
func removeSocket(socket, name string) {
	if socket != "" {
		_ = os.Remove(socket)
		return
	}
	root := os.Getenv("TMUX_TMPDIR")
	if root == "" {
		root = "/tmp"
	}
	matches, _ := filepath.Glob(filepath.Join(root, "tmux-*", name))
	for _, match := range matches {
		_ = os.Remove(match)
	}
}

func readStatus(path string) (int, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(body)))
}

// event is the part of pi's JSON stream this reads: the assistant messages,
// and the model each one came from.
type event struct {
	Type    string `json:"type"`
	Message struct {
		Role     string `json:"role"`
		Model    string `json:"model"`
		Provider string `json:"provider"`
		Content  []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// readTranscript returns the session's last assistant message and the model
// that produced it. The last one is the role's report: pi's -p mode ends the
// turn when the model stops calling tools.
//
// A line it cannot parse is skipped. The stream is pi's and it carries events
// this does not model, and a report refused over an unfamiliar event would
// lose work that already ran.
func readTranscript(path string) (report, model string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("no transcript: %w", err)
	}
	defer f.Close()

	// A single event carries a whole message, which outgrows any fixed buffer
	// a Scanner would be given.
	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			var e event
			if json.Unmarshal([]byte(line), &e) == nil &&
				e.Type == "message_end" && e.Message.Role == "assistant" {
				var text []string
				for _, c := range e.Message.Content {
					if c.Type == "text" && strings.TrimSpace(c.Text) != "" {
						text = append(text, c.Text)
					}
				}
				if len(text) > 0 {
					report = strings.Join(text, "\n\n")
					model = e.Message.Model
					if e.Message.Provider != "" {
						model = e.Message.Provider + "/" + model
					}
				}
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", "", err
		}
	}
	if strings.TrimSpace(report) == "" {
		return "", model, fmt.Errorf("the transcript at %s holds no report", path)
	}
	return report, model, nil
}
