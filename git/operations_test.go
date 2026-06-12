package git

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

func TestStatusUsesHostProcess(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{stdout: "## main\n M file.txt\n"}}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationStatus, StatusInput{}, plugintest.WithHost(host))
	if !strings.Contains(out.Text, "M file.txt") {
		t.Fatalf("text = %q", out.Text)
	}
	host.expectCalls(t, []string{"status --short --branch"})
}

func TestDiffBuildsCompactArgsAndBoundsOutput(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{stdout: strings.Repeat("x", 80)}}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationDiff, DiffInput{
		Staged:   true,
		StatOnly: true,
		Ref:      "HEAD~1",
		Paths:    []string{"file.txt"},
		MaxBytes: 20,
	}, plugintest.WithHost(host))
	if !strings.Contains(out.Text, "[git diff truncated") || !out.Data.Truncated {
		t.Fatalf("diff output = %#v", out)
	}
	if out.Data.Mode != "stat" || out.Data.MaxBytes != 20 {
		t.Fatalf("diff data = %#v", out.Data)
	}
	host.expectCalls(t, []string{"diff --staged --stat HEAD~1 -- file.txt"})
}

func TestDiffRejectsConflictingModes(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationDiff, DiffInput{StatOnly: true, NamesOnly: true})
	if err.Code != "invalid_git_diff_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestDiffReportsNonRepositoryConciseError(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{
		stderr: "fatal: not a git repository (or any of the parent directories): .git\nusage: git diff\n",
		err:    errors.New("exit status 129"),
	}}}
	err := plugintest.RunError(t, NewPlugin(), OperationDiff, DiffInput{}, plugintest.WithHost(host))
	if err.Code != "git_diff_failed" || err.Message != "workspace is not a git repository" {
		t.Fatalf("error = %#v", err)
	}
}

func TestAddRejectsEmptyRequestAndStagesExplicitPaths(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationAdd, AddInput{})
	if err.Code != "invalid_git_add_input" {
		t.Fatalf("empty add error = %#v", err)
	}
	host := &fakeHost{responses: []processResponse{{}}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationAdd, AddInput{Paths: []string{"README.md"}}, plugintest.WithHost(host))
	if out.Text != "Staged changes." {
		t.Fatalf("text = %q", out.Text)
	}
	host.expectCalls(t, []string{"add -- README.md"})
}

func TestCommitStagesCommitsAndReturnsHash(t *testing.T) {
	host := &fakeHost{responses: []processResponse{
		{},
		{stdout: "[main abc123] test\n"},
		{stdout: "abc123\n"},
		{stdout: " M other.go\n?? new.go\n"},
	}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationCommit, CommitInput{
		Message: "test: add file",
		Stage:   true,
		Paths:   []string{"file.go"},
	}, plugintest.WithHost(host))
	if out.Data.Commit != "abc123" || !strings.Contains(out.Text, "Uncommitted changes remain in: other.go, new.go") {
		t.Fatalf("commit output = %#v", out)
	}
	host.expectCalls(t, []string{
		"add -- file.go",
		"-c core.hooksPath=/dev/null commit --no-verify --no-gpg-sign -m test: add file",
		"rev-parse HEAD",
		"status --porcelain",
	})
}

func TestCommitRejectsInvalidInputs(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationCommit, CommitInput{})
	if err.Code != "invalid_git_commit_input" {
		t.Fatalf("empty commit error = %#v", err)
	}
	err = plugintest.RunError(t, NewPlugin(), OperationCommit, CommitInput{Message: "x", Paths: []string{"file.go"}})
	if err.Code != "invalid_git_commit_input" {
		t.Fatalf("paths without stage error = %#v", err)
	}
}

func TestTagBuildsAnnotatedTagAndRejectsUnsafeName(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{}}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationTag, TagInput{
		Name:    "v1.0.0",
		Message: "release",
		Ref:     "HEAD",
	}, plugintest.WithHost(host))
	if out.Data.Tag != "v1.0.0" {
		t.Fatalf("tag output = %#v", out)
	}
	host.expectCalls(t, []string{"tag -a v1.0.0 -m release HEAD"})

	err := plugintest.RunError(t, NewPlugin(), OperationTag, TagInput{Name: "-bad"})
	if err.Code != "invalid_git_tag_input" {
		t.Fatalf("unsafe tag error = %#v", err)
	}
}

