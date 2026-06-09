package jira

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugins/atlassian/internal/atlassian"
)

// bodyFormat selects how rich-text bodies (issue descriptions, comments) are
// rendered for callers. The default keeps agents away from raw ADF.
type bodyFormat string

const (
	bodyFormatMarkdown bodyFormat = "markdown"
	bodyFormatADF      bodyFormat = "adf"
	bodyFormatBoth     bodyFormat = "both"
)

func parseBodyFormat(value string) bodyFormat {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(bodyFormatADF):
		return bodyFormatADF
	case string(bodyFormatBoth):
		return bodyFormatBoth
	default:
		return bodyFormatMarkdown
	}
}

func nonEmptyRaw(raw json.RawMessage) json.RawMessage {
	if trimmed := strings.TrimSpace(string(raw)); trimmed == "" || trimmed == "null" {
		return nil
	}
	return raw
}

// captureADF interprets a rich-text JSON value that may be either an ADF
// document (object/array, as Jira sends) or an already-rendered Markdown string
// (our own output form, so the type round-trips). Exactly one return is set.
func captureADF(raw json.RawMessage) (rendered string, adf json.RawMessage) {
	raw = nonEmptyRaw(raw)
	if len(raw) == 0 {
		return "", nil
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s, nil
		}
	}
	return "", raw
}

// renderedMarkdown renders ADF to Markdown, falling back to an already-rendered
// string when no raw ADF is present (e.g. a round-tripped value).
func renderedMarkdown(existing string, raw json.RawMessage) string {
	if len(nonEmptyRaw(raw)) > 0 {
		return atlassian.ADFToMarkdown(raw)
	}
	return existing
}

