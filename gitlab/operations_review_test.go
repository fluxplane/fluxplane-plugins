package gitlab

import (
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

// reviewFakeClient layers the review-workflow surface over fakeClient.
type reviewFakeClient struct {
	*fakeClient
	fileDiffs         []FileDiff
	diffsTruncated    bool
	diffRefs          DiffRefs
	compareCommits    []CompareCommit
	compareFiles      []FileDiff
	discussions       []DiscussionInfo
	createdDiscussion DiscussionInfo
	createdPosition   *PositionInput
	createdBody       string
	replyDiscussionID string
	resolvedID        string
	resolvedValue     bool
	updateOptions     MergeRequestUpdateOptions
	treeEntries       []TreeEntry
	treeOptions       TreeListOptions
	fileRaw           RepoFileRaw
	filePath          string
	fileRef           string
	archiveData       []byte
	archiveFormat     string
	createdProject    ProjectCreateOptions
	diffsLimit        int
	blobMatches       []BlobMatch
	blobTruncated     bool
	blobProject       any
	blobGroup         any
	blobQuery         string
}

func (f *reviewFakeClient) ListMergeRequestDiffs(project any, iid int64, limit int) ([]FileDiff, bool, error) {
	f.mrProject, f.mrIID, f.diffsLimit = project, iid, limit
	return f.fileDiffs, f.diffsTruncated, nil
}

func (f *reviewFakeClient) GetMergeRequestDiffVersion(project any, iid int64) (DiffRefs, error) {
	return f.diffRefs, nil
}

func (f *reviewFakeClient) CompareRefs(project any, from, to string, straight bool) ([]CompareCommit, []FileDiff, string, error) {
	return f.compareCommits, f.compareFiles, "https://gitlab.example.com/compare", nil
}

func (f *reviewFakeClient) ListMergeRequestDiscussions(project any, iid int64, limit int) ([]DiscussionInfo, bool, error) {
	return f.discussions, false, nil
}

func (f *reviewFakeClient) CreateMergeRequestNote(project any, iid int64, body string) (Note, error) {
	f.createdBody = body
	return Note{ID: 7, Body: body}, nil
}

func (f *reviewFakeClient) CreateMergeRequestDiscussion(project any, iid int64, body string, position *PositionInput) (DiscussionInfo, error) {
	f.createdBody = body
	f.createdPosition = position
	return f.createdDiscussion, nil
}

func (f *reviewFakeClient) AddMergeRequestDiscussionNote(project any, iid int64, discussionID, body string) (Note, error) {
	f.replyDiscussionID = discussionID
	return Note{ID: 8, Body: body}, nil
}

func (f *reviewFakeClient) ResolveMergeRequestDiscussion(project any, iid int64, discussionID string, resolved bool) (DiscussionInfo, error) {
	f.resolvedID, f.resolvedValue = discussionID, resolved
	return DiscussionInfo{ID: discussionID}, nil
}

func (f *reviewFakeClient) UpdateMergeRequest(project any, iid int64, input MergeRequestUpdateOptions) (MergeRequest, error) {
	f.updateOptions = input
	return MergeRequest{IID: iid, State: "closed"}, nil
}

func (f *reviewFakeClient) ListRepositoryTree(project any, input TreeListOptions) ([]TreeEntry, bool, error) {
	f.treeOptions = input
	return f.treeEntries, false, nil
}

func (f *reviewFakeClient) GetRepositoryFile(project any, path, ref string) (RepoFileRaw, error) {
	f.filePath, f.fileRef = path, ref
	return f.fileRaw, nil
}

func (f *reviewFakeClient) GetRepositoryArchive(project any, format, sha, path string) ([]byte, error) {
	f.archiveFormat = format
	return f.archiveData, nil
}

func (f *reviewFakeClient) CreateProject(input ProjectCreateOptions) (Project, error) {
	f.createdProject = input
	return Project{ID: 99, PathWithNamespace: "testing/" + input.Name}, nil
}

const reviewTestDiff = `@@ -8,4 +8,6 @@ func main() {
 	fmt.Println("start")
-	oldCall()
+	newCall()
+	logResult()
 	fmt.Println("done")
 }`

func newReviewClient() *reviewFakeClient {
	return &reviewFakeClient{
		fakeClient: &fakeClient{},
		fileDiffs: []FileDiff{{
			OldPath: "src/main.go",
			NewPath: "src/main.go",
			Diff:    reviewTestDiff,
		}},
		diffRefs:          DiffRefs{BaseSHA: "base-sha", StartSHA: "start-sha", HeadSHA: "head-sha"},
		createdDiscussion: DiscussionInfo{ID: "disc-1"},
	}
}

func TestMRChangesCapsDiffsAndCarriesDiffRefs(t *testing.T) {
	client := newReviewClient()
	out := plugintest.RunOK[MRChangesResult](t, testPlugin(client), OperationMRChanges, MRChangesInput{
		Ref:          "group/app!42",
		MaxDiffBytes: 24,
	})
	if out.Project != "group/app" || out.IID != 42 || out.Count != 1 {
		t.Fatalf("out = %#v", out)
	}
	if out.DiffRefs.BaseSHA != "base-sha" || out.DiffRefs.HeadSHA != "head-sha" {
		t.Fatalf("diff refs = %#v", out.DiffRefs)
	}
	if !out.Files[0].DiffTruncated || !strings.Contains(out.Files[0].Diff, "[diff truncated]") {
		t.Fatalf("diff cap not applied: %#v", out.Files[0])
	}
}

func TestMRDiffLinesModes(t *testing.T) {
	client := newReviewClient()
	// Full listing: typed line classification with old/new numbers.
	out := plugintest.RunOK[MRDiffLinesResult](t, testPlugin(client), OperationMRDiffLines, MRDiffLinesInput{
		Ref:  "group/app!42",
		File: "src/main.go",
	})
	if out.Count != 6 {
		t.Fatalf("lines = %#v", out.Lines)
	}
	byContent := map[string]DiffLineInfo{}
	for _, line := range out.Lines {
		byContent[strings.TrimSpace(line.Content)] = line
	}
	if added := byContent["newCall()"]; added.Type != "added" || added.NewLine != 9 || added.OldLine != 0 {
		t.Fatalf("added line = %#v", added)
	}
	if deleted := byContent["oldCall()"]; deleted.Type != "deleted" || deleted.OldLine != 9 || deleted.NewLine != 0 {
		t.Fatalf("deleted line = %#v", deleted)
	}
	if ctxLine := byContent[`fmt.Println("start")`]; ctxLine.Type != "context" || ctxLine.OldLine != 8 || ctxLine.NewLine != 8 {
		t.Fatalf("context line = %#v", ctxLine)
	}
	// Line+context mode flags the target.
	out = plugintest.RunOK[MRDiffLinesResult](t, testPlugin(client), OperationMRDiffLines, MRDiffLinesInput{
		Ref: "group/app!42", File: "src/main.go", Line: 9, Context: 1,
	})
	var target *DiffLineInfo
	for i := range out.Lines {
		if out.Lines[i].Target {
			target = &out.Lines[i]
		}
	}
	if target == nil || strings.TrimSpace(target.Content) != "newCall()" {
		t.Fatalf("target = %#v lines = %#v", target, out.Lines)
	}
	// Search mode.
	out = plugintest.RunOK[MRDiffLinesResult](t, testPlugin(client), OperationMRDiffLines, MRDiffLinesInput{
		Ref: "group/app!42", File: "src/main.go", Search: `logResult`,
	})
	if out.Count != 1 || out.Lines[0].Type != "added" {
		t.Fatalf("search lines = %#v", out.Lines)
	}
	// Unknown file errors with the changed-file list.
	err := plugintest.RunError(t, testPlugin(client), OperationMRDiffLines, MRDiffLinesInput{
		Ref: "group/app!42", File: "nope.go",
	})
	if err.Code != "not_found" || !strings.Contains(err.Message, "src/main.go") {
		t.Fatalf("err = %#v", err)
	}
}

func TestMRDiscussionCreatePositionShapes(t *testing.T) {
	// Added line: new_line only — old_line must stay 0.
	client := newReviewClient()
	out := plugintest.RunOK[MRDiscussionCreateResult](t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "why?", Path: "src/main.go", NewLine: 9,
	})
	if !out.Posted || out.Discussion == nil || out.Discussion.ID != "disc-1" {
		t.Fatalf("out = %#v", out)
	}
	if client.createdPosition == nil || client.createdPosition.NewLine != 9 || client.createdPosition.OldLine != 0 {
		t.Fatalf("added-line position = %#v", client.createdPosition)
	}
	if client.createdPosition.BaseSHA != "base-sha" || client.createdPosition.HeadSHA != "head-sha" || client.createdPosition.StartSHA != "start-sha" {
		t.Fatalf("position SHAs = %#v", client.createdPosition)
	}

	// Context line: auto old_line detection sets both sides.
	client = newReviewClient()
	plugintest.RunOK[MRDiscussionCreateResult](t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "note", Path: "src/main.go", NewLine: 8,
	})
	if client.createdPosition.NewLine != 8 || client.createdPosition.OldLine != 8 {
		t.Fatalf("context-line position = %#v", client.createdPosition)
	}

	// Deleted line: old_line only — new_line must stay 0.
	client = newReviewClient()
	plugintest.RunOK[MRDiscussionCreateResult](t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "note", Path: "src/main.go", OldLine: 9,
	})
	if client.createdPosition.OldLine != 9 || client.createdPosition.NewLine != 0 {
		t.Fatalf("deleted-line position = %#v", client.createdPosition)
	}

	// Line not in the diff: actionable error.
	client = newReviewClient()
	err := plugintest.RunError(t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "note", Path: "src/main.go", NewLine: 500,
	})
	if err.Code != "bad_input" || !strings.Contains(err.Message, "gitlab.mr.diff.lines") {
		t.Fatalf("err = %#v", err)
	}
}

