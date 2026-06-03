package git

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const (
	defaultGitDiffMaxBytes = 32 * 1024
	maximumGitDiffMaxBytes = 128 * 1024
)

type StatusInput struct{}

type DiffInput struct {
	Staged    bool     `json:"staged,omitempty" jsonschema:"description=Show staged changes instead of unstaged changes."`
	Ref       string   `json:"ref,omitempty" jsonschema:"description=Optional ref or ref range."`
	Paths     []string `json:"paths,omitempty" jsonschema:"description=Limit diff to paths."`
	StatOnly  bool     `json:"stat_only,omitempty" jsonschema:"description=Show only diffstat instead of full patch."`
	NamesOnly bool     `json:"names_only,omitempty" jsonschema:"description=Show only changed file names instead of full patch."`
	MaxBytes  int      `json:"max_bytes,omitempty" jsonschema:"description=Maximum diff text bytes returned. Defaults to a compact provider-safe limit."`
}

type AddInput struct {
	All   bool     `json:"all,omitempty" jsonschema:"description=Stage all tracked and untracked workspace changes, equivalent to git add -A."`
	Paths []string `json:"paths,omitempty" jsonschema:"description=Paths to stage. Required unless all is true."`
}

type CommitInput struct {
	Message    string   `json:"message" jsonschema:"description=Commit message. Prefer a concise conventional or semantic commit subject with optional body."`
	Stage      bool     `json:"stage,omitempty" jsonschema:"description=Stage paths or all changes before committing."`
	All        bool     `json:"all,omitempty" jsonschema:"description=When stage is true, stage all tracked and untracked workspace changes with git add -A."`
	Paths      []string `json:"paths,omitempty" jsonschema:"description=When stage is true, stage only these paths unless all is true."`
	AllowEmpty bool     `json:"allow_empty,omitempty" jsonschema:"description=Allow creating an empty commit."`
}

type TagInput struct {
	Name    string `json:"name" jsonschema:"description=Tag name to create.,required"`
	Ref     string `json:"ref,omitempty" jsonschema:"description=Optional commit-ish to tag. Defaults to HEAD."`
	Message string `json:"message,omitempty" jsonschema:"description=Annotated tag message. When set, creates an annotated tag."`
}

type PushInput struct {
	Remote         string   `json:"remote,omitempty" jsonschema:"description=Remote name or URL. Defaults to origin."`
	Refspecs       []string `json:"refspecs,omitempty" jsonschema:"description=Explicit refspecs to push, for example main or HEAD:refs/heads/main."`
	Tags           bool     `json:"tags,omitempty" jsonschema:"description=Push tags with --tags."`
	SetUpstream    bool     `json:"set_upstream,omitempty" jsonschema:"description=Set upstream tracking with -u."`
	ForceWithLease bool     `json:"force_with_lease,omitempty" jsonschema:"description=Use --force-with-lease. Raw force refspecs are rejected."`
	DryRun         bool     `json:"dry_run,omitempty" jsonschema:"description=Show what would be pushed without updating the remote."`
}