type User struct {
	AccountID    string `json:"accountId,omitempty"`
	AccountType  string `json:"accountType,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Self         string `json:"self,omitempty"`
}

type UserRecord struct {
	pluginbinding.DatasourceRecord
	AccountID    string `json:"account_id" datasource:"id,completion,view=compact|lookup|table"`
	DisplayName  string `json:"display_name,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	EmailAddress string `json:"email,omitempty" datasource:"completion,view=lookup|table"`
	AccountType  string `json:"account_type,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Self         string `json:"self,omitempty"`
}

type Issue struct {
	ID     string      `json:"id,omitempty"`
	Key    string      `json:"key,omitempty"`
	Self   string      `json:"self,omitempty"`
	Fields IssueFields `json:"fields,omitempty"`
}

type IssueFields struct {
	Summary string `json:"summary,omitempty"`
	// Description is the issue description rendered to Markdown by default so
	// callers never handle raw ADF. DescriptionADF carries the raw Atlassian
	// Document Format and is only populated when body_format is adf or both.
	Description    string          `json:"description,omitempty"`
	DescriptionADF json.RawMessage `json:"description_adf,omitempty"`
	Attachments    []Attachment    `json:"attachment,omitempty"`
	Status         NamedValue      `json:"status,omitempty"`
	Assignee       *User           `json:"assignee,omitempty"`
	Reporter       *User           `json:"reporter,omitempty"`
	Creator        *User           `json:"creator,omitempty"`
	Project        Project         `json:"project,omitempty"`
	IssueType      NamedValue      `json:"issuetype,omitempty"`
	Priority       NamedValue      `json:"priority,omitempty"`
	Labels         []string        `json:"labels,omitempty"`
	Updated        string          `json:"updated,omitempty"`
	Created        string          `json:"created,omitempty"`
	Raw            interface{}     `json:"-"`

	// rawDescription holds the description ADF as received from Jira until
	// render decides which representation(s) to expose.
	rawDescription json.RawMessage
}

// UnmarshalJSON captures Jira's ADF description object into rawDescription
// (leaving render to choose the output representation) while still accepting an
// already-rendered string body so the type round-trips through its own output.
func (f *IssueFields) UnmarshalJSON(data []byte) error {
	type wire IssueFields
	aux := struct {
		*wire
		Description json.RawMessage `json:"description"`
	}{wire: (*wire)(f)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	f.Description, f.rawDescription = captureADF(aux.Description)
	return nil
}

func (f *IssueFields) render(format bodyFormat) {
	md := renderedMarkdown(f.Description, f.rawDescription)
	adf := nonEmptyRaw(f.rawDescription)
	switch format {
	case bodyFormatADF:
		f.Description, f.DescriptionADF = "", adf
	case bodyFormatBoth:
		f.Description, f.DescriptionADF = md, adf
	default:
		f.Description, f.DescriptionADF = md, nil
	}
}

func (i *Issue) render(format bodyFormat) {
	if i == nil {
		return
	}
	i.Fields.render(format)
}

type Project struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
	Self string `json:"self,omitempty"`
}

type NamedValue struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Self string `json:"self,omitempty"`
}

type IssueRecord struct {
	pluginbinding.DatasourceRecord
	IssueID             string   `json:"issue_id,omitempty"`
	Key                 string   `json:"key" datasource:"id,completion,view=compact|lookup|table"`
	Summary             string   `json:"summary,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	ProjectKey          string   `json:"project_key,omitempty" datasource:"completion,view=compact|lookup|table"`
	ProjectName         string   `json:"project_name,omitempty" datasource:"completion,view=lookup|table"`
	IssueType           string   `json:"issue_type,omitempty" datasource:"completion,view=compact|lookup|table"`
	Status              string   `json:"status,omitempty" datasource:"completion,view=compact|lookup|table"`
	Priority            string   `json:"priority,omitempty" datasource:"completion,view=lookup|table"`
	AssigneeAccountID   string   `json:"assignee_account_id,omitempty" datasource:"relation=jira.user:assignee"`
	AssigneeDisplayName string   `json:"assignee_display_name,omitempty" datasource:"completion,view=compact|lookup|table"`
	ReporterAccountID   string   `json:"reporter_account_id,omitempty" datasource:"relation=jira.user:reporter"`
	ReporterDisplayName string   `json:"reporter_display_name,omitempty" datasource:"completion,view=lookup|table"`
	CreatorAccountID    string   `json:"creator_account_id,omitempty" datasource:"relation=jira.user:creator"`
	CreatorDisplayName  string   `json:"creator_display_name,omitempty"`
	Labels              []string `json:"labels,omitempty"`
	Created             string   `json:"created,omitempty"`
	Updated             string   `json:"updated,omitempty" datasource:"view=compact|lookup|table"`
	WebURL              string   `json:"web_url,omitempty" datasource:"completion,view=lookup|table"`
	Self                string   `json:"self,omitempty"`
}

type IssueSearchOptions struct {
	Query         string
	JQL           string
	Project       string
	Status        string
	Limit         int
	All           bool
	NextPageToken string
	Fields        []string
	OrderBy       string
}

type IssueCreateMetaOptions struct {
	ProjectKey string
	IssueType  string
}

type IssueCreateRequest struct {
	Fields map[string]any `json:"fields"`
	Update map[string]any `json:"update,omitempty"`
}

type IssueEditRequest struct {
	Fields map[string]any `json:"fields,omitempty"`
	Update map[string]any `json:"update,omitempty"`
}

type IssueMutationResult struct {
	OK      bool   `json:"ok"`
	ID      string `json:"id,omitempty"`
	Key     string `json:"key,omitempty"`
	Self    string `json:"self,omitempty"`
	Issue   *Issue `json:"issue,omitempty"`
	Warning string `json:"warning,omitempty"`
}

type IssueTransition struct {
	ID   string     `json:"id,omitempty"`
	Name string     `json:"name,omitempty"`
	To   NamedValue `json:"to,omitempty"`
}

type IssueTransitionListResult struct {
	IssueKey      string            `json:"issue_key,omitempty"`
	CurrentStatus NamedValue        `json:"current_status,omitempty"`
	Transitions   []IssueTransition `json:"transitions"`
}