func TestPushBuildsSafeExplicitPush(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{stderr: "Everything up-to-date\n"}}}
	out := plugintest.RunOK[GitResult](t, NewPlugin(), OperationPush, PushInput{
		Remote:         "origin",
		Refspecs:       []string{"main"},
		Tags:           true,
		SetUpstream:    true,
		ForceWithLease: true,
		DryRun:         true,
	}, plugintest.WithHost(host))
	if !strings.Contains(out.Text, "Everything up-to-date") || out.Data.Remote != "origin" {
		t.Fatalf("push output = %#v", out)
	}
	host.expectCalls(t, []string{"push --dry-run -u --force-with-lease --tags origin main"})
}

func TestPushRejectsImplicitAndRawForcePush(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationPush, PushInput{})
	if err.Code != "invalid_git_push_input" {
		t.Fatalf("empty push error = %#v", err)
	}
	err = plugintest.RunError(t, NewPlugin(), OperationPush, PushInput{Refspecs: []string{"+main"}})
	if err.Code != "invalid_git_push_input" {
		t.Fatalf("force push error = %#v", err)
	}
}

type processResponse struct {
	stdout string
	stderr string
	code   int
	err    error
}

type fakeHost struct {
	responses []processResponse
	calls     []pluginbinding.ProcessRunRequest
}

func (h *fakeHost) ProcessRun(req pluginbinding.ProcessRunRequest) (pluginbinding.ProcessRunResponse, error) {
	h.calls = append(h.calls, req)
	var resp processResponse
	if len(h.responses) > 0 {
		resp = h.responses[0]
		h.responses = h.responses[1:]
	}
	return pluginbinding.ProcessRunResponse{
		Command:  req.Command,
		Args:     append([]string(nil), req.Args...),
		ExitCode: resp.code,
		Stdout:   resp.stdout,
		Stderr:   resp.stderr,
	}, resp.err
}

func (h *fakeHost) ProcessStart(pluginbinding.ProcessStartRequest) (pluginbinding.ProcessStartResponse, error) {
	return pluginbinding.ProcessStartResponse{}, nil
}

func (h *fakeHost) ProcessStop(pluginbinding.ProcessStopRequest) (pluginbinding.ProcessStopResponse, error) {
	return pluginbinding.ProcessStopResponse{}, nil
}

func (h *fakeHost) expectCalls(t *testing.T, want []string) {
	t.Helper()
	var got []string
	for _, call := range h.calls {
		if call.Command != "git" {
			t.Fatalf("command = %q, want git", call.Command)
		}
		got = append(got, strings.Join(call.Args, " "))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("process calls = %#v, want %#v", got, want)
	}
}

func (h *fakeHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *fakeHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *fakeHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *fakeHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *fakeHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *fakeHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h *fakeHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h *fakeHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *fakeHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *fakeHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *fakeHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*fakeHost)(nil)

func TestStatusNonZeroExitIsAnError(t *testing.T) {
	// Exit 128 ("not a git repository") used to come back as a SUCCESSFUL
	// result with "No git status output."
	host := &fakeHost{responses: []processResponse{{code: 128, stderr: "fatal: not a git repository (or any of the parent directories): .git"}}}
	perr := plugintest.RunError(t, NewPlugin(), OperationStatus, StatusInput{}, plugintest.WithHost(host))
	if perr.Code != "git_status_failed" || !strings.Contains(perr.Message, "not a git repository") {
		t.Fatalf("err = %#v", perr)
	}
}

func TestRepoInputTargetsRepositoryViaDashC(t *testing.T) {
	host := &fakeHost{responses: []processResponse{{stdout: "## main\n"}}}
	_ = plugintest.RunOK[GitResult](t, NewPlugin(), OperationStatus, StatusInput{RepoInput: RepoInput{Repo: "/work/repo"}}, plugintest.WithHost(host))
	host.expectCalls(t, []string{"-C /work/repo status --short --branch"})
}

func TestRepoInputRejectsFlagInjection(t *testing.T) {
	perr := plugintest.RunError(t, NewPlugin(), OperationStatus, StatusInput{RepoInput: RepoInput{Repo: "--upload-pack=evil"}}, plugintest.WithHost(&fakeHost{}))
	if perr.Code != "invalid_git_status_input" {
		t.Fatalf("err = %#v", perr)
	}
}