type GitResult struct {
	Text    string         `json:"text,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func Status(ctx pluginbinding.Context, _ StatusInput) (GitResult, error) {
	run, err := runGit(ctx, []string{"status", "--short", "--branch"}, processLimits{TimeoutMS: 30000})
	data := processData(run)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_status_failed", gitProcessErrorMessage(err, run.Stderr))
	}
	text := strings.TrimSpace(run.Stdout)
	if text == "" {
		text = "No git status output."
	}
	return gitResult(text, data), nil
}

func Diff(ctx pluginbinding.Context, req DiffInput) (GitResult, error) {
	args, err := gitDiffArgs(req)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("invalid_git_diff_input", err.Error())
	}
	maxBytes := gitDiffMaxBytes(req)
	run, err := runGit(ctx, args, processLimits{TimeoutMS: 30000, MaxStdout: 256 * 1024})
	text, truncated := capGitDiffText(strings.TrimSpace(run.Stdout), maxBytes)
	data := processData(run)
	data["stdout"] = text
	data["stderr"] = compactGitErrorText(run.Stderr)
	data["mode"] = gitDiffMode(req)
	data["truncated"] = truncated
	data["max_bytes"] = maxBytes
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_diff_failed", gitProcessErrorMessage(err, run.Stderr))
	}
	if text == "" {
		text = "No changes."
	}
	if truncated {
		text += "\n\n[git diff truncated; narrow paths or use stat_only, names_only, or a larger max_bytes.]"
	}
	return gitResult(text, data), nil
}

func Add(ctx pluginbinding.Context, req AddInput) (GitResult, error) {
	args, err := gitAddArgs(req.All, req.Paths)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("invalid_git_add_input", err.Error())
	}
	run, err := runGit(ctx, args, processLimits{TimeoutMS: 30000})
	data := processData(run)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_add_failed", err.Error())
	}
	return gitResult(processText(run, "Staged changes."), data), nil
}

func Commit(ctx pluginbinding.Context, req CommitInput) (GitResult, error) {
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return GitResult{}, pluginbinding.Fail("invalid_git_commit_input", "message is required")
	}
	if !req.Stage && (req.All || len(req.Paths) > 0) {
		return GitResult{}, pluginbinding.Fail("invalid_git_commit_input", "paths or all require stage to be true")
	}
	if req.Stage {
		args, err := gitAddArgs(req.All, req.Paths)
		if err != nil {
			return GitResult{}, pluginbinding.Fail("invalid_git_commit_input", err.Error())
		}
		_, err = runGit(ctx, args, processLimits{TimeoutMS: 30000})
		if err != nil {
			return GitResult{}, pluginbinding.Fail("git_commit_stage_failed", err.Error())
		}
	}
	args := []string{"-c", "core.hooksPath=/dev/null", "commit", "--no-verify", "--no-gpg-sign"}
	if req.AllowEmpty {
		args = append(args, "--allow-empty")
	}
	args = append(args, "-m", message)
	commitRun, err := runGit(ctx, args, processLimits{TimeoutMS: 30000, MaxStdout: 128 * 1024, MaxStderr: 128 * 1024})
	data := processData(commitRun)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_commit_failed", err.Error())
	}
	headRun, err := runGit(ctx, []string{"rev-parse", "HEAD"}, processLimits{TimeoutMS: 30000, MaxStdout: 1024, MaxStderr: 1024})
	if err != nil {
		data["rev_parse_stdout"] = headRun.Stdout
		data["rev_parse_stderr"] = headRun.Stderr
		data["rev_parse_exit_code"] = headRun.ExitCode
		return GitResult{}, pluginbinding.Fail("git_commit_rev_parse_failed", err.Error())
	}
	commit := strings.TrimSpace(headRun.Stdout)
	data["commit"] = commit
	text := commitText(commit, commitRun)
	if req.Stage && !req.All && len(req.Paths) > 0 {
		if dirty := remainingDirtyFiles(ctx); len(dirty) > 0 {
			data["remaining_dirty"] = dirty
			text += "\n\nUncommitted changes remain in: " + strings.Join(dirty, ", ")
		}
	}
	return gitResult(text, data), nil
}

func Tag(ctx pluginbinding.Context, req TagInput) (GitResult, error) {
	args, err := gitTagArgs(req)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("invalid_git_tag_input", err.Error())
	}
	run, err := runGit(ctx, args, processLimits{TimeoutMS: 30000, MaxStdout: 128 * 1024, MaxStderr: 128 * 1024})
	data := processData(run)
	data["tag"] = strings.TrimSpace(req.Name)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_tag_failed", err.Error())
	}
	return gitResult(processText(run, "Created tag "+strings.TrimSpace(req.Name)), data), nil
}

func Push(ctx pluginbinding.Context, req PushInput) (GitResult, error) {
	args, err := gitPushArgs(req)
	if err != nil {
		return GitResult{}, pluginbinding.Fail("invalid_git_push_input", err.Error())
	}
	run, err := runGit(ctx, args, processLimits{TimeoutMS: 120000, MaxStdout: 128 * 1024, MaxStderr: 128 * 1024})
	data := processData(run)
	data["remote"] = gitPushRemote(req)
	data["refspecs"] = append([]string(nil), req.Refspecs...)
	data["tags"] = req.Tags
	data["dry_run"] = req.DryRun
	if err != nil {
		return GitResult{}, pluginbinding.Fail("git_push_failed", err.Error())
	}
	return gitResult(processText(run, "Pushed to "+gitPushRemote(req)), data), nil
}

type processLimits struct {
	TimeoutMS int
	MaxStdout int64
	MaxStderr int64
}

func runGit(ctx pluginbinding.Context, args []string, limits processLimits) (pluginbinding.ProcessRunResponse, error) {
	return ctx.Host.ProcessRun(pluginbinding.ProcessRunRequest{
		Command:   "git",
		Args:      append([]string(nil), args...),
		TimeoutMS: limits.TimeoutMS,
		MaxStdout: limits.MaxStdout,
		MaxStderr: limits.MaxStderr,
		Label:     "git " + strings.Join(args, " "),
		Group:     "git",
		Tags:      []string{"git"},
	})
}

func gitDiffArgs(req DiffInput) ([]string, error) {
	if req.StatOnly && req.NamesOnly {
		return nil, fmt.Errorf("stat_only and names_only cannot be combined")
	}
	args := []string{"diff"}
	if req.Staged {
		args = append(args, "--staged")
	}
	switch {
	case req.StatOnly:
		args = append(args, "--stat")
	case req.NamesOnly:
		args = append(args, "--name-only")
	}
	if ref := strings.TrimSpace(req.Ref); ref != "" {
		args = append(args, ref)
	}
	if len(req.Paths) > 0 {
		args = append(args, "--")
		args = append(args, req.Paths...)
	}
	return args, nil
}

func gitDiffMode(req DiffInput) string {
	switch {
	case req.StatOnly:
		return "stat"
	case req.NamesOnly:
		return "names"
	default:
		return "patch"
	}
}

func gitDiffMaxBytes(req DiffInput) int {
	if req.MaxBytes <= 0 {
		return defaultGitDiffMaxBytes
	}
	if req.MaxBytes > maximumGitDiffMaxBytes {
		return maximumGitDiffMaxBytes
	}
	return req.MaxBytes
}

func capGitDiffText(text string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text, false
	}
	if maxBytes < 4 {
		return text[:maxBytes], true
	}
	return text[:maxBytes-3] + "...", true
}

func gitAddArgs(all bool, paths []string) ([]string, error) {
	if all {
		return []string{"add", "-A"}, nil
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("all must be true or at least one path is required")
	}
	args := []string{"add", "--"}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("paths must not contain empty values")
		}
		args = append(args, path)
	}
	return args, nil
}

func gitTagArgs(req TagInput) ([]string, error) {
	name := strings.TrimSpace(req.Name)
	if err := validateGitToken(name, "tag name"); err != nil {
		return nil, err
	}
	args := []string{"tag"}
	message := strings.TrimSpace(req.Message)
	if message != "" {
		args = append(args, "-a", name, "-m", message)
	} else {
		args = append(args, name)
	}
	ref := strings.TrimSpace(req.Ref)
	if ref != "" {
		if err := validateGitToken(ref, "ref"); err != nil {
			return nil, err
		}
		args = append(args, ref)
	}
	return args, nil
}

func gitPushArgs(req PushInput) ([]string, error) {
	remote := gitPushRemote(req)
	if err := validateGitToken(remote, "remote"); err != nil {
		return nil, err
	}
	if !req.Tags && len(req.Refspecs) == 0 {
		return nil, fmt.Errorf("refspecs or tags are required")
	}
	args := []string{"push"}
	if req.DryRun {
		args = append(args, "--dry-run")
	}
	if req.SetUpstream {
		args = append(args, "-u")
	}
	if req.ForceWithLease {
		args = append(args, "--force-with-lease")
	}
	if req.Tags {
		args = append(args, "--tags")
	}
	args = append(args, remote)
	for _, refspec := range req.Refspecs {
		refspec = strings.TrimSpace(refspec)
		if err := validateGitRefspec(refspec); err != nil {
			return nil, err
		}
		args = append(args, refspec)
	}
	return args, nil
}

func gitPushRemote(req PushInput) string {
	remote := strings.TrimSpace(req.Remote)
	if remote == "" {
		return "origin"
	}
	return remote
}

func validateGitToken(value, label string) error {
	if value == "" {
		return fmt.Errorf("%s is required", label)
	}
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("%s must not start with '-'", label)
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s must not contain whitespace", label)
	}
	if strings.Contains(value, "..") || strings.Contains(value, "@{") || strings.Contains(value, "\\") {
		return fmt.Errorf("%s is not a safe git ref token", label)
	}
	if strings.HasSuffix(value, ".") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".lock") {
		return fmt.Errorf("%s is not a safe git ref token", label)
	}
	return nil
}

func validateGitRefspec(value string) error {
	if err := validateGitToken(value, "refspec"); err != nil {
		return err
	}
	if strings.HasPrefix(value, "+") {
		return fmt.Errorf("force refspecs are rejected; use force_with_lease")
	}
	return nil
}

func gitProcessErrorMessage(err error, stderr string) string {
	if strings.Contains(stderr, "Not a git repository") || strings.Contains(stderr, "not a git repository") {
		return "workspace is not a git repository"
	}
	if line := firstNonEmptyLine(stderr); line != "" {
		return line
	}
	if err != nil {
		return err.Error()
	}
	return "git command failed"
}

func compactGitErrorText(stderr string) string {
	if strings.Contains(stderr, "Not a git repository") || strings.Contains(stderr, "not a git repository") {
		return firstNonEmptyLine(stderr)
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	out := make([]string, 0, 6)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= 6 {
			break
		}
	}
	text := strings.Join(out, "\n")
	if len(out) < len(lines) && text != "" {
		text += "\n[git stderr truncated]"
	}
	return text
}

func firstNonEmptyLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func processData(result pluginbinding.ProcessRunResponse) map[string]any {
	return map[string]any{
		"stdout":            result.Stdout,
		"stderr":            result.Stderr,
		"exit_code":         result.ExitCode,
		"stdout_truncated":  result.StdoutTruncated,
		"stderr_truncated":  result.StderrTruncated,
		"process_timed_out": result.TimedOut,
	}
}

func processText(result pluginbinding.ProcessRunResponse, fallback string) string {
	var parts []string
	if stdout := strings.TrimSpace(result.Stdout); stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr := strings.TrimSpace(result.Stderr); stderr != "" {
		parts = append(parts, stderr)
	}
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, "\n")
}

func commitText(commit string, result pluginbinding.ProcessRunResponse) string {
	text := "Committed " + commit
	if output := processText(result, ""); output != "" {
		text += "\n" + output
	}
	return text
}

func remainingDirtyFiles(ctx pluginbinding.Context) []string {
	result, err := runGit(ctx, []string{"status", "--porcelain"}, processLimits{TimeoutMS: 30000})
	if err != nil {
		return nil
	}
	var dirty []string
	for _, line := range strings.Split(result.Stdout, "\n") {
		if len(line) < 4 {
			continue
		}
		xy := line[:2]
		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}
		if xy == "??" || xy[1] != ' ' {
			dirty = append(dirty, path)
		}
	}
	return dirty
}

func gitResult(text string, data map[string]any) GitResult {
	return GitResult{Text: text, Summary: firstNonEmptyLine(text), Data: data}
}