type IssueTransitionRequest struct {
	TransitionID string `json:"-"`
}

type IssueTransitionRunResult struct {
	OK                   bool              `json:"ok"`
	IssueKey             string            `json:"issue_key,omitempty"`
	InitialStatus        NamedValue        `json:"initial_status,omitempty"`
	CurrentStatus        NamedValue        `json:"current_status,omitempty"`
	TargetStatus         string            `json:"target_status,omitempty"`
	AppliedTransitions   []IssueTransition `json:"applied_transitions,omitempty"`
	AvailableTransitions []IssueTransition `json:"available_transitions,omitempty"`
	Steps                int               `json:"steps"`
	Issue                *Issue            `json:"issue,omitempty"`
}

type CommentRequest struct {
	Body any `json:"body"`
}

type Comment struct {
	ID   string `json:"id,omitempty"`
	Self string `json:"self,omitempty"`
	// Body is the comment rendered to Markdown by default so callers never
	// handle raw ADF. BodyADF carries the raw Atlassian Document Format and is
	// only populated when body_format is adf or both.
	Body         string          `json:"body,omitempty"`
	BodyADF      json.RawMessage `json:"body_adf,omitempty"`
	Author       *User           `json:"author,omitempty"`
	UpdateAuthor *User           `json:"updateAuthor,omitempty"`
	Created      string          `json:"created,omitempty"`
	Updated      string          `json:"updated,omitempty"`

	// rawBody holds the comment ADF as received from Jira until render decides
	// which representation(s) to expose.
	rawBody json.RawMessage
}

