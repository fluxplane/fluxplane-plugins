package gitlab

// Types backing the merge-request review workflow (issue #3): diffs, ref
// comparison, discussions with line positions, repository tree/file reads,
// and source archives.

// FileDiff is one changed file with its unified diff text.
type FileDiff struct {
	OldPath       string `json:"old_path,omitempty"`
	NewPath       string `json:"new_path,omitempty"`
	NewFile       bool   `json:"new_file,omitempty"`
	RenamedFile   bool   `json:"renamed_file,omitempty"`
	DeletedFile   bool   `json:"deleted_file,omitempty"`
	GeneratedFile bool   `json:"generated_file,omitempty"`
	TooLarge      bool   `json:"too_large,omitempty"`
	Diff          string `json:"diff,omitempty"`
	DiffTruncated bool   `json:"diff_truncated,omitempty"`
}

// DiffRefs are the three SHAs a positioned review comment is anchored to.
type DiffRefs struct {
	BaseSHA  string `json:"base_sha,omitempty"`
	StartSHA string `json:"start_sha,omitempty"`
	HeadSHA  string `json:"head_sha,omitempty"`
}

// CompareCommit is one commit in a ref comparison.
type CompareCommit struct {
	ID         string `json:"id"`
	ShortID    string `json:"short_id,omitempty"`
	Title      string `json:"title,omitempty"`
	AuthorName string `json:"author_name,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

// NotePositionInfo is the line anchor of a positioned discussion note.
type NotePositionInfo struct {
	OldPath string `json:"old_path,omitempty"`
	NewPath string `json:"new_path,omitempty"`
	OldLine int64  `json:"old_line,omitempty"`
	NewLine int64  `json:"new_line,omitempty"`
}

// DiscussionNote is one note inside a discussion thread.
type DiscussionNote struct {
	ID             int64             `json:"id"`
	Body           string            `json:"body,omitempty"`
	AuthorUsername string            `json:"author_username,omitempty"`
	CreatedAt      string            `json:"created_at,omitempty"`
	System         bool              `json:"system,omitempty"`
	Resolvable     bool              `json:"resolvable,omitempty"`
	Resolved       bool              `json:"resolved,omitempty"`
	Position       *NotePositionInfo `json:"position,omitempty"`
}

// DiscussionInfo is one MR discussion thread.
type DiscussionInfo struct {
	ID             string           `json:"id"`
	IndividualNote bool             `json:"individual_note,omitempty"`
	Notes          []DiscussionNote `json:"notes,omitempty"`
}

// PositionInput is the resolved text position sent with a line-level
// discussion (position_type is always "text").
type PositionInput struct {
	BaseSHA  string `json:"base_sha"`
	StartSHA string `json:"start_sha"`
	HeadSHA  string `json:"head_sha"`
	OldPath  string `json:"old_path"`
	NewPath  string `json:"new_path"`
	OldLine  int64  `json:"old_line,omitempty"`
	NewLine  int64  `json:"new_line,omitempty"`
}

// MergeRequestUpdateOptions are the mutable MR fields exposed by mr.update.
type MergeRequestUpdateOptions struct {
	Title        string
	Description  *string
	TargetBranch string
	StateEvent   string
	Labels       []string
}

// TreeListOptions select a repository tree listing.
type TreeListOptions struct {
	Path      string
	Ref       string
	Recursive bool
	Limit     int
}

// TreeEntry is one repository tree node.
type TreeEntry struct {
	Path string `json:"path"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"` // tree | blob
	Mode string `json:"mode,omitempty"`
}

// RepoFileRaw is a repository file fetched at a ref, content decoded.
type RepoFileRaw struct {
	FileName     string
	FilePath     string
	Ref          string
	Size         int64
	Content      []byte
	BlobID       string
	CommitID     string
	LastCommitID string
}

// ProjectCreateOptions create a project, optionally inside a namespace.
type ProjectCreateOptions struct {
	Name                 string
	Path                 string
	NamespaceID          int64
	Description          string
	Visibility           string
	InitializeWithReadme bool
}

// BlobMatch is one file-content search hit (scope=blobs).
type BlobMatch struct {
	ProjectID     int64  `json:"project_id,omitempty"`
	Path          string `json:"path"`
	Filename      string `json:"filename,omitempty"`
	Basename      string `json:"basename,omitempty"`
	Ref           string `json:"ref,omitempty"`
	StartLine     int64  `json:"start_line,omitempty"`
	Data          string `json:"data,omitempty"`
	DataTruncated bool   `json:"data_truncated,omitempty"`
}