func TestMRDiscussionCreateDryRunPreviewsWithoutPosting(t *testing.T) {
	client := newReviewClient()
	out := plugintest.RunOK[MRDiscussionCreateResult](t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "why?", Path: "src/main.go", NewLine: 9, DryRun: true,
	})
	if out.Posted || out.Discussion != nil {
		t.Fatalf("dry run must not post: %#v", out)
	}
	if client.createdPosition != nil {
		t.Fatal("dry run reached the client")
	}
	if out.Position == nil || out.Position.NewLine != 9 {
		t.Fatalf("position preview = %#v", out.Position)
	}
	var target *DiffLineInfo
	for i := range out.Lines {
		if out.Lines[i].Target {
			target = &out.Lines[i]
		}
	}
	if target == nil || strings.TrimSpace(target.Content) != "newCall()" {
		t.Fatalf("preview lines = %#v", out.Lines)
	}
}

func TestMRDiscussionCreateWithoutPositionIsPlainThread(t *testing.T) {
	client := newReviewClient()
	out := plugintest.RunOK[MRDiscussionCreateResult](t, testPlugin(client), OperationMRDiscussionCreate, MRDiscussionCreateInput{
		Ref: "group/app!42", Body: "general remark",
	})
	if !out.Posted || client.createdPosition != nil {
		t.Fatalf("plain discussion should carry no position: %#v", client.createdPosition)
	}
}

