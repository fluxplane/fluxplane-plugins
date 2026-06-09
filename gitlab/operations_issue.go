package gitlab

import (
	"fmt"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type IssueListInput struct {
	pluginbinding.ListInput
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	State     string `json:"state,omitempty" jsonschema:"description=Issue state filter,enum=opened,enum=closed,enum=all"`
}

type IssueShowInput struct {
	Ref       string `json:"ref,omitempty" jsonschema:"description=Issue reference as PROJECT#IID"`
	ID        string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID       int64  `json:"iid,omitempty" jsonschema:"description=Issue IID within the project"`
	IssueIID  int64  `json:"issue_iid,omitempty" jsonschema:"description=Alias for iid"`
}

type IssueCreateInput struct {
	Project      string   `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID    string   `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path         string   `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Title        string   `json:"title,omitempty" jsonschema:"description=Issue title"`
	Description  string   `json:"description,omitempty" jsonschema:"description=Issue description as GitLab-flavored Markdown"`
	Labels       []string `json:"labels,omitempty" jsonschema:"description=Labels to set"`
	AssigneeIDs  []int64  `json:"assignee_ids,omitempty" jsonschema:"description=Assignee user IDs"`
	MilestoneID  int64    `json:"milestone_id,omitempty" jsonschema:"description=Milestone ID"`
	Confidential *bool    `json:"confidential,omitempty" jsonschema:"description=Mark the issue confidential"`
}

type IssueUpdateInput struct {
	Ref          string   `json:"ref,omitempty" jsonschema:"description=Issue reference as PROJECT#IID"`
	ID           string   `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project      string   `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID    string   `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path         string   `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID          int64    `json:"iid,omitempty" jsonschema:"description=Issue IID within the project"`
	IssueIID     int64    `json:"issue_iid,omitempty" jsonschema:"description=Alias for iid"`
	Title        string   `json:"title,omitempty" jsonschema:"description=New title"`
	Description  string   `json:"description,omitempty" jsonschema:"description=New description (Markdown)"`
	Labels       []string `json:"labels,omitempty" jsonschema:"description=Replace the full label set"`
	AddLabels    []string `json:"add_labels,omitempty" jsonschema:"description=Labels to add"`
	RemoveLabels []string `json:"remove_labels,omitempty" jsonschema:"description=Labels to remove"`
	StateEvent   string   `json:"state_event,omitempty" jsonschema:"description=Transition the issue,enum=close,enum=reopen"`
	AssigneeIDs  []int64  `json:"assignee_ids,omitempty" jsonschema:"description=Replace assignee user IDs"`
}

type IssueNoteListInput struct {
	Ref       string `json:"ref,omitempty" jsonschema:"description=Issue reference as PROJECT#IID"`
	ID        string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID       int64  `json:"iid,omitempty" jsonschema:"description=Issue IID within the project"`
	IssueIID  int64  `json:"issue_iid,omitempty" jsonschema:"description=Alias for iid"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum notes to return"`
	Sort      string `json:"sort,omitempty" jsonschema:"description=Sort direction,enum=asc,enum=desc"`
	OrderBy   string `json:"order_by,omitempty" jsonschema:"description=Order notes by this field,enum=created_at,enum=updated_at"`
}

type IssueNoteCreateInput struct {
	Ref       string `json:"ref,omitempty" jsonschema:"description=Issue reference as PROJECT#IID"`
	ID        string `json:"id,omitempty" jsonschema:"description=Alias for ref"`
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	IID       int64  `json:"iid,omitempty" jsonschema:"description=Issue IID within the project"`
	IssueIID  int64  `json:"issue_iid,omitempty" jsonschema:"description=Alias for iid"`
	Body      string `json:"body,omitempty" jsonschema:"description=Comment body as GitLab-flavored Markdown"`
}

func (s Service) IssueList(ctx pluginbinding.Context, input IssueListInput) (pluginbinding.ListResult[Issue], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[Issue]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	issues, err := client.ListIssues(IssueListOptions{
		Limit:   input.Limit,
		Search:  strings.TrimSpace(firstNonEmpty(input.Search, input.Query)),
		State:   strings.TrimSpace(input.State),
		OrderBy: strings.TrimSpace(input.OrderBy),
		Sort:    strings.TrimSpace(input.Sort),
	})
	if err != nil {
		return pluginbinding.ListResult[Issue]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewListResult(issues), nil
}

func (s Service) IssueShow(ctx pluginbinding.Context, input IssueShowInput) (pluginbinding.ShowResult[Issue], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := issueAddress(input.Ref, input.ID, issueProject(input.Project, input.ProjectID, input.Path), input.IID, input.IssueIID)
	if err != nil {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	issue, err := client.GetIssue(projectID(project), iid)
	if err != nil {
		return pluginbinding.ShowResult[Issue]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return pluginbinding.NewShowResult(issue, map[string]any{"project": project, "iid": iid}), nil
}

func (s Service) IssueCreate(ctx pluginbinding.Context, input IssueCreateInput) (Issue, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Issue{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := issueProject(input.Project, input.ProjectID, input.Path)
	if project == "" {
		return Issue{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return Issue{}, pluginbinding.Fail("bad_input", "title is required")
	}
	issue, err := client.CreateIssue(projectID(project), IssueCreateOptions{
		Project:      project,
		Title:        strings.TrimSpace(input.Title),
		Description:  input.Description,
		Labels:       input.Labels,
		AssigneeIDs:  input.AssigneeIDs,
		MilestoneID:  input.MilestoneID,
		Confidential: input.Confidential,
	})
	if err != nil {
		return Issue{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return issue, nil
}

func (s Service) IssueUpdate(ctx pluginbinding.Context, input IssueUpdateInput) (Issue, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Issue{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := issueAddress(input.Ref, input.ID, issueProject(input.Project, input.ProjectID, input.Path), input.IID, input.IssueIID)
	if err != nil {
		return Issue{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	issue, err := client.UpdateIssue(projectID(project), iid, IssueUpdateOptions{
		Project:      project,
		IID:          iid,
		Title:        strings.TrimSpace(input.Title),
		Description:  input.Description,
		Labels:       input.Labels,
		AddLabels:    input.AddLabels,
		RemoveLabels: input.RemoveLabels,
		StateEvent:   strings.TrimSpace(input.StateEvent),
		AssigneeIDs:  input.AssigneeIDs,
	})
	if err != nil {
		return Issue{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return issue, nil
}

func (s Service) IssueNoteList(ctx pluginbinding.Context, input IssueNoteListInput) (IssueNoteListResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return IssueNoteListResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := issueAddress(input.Ref, input.ID, issueProject(input.Project, input.ProjectID, input.Path), input.IID, input.IssueIID)
	if err != nil {
		return IssueNoteListResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	notes, err := client.ListIssueNotes(projectID(project), iid, IssueNoteListOptions{Project: project, IID: iid, Limit: input.Limit, Sort: strings.TrimSpace(input.Sort), OrderBy: strings.TrimSpace(input.OrderBy)})
	if err != nil {
		return IssueNoteListResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return IssueNoteListResult{Project: project, IID: iid, Count: len(notes), Notes: notes}, nil
}

func (s Service) IssueNoteCreate(ctx pluginbinding.Context, input IssueNoteCreateInput) (Note, error) {
	client, err := s.client(ctx)
	if err != nil {
		return Note{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project, iid, err := issueAddress(input.Ref, input.ID, issueProject(input.Project, input.ProjectID, input.Path), input.IID, input.IssueIID)
	if err != nil {
		return Note{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(input.Body) == "" {
		return Note{}, pluginbinding.Fail("bad_input", "body is required")
	}
	note, err := client.CreateIssueNote(projectID(project), iid, IssueNoteCreateOptions{Project: project, IID: iid, Body: input.Body})
	if err != nil {
		return Note{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return note, nil
}

func issueProject(values ...string) string {
	return firstNonEmpty(values...)
}

// issueAddress resolves the (project, iid) of an issue from either a PROJECT#IID
// ref (parsed by the existing parseIssueRef) or explicit project + iid fields.
func issueAddress(ref, id, project string, iid, issueIID int64) (string, int64, error) {
	if r := strings.TrimSpace(firstNonEmpty(ref, id)); r != "" {
		return parseIssueRef(r)
	}
	project = strings.TrimSpace(project)
	if iid <= 0 {
		iid = issueIID
	}
	if project == "" || iid <= 0 {
		return "", 0, fmt.Errorf("provide an issue ref (PROJECT#IID) or project and iid")
	}
	return project, iid, nil
}
