package gitlab

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Review-workflow operation specs (issue #3). Pagination and large-diff
// handling are documented in the descriptions: list results carry `truncated`
// when a cap was hit, and per-file diff text is bounded by max_diff_bytes
// (per-file `diff_truncated`).

func mrChangesSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[MRChangesInput, MRChangesResult](
		OperationMRChanges,
		"List a merge request's changed files with bounded unified diffs, plus the diff refs (base/start/head SHA) line-level comments anchor to. "+
			"Caps: max_files (default 50, cap 200, result truncated flag), max_diff_bytes per file (default 16KB, per-file diff_truncated flag)."),
		map[string]any{"ref": "group/app!42"},
	)
}

func mrDiffLinesSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[MRDiffLinesInput, MRDiffLinesResult](
		OperationMRDiffLines,
		"Parse one changed file's diff into typed lines (added/deleted/context with old/new line numbers) — exactly the lines a review comment can anchor to. "+
			"Modes: full listing (limit/truncated), line+context, or regex search."),
		map[string]any{"ref": "group/app!42", "file": "src/main.go", "line": 120, "context": 3},
	)
}

func compareSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[CompareInput, CompareResult](
		OperationCompare,
		"Compare two refs (branches, tags, or commits): commits between them and bounded file diffs. "+
			"straight=true compares the refs directly instead of from their merge base."),
		map[string]any{"project": "group/app", "from": "main", "to": "feature/login"},
	)
}

func mrDiscussionListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[MRDiscussionListInput, MRDiscussionListResult](
		OperationMRDiscussionList,
		"List a merge request's discussion threads with resolution state and the file/line positions of inline comments."),
		map[string]any{"ref": "group/app!42"},
	)
}

func mrNoteCreateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[MRNoteCreateInput, Note](
		OperationMRNoteCreate,
		"Post a top-level merge request note.",
		core.OperationNonIdempotent),
		map[string]any{"ref": "group/app!42", "body": "LGTM overall — two inline comments."},
	)
}

func mrDiscussionCreateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[MRDiscussionCreateInput, MRDiscussionCreateResult](
		OperationMRDiscussionCreate,
		"Open a merge request discussion, optionally anchored to a diff line (path + new_line/old_line). "+
			"SHAs resolve from the latest diff version; old_line is auto-derived for context lines. "+
			"dry_run previews the resolved position and target line without posting.",
		core.OperationNonIdempotent),
		map[string]any{"ref": "group/app!42", "body": "This can fail when the slice is empty.", "path": "src/main.go", "new_line": 120, "dry_run": true},
	)
}

func mrDiscussionReplySpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[MRDiscussionReplyInput, Note](
		OperationMRDiscussionReply,
		"Reply into an existing merge request discussion thread.",
		core.OperationNonIdempotent),
		map[string]any{"ref": "group/app!42", "discussion_id": "6f4…", "body": "Fixed in the latest push."},
	)
}

func mrDiscussionResolveSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[MRDiscussionResolveInput, DiscussionInfo](
		OperationMRDiscussionResolve,
		"Resolve (or unresolve with resolved=false) a merge request discussion thread.",
		core.OperationConditional),
		map[string]any{"ref": "group/app!42", "discussion_id": "6f4…", "resolved": true},
	)
}

func mrUpdateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[MRUpdateInput, MergeRequest](
		OperationMRUpdate,
		"Update merge request fields (title, description, target branch, labels) or close/reopen it via state_event.",
		core.OperationConditional),
		map[string]any{"ref": "group/app!42", "state_event": "close"},
	)
}

func repositoryTreeSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[RepositoryTreeInput, RepositoryTreeResult](
		OperationRepositoryTree,
		"List a repository tree at a ref (optionally recursive); entries are blobs and trees with modes."),
		map[string]any{"project": "group/app", "ref": "main", "path": "src", "recursive": true},
	)
}

func repositoryFileShowSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[RepositoryFileShowInput, RepositoryFileShowResult](
		OperationRepositoryFileShow,
		"Read a file's content at a ref, bounded by max_bytes (default 64KB, truncated flag); binary files are reported with size and blob id instead of dumped."),
		map[string]any{"project": "group/app", "path": "src/main.go", "ref": "main"},
	)
}

func repositoryArchiveSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[RepositoryArchiveInput, RepositoryArchiveResult](
		OperationRepositoryArchive,
		"Materialize reviewable source: download a repository archive (tar.gz/zip/tar) at a ref into the host blob store without exposing secrets."),
		map[string]any{"project": "group/app", "ref": "feature/login"},
	)
}

func projectCreateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ProjectCreateInput, Project](
		OperationProjectCreate,
		"Create a project, optionally inside a group namespace (resolved by path).",
		core.OperationNonIdempotent),
		map[string]any{"name": "dummy-project", "namespace": "testing", "initialize_with_readme": true},
	)
}

func reviewOperationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		mrChangesSpec(),
		mrDiffLinesSpec(),
		compareSpec(),
		mrDiscussionListSpec(),
		mrNoteCreateSpec(),
		mrDiscussionCreateSpec(),
		mrDiscussionReplySpec(),
		mrDiscussionResolveSpec(),
		mrUpdateSpec(),
		repositoryTreeSpec(),
		repositoryFileShowSpec(),
		repositoryArchiveSpec(),
		projectCreateSpec(),
	}
}

func registerReviewOperations(service Service) []pluginbinding.PluginOption {
	return []pluginbinding.PluginOption{
		pluginbinding.RegisterOperation(mrChangesSpec(), service.MRChanges),
		pluginbinding.RegisterOperation(mrDiffLinesSpec(), service.MRDiffLines),
		pluginbinding.RegisterOperation(compareSpec(), service.Compare),
		pluginbinding.RegisterOperation(mrDiscussionListSpec(), service.MRDiscussionList),
		pluginbinding.RegisterOperation(mrNoteCreateSpec(), service.MRNoteCreate),
		pluginbinding.RegisterOperation(mrDiscussionCreateSpec(), service.MRDiscussionCreate),
		pluginbinding.RegisterOperation(mrDiscussionReplySpec(), service.MRDiscussionReply),
		pluginbinding.RegisterOperation(mrDiscussionResolveSpec(), service.MRDiscussionResolve),
		pluginbinding.RegisterOperation(mrUpdateSpec(), service.MRUpdate),
		pluginbinding.RegisterOperation(repositoryTreeSpec(), service.RepositoryTree),
		pluginbinding.RegisterOperation(repositoryFileShowSpec(), service.RepositoryFileShow),
		pluginbinding.RegisterOperation(repositoryArchiveSpec(), service.RepositoryArchive),
		pluginbinding.RegisterOperation(projectCreateSpec(), service.ProjectCreate),
	}
}
