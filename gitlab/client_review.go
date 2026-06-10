package gitlab

import (
	"encoding/base64"
	"fmt"
	"strings"

	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

// Review-workflow client methods (issue #3). Pagination loops mirror the
// existing list methods; limits are enforced by the operations layer, the
// client reports whether more results existed via the bool return.

func (c liveClient) ListMergeRequestDiffs(project any, iid int64, limit int) ([]FileDiff, bool, error) {
	opt := &gitlabapi.ListMergeRequestDiffsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	var out []FileDiff
	for {
		diffs, resp, err := c.client.MergeRequests.ListMergeRequestDiffs(project, iid, opt)
		if err != nil {
			return nil, false, err
		}
		for _, diff := range diffs {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, FileDiff{
				OldPath:       diff.OldPath,
				NewPath:       diff.NewPath,
				NewFile:       diff.NewFile,
				RenamedFile:   diff.RenamedFile,
				DeletedFile:   diff.DeletedFile,
				GeneratedFile: diff.GeneratedFile,
				TooLarge:      diff.TooLarge,
				Diff:          diff.Diff,
			})
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

// GetMergeRequestDiffVersion returns the latest diff version's SHAs (the
// canonical anchors for positioned discussions), falling back to the MR's
// diff_refs when the versions API returns nothing.
func (c liveClient) GetMergeRequestDiffVersion(project any, iid int64) (DiffRefs, error) {
	versions, _, err := c.client.MergeRequests.GetMergeRequestDiffVersions(project, iid, nil)
	if err == nil && len(versions) > 0 && versions[0] != nil {
		v := versions[0]
		if v.BaseCommitSHA != "" && v.HeadCommitSHA != "" {
			return DiffRefs{BaseSHA: v.BaseCommitSHA, StartSHA: v.StartCommitSHA, HeadSHA: v.HeadCommitSHA}, nil
		}
	}
	mr, _, mrErr := c.client.MergeRequests.GetMergeRequest(project, iid, nil)
	if mrErr != nil {
		if err != nil {
			return DiffRefs{}, err
		}
		return DiffRefs{}, mrErr
	}
	refs := DiffRefs{BaseSHA: mr.DiffRefs.BaseSha, StartSHA: mr.DiffRefs.StartSha, HeadSHA: mr.DiffRefs.HeadSha}
	if refs.BaseSHA == "" || refs.HeadSHA == "" {
		return DiffRefs{}, fmt.Errorf("merge request has no diff refs yet")
	}
	return refs, nil
}

func (c liveClient) CompareRefs(project any, from, to string, straight bool) ([]CompareCommit, []FileDiff, string, error) {
	opt := &gitlabapi.CompareOptions{
		From: gitlabapi.Ptr(from),
		To:   gitlabapi.Ptr(to),
	}
	if straight {
		opt.Straight = gitlabapi.Ptr(true)
	}
	compare, _, err := c.client.Repositories.Compare(project, opt)
	if err != nil {
		return nil, nil, "", err
	}
	commits := make([]CompareCommit, 0, len(compare.Commits))
	for _, commit := range compare.Commits {
		if commit == nil {
			continue
		}
		commits = append(commits, CompareCommit{
			ID:         commit.ID,
			ShortID:    commit.ShortID,
			Title:      commit.Title,
			AuthorName: commit.AuthorName,
			CreatedAt:  formatTime(commit.CreatedAt),
		})
	}
	files := make([]FileDiff, 0, len(compare.Diffs))
	for _, diff := range compare.Diffs {
		if diff == nil {
			continue
		}
		files = append(files, FileDiff{
			OldPath:     diff.OldPath,
			NewPath:     diff.NewPath,
			NewFile:     diff.NewFile,
			RenamedFile: diff.RenamedFile,
			DeletedFile: diff.DeletedFile,
			Diff:        diff.Diff,
		})
	}
	return commits, files, compare.WebURL, nil
}

func (c liveClient) ListMergeRequestDiscussions(project any, iid int64, limit int) ([]DiscussionInfo, bool, error) {
	opt := &gitlabapi.ListMergeRequestDiscussionsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	var out []DiscussionInfo
	for {
		discussions, resp, err := c.client.Discussions.ListMergeRequestDiscussions(project, iid, opt)
		if err != nil {
			return nil, false, err
		}
		for _, discussion := range discussions {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, discussionFromAPI(discussion))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) CreateMergeRequestNote(project any, iid int64, body string) (Note, error) {
	note, _, err := c.client.Notes.CreateMergeRequestNote(project, iid, &gitlabapi.CreateMergeRequestNoteOptions{Body: gitlabapi.Ptr(body)})
	if err != nil {
		return Note{}, err
	}
	return noteFromAPI(note), nil
}

func (c liveClient) CreateMergeRequestDiscussion(project any, iid int64, body string, position *PositionInput) (DiscussionInfo, error) {
	opt := &gitlabapi.CreateMergeRequestDiscussionOptions{Body: gitlabapi.Ptr(body)}
	if position != nil {
		// position_type is always "text"; old/new line are only sent when set
		// (added lines carry new_line only, deleted lines old_line only,
		// context lines both) — anything else triggers GitLab 400s.
		positionOptions := &gitlabapi.PositionOptions{
			BaseSHA:      gitlabapi.Ptr(position.BaseSHA),
			StartSHA:     gitlabapi.Ptr(position.StartSHA),
			HeadSHA:      gitlabapi.Ptr(position.HeadSHA),
			PositionType: gitlabapi.Ptr("text"),
			NewPath:      gitlabapi.Ptr(position.NewPath),
			OldPath:      gitlabapi.Ptr(position.OldPath),
		}
		if position.NewLine > 0 {
			positionOptions.NewLine = gitlabapi.Ptr(position.NewLine)
		}
		if position.OldLine > 0 {
			positionOptions.OldLine = gitlabapi.Ptr(position.OldLine)
		}
		opt.Position = positionOptions
	}
	discussion, _, err := c.client.Discussions.CreateMergeRequestDiscussion(project, iid, opt)
	if err != nil {
		return DiscussionInfo{}, err
	}
	return discussionFromAPI(discussion), nil
}

func (c liveClient) AddMergeRequestDiscussionNote(project any, iid int64, discussionID, body string) (Note, error) {
	note, _, err := c.client.Discussions.AddMergeRequestDiscussionNote(project, iid, discussionID, &gitlabapi.AddMergeRequestDiscussionNoteOptions{Body: gitlabapi.Ptr(body)})
	if err != nil {
		return Note{}, err
	}
	return noteFromAPI(note), nil
}

func (c liveClient) ResolveMergeRequestDiscussion(project any, iid int64, discussionID string, resolved bool) (DiscussionInfo, error) {
	discussion, _, err := c.client.Discussions.ResolveMergeRequestDiscussion(project, iid, discussionID, &gitlabapi.ResolveMergeRequestDiscussionOptions{Resolved: gitlabapi.Ptr(resolved)})
	if err != nil {
		return DiscussionInfo{}, err
	}
	return discussionFromAPI(discussion), nil
}

func (c liveClient) UpdateMergeRequest(project any, iid int64, input MergeRequestUpdateOptions) (MergeRequest, error) {
	opt := &gitlabapi.UpdateMergeRequestOptions{}
	if strings.TrimSpace(input.Title) != "" {
		opt.Title = gitlabapi.Ptr(strings.TrimSpace(input.Title))
	}
	if input.Description != nil {
		opt.Description = input.Description
	}
	if strings.TrimSpace(input.TargetBranch) != "" {
		opt.TargetBranch = gitlabapi.Ptr(strings.TrimSpace(input.TargetBranch))
	}
	if strings.TrimSpace(input.StateEvent) != "" {
		opt.StateEvent = gitlabapi.Ptr(strings.TrimSpace(input.StateEvent))
	}
	if input.Labels != nil {
		labels := gitlabapi.LabelOptions(input.Labels)
		opt.Labels = &labels
	}
	mr, _, err := c.client.MergeRequests.UpdateMergeRequest(project, iid, opt)
	if err != nil {
		return MergeRequest{}, err
	}
	return mergeRequestFromAPI(mr), nil
}

func (c liveClient) ListRepositoryTree(project any, input TreeListOptions) ([]TreeEntry, bool, error) {
	opt := &gitlabapi.ListTreeOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 100))
	opt.Page = 1
	if strings.TrimSpace(input.Path) != "" {
		opt.Path = gitlabapi.Ptr(strings.TrimSpace(input.Path))
	}
	if strings.TrimSpace(input.Ref) != "" {
		opt.Ref = gitlabapi.Ptr(strings.TrimSpace(input.Ref))
	}
	if input.Recursive {
		opt.Recursive = gitlabapi.Ptr(true)
	}
	var out []TreeEntry
	for {
		nodes, resp, err := c.client.Repositories.ListTree(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, node := range nodes {
			if input.Limit > 0 && len(out) >= input.Limit {
				return out, true, nil
			}
			out = append(out, TreeEntry{Path: node.Path, Name: node.Name, Type: node.Type, Mode: node.Mode})
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) GetRepositoryFile(project any, path, ref string) (RepoFileRaw, error) {
	opt := &gitlabapi.GetFileOptions{}
	if strings.TrimSpace(ref) != "" {
		opt.Ref = gitlabapi.Ptr(strings.TrimSpace(ref))
	}
	file, _, err := c.client.RepositoryFiles.GetFile(project, path, opt)
	if err != nil {
		return RepoFileRaw{}, err
	}
	content := []byte(file.Content)
	if strings.EqualFold(file.Encoding, "base64") {
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			return RepoFileRaw{}, fmt.Errorf("decode file content: %w", err)
		}
		content = decoded
	}
	return RepoFileRaw{
		FileName:     file.FileName,
		FilePath:     file.FilePath,
		Ref:          file.Ref,
		Size:         file.Size,
		Content:      content,
		BlobID:       file.BlobID,
		CommitID:     file.CommitID,
		LastCommitID: file.LastCommitID,
	}, nil
}

func (c liveClient) GetRepositoryArchive(project any, format, sha, path string) ([]byte, error) {
	opt := &gitlabapi.ArchiveOptions{Format: gitlabapi.Ptr(format)}
	if strings.TrimSpace(sha) != "" {
		opt.SHA = gitlabapi.Ptr(strings.TrimSpace(sha))
	}
	if strings.TrimSpace(path) != "" {
		opt.Path = gitlabapi.Ptr(strings.TrimSpace(path))
	}
	data, _, err := c.client.Repositories.Archive(project, opt)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c liveClient) CreateProject(input ProjectCreateOptions) (Project, error) {
	opt := &gitlabapi.CreateProjectOptions{Name: gitlabapi.Ptr(input.Name)}
	if strings.TrimSpace(input.Path) != "" {
		opt.Path = gitlabapi.Ptr(strings.TrimSpace(input.Path))
	}
	if input.NamespaceID > 0 {
		opt.NamespaceID = gitlabapi.Ptr(input.NamespaceID)
	}
	if strings.TrimSpace(input.Description) != "" {
		opt.Description = gitlabapi.Ptr(strings.TrimSpace(input.Description))
	}
	if strings.TrimSpace(input.Visibility) != "" {
		visibility := gitlabapi.VisibilityValue(strings.TrimSpace(input.Visibility))
		opt.Visibility = &visibility
	}
	if input.InitializeWithReadme {
		opt.InitializeWithReadme = gitlabapi.Ptr(true)
	}
	project, _, err := c.client.Projects.CreateProject(opt)
	if err != nil {
		return Project{}, err
	}
	return projectFromAPI(project), nil
}

func discussionFromAPI(discussion *gitlabapi.Discussion) DiscussionInfo {
	if discussion == nil {
		return DiscussionInfo{}
	}
	out := DiscussionInfo{ID: discussion.ID, IndividualNote: discussion.IndividualNote}
	for _, note := range discussion.Notes {
		if note == nil {
			continue
		}
		mapped := DiscussionNote{
			ID:             note.ID,
			Body:           note.Body,
			AuthorUsername: note.Author.Username,
			CreatedAt:      formatTime(note.CreatedAt),
			System:         note.System,
			Resolvable:     note.Resolvable,
			Resolved:       note.Resolved,
		}
		if note.Position != nil {
			mapped.Position = &NotePositionInfo{
				OldPath: note.Position.OldPath,
				NewPath: note.Position.NewPath,
				OldLine: note.Position.OldLine,
				NewLine: note.Position.NewLine,
			}
		}
		out.Notes = append(out.Notes, mapped)
	}
	return out
}