func TestMRDiscussionReplyAndResolve(t *testing.T) {
	client := newReviewClient()
	note := plugintest.RunOK[Note](t, testPlugin(client), OperationMRDiscussionReply, MRDiscussionReplyInput{
		Ref: "group/app!42", DiscussionID: "disc-1", Body: "fixed",
	})
	if note.ID != 8 || client.replyDiscussionID != "disc-1" {
		t.Fatalf("reply = %#v id = %q", note, client.replyDiscussionID)
	}
	discussion := plugintest.RunOK[DiscussionInfo](t, testPlugin(client), OperationMRDiscussionResolve, MRDiscussionResolveInput{
		Ref: "group/app!42", DiscussionID: "disc-1",
	})
	if discussion.ID != "disc-1" || client.resolvedID != "disc-1" || !client.resolvedValue {
		t.Fatalf("resolve = %#v", client)
	}
}

func TestMRUpdateRequiresAChange(t *testing.T) {
	client := newReviewClient()
	err := plugintest.RunError(t, testPlugin(client), OperationMRUpdate, MRUpdateInput{Ref: "group/app!42"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
	mr := plugintest.RunOK[MergeRequest](t, testPlugin(client), OperationMRUpdate, MRUpdateInput{Ref: "group/app!42", StateEvent: "close"})
	if mr.State != "closed" || client.updateOptions.StateEvent != "close" {
		t.Fatalf("update = %#v opts = %#v", mr, client.updateOptions)
	}
}

func TestCompareCapsFiles(t *testing.T) {
	client := newReviewClient()
	client.compareCommits = []CompareCommit{{ID: "c1", Title: "feat: x"}}
	client.compareFiles = []FileDiff{
		{NewPath: "a.go", Diff: reviewTestDiff},
		{NewPath: "b.go", Diff: reviewTestDiff},
	}
	out := plugintest.RunOK[CompareResult](t, testPlugin(client), OperationCompare, CompareInput{
		Project: "group/app", From: "main", To: "feature", MaxFiles: 1,
	})
	if out.CommitCount != 1 || out.FileCount != 1 || !out.Truncated {
		t.Fatalf("out = %#v", out)
	}
	err := plugintest.RunError(t, testPlugin(client), OperationCompare, CompareInput{Project: "group/app"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestRepositoryTreeAndFileShow(t *testing.T) {
	client := newReviewClient()
	client.treeEntries = []TreeEntry{{Path: "src", Type: "tree"}, {Path: "src/main.go", Type: "blob", Mode: "100644"}}
	tree := plugintest.RunOK[RepositoryTreeResult](t, testPlugin(client), OperationRepositoryTree, RepositoryTreeInput{
		Project: "group/app", Ref: "main", Recursive: true,
	})
	if tree.Count != 2 || !client.treeOptions.Recursive {
		t.Fatalf("tree = %#v", tree)
	}

	client.fileRaw = RepoFileRaw{FilePath: "src/main.go", Ref: "main", Size: 20, Content: []byte("package main\n// hi\n"), BlobID: "blob-1", LastCommitID: "c-9"}
	file := plugintest.RunOK[RepositoryFileShowResult](t, testPlugin(client), OperationRepositoryFileShow, RepositoryFileShowInput{
		Project: "group/app", Path: "src/main.go", Ref: "main", MaxBytes: 12,
	})
	if !file.Truncated || file.Content != "package main" || file.BlobID != "blob-1" {
		t.Fatalf("file = %#v", file)
	}

	// Ref-less show resolves the project's default branch (the files API
	// rejects a missing ref) and echoes the effective ref.
	client.project = Project{ID: 7, PathWithNamespace: "group/app", DefaultBranch: "develop"}
	client.fileRaw = RepoFileRaw{FilePath: "logo.png", Size: 4, Content: []byte{0x89, 0x50, 0x00, 0x47}, BlobID: "blob-2"}
	binary := plugintest.RunOK[RepositoryFileShowResult](t, testPlugin(client), OperationRepositoryFileShow, RepositoryFileShowInput{
		Project: "group/app", Path: "logo.png",
	})
	if !binary.Binary || binary.Content != "" {
		t.Fatalf("binary = %#v", binary)
	}
	if binary.Ref != "develop" {
		t.Fatalf("ref = %q, want resolved default branch", binary.Ref)
	}
}

func TestRepositoryArchiveStoresBlob(t *testing.T) {
	client := newReviewClient()
	client.archiveData = []byte("tar-bytes")
	host := &reviewBlobHost{}
	out := plugintest.RunOK[RepositoryArchiveResult](t, testPlugin(client), OperationRepositoryArchive, RepositoryArchiveInput{
		Project: "testing/dummy-project", Ref: "feature/x",
	}, plugintest.WithHost(host))
	if out.BlobRef != "blob://testing-dummy-project-feature-x.tar.gz" || out.Bytes != 9 || client.archiveFormat != "tar.gz" {
		t.Fatalf("out = %#v", out)
	}
	if host.written.MediaType != "application/gzip" {
		t.Fatalf("blob write = %#v", host.written)
	}
}

type reviewBlobHost struct {
	pluginbinding.HostClient
	written pluginbinding.BlobWriteRequest
}

func (h *reviewBlobHost) BlobWrite(input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	h.written = input
	return pluginbinding.BlobRef{Ref: "blob://" + input.Filename, Filename: input.Filename, Size: int64(len(input.Content))}, nil
}

func TestProjectCreateResolvesNamespace(t *testing.T) {
	client := newReviewClient()
	client.fakeClient.groups = []Group{
		{ID: 31, Path: "testing", FullPath: "testing"},
		{ID: 32, Path: "testing-archive", FullPath: "archive/testing-archive"},
	}
	project := plugintest.RunOK[Project](t, testPlugin(client), OperationProjectCreate, ProjectCreateInput{
		Name: "dummy-project", Namespace: "testing", InitializeWithReadme: true,
	})
	if project.PathWithNamespace != "testing/dummy-project" {
		t.Fatalf("project = %#v", project)
	}
	if client.createdProject.NamespaceID != 31 || !client.createdProject.InitializeWithReadme {
		t.Fatalf("options = %#v", client.createdProject)
	}
	err := plugintest.RunError(t, testPlugin(client), OperationProjectCreate, ProjectCreateInput{
		Name: "x", Namespace: "missing-group",
	})
	if err.Code != "not_found" || !strings.Contains(err.Message, "testing") {
		t.Fatalf("err = %#v", err)
	}
}

// Interface stubs so the pre-review fakeClient keeps satisfying Client.
func (f *fakeClient) ListMergeRequestDiffs(any, int64, int) ([]FileDiff, bool, error) {
	return nil, false, nil
}
func (f *fakeClient) GetMergeRequestDiffVersion(any, int64) (DiffRefs, error) {
	return DiffRefs{}, nil
}
func (f *fakeClient) CompareRefs(any, string, string, bool) ([]CompareCommit, []FileDiff, string, error) {
	return nil, nil, "", nil
}
func (f *fakeClient) ListMergeRequestDiscussions(any, int64, int) ([]DiscussionInfo, bool, error) {
	return nil, false, nil
}
func (f *fakeClient) CreateMergeRequestNote(any, int64, string) (Note, error) { return Note{}, nil }
func (f *fakeClient) CreateMergeRequestDiscussion(any, int64, string, *PositionInput) (DiscussionInfo, error) {
	return DiscussionInfo{}, nil
}
func (f *fakeClient) AddMergeRequestDiscussionNote(any, int64, string, string) (Note, error) {
	return Note{}, nil
}
func (f *fakeClient) ResolveMergeRequestDiscussion(any, int64, string, bool) (DiscussionInfo, error) {
	return DiscussionInfo{}, nil
}
func (f *fakeClient) UpdateMergeRequest(any, int64, MergeRequestUpdateOptions) (MergeRequest, error) {
	return MergeRequest{}, nil
}
func (f *fakeClient) ListRepositoryTree(any, TreeListOptions) ([]TreeEntry, bool, error) {
	return nil, false, nil
}
func (f *fakeClient) GetRepositoryFile(any, string, string) (RepoFileRaw, error) {
	return RepoFileRaw{}, nil
}
func (f *fakeClient) GetRepositoryArchive(any, string, string, string) ([]byte, error) {
	return nil, nil
}
func (f *fakeClient) CreateProject(ProjectCreateOptions) (Project, error) { return Project{}, nil }

func (f *fakeClient) SearchBlobs(any, any, string, string, int) ([]BlobMatch, bool, error) {
	return nil, false, nil
}

func (f *reviewFakeClient) SearchBlobs(project any, group any, query, ref string, limit int) ([]BlobMatch, bool, error) {
	f.blobProject, f.blobGroup, f.blobQuery = project, group, query
	return f.blobMatches, f.blobTruncated, nil
}

func TestSearchBlobsScopesAndCapsSnippets(t *testing.T) {
	client := newReviewClient()
	client.blobMatches = []BlobMatch{{
		ProjectID: 7,
		Path:      "internal/server/dial.go",
		Ref:       "main",
		StartLine: 41,
		Data:      strings.Repeat("x", 50),
	}}
	out := plugintest.RunOK[BlobSearchResult](t, testPlugin(client), OperationSearchBlobs, BlobSearchInput{
		Query: "connection refused", Group: "backend", MaxDataBytes: 10,
	})
	if client.blobGroup != "backend" || client.blobProject != nil {
		t.Fatalf("scope = project %v group %v", client.blobProject, client.blobGroup)
	}
	if out.Count != 1 || !out.Matches[0].DataTruncated || !strings.Contains(out.Matches[0].Data, "[snippet truncated]") {
		t.Fatalf("out = %#v", out)
	}
	// Project scope wins over group.
	plugintest.RunOK[BlobSearchResult](t, testPlugin(client), OperationSearchBlobs, BlobSearchInput{
		Query: "x", Project: "group/app", Group: "backend",
	})
	if client.blobProject != "group/app" || client.blobGroup != nil {
		t.Fatalf("scope = project %v group %v", client.blobProject, client.blobGroup)
	}
	err := plugintest.RunError(t, testPlugin(client), OperationSearchBlobs, BlobSearchInput{})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}
