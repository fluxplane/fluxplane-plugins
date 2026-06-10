package gitlab

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Merge-request review workflow operations (issue #3).

const (
	defaultChangesMaxFiles = 50
	maximumChangesMaxFiles = 200
	defaultDiffMaxBytes    = 16 * 1024
	maximumDiffMaxBytes    = 256 * 1024
	defaultFileMaxBytes    = 64 * 1024
	maximumFileMaxBytes    = 256 * 1024
)

// reviewMRAddress resolves a merge request target from ref (PROJECT!IID) or
// project+iid inputs.
func reviewMRAddress(ref, project string, iid int64) (string, int64, error) {
	if strings.TrimSpace(ref) != "" {
		return parseMergeRequestRef(ref)
	}
	if strings.TrimSpace(project) == "" {
		return "", 0, fmt.Errorf("ref (PROJECT!IID) or project and iid are required")
	}
	if iid <= 0 {
		return "", 0, fmt.Errorf("iid must be a positive integer")
	}
	return strings.TrimSpace(project), iid, nil
}

func clampInt(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}

// capDiffText bounds one file's unified diff text.
func capDiffText(diff string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(diff) <= maxBytes {
		return diff, false
	}
	return diff[:maxBytes] + "\n[diff truncated]", true
}

// ---- gitlab.mr.changes ----

type MRChangesInput struct {
	Ref          string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project      string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID          int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	File         string `json:"file,omitempty" jsonschema:"description=Only the file whose new_path or old_path equals this"`
	MaxFiles     int    `json:"max_files,omitempty" jsonschema:"description=Maximum files returned. Defaults to 50 and is capped at 200.,minimum=0,maximum=200"`
	MaxDiffBytes int    `json:"max_diff_bytes,omitempty" jsonschema:"description=Per-file diff text byte cap. Defaults to 16384 and is capped at 262144.,minimum=0"`
}

type MRChangesResult struct {
	Project   string     `json:"project"`
	IID       int64      `json:"iid"`
	DiffRefs  DiffRefs   `json:"diff_refs"`
	Files     []FileDiff `json:"files,omitempty"`
	Count     int        `json:"count"`
	Truncated bool       `json:"truncated,omitempty"`
}

// MRChanges lists a merge request's changed files with bounded diffs and the
// diff refs needed for positioned review comments.
func (s Service) MRChanges(ctx pluginbinding.Context, input MRChangesInput) (MRChangesResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MRChangesResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return MRChangesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	maxFiles := clampInt(input.MaxFiles, defaultChangesMaxFiles, maximumChangesMaxFiles)
	maxDiffBytes := clampInt(input.MaxDiffBytes, defaultDiffMaxBytes, maximumDiffMaxBytes)
	files, truncated, err := client.ListMergeRequestDiffs(projectID(project), iid, maxFiles)
	if err != nil {
		return MRChangesResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	refs, err := client.GetMergeRequestDiffVersion(projectID(project), iid)
	if err != nil {
		return MRChangesResult{}, pluginbinding.Errorf("gitlab", "diff refs: %s", err)
	}
	out := MRChangesResult{Project: project, IID: iid, DiffRefs: refs, Truncated: truncated}
	fileFilter := strings.TrimSpace(input.File)
	for _, file := range files {
		if fileFilter != "" && file.NewPath != fileFilter && file.OldPath != fileFilter {
			continue
		}
		file.Diff, file.DiffTruncated = capDiffText(file.Diff, maxDiffBytes)
		out.Files = append(out.Files, file)
	}
	out.Count = len(out.Files)
	return out, nil
}

// ---- gitlab.mr.diff.lines ----

type MRDiffLinesInput struct {
	Ref     string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID     int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	File    string `json:"file,omitempty" jsonschema:"required,description=Changed file path (new_path or old_path)"`
	Line    int64  `json:"line,omitempty" jsonschema:"description=Show only this new-file line with surrounding context"`
	Context int    `json:"context,omitempty" jsonschema:"description=Context lines around line. Defaults to 3.,minimum=0,maximum=20"`
	Search  string `json:"search,omitempty" jsonschema:"description=Regex over line content; returns matching lines"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum lines returned. Defaults to 200 and is capped at 2000.,minimum=0"`
}

type DiffLineInfo struct {
	Type    string `json:"type"` // added | deleted | context
	OldLine int64  `json:"old_line,omitempty"`
	NewLine int64  `json:"new_line,omitempty"`
	Content string `json:"content"`
	Target  bool   `json:"target,omitempty"`
}

type MRDiffLinesResult struct {
	Project   string         `json:"project"`
	IID       int64          `json:"iid"`
	File      string         `json:"file"`
	OldPath   string         `json:"old_path,omitempty"`
	NewPath   string         `json:"new_path,omitempty"`
	Lines     []DiffLineInfo `json:"lines,omitempty"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated,omitempty"`
	Hint      string         `json:"hint,omitempty"`
}

func diffLineInfo(line DiffLine) DiffLineInfo {
	lineType := "context"
	switch line.Type {
	case LineAdded:
		lineType = "added"
	case LineDeleted:
		lineType = "deleted"
	}
	return DiffLineInfo{Type: lineType, OldLine: int64(line.OldLine), NewLine: int64(line.NewLine), Content: line.Content}
}

// MRDiffLines parses one changed file's diff into typed lines for precise
// review targeting: which lines exist in the diff (and are therefore
// commentable) and with which old/new line numbers.
func (s Service) MRDiffLines(ctx pluginbinding.Context, input MRDiffLinesInput) (MRDiffLinesResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MRDiffLinesResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return MRDiffLinesResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	file := strings.TrimSpace(input.File)
	if file == "" {
		return MRDiffLinesResult{}, pluginbinding.Fail("bad_input", "file is required")
	}
	fileDiff, err := findMergeRequestFileDiff(client, project, iid, file)
	if err != nil {
		return MRDiffLinesResult{}, err
	}
	parsed := ParseUnifiedDiff(fileDiff.Diff)
	out := MRDiffLinesResult{Project: project, IID: iid, File: file, OldPath: fileDiff.OldPath, NewPath: fileDiff.NewPath}
	limit := clampInt(input.Limit, 200, 2000)
	switch {
	case input.Line > 0:
		contextLines := input.Context
		if contextLines <= 0 {
			contextLines = 3
		}
		target, before, after := parsed.GetLineWithContext(int(input.Line), contextLines)
		if target == nil {
			out.Hint = fmt.Sprintf("new-file line %d is not part of this file's diff; only lines present in the diff are commentable", input.Line)
			return out, nil
		}
		for _, line := range before {
			out.Lines = append(out.Lines, diffLineInfo(line))
		}
		targetInfo := diffLineInfo(*target)
		targetInfo.Target = true
		out.Lines = append(out.Lines, targetInfo)
		for _, line := range after {
			out.Lines = append(out.Lines, diffLineInfo(line))
		}
	case strings.TrimSpace(input.Search) != "":
		matches, err := parsed.SearchLines(strings.TrimSpace(input.Search))
		if err != nil {
			return MRDiffLinesResult{}, pluginbinding.Errorf("bad_input", "search: %s", err)
		}
		for _, line := range matches {
			if len(out.Lines) >= limit {
				out.Truncated = true
				break
			}
			out.Lines = append(out.Lines, diffLineInfo(line))
		}
	default:
		for _, line := range parsed.Lines {
			if len(out.Lines) >= limit {
				out.Truncated = true
				break
			}
			out.Lines = append(out.Lines, diffLineInfo(line))
		}
	}
	out.Count = len(out.Lines)
	return out, nil
}

// findMergeRequestFileDiff locates one file's diff within an MR change set.
func findMergeRequestFileDiff(client Client, project string, iid int64, file string) (FileDiff, error) {
	files, _, err := client.ListMergeRequestDiffs(projectID(project), iid, maximumChangesMaxFiles)
	if err != nil {
		return FileDiff{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	available := make([]string, 0, len(files))
	for _, candidate := range files {
		if candidate.NewPath == file || candidate.OldPath == file {
			return candidate, nil
		}
		available = append(available, candidate.NewPath)
	}
	return FileDiff{}, pluginbinding.Errorf("not_found", "file %q is not part of this merge request; changed files: %s", file, strings.Join(available, ", "))
}

// ---- gitlab.compare ----

type CompareInput struct {
	Project      string `json:"project,omitempty" jsonschema:"required,description=Project path or numeric ID"`
	From         string `json:"from,omitempty" jsonschema:"required,description=Base ref: branch, tag, or commit SHA"`
	To           string `json:"to,omitempty" jsonschema:"required,description=Head ref: branch, tag, or commit SHA"`
	Straight     bool   `json:"straight,omitempty" jsonschema:"description=Compare refs directly instead of from the merge base"`
	MaxFiles     int    `json:"max_files,omitempty" jsonschema:"description=Maximum file diffs returned. Defaults to 50 and is capped at 200.,minimum=0,maximum=200"`
	MaxDiffBytes int    `json:"max_diff_bytes,omitempty" jsonschema:"description=Per-file diff text byte cap. Defaults to 16384.,minimum=0"`
}

type CompareResult struct {
	Project     string          `json:"project"`
	From        string          `json:"from"`
	To          string          `json:"to"`
	WebURL      string          `json:"web_url,omitempty"`
	Commits     []CompareCommit `json:"commits,omitempty"`
	CommitCount int             `json:"commit_count"`
	Files       []FileDiff      `json:"files,omitempty"`
	FileCount   int             `json:"file_count"`
	Truncated   bool            `json:"truncated,omitempty"`
}

// Compare diffs two refs (branches, tags, or commits).
func (s Service) Compare(ctx pluginbinding.Context, input CompareInput) (CompareResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return CompareResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(firstNonEmpty(input.Project))
	from := strings.TrimSpace(input.From)
	to := strings.TrimSpace(input.To)
	if project == "" || from == "" || to == "" {
		return CompareResult{}, pluginbinding.Fail("bad_input", "project, from, and to are required")
	}
	commits, files, webURL, err := client.CompareRefs(projectID(project), from, to, input.Straight)
	if err != nil {
		return CompareResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	maxFiles := clampInt(input.MaxFiles, defaultChangesMaxFiles, maximumChangesMaxFiles)
	maxDiffBytes := clampInt(input.MaxDiffBytes, defaultDiffMaxBytes, maximumDiffMaxBytes)
	out := CompareResult{Project: project, From: from, To: to, WebURL: webURL, Commits: commits, CommitCount: len(commits)}
	for _, file := range files {
		if len(out.Files) >= maxFiles {
			out.Truncated = true
			break
		}
		file.Diff, file.DiffTruncated = capDiffText(file.Diff, maxDiffBytes)
		out.Files = append(out.Files, file)
	}
	out.FileCount = len(out.Files)
	return out, nil
}

// ---- gitlab.mr.discussion.list ----

type MRDiscussionListInput struct {
	Ref     string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID     int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum discussions returned. Defaults to 50 and is capped at 200.,minimum=0,maximum=200"`
}

type MRDiscussionListResult struct {
	Project     string           `json:"project"`
	IID         int64            `json:"iid"`
	Discussions []DiscussionInfo `json:"discussions,omitempty"`
	Count       int              `json:"count"`
	Truncated   bool             `json:"truncated,omitempty"`
}

// MRDiscussionList lists a merge request's discussion threads including line
// positions of inline review comments.
func (s Service) MRDiscussionList(ctx pluginbinding.Context, input MRDiscussionListInput) (MRDiscussionListResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MRDiscussionListResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return MRDiscussionListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	limit := clampInt(input.Limit, 50, 200)
	discussions, truncated, err := client.ListMergeRequestDiscussions(projectID(project), iid, limit)
	if err != nil {
		return MRDiscussionListResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return MRDiscussionListResult{Project: project, IID: iid, Discussions: discussions, Count: len(discussions), Truncated: truncated}, nil
}

// ---- gitlab.mr.note.create ----

type MRNoteCreateInput struct {
	Ref     string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID     int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	Body    string `json:"body,omitempty" jsonschema:"required,description=Markdown note body"`
}

// MRNoteCreate posts a top-level merge request note.
func (s Service) MRNoteCreate(ctx pluginbinding.Context, input MRNoteCreateInput) (Note, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Note{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return Note{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.Body) == "" {
		return Note{}, pluginbinding.Fail("bad_input", "body is required")
	}
	note, err := client.CreateMergeRequestNote(projectID(project), iid, input.Body)
	if err != nil {
		return Note{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return note, nil
}

// ---- gitlab.mr.discussion.create ----

type MRDiscussionCreateInput struct {
	Ref     string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID     int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	Body    string `json:"body,omitempty" jsonschema:"required,description=Markdown comment body"`
	Path    string `json:"path,omitempty" jsonschema:"description=Changed file path for a line-level comment"`
	NewLine int64  `json:"new_line,omitempty" jsonschema:"description=Line number in the new file (added or context lines)"`
	OldLine int64  `json:"old_line,omitempty" jsonschema:"description=Line number in the old file (deleted or context lines)"`
	DryRun  bool   `json:"dry_run,omitempty" jsonschema:"description=Resolve and return the position with the target line in context without posting"`
}

type MRDiscussionCreateResult struct {
	Project    string          `json:"project"`
	IID        int64           `json:"iid"`
	Posted     bool            `json:"posted"`
	DryRun     bool            `json:"dry_run,omitempty"`
	Discussion *DiscussionInfo `json:"discussion,omitempty"`
	Position   *PositionInput  `json:"position,omitempty"`
	Lines      []DiffLineInfo  `json:"lines,omitempty"` // target line with context (dry run)
}

// MRDiscussionCreate opens a discussion thread, optionally anchored to a diff
// line. SHAs come from the merge request's latest diff version; when only
// new_line is given, the file's diff decides whether the anchor is an added
// line (new_line only) or a context line (old_line set too) — the combination
// GitLab accepts without 400s.
func (s Service) MRDiscussionCreate(ctx pluginbinding.Context, input MRDiscussionCreateInput) (MRDiscussionCreateResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MRDiscussionCreateResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return MRDiscussionCreateResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.Body) == "" {
		return MRDiscussionCreateResult{}, pluginbinding.Fail("bad_input", "body is required")
	}
	out := MRDiscussionCreateResult{Project: project, IID: iid, DryRun: input.DryRun}

	positioned := strings.TrimSpace(input.Path) != "" || input.NewLine > 0 || input.OldLine > 0
	var position *PositionInput
	if positioned {
		if strings.TrimSpace(input.Path) == "" {
			return MRDiscussionCreateResult{}, pluginbinding.Fail("bad_input", "path is required for a line-level comment")
		}
		if input.NewLine <= 0 && input.OldLine <= 0 {
			return MRDiscussionCreateResult{}, pluginbinding.Fail("bad_input", "new_line or old_line is required for a line-level comment")
		}
		refs, err := client.GetMergeRequestDiffVersion(projectID(project), iid)
		if err != nil {
			return MRDiscussionCreateResult{}, pluginbinding.Errorf("gitlab", "diff refs: %s", err)
		}
		fileDiff, err := findMergeRequestFileDiff(client, project, iid, strings.TrimSpace(input.Path))
		if err != nil {
			return MRDiscussionCreateResult{}, err
		}
		parsed := ParseUnifiedDiff(fileDiff.Diff)
		newLine, oldLine := input.NewLine, input.OldLine
		var anchor *DiffLine
		switch {
		case newLine > 0 && oldLine == 0:
			if line, found := parsed.FindLineByNew(int(newLine)); found {
				anchor = line
				// context lines need both sides; added lines keep old_line 0
				oldLine = int64(line.OldLine)
			} else {
				return MRDiscussionCreateResult{}, pluginbinding.Errorf("bad_input", "new-file line %d is not part of %s's diff — only diff lines are commentable (inspect with gitlab.mr.diff.lines)", newLine, fileDiff.NewPath)
			}
		case oldLine > 0 && newLine == 0:
			if line, found := parsed.FindLineByOld(int(oldLine)); found {
				anchor = line
				// context lines need both sides; deleted lines keep new_line 0
				newLine = int64(line.NewLine)
			} else {
				return MRDiscussionCreateResult{}, pluginbinding.Errorf("bad_input", "old-file line %d is not part of %s's diff — only diff lines are commentable (inspect with gitlab.mr.diff.lines)", oldLine, fileDiff.NewPath)
			}
		default:
			if line, found := parsed.FindLineByNew(int(newLine)); found {
				anchor = line
			}
		}
		oldPath := fileDiff.OldPath
		if oldPath == "" {
			oldPath = fileDiff.NewPath
		}
		newPath := fileDiff.NewPath
		if newPath == "" {
			newPath = fileDiff.OldPath
		}
		position = &PositionInput{
			BaseSHA:  refs.BaseSHA,
			StartSHA: refs.StartSHA,
			HeadSHA:  refs.HeadSHA,
			OldPath:  oldPath,
			NewPath:  newPath,
			NewLine:  newLine,
			OldLine:  oldLine,
		}
		out.Position = position
		if input.DryRun && anchor != nil {
			_, before, after := parsed.GetLineWithContext(anchor.NewLine, 2)
			if anchor.NewLine == 0 {
				before, after = nil, nil
			}
			for _, line := range before {
				out.Lines = append(out.Lines, diffLineInfo(line))
			}
			target := diffLineInfo(*anchor)
			target.Target = true
			out.Lines = append(out.Lines, target)
			for _, line := range after {
				out.Lines = append(out.Lines, diffLineInfo(line))
			}
		}
	}
	if input.DryRun {
		return out, nil
	}
	discussion, err := client.CreateMergeRequestDiscussion(projectID(project), iid, input.Body, position)
	if err != nil {
		return MRDiscussionCreateResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	out.Posted = true
	out.Discussion = &discussion
	return out, nil
}

// ---- gitlab.mr.discussion.reply ----

type MRDiscussionReplyInput struct {
	Ref          string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project      string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID          int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	DiscussionID string `json:"discussion_id,omitempty" jsonschema:"required,description=Discussion thread ID from gitlab.mr.discussion.list"`
	Body         string `json:"body,omitempty" jsonschema:"required,description=Markdown reply body"`
}

// MRDiscussionReply adds a note to an existing discussion thread.
func (s Service) MRDiscussionReply(ctx pluginbinding.Context, input MRDiscussionReplyInput) (Note, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Note{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return Note{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.DiscussionID) == "" || strings.TrimSpace(input.Body) == "" {
		return Note{}, pluginbinding.Fail("bad_input", "discussion_id and body are required")
	}
	note, err := client.AddMergeRequestDiscussionNote(projectID(project), iid, strings.TrimSpace(input.DiscussionID), input.Body)
	if err != nil {
		return Note{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return note, nil
}

// ---- gitlab.mr.discussion.resolve ----

type MRDiscussionResolveInput struct {
	Ref          string `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project      string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID          int64  `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	DiscussionID string `json:"discussion_id,omitempty" jsonschema:"required,description=Discussion thread ID from gitlab.mr.discussion.list"`
	Resolved     *bool  `json:"resolved,omitempty" jsonschema:"description=Resolve (true, default) or unresolve (false)"`
}

// MRDiscussionResolve resolves or unresolves a discussion thread.
func (s Service) MRDiscussionResolve(ctx pluginbinding.Context, input MRDiscussionResolveInput) (DiscussionInfo, error) {
	client, err := s.client(ctx)
	if err != nil {
		return DiscussionInfo{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return DiscussionInfo{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.DiscussionID) == "" {
		return DiscussionInfo{}, pluginbinding.Fail("bad_input", "discussion_id is required")
	}
	resolved := true
	if input.Resolved != nil {
		resolved = *input.Resolved
	}
	discussion, err := client.ResolveMergeRequestDiscussion(projectID(project), iid, strings.TrimSpace(input.DiscussionID), resolved)
	if err != nil {
		return DiscussionInfo{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return discussion, nil
}

// ---- gitlab.mr.update ----

type MRUpdateInput struct {
	Ref          string   `json:"ref,omitempty" jsonschema:"description=Merge request reference as PROJECT!IID"`
	Project      string   `json:"project,omitempty" jsonschema:"description=Project path or numeric ID (with iid)"`
	IID          int64    `json:"iid,omitempty" jsonschema:"description=Merge request IID (with project)"`
	Title        string   `json:"title,omitempty" jsonschema:"description=New title"`
	Description  *string  `json:"description,omitempty" jsonschema:"description=New description"`
	TargetBranch string   `json:"target_branch,omitempty" jsonschema:"description=New target branch"`
	StateEvent   string   `json:"state_event,omitempty" jsonschema:"description=Lifecycle event,enum=close,enum=reopen"`
	Labels       []string `json:"labels,omitempty" jsonschema:"description=Replace labels"`
}

// MRUpdate updates merge request fields or closes/reopens it.
func (s Service) MRUpdate(ctx pluginbinding.Context, input MRUpdateInput) (MergeRequest, error) {
	client, err := s.client(ctx)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := reviewMRAddress(input.Ref, input.Project, input.IID)
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.Title) == "" && input.Description == nil && strings.TrimSpace(input.TargetBranch) == "" &&
		strings.TrimSpace(input.StateEvent) == "" && input.Labels == nil {
		return MergeRequest{}, pluginbinding.Fail("bad_input", "nothing to update: pass title, description, target_branch, state_event, or labels")
	}
	mr, err := client.UpdateMergeRequest(projectID(project), iid, MergeRequestUpdateOptions{
		Title:        input.Title,
		Description:  input.Description,
		TargetBranch: input.TargetBranch,
		StateEvent:   input.StateEvent,
		Labels:       input.Labels,
	})
	if err != nil {
		return MergeRequest{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return mr, nil
}

// ---- gitlab.repository.tree ----

type RepositoryTreeInput struct {
	Project   string `json:"project,omitempty" jsonschema:"required,description=Project path or numeric ID"`
	Path      string `json:"path,omitempty" jsonschema:"description=Subdirectory to list. Defaults to the repository root"`
	Ref       string `json:"ref,omitempty" jsonschema:"description=Branch, tag, or commit SHA. Defaults to the default branch"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"description=Descend into subdirectories"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum entries returned. Defaults to 200 and is capped at 2000.,minimum=0"`
}

type RepositoryTreeResult struct {
	Project   string      `json:"project"`
	Ref       string      `json:"ref,omitempty"`
	Path      string      `json:"path,omitempty"`
	Entries   []TreeEntry `json:"entries,omitempty"`
	Count     int         `json:"count"`
	Truncated bool        `json:"truncated,omitempty"`
}

// RepositoryTree lists a repository tree at a ref.
func (s Service) RepositoryTree(ctx pluginbinding.Context, input RepositoryTreeInput) (RepositoryTreeResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepositoryTreeResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(input.Project)
	if project == "" {
		return RepositoryTreeResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	limit := clampInt(input.Limit, 200, 2000)
	entries, truncated, err := client.ListRepositoryTree(projectID(project), TreeListOptions{
		Path:      input.Path,
		Ref:       input.Ref,
		Recursive: input.Recursive,
		Limit:     limit,
	})
	if err != nil {
		return RepositoryTreeResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return RepositoryTreeResult{Project: project, Ref: input.Ref, Path: input.Path, Entries: entries, Count: len(entries), Truncated: truncated}, nil
}

// ---- gitlab.repository.file.show ----

type RepositoryFileShowInput struct {
	Project  string `json:"project,omitempty" jsonschema:"required,description=Project path or numeric ID"`
	Path     string `json:"path,omitempty" jsonschema:"required,description=File path inside the repository"`
	Ref      string `json:"ref,omitempty" jsonschema:"description=Branch, tag, or commit SHA. Defaults to the default branch"`
	MaxBytes int    `json:"max_bytes,omitempty" jsonschema:"description=Content byte cap. Defaults to 65536 and is capped at 262144.,minimum=0"`
}

type RepositoryFileShowResult struct {
	Project      string `json:"project"`
	Path         string `json:"path"`
	Ref          string `json:"ref,omitempty"`
	Size         int64  `json:"size"`
	Content      string `json:"content,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	Binary       bool   `json:"binary,omitempty"`
	BlobID       string `json:"blob_id,omitempty"`
	LastCommitID string `json:"last_commit_id,omitempty"`
}

// RepositoryFileShow reads a file's content at a ref, bounded; binary files
// are reported (size + blob id) instead of dumped.
func (s Service) RepositoryFileShow(ctx pluginbinding.Context, input RepositoryFileShowInput) (RepositoryFileShowResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepositoryFileShowResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(input.Project)
	path := strings.TrimSpace(input.Path)
	if project == "" || path == "" {
		return RepositoryFileShowResult{}, pluginbinding.Fail("bad_input", "project and path are required")
	}
	file, err := client.GetRepositoryFile(projectID(project), path, input.Ref)
	if err != nil {
		return RepositoryFileShowResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	out := RepositoryFileShowResult{
		Project:      project,
		Path:         file.FilePath,
		Ref:          file.Ref,
		Size:         file.Size,
		BlobID:       file.BlobID,
		LastCommitID: file.LastCommitID,
	}
	content := file.Content
	if isBinaryContent(content) {
		out.Binary = true
		return out, nil
	}
	maxBytes := clampInt(input.MaxBytes, defaultFileMaxBytes, maximumFileMaxBytes)
	if len(content) > maxBytes {
		content = content[:maxBytes]
		out.Truncated = true
	}
	out.Content = string(content)
	return out, nil
}

// isBinaryContent reports whether content looks binary (NUL byte or invalid
// UTF-8 in the first chunk).
func isBinaryContent(content []byte) bool {
	probe := content
	if len(probe) > 8192 {
		probe = probe[:8192]
	}
	for _, b := range probe {
		if b == 0 {
			return true
		}
	}
	return len(probe) > 0 && !utf8.Valid(probe) && !utf8.Valid(content)
}

// ---- gitlab.repository.archive ----

type RepositoryArchiveInput struct {
	Project string `json:"project,omitempty" jsonschema:"required,description=Project path or numeric ID"`
	Ref     string `json:"ref,omitempty" jsonschema:"description=Branch, tag, or commit SHA. Defaults to the default branch"`
	Path    string `json:"path,omitempty" jsonschema:"description=Subdirectory to archive"`
	Format  string `json:"format,omitempty" jsonschema:"description=Archive format. Defaults to tar.gz.,enum=tar.gz,enum=zip,enum=tar"`
}

type RepositoryArchiveResult struct {
	Project  string `json:"project"`
	Ref      string `json:"ref,omitempty"`
	Format   string `json:"format"`
	BlobRef  string `json:"blob_ref"`
	BlobPath string `json:"blob_path,omitempty"`
	Filename string `json:"filename"`
	Bytes    int    `json:"bytes"`
}

// RepositoryArchive materializes reviewable source: downloads a repository
// archive at a ref and stores it through the host blob capability.
func (s Service) RepositoryArchive(ctx pluginbinding.Context, input RepositoryArchiveInput) (RepositoryArchiveResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepositoryArchiveResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := strings.TrimSpace(input.Project)
	if project == "" {
		return RepositoryArchiveResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	format := strings.TrimSpace(input.Format)
	if format == "" {
		format = "tar.gz"
	}
	data, err := client.GetRepositoryArchive(projectID(project), format, input.Ref, input.Path)
	if err != nil {
		return RepositoryArchiveResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	mediaType := "application/gzip"
	switch format {
	case "zip":
		mediaType = "application/zip"
	case "tar":
		mediaType = "application/x-tar"
	}
	name := strings.NewReplacer("/", "-", " ", "-").Replace(project)
	if ref := strings.TrimSpace(input.Ref); ref != "" {
		name += "-" + strings.NewReplacer("/", "-", " ", "-").Replace(ref)
	}
	filename := name + "." + format
	blob, err := ctx.Host.BlobWrite(pluginbinding.BlobWriteRequest{
		Content:   data,
		MediaType: mediaType,
		Filename:  filename,
		Metadata:  map[string]string{"project": project, "ref": input.Ref, "format": format},
	})
	if err != nil {
		return RepositoryArchiveResult{}, pluginbinding.Errorf("gitlab", "store archive blob: %s", err)
	}
	return RepositoryArchiveResult{
		Project:  project,
		Ref:      input.Ref,
		Format:   format,
		BlobRef:  blob.Ref,
		BlobPath: blob.Path,
		Filename: filename,
		Bytes:    len(data),
	}, nil
}

// ---- gitlab.project.create ----

type ProjectCreateInput struct {
	Name                 string `json:"name,omitempty" jsonschema:"required,description=Project name"`
	Path                 string `json:"path,omitempty" jsonschema:"description=Project path slug. Defaults to a slug of the name"`
	Namespace            string `json:"namespace,omitempty" jsonschema:"description=Group path to create the project in (e.g. testing). Defaults to the personal namespace"`
	Description          string `json:"description,omitempty" jsonschema:"description=Project description"`
	Visibility           string `json:"visibility,omitempty" jsonschema:"description=Visibility level,enum=private,enum=internal,enum=public"`
	InitializeWithReadme bool   `json:"initialize_with_readme,omitempty" jsonschema:"description=Create an initial README so the repository has a default branch"`
}

// ProjectCreate creates a project, resolving a group path to its namespace.
func (s Service) ProjectCreate(ctx pluginbinding.Context, input ProjectCreateInput) (Project, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Project{}, pluginbinding.Errorf("secret", "%s", err)
	}
	if strings.TrimSpace(input.Name) == "" {
		return Project{}, pluginbinding.Fail("bad_input", "name is required")
	}
	options := ProjectCreateOptions{
		Name:                 strings.TrimSpace(input.Name),
		Path:                 strings.TrimSpace(input.Path),
		Description:          input.Description,
		Visibility:           input.Visibility,
		InitializeWithReadme: input.InitializeWithReadme,
	}
	if namespace := strings.TrimSpace(input.Namespace); namespace != "" {
		groups, err := client.ListGroups(GroupListOptions{Search: namespace, Limit: 20})
		if err != nil {
			return Project{}, pluginbinding.Errorf("gitlab", "resolve namespace: %s", err)
		}
		var match *Group
		var candidates []string
		for i := range groups {
			candidates = append(candidates, groups[i].FullPath)
			if strings.EqualFold(groups[i].FullPath, namespace) || strings.EqualFold(groups[i].Path, namespace) {
				if match == nil {
					match = &groups[i]
				}
			}
		}
		if match == nil {
			return Project{}, pluginbinding.Errorf("not_found", "group %q not found; candidates: %s", namespace, strings.Join(candidates, ", "))
		}
		options.NamespaceID = match.ID
	}
	project, err := client.CreateProject(options)
	if err != nil {
		return Project{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return project, nil
}