// UnmarshalJSON captures Jira's ADF comment body into rawBody (leaving render
// to choose the output representation) while still accepting an already-rendered
// string body so the type round-trips through its own output.
func (c *Comment) UnmarshalJSON(data []byte) error {
	type wire Comment
	aux := struct {
		*wire
		Body json.RawMessage `json:"body"`
	}{wire: (*wire)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	c.Body, c.rawBody = captureADF(aux.Body)
	return nil
}

func (c *Comment) render(format bodyFormat) {
	md := renderedMarkdown(c.Body, c.rawBody)
	adf := nonEmptyRaw(c.rawBody)
	switch format {
	case bodyFormatADF:
		c.Body, c.BodyADF = "", adf
	case bodyFormatBoth:
		c.Body, c.BodyADF = md, adf
	default:
		c.Body, c.BodyADF = md, nil
	}
}

type CommentResult struct {
	OK       bool    `json:"ok"`
	IssueKey string  `json:"issue_key,omitempty"`
	Comment  Comment `json:"comment"`
}

type CommentMutationResult struct {
	OK        bool   `json:"ok"`
	IssueKey  string `json:"issue_key,omitempty"`
	CommentID string `json:"comment_id,omitempty"`
}

// CommentListOptions controls a paginated comment fetch.
type CommentListOptions struct {
	Limit   int
	StartAt int
	Order   string
}

type CommentListResult struct {
	IssueKey    string    `json:"issue_key,omitempty"`
	Count       int       `json:"count"`
	Total       int       `json:"total"`
	StartAt     int       `json:"start_at"`
	NextStartAt int       `json:"next_start_at,omitempty"`
	Comments    []Comment `json:"comments"`
}

type commentListResponse struct {
	StartAt    int       `json:"startAt"`
	MaxResults int       `json:"maxResults"`
	Total      int       `json:"total"`
	Comments   []Comment `json:"comments"`
}

type Attachment struct {
	ID        string `json:"id,omitempty"`
	Filename  string `json:"filename,omitempty"`
	MimeType  string `json:"mimeType,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Content   string `json:"content,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Self      string `json:"self,omitempty"`
	Created   string `json:"created,omitempty"`
	Author    *User  `json:"author,omitempty"`
}

type AttachmentUploadRequest struct {
	Filename    string
	ContentType string
	Data        []byte
}

type AttachmentUploadResult struct {
	OK          bool         `json:"ok"`
	IssueKey    string       `json:"issue_key,omitempty"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentListResult struct {
	IssueKey    string       `json:"issue_key,omitempty"`
	Count       int          `json:"count"`
	Attachments []Attachment `json:"attachments"`
}

type AttachmentGetResult struct {
	ID           string                `json:"id"`
	Filename     string                `json:"filename,omitempty"`
	MimeType     string                `json:"mime_type,omitempty"`
	Size         int                   `json:"size"`
	ContentBytes []byte                `json:"content_bytes,omitempty"`
	Blob         pluginbinding.BlobRef `json:"blob,omitempty"`
}

type AttachmentDeleteResult struct {
	OK           bool   `json:"ok"`
	AttachmentID string `json:"attachment_id"`
}

type IssueMetaResult struct {
	Metadata json.RawMessage `json:"metadata"`
}

type UserSearchOptions struct {
	Query   string
	Limit   int
	All     bool
	StartAt int
}

type issueSearchResponse struct {
	Issues        []Issue `json:"issues"`
	NextPageToken string  `json:"nextPageToken,omitempty"`
	IsLast        bool    `json:"isLast,omitempty"`
}

type issueTransitionsResponse struct {
	Transitions []IssueTransition `json:"transitions"`
}

func normalizeIssueRecord(source pluginbinding.DatasourceSource, baseURL string, issue Issue) (IssueRecord, bool) {
	key := strings.TrimSpace(issue.Key)
	if key == "" {
		return IssueRecord{}, false
	}
	fields := issue.Fields
	record := IssueRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityIssue, key, pluginbinding.RecordTitle(fields.Summary), pluginbinding.RecordLink("self", issueWebURL(baseURL, key))),
		IssueID:          issue.ID,
		Key:              key,
		Summary:          fields.Summary,
		ProjectKey:       fields.Project.Key,
		ProjectName:      fields.Project.Name,
		IssueType:        fields.IssueType.Name,
		Status:           fields.Status.Name,
		Priority:         fields.Priority.Name,
		Labels:           fields.Labels,
		Created:          fields.Created,
		Updated:          fields.Updated,
		WebURL:           issueWebURL(baseURL, key),
		Self:             issue.Self,
	}
	if fields.Assignee != nil {
		record.AssigneeAccountID = fields.Assignee.AccountID
		record.AssigneeDisplayName = fields.Assignee.DisplayName
	}
	if fields.Reporter != nil {
		record.ReporterAccountID = fields.Reporter.AccountID
		record.ReporterDisplayName = fields.Reporter.DisplayName
	}
	if fields.Creator != nil {
		record.CreatorAccountID = fields.Creator.AccountID
		record.CreatorDisplayName = fields.Creator.DisplayName
	}
	return record, true
}

func normalizeUserRecord(source pluginbinding.DatasourceSource, user User) (UserRecord, bool) {
	id := strings.TrimSpace(user.AccountID)
	if id == "" {
		return UserRecord{}, false
	}
	return UserRecord{
		DatasourceRecord: pluginbinding.NewDatasourceRecord(source, EntityUser, id, pluginbinding.RecordTitle(user.DisplayName), pluginbinding.RecordLink("self", user.Self)),
		AccountID:        id,
		DisplayName:      user.DisplayName,
		EmailAddress:     user.EmailAddress,
		AccountType:      user.AccountType,
		Active:           user.Active,
		Self:             user.Self,
	}, true
}

func issueWebURL(baseURL, key string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	key = strings.TrimSpace(key)
	if baseURL == "" || key == "" {
		return ""
	}
	if parsed, err := url.Parse(baseURL); err == nil && parsed.Host == "api.atlassian.com" {
		return ""
	}
	return baseURL + "/browse/" + url.PathEscape(key)
}
