package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func requestJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func protocolRequestWithConfig(config map[string]any) protocol.Request {
	return protocol.Request{Plugin: PluginName, Instance: "default", Config: config}
}

func TestServiceBuildsClientFromEndpointRef(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada"}}
	var capturedEndpointRef string
	plugin := NewPluginWithService(Service{
		ClientFactory: func(_ pluginbinding.Context, endpointRef string) (Client, error) {
			capturedEndpointRef = endpointRef
			return client, nil
		},
	})

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationTest, map[string]any{"endpoint_ref": "jira-dev"}, plugintest.WithInstance("work"))
	if out.Status != "ok" || out.User.AccountID != "acct-1" {
		t.Fatalf("auth output = %#v", out)
	}
	if capturedEndpointRef != "jira-dev" {
		t.Fatalf("endpoint_ref = %q", capturedEndpointRef)
	}
}

func TestServiceBuildsClientFromStoredEndpointConfig(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada"}}
	var capturedEndpointRef string
	plugin := NewPluginWithService(Service{
		ClientFactory: func(_ pluginbinding.Context, endpointRef string) (Client, error) {
			capturedEndpointRef = endpointRef
			return client, nil
		},
	})

	out := plugintest.RunOK[AuthTestResult](t, plugin, OperationTest, nil, plugintest.WithRequest(protocolRequestWithConfig(map[string]any{
		"endpoint_refs": map[string]any{EndpointName: "jira-stored"},
	})))
	if out.Status != "ok" || capturedEndpointRef != "jira-stored" {
		t.Fatalf("out = %#v endpoint_ref = %q", out, capturedEndpointRef)
	}
}

func TestIssueSearchAndDatasourceNormalizeRecords(t *testing.T) {
	client := &fakeClient{issues: []Issue{{
		ID:  "10001",
		Key: "DEX-7",
		Fields: IssueFields{
			Summary: "Ship Jira plugin",
			Status:  NamedValue{Name: "In Progress"},
			Project: Project{Key: "DEX", Name: "Dex"},
			Assignee: &User{
				AccountID: "acct-1", DisplayName: "Ada",
			},
			Reporter: &User{AccountID: "acct-2", DisplayName: "Jane"},
			Updated:  "2026-05-28T10:00:00.000+0000",
		},
	}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueSearchResult](t, plugin, OperationIssueSearch, map[string]any{"query": "plugin", "project": "DEX", "limit": 5})
	if out.Count != 1 || out.Items[0].Key != "DEX-7" {
		t.Fatalf("issue search output = %#v", out)
	}
	if client.issueOptions.Query != "plugin" || client.issueOptions.Project != "DEX" || client.issueOptions.Limit != 5 {
		t.Fatalf("issue options = %#v", client.issueOptions)
	}

	ds := plugintest.DatasourceSearchOK[IssueDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceIssues, "query": "plugin"})
	if ds.Count != 1 || ds.Records[0].ID != "DEX-7" || ds.Records[0].WebURL != "" {
		t.Fatalf("datasource output = %#v", ds)
	}
}

func TestDatasourceSearchAcceptsEndpointRef(t *testing.T) {
	client := &fakeClient{}
	var capturedEndpointRef string
	plugin := NewPluginWithService(Service{ClientFactory: func(_ pluginbinding.Context, endpointRef string) (Client, error) {
		capturedEndpointRef = endpointRef
		return client, nil
	}})

	_ = plugintest.DatasourceSearchOK[IssueDatasourceResult](t, plugin, map[string]any{"datasource": DatasourceIssues, "entity": EntityIssue, "query": "bug", "endpoint_ref": "jira-prod"})
	if capturedEndpointRef != "jira-prod" {
		t.Fatalf("endpoint_ref = %q", capturedEndpointRef)
	}
}

func TestDatasourceGetFetchesLiveIssue(t *testing.T) {
	client := &fakeClient{issue: Issue{
		ID:  "10001",
		Key: "DEX-7",
		Fields: IssueFields{
			Summary: "Ship Jira plugin",
			Status:  NamedValue{Name: "In Progress"},
			Project: Project{Key: "DEX", Name: "Dex"},
		},
	}}
	plugin := testPlugin(client)

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[IssueRecord]](t, plugin, map[string]any{"datasource": DatasourceIssues, "entity": EntityIssue, "id": "DEX-7"})
	if out.Record.ID != "DEX-7" || out.Record.Summary != "Ship Jira plugin" || out.Record.ProjectKey != "DEX" {
		t.Fatalf("get output = %#v", out)
	}
	if client.issueKey != "DEX-7" {
		t.Fatalf("issue key = %q", client.issueKey)
	}
}

func TestDatasourceGetFetchesLiveUser(t *testing.T) {
	client := &fakeClient{user: User{AccountID: "acct-1", DisplayName: "Ada Lovelace", Active: true}}
	plugin := testPlugin(client)

	out := plugintest.DatasourceGetOK[pluginbinding.DatasourceGetResult[UserRecord]](t, plugin, map[string]any{"datasource": DatasourceUsers, "entity": EntityUser, "id": "acct-1"})
	if out.Record.ID != "acct-1" || out.Record.DisplayName != "Ada Lovelace" || !out.Record.Active {
		t.Fatalf("get output = %#v", out)
	}
	if client.userOptions.Query != "acct-1" {
		t.Fatalf("user id = %q", client.userOptions.Query)
	}
}

func TestIssueJQLCombinesFiltersWithAnd(t *testing.T) {
	got := issueJQL(IssueSearchOptions{Project: "DEX", Status: "In Progress", Query: "plugin", OrderBy: "updated DESC"})
	want := `project = "DEX" and status = "In Progress" and text ~ "plugin" order by updated DESC`
	if got != want {
		t.Fatalf("jql = %q want %q", got, want)
	}
}

func TestIndexBuildCanTargetUsers(t *testing.T) {
	client := &fakeClient{users: []User{{AccountID: "acct-1", DisplayName: "Ada"}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[struct {
		Indexes []struct {
			Index   string            `json:"index"`
			Records []json.RawMessage `json:"records"`
		} `json:"indexes"`
	}](t, plugin, OperationIndexBuild, map[string]any{"entity": EntityUser, "user_query": "ada"})
	if len(out.Indexes) != 1 || out.Indexes[0].Index != DatasourceUsers || len(out.Indexes[0].Records) != 1 {
		t.Fatalf("index output = %#v", out.Indexes)
	}
	if client.issueOptions.All {
		t.Fatalf("targeted user build fetched issues: %#v", client.issueOptions)
	}
	if !client.userOptions.All || client.userOptions.Query != "ada" {
		t.Fatalf("user options = %#v", client.userOptions)
	}
}

func TestLookupPrefersExactIssueKey(t *testing.T) {
	client := &fakeClient{
		issue: Issue{ID: "10001", Key: "DEX-7", Fields: IssueFields{Summary: "Ship Jira plugin"}},
		users: []User{{AccountID: "acct-1", DisplayName: "Ada Lovelace"}},
	}
	plugin := testPlugin(client)

	out := plugintest.DatasourceLookupOK[LookupResult](t, plugin, map[string]any{"text": "See https://example.atlassian.net/browse/DEX-7 with Ada", "limit": 10})
	if out.Count < 2 || out.Matches[0].Entity != EntityIssue || out.Matches[0].ID != "DEX-7" {
		t.Fatalf("lookup output = %#v", out)
	}
	if client.issueKey != "DEX-7" {
		t.Fatalf("issue key = %q", client.issueKey)
	}
}

func TestIssueCreateConvertsMarkdownDescriptionToADF(t *testing.T) {
	client := &fakeClient{createResult: IssueMutationResult{OK: true, ID: "10001", Key: "DEX-9", Self: "https://api.example/issue/10001"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key":          "DEX",
		"issue_type":           "Task",
		"summary":              "Create from markdown",
		"description_markdown": "# Heading\n\nSome **bold** text.\n\n- one\n- two",
		"labels":               []string{"ai", "jira"},
		"assignee_account_id":  "acct-1",
		"reporter_account_id":  "acct-2",
		"priority":             "High",
		"parent_key":           "DEX-1",
		"fields":               map[string]any{"customfield_10042": "custom"},
	})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("create output = %#v", out)
	}
	body := requestJSON(client.createRequest)
	for _, want := range []string{
		`"project":{"key":"DEX"}`,
		`"issuetype":{"name":"Task"}`,
		`"summary":"Create from markdown"`,
		`"customfield_10042":"custom"`,
		`"description":{"content"`,
		`"type":"heading"`,
		`"type":"strong"`,
		`"type":"bulletList"`,
		`"assignee":{"accountId":"acct-1"}`,
		`"reporter":{"accountId":"acct-2"}`,
		`"priority":{"name":"High"}`,
		`"parent":{"key":"DEX-1"}`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("create request = %s, missing %s", body, want)
		}
	}
}

func TestJiraHTTPErrorIncludesFieldErrors(t *testing.T) {
	body := []byte(`{"errorMessages":["You must specify a project."],"errors":{"parent":"Issue does not exist or you do not have permission to see it.","summary":"You must specify a summary."}}`)
	err := jiraHTTPError(400, body)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"status 400",
		"You must specify a project.",
		"parent: Issue does not exist or you do not have permission to see it.",
		"summary: You must specify a summary.",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error = %q, missing %q", msg, want)
		}
	}
}

func TestJiraHTTPErrorFallsBackToMessageThenBody(t *testing.T) {
	if got := jiraHTTPError(401, []byte(`{"message":"Unauthorized"}`)).Error(); !strings.Contains(got, "Unauthorized") {
		t.Fatalf("message fallback = %q", got)
	}
	if got := jiraHTTPError(503, []byte("upstream down")).Error(); !strings.Contains(got, "upstream down") {
		t.Fatalf("raw-body fallback = %q", got)
	}
}

func TestIssueCreateWarnsWhenParentSilentlyDropped(t *testing.T) {
	// The created issue comes back without the requested parent — the Jira
	// silent no-op the warning is meant to surface.
	client := &fakeClient{createResult: IssueMutationResult{
		OK:    true,
		Key:   "DEV-9",
		Issue: &Issue{Key: "DEV-9", Fields: IssueFields{Summary: "Story"}},
	}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key": "DEV",
		"issue_type":  "Story",
		"summary":     "Story",
		"parent_key":  "EPIC-1",
	})
	if out.Warning == "" || !strings.Contains(out.Warning, "parent") || !strings.Contains(out.Warning, "EPIC-1") {
		t.Fatalf("expected parent warning, got %q", out.Warning)
	}
}

func TestIssueCreateNoWarningWhenParentApplied(t *testing.T) {
	client := &fakeClient{createResult: IssueMutationResult{
		OK:    true,
		Key:   "DEV-9",
		Issue: &Issue{Key: "DEV-9", Fields: IssueFields{Summary: "Story", Parent: &IssueReference{Key: "EPIC-1"}}},
	}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key": "DEV",
		"issue_type":  "Story",
		"summary":     "Story",
		"parent_key":  "EPIC-1",
	})
	if out.Warning != "" {
		t.Fatalf("unexpected warning: %q", out.Warning)
	}
}

func TestIssueOutputsCarryBrowseURL(t *testing.T) {
	// site_url not stored → one accessible-resources call resolves the site.
	client := &fakeClient{
		siteURL:      "https://example.atlassian.net",
		issue:        Issue{ID: "1", Key: "DEX-7", Fields: IssueFields{Summary: "Bug"}},
		createResult: IssueMutationResult{OK: true, Key: "DEV-9", Issue: &Issue{Key: "DEV-9"}},
	}
	plugin := testPlugin(client)

	show := plugintest.RunOK[pluginbinding.ShowResult[Issue]](t, plugin, OperationIssueShow, map[string]any{"key": "DEX-7"})
	if show.Record.WebURL != "https://example.atlassian.net/browse/DEX-7" {
		t.Fatalf("show web_url = %q", show.Record.WebURL)
	}
	created := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key": "DEV", "issue_type": "Story", "summary": "Story",
	})
	if created.WebURL != "https://example.atlassian.net/browse/DEV-9" {
		t.Fatalf("create web_url = %q", created.WebURL)
	}
}

func TestBrowseURLPrefersStoredSiteURL(t *testing.T) {
	// With site_url persisted, no accessible-resources call is needed even if
	// it would fail.
	client := &fakeClient{issue: Issue{ID: "1", Key: "DEX-7", Fields: IssueFields{Summary: "Bug"}}}
	plugin := testPlugin(client)

	show := plugintest.RunOK[pluginbinding.ShowResult[Issue]](t, plugin, OperationIssueShow, map[string]any{"key": "DEX-7"},
		plugintest.WithHost(&siteURLHost{site: "https://stored.atlassian.net/"}))
	if show.Record.WebURL != "https://stored.atlassian.net/browse/DEX-7" {
		t.Fatalf("show web_url = %q", show.Record.WebURL)
	}
}

type siteURLHost struct {
	pluginbinding.HostClient
	site string
}

func (h *siteURLHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	if purpose == AuthPurposeSiteURL {
		return pluginbinding.SecretMaterial{Purpose: purpose, Value: h.site}, nil
	}
	return pluginbinding.SecretMaterial{Purpose: purpose}, nil
}

func TestIssueEditSetsParentAndVerifies(t *testing.T) {
	client := &fakeClient{editResult: IssueMutationResult{
		OK:    true,
		Key:   "DEV-9",
		Issue: &Issue{Key: "DEV-9", Fields: IssueFields{Parent: &IssueReference{Key: "EPIC-2"}}},
	}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueEdit, map[string]any{
		"key":        "DEV-9",
		"parent_key": "EPIC-2",
	})
	body := requestJSON(client.editRequest)
	if !strings.Contains(body, `"parent":{"key":"EPIC-2"}`) {
		t.Fatalf("edit request = %s, missing parent field", body)
	}
	if out.Warning != "" {
		t.Fatalf("unexpected warning: %q", out.Warning)
	}
}

func TestIssueCreateConvertsBoldNestedCodeToValidADF(t *testing.T) {
	client := &fakeClient{createResult: IssueMutationResult{OK: true, Key: "DEV-9"}}
	plugin := testPlugin(client)

	plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{
		"project_key":          "DEV",
		"issue_type":           "Task",
		"summary":              "S",
		"description_markdown": "**bold with `code` inside**",
	})
	desc, ok := client.createRequest.Fields["description"]
	if !ok {
		t.Fatal("description field missing from create request")
	}
	raw, err := json.Marshal(desc)
	if err != nil {
		t.Fatalf("marshal description: %v", err)
	}
	// The code-marked text node must not also carry the strong mark, or Jira
	// rejects the document. Assert the invalid combination is absent.
	if strings.Contains(string(raw), `{"type":"code"},{"type":"strong"}`) ||
		strings.Contains(string(raw), `{"type":"strong"},{"type":"code"}`) {
		t.Fatalf("description ADF still nests code inside strong: %s", raw)
	}
	if !strings.Contains(string(raw), `"type":"code"`) {
		t.Fatalf("description ADF lost the code mark entirely: %s", raw)
	}
}

func TestIssueEditSendsOnlyProvidedFieldsAndRawUpdate(t *testing.T) {
	client := &fakeClient{editResult: IssueMutationResult{OK: true, Key: "DEX-9"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueEdit, map[string]any{
		"key":                  "DEX-9",
		"description_markdown": "See `worker.go`.",
		"labels":               []string{"edited"},
		"fields":               map[string]any{"customfield_10042": "new"},
		"update":               map[string]any{"comment": []any{map[string]any{"add": map[string]any{"body": "raw"}}}},
	})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("edit output = %#v", out)
	}
	if client.editKey != "DEX-9" {
		t.Fatalf("edit key = %q", client.editKey)
	}
	body := requestJSON(client.editRequest)
	for _, want := range []string{
		`"customfield_10042":"new"`,
		`"description":{"content"`,
		`"type":"code"`,
		`"labels":["edited"]`,
		`"update":{"comment"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("edit request = %s, missing %s", body, want)
		}
	}
	if strings.Contains(body, `"summary"`) {
		t.Fatalf("edit request should not include omitted summary: %s", body)
	}
}

func TestIssueEditRequiresAChange(t *testing.T) {
	plugin := testPlugin(&fakeClient{})
	err := plugintest.RunError(t, plugin, OperationIssueEdit, map[string]any{"key": "DEX-9"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestCommentsConvertMarkdownAndDeleteByID(t *testing.T) {
	client := &fakeClient{
		commentResult:       CommentResult{OK: true, IssueKey: "DEX-9", Comment: Comment{ID: "10042"}},
		commentDeleteResult: CommentMutationResult{OK: true, IssueKey: "DEX-9", CommentID: "10042"},
	}
	plugin := testPlugin(client)

	add := plugintest.RunOK[CommentResult](t, plugin, OperationCommentAdd, map[string]any{
		"key":           "DEX-9",
		"body_markdown": "## Added\n\n- **comment**",
	})
	if !add.OK || add.Comment.ID != "10042" || client.commentKey != "DEX-9" {
		t.Fatalf("add output = %#v key=%q", add, client.commentKey)
	}
	addBody := requestJSON(client.commentRequest)
	for _, want := range []string{`"body":{"content"`, `"type":"heading"`, `"type":"strong"`, `"type":"bulletList"`} {
		if !strings.Contains(addBody, want) {
			t.Fatalf("comment add request = %s, missing %s", addBody, want)
		}
	}

	edit := plugintest.RunOK[CommentResult](t, plugin, OperationCommentEdit, map[string]any{
		"key":           "DEX-9",
		"comment_id":    "10042",
		"body_markdown": "Edited with `code`.",
	})
	if !edit.OK || client.commentID != "10042" {
		t.Fatalf("edit output = %#v comment_id=%q", edit, client.commentID)
	}
	editBody := requestJSON(client.commentRequest)
	if !strings.Contains(editBody, `"type":"code"`) {
		t.Fatalf("comment edit request = %s", editBody)
	}

	deleted := plugintest.RunOK[CommentMutationResult](t, plugin, OperationCommentDelete, map[string]any{
		"key":        "DEX-9",
		"comment_id": "10042",
	})
	if !deleted.OK || deleted.CommentID != "10042" || client.deletedCommentID != "10042" {
		t.Fatalf("delete output = %#v deleted=%q", deleted, client.deletedCommentID)
	}
}

func TestCommentListRendersMarkdownAndPaginates(t *testing.T) {
	commentADF := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello "},{"type":"text","text":"world","marks":[{"type":"strong"}]}]}]}`)
	client := &fakeClient{commentList: CommentListResult{
		Count:    1,
		Total:    3,
		StartAt:  0,
		Comments: []Comment{{ID: "1", rawBody: commentADF}},
	}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[CommentListResult](t, plugin, OperationCommentList, map[string]any{
		"key":      "DEX-9",
		"limit":    2,
		"start_at": 0,
		"order":    "-created",
	})
	if out.IssueKey != "DEX-9" || out.Total != 3 || len(out.Comments) != 1 {
		t.Fatalf("list output = %#v", out)
	}
	if out.NextStartAt != 1 {
		t.Fatalf("next_start_at = %d, want 1", out.NextStartAt)
	}
	if out.Comments[0].Body != "Hello **world**" {
		t.Fatalf("rendered body = %q", out.Comments[0].Body)
	}
	if len(out.Comments[0].BodyADF) != 0 {
		t.Fatalf("default body_format should omit raw ADF: %s", out.Comments[0].BodyADF)
	}
	if client.commentKey != "DEX-9" || client.commentListOptions.Limit != 2 || client.commentListOptions.Order != "-created" {
		t.Fatalf("list options = %q %#v", client.commentKey, client.commentListOptions)
	}
}

func TestCommentListBodyFormatADFAndBoth(t *testing.T) {
	commentADF := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"raw"}]}]}`)
	client := &fakeClient{commentList: CommentListResult{Count: 1, Total: 1, Comments: []Comment{{ID: "1", rawBody: commentADF}}}}
	plugin := testPlugin(client)

	adf := plugintest.RunOK[CommentListResult](t, plugin, OperationCommentList, map[string]any{"key": "DEX-9", "body_format": "adf"})
	if adf.Comments[0].Body != "" || len(adf.Comments[0].BodyADF) == 0 {
		t.Fatalf("adf format = %#v", adf.Comments[0])
	}

	both := plugintest.RunOK[CommentListResult](t, plugin, OperationCommentList, map[string]any{"key": "DEX-9", "body_format": "both"})
	if both.Comments[0].Body != "raw" || len(both.Comments[0].BodyADF) == 0 {
		t.Fatalf("both format = %#v", both.Comments[0])
	}
}

func TestCommentListRejectsInvalidOrder(t *testing.T) {
	plugin := testPlugin(&fakeClient{})
	err := plugintest.RunError(t, plugin, OperationCommentList, map[string]any{"key": "DEX-9", "order": "priority"})
	if err == nil || err.Code != "bad_input" {
		t.Fatalf("error = %#v", err)
	}
}

func TestIssueShowRendersDescriptionMarkdown(t *testing.T) {
	descADF := json.RawMessage(`{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Goal"}]},{"type":"paragraph","content":[{"type":"text","text":"ship it"}]}]}`)
	client := &fakeClient{issue: Issue{Key: "DEX-7", Fields: IssueFields{Summary: "S", rawDescription: descADF}}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[pluginbinding.ShowResult[Issue]](t, plugin, OperationIssueShow, map[string]any{"key": "DEX-7"})
	if out.Record.Fields.Description != "## Goal\n\nship it" {
		t.Fatalf("rendered description = %q", out.Record.Fields.Description)
	}
	if len(out.Record.Fields.DescriptionADF) != 0 {
		t.Fatalf("default body_format should omit description ADF: %s", out.Record.Fields.DescriptionADF)
	}

	adf := plugintest.RunOK[pluginbinding.ShowResult[Issue]](t, plugin, OperationIssueShow, map[string]any{"key": "DEX-7", "body_format": "adf"})
	if adf.Record.Fields.Description != "" || len(adf.Record.Fields.DescriptionADF) == 0 {
		t.Fatalf("adf format = %#v", adf.Record.Fields)
	}
}

func TestIssueDeletePassesDeleteSubtasks(t *testing.T) {
	client := &fakeClient{deleteResult: IssueMutationResult{OK: true, Key: "DEX-9"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueDelete, map[string]any{"key": "DEX-9", "delete_subtasks": true})
	if !out.OK || out.Key != "DEX-9" {
		t.Fatalf("delete output = %#v", out)
	}
	if client.deleteKey != "DEX-9" || !client.deleteSubtasks {
		t.Fatalf("delete key/subtasks = %q/%v", client.deleteKey, client.deleteSubtasks)
	}
}

func TestAttachmentOperations(t *testing.T) {
	client := &fakeClient{
		issue:                  Issue{Key: "DEX-9", Fields: IssueFields{Attachments: []Attachment{{ID: "A1", Filename: "chart.png", MimeType: "image/png", Content: "https://api/attachment/content/A1"}}}},
		attachmentUploadResult: AttachmentUploadResult{OK: true, IssueKey: "DEX-9", Attachments: []Attachment{{ID: "A1", Filename: "chart.png"}}},
		attachmentGetResult:    AttachmentGetResult{ID: "A1", Filename: "chart.png", MimeType: "image/png", ContentBytes: []byte("png bytes"), Size: 9},
		attachmentDeleteResult: AttachmentDeleteResult{OK: true, AttachmentID: "A1"},
	}
	plugin := testPlugin(client)

	add := plugintest.RunOK[AttachmentUploadResult](t, plugin, OperationAttachmentAdd, map[string]any{"key": "DEX-9", "filename": "chart.png", "content_bytes": []byte("png bytes")})
	if !add.OK || client.attachmentKey != "DEX-9" || client.attachmentRequest.Filename != "chart.png" {
		t.Fatalf("add = %#v key=%q request=%#v", add, client.attachmentKey, client.attachmentRequest)
	}
	list := plugintest.RunOK[AttachmentListResult](t, plugin, OperationAttachmentList, map[string]any{"key": "DEX-9"})
	if list.Count != 1 || list.Attachments[0].ID != "A1" {
		t.Fatalf("list = %#v", list)
	}
	get := plugintest.RunOK[AttachmentGetResult](t, plugin, OperationAttachmentGet, map[string]any{"attachment_id": "A1", "filename": "chart.png"})
	if get.ID != "A1" || string(get.ContentBytes) != "png bytes" {
		t.Fatalf("get = %#v", get)
	}
	deleted := plugintest.RunOK[AttachmentDeleteResult](t, plugin, OperationAttachmentDelete, map[string]any{"attachment_id": "A1"})
	if !deleted.OK || client.deletedAttachmentID != "A1" {
		t.Fatalf("delete = %#v deleted=%q", deleted, client.deletedAttachmentID)
	}
}

func TestTransitionOperations(t *testing.T) {
	client := &fakeClient{
		transitions: []IssueTransitionListResult{{
			IssueKey:      "DEX-9",
			CurrentStatus: NamedValue{ID: "1", Name: "Backlog"},
			Transitions: []IssueTransition{
				{ID: "11", Name: "Start progress", To: NamedValue{ID: "3", Name: "In Progress"}},
				{ID: "21", Name: "Done", To: NamedValue{ID: "100", Name: "Done"}},
			},
		}},
		transitionResults: []IssueMutationResult{{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "100", Name: "Done"}}}}},
	}
	plugin := testPlugin(client)

	list := plugintest.RunOK[IssueTransitionListResult](t, plugin, OperationTransitionList, map[string]any{"key": "DEX-9"})
	if list.CurrentStatus.Name != "Backlog" || len(list.Transitions) != 2 {
		t.Fatalf("list = %#v", list)
	}
	run := plugintest.RunOK[IssueTransitionRunResult](t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done"})
	if !run.OK || run.Steps != 1 || client.transitionRequests[0].TransitionID != "21" {
		t.Fatalf("run = %#v requests=%#v", run, client.transitionRequests)
	}
}

func TestTransitionRunAutoTransition(t *testing.T) {
	client := &fakeClient{
		transitions: []IssueTransitionListResult{
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "1", Name: "Backlog"},
				Transitions:   []IssueTransition{{ID: "11", Name: "Start progress", To: NamedValue{ID: "3", Name: "In Progress"}}},
			},
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "3", Name: "In Progress"},
				Transitions:   []IssueTransition{{ID: "21", Name: "Done", To: NamedValue{ID: "100", Name: "Done"}}},
			},
			{
				IssueKey:      "DEX-9",
				CurrentStatus: NamedValue{ID: "100", Name: "Done"},
				Transitions:   nil,
			},
		},
		transitionResults: []IssueMutationResult{
			{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "3", Name: "In Progress"}}}},
			{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{ID: "100", Name: "Done"}}}},
		},
	}
	plugin := testPlugin(client)

	run := plugintest.RunOK[IssueTransitionRunResult](t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done", "auto_transition": true})
	if !run.OK || run.Steps != 2 {
		t.Fatalf("run = %#v", run)
	}
	if got := []string{client.transitionRequests[0].TransitionID, client.transitionRequests[1].TransitionID}; got[0] != "11" || got[1] != "21" {
		t.Fatalf("transition requests = %#v", got)
	}
}

func TestUploadMarkdownBlobImagesRewritesBlobReferences(t *testing.T) {
	host := &jiraTestHost{blobs: map[string]pluginbinding.BlobReadResponse{
		"blob://diagram": {
			Blob:    pluginbinding.BlobRef{Ref: "blob://diagram", Filename: "diagram.png", MediaType: "image/png"},
			Content: []byte("png-bytes"),
		},
	}}
	ctx := pluginbinding.Context{Host: host}
	client := &fakeClient{
		attachmentUploadResult: AttachmentUploadResult{OK: true, IssueKey: "DEX-9", Attachments: []Attachment{{ID: "A1", Filename: "diagram.png", Content: "https://api.atlassian.com/ex/jira/cloud-1/rest/api/3/attachment/content/A1"}}},
	}

	markdown := `See ![architecture](blob://diagram) and ![remote](https://example.com/img.png) and titled ![alt](blob://diagram "title text")`
	rewritten, err := uploadMarkdownBlobImages(ctx, client, "DEX-9", markdown)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(rewritten, "blob://diagram") {
		t.Fatalf("rewritten still references blob: %s", rewritten)
	}
	if !strings.Contains(rewritten, "rest/api/3/attachment/content/A1") {
		t.Fatalf("rewritten missing uploaded URL: %s", rewritten)
	}
	if !strings.Contains(rewritten, "https://example.com/img.png") {
		t.Fatalf("remote URL was disturbed: %s", rewritten)
	}
	if client.attachmentRequest.Filename != "diagram.png" {
		t.Fatalf("upload request filename = %q", client.attachmentRequest.Filename)
	}
	if string(client.attachmentRequest.Data) != "png-bytes" {
		t.Fatalf("upload request data = %q", string(client.attachmentRequest.Data))
	}
}

func TestUploadMarkdownBlobImagesNoOpsWhenNoImages(t *testing.T) {
	client := &fakeClient{}
	out, err := uploadMarkdownBlobImages(pluginbinding.Context{Host: &jiraTestHost{}}, client, "DEX-9", "just text, no images")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != "just text, no images" {
		t.Fatalf("out = %q", out)
	}
}

func testPlugin(client Client) *pluginbinding.Plugin {
	return NewPluginWithService(Service{ClientFactory: func(pluginbinding.Context, string) (Client, error) { return client, nil }})
}

type jiraTestHost struct {
	pluginbinding.HostClient
	blobs map[string]pluginbinding.BlobReadResponse
}

func (h *jiraTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h *jiraTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h *jiraTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h *jiraTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h *jiraTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{}, nil
}

func (h *jiraTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h *jiraTestHost) BlobRead(input pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return h.blobs[strings.TrimSpace(input.Ref)], nil
}

func (h *jiraTestHost) BlobWrite(input pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{Ref: firstNonEmpty(input.Ref, "blob://written"), Filename: input.Filename, MediaType: input.MediaType, Size: int64(len(input.Content))}, nil
}

func (h *jiraTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h *jiraTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

func (h *jiraTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

var _ pluginbinding.HostClient = (*jiraTestHost)(nil)

type fakeClient struct {
	user                   User
	issues                 []Issue
	issue                  Issue
	siteURL                string
	users                  []User
	createResult           IssueMutationResult
	editResult             IssueMutationResult
	deleteResult           IssueMutationResult
	commentResult          CommentResult
	commentDeleteResult    CommentMutationResult
	attachmentUploadResult AttachmentUploadResult
	attachmentGetResult    AttachmentGetResult
	attachmentDeleteResult AttachmentDeleteResult
	metaResult             IssueMetaResult
	transitions            []IssueTransitionListResult
	transitionResults      []IssueMutationResult
	issueOptions           IssueSearchOptions
	userOptions            UserSearchOptions
	issueKey               string
	editKey                string
	deleteKey              string
	deleteSubtasks         bool
	commentKey             string
	commentID              string
	deletedCommentID       string
	attachmentKey          string
	attachmentRequest      AttachmentUploadRequest
	deletedAttachmentID    string
	createMeta             IssueCreateMetaOptions
	editMetaKey            string
	createRequest          IssueCreateRequest
	editRequest            IssueEditRequest
	transitionRequests     []IssueTransitionRequest
	commentRequest         CommentRequest
	commentList            CommentListResult
	commentListOptions     CommentListOptions
	linkRequests           []IssueLinkRequest
	linkErr                error
	linkTypes              []IssueLinkType
}

func (c *fakeClient) CurrentUser(context.Context) (User, error) {
	return c.user, nil
}

func (c *fakeClient) SearchIssues(_ context.Context, options IssueSearchOptions) ([]Issue, error) {
	c.issueOptions = options
	return c.issues, nil
}

func (c *fakeClient) GetIssue(_ context.Context, key string) (Issue, error) {
	c.issueKey = key
	return c.issue, nil
}

func (c *fakeClient) LinkIssues(_ context.Context, request IssueLinkRequest) error {
	c.linkRequests = append(c.linkRequests, request)
	return c.linkErr
}

func (c *fakeClient) ListIssueLinkTypes(context.Context) ([]IssueLinkType, error) {
	return c.linkTypes, nil
}

func (c *fakeClient) CreateIssue(_ context.Context, request IssueCreateRequest) (IssueMutationResult, error) {
	c.createRequest = request
	return c.createResult, nil
}

func (c *fakeClient) EditIssue(_ context.Context, key string, request IssueEditRequest) (IssueMutationResult, error) {
	c.editKey = key
	c.editRequest = request
	return c.editResult, nil
}

func (c *fakeClient) DeleteIssue(_ context.Context, key string, deleteSubtasks bool) (IssueMutationResult, error) {
	c.deleteKey = key
	c.deleteSubtasks = deleteSubtasks
	return c.deleteResult, nil
}

func (c *fakeClient) ListTransitions(_ context.Context, key string) (IssueTransitionListResult, error) {
	c.issueKey = key
	if len(c.transitions) == 0 {
		return IssueTransitionListResult{IssueKey: key}, nil
	}
	out := c.transitions[0]
	if len(c.transitions) > 1 {
		c.transitions = c.transitions[1:]
	}
	return out, nil
}

func (c *fakeClient) TransitionIssue(_ context.Context, key string, request IssueTransitionRequest) (IssueMutationResult, error) {
	c.editKey = key
	c.transitionRequests = append(c.transitionRequests, request)
	if len(c.transitionResults) == 0 {
		return IssueMutationResult{OK: true, Key: key}, nil
	}
	out := c.transitionResults[0]
	if len(c.transitionResults) > 1 {
		c.transitionResults = c.transitionResults[1:]
	}
	return out, nil
}

func (c *fakeClient) AddComment(_ context.Context, key string, request CommentRequest) (CommentResult, error) {
	c.commentKey = key
	c.commentRequest = request
	return c.commentResult, nil
}

func (c *fakeClient) EditComment(_ context.Context, key, commentID string, request CommentRequest) (CommentResult, error) {
	c.commentKey = key
	c.commentID = commentID
	c.commentRequest = request
	return c.commentResult, nil
}

func (c *fakeClient) DeleteComment(_ context.Context, key, commentID string) (CommentMutationResult, error) {
	c.commentKey = key
	c.deletedCommentID = commentID
	return c.commentDeleteResult, nil
}

func (c *fakeClient) ListComments(_ context.Context, key string, opts CommentListOptions) (CommentListResult, error) {
	c.commentKey = key
	c.commentListOptions = opts
	result := c.commentList
	result.IssueKey = key
	if next := result.StartAt + len(result.Comments); next < result.Total {
		result.NextStartAt = next
	}
	return result, nil
}

func (c *fakeClient) UploadIssueAttachment(_ context.Context, key string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	c.attachmentKey = key
	c.attachmentRequest = request
	return c.attachmentUploadResult, nil
}

func (c *fakeClient) GetAttachment(_ context.Context, attachment Attachment) (AttachmentGetResult, error) {
	c.deletedAttachmentID = attachment.ID
	return c.attachmentGetResult, nil
}

func (c *fakeClient) DeleteAttachment(_ context.Context, id string) (AttachmentDeleteResult, error) {
	c.deletedAttachmentID = id
	return c.attachmentDeleteResult, nil
}

func (c *fakeClient) CreateMeta(_ context.Context, options IssueCreateMetaOptions) (IssueMetaResult, error) {
	c.createMeta = options
	return c.metaResult, nil
}

func (c *fakeClient) EditMeta(_ context.Context, key string) (IssueMetaResult, error) {
	c.editMetaKey = key
	return c.metaResult, nil
}

func (c *fakeClient) SearchUsers(_ context.Context, options UserSearchOptions) ([]User, error) {
	c.userOptions = options
	return c.users, nil
}

func (c *fakeClient) AccessibleSiteURL(context.Context) (string, error) {
	if c.siteURL != "" {
		return c.siteURL, nil
	}
	return "", fmt.Errorf("no accessible resource with a site url")
}

func (c *fakeClient) GetUser(_ context.Context, accountID string) (User, error) {
	c.userOptions.Query = accountID
	if c.user.AccountID != "" {
		return c.user, nil
	}
	if len(c.users) > 0 {
		return c.users[0], nil
	}
	return User{}, nil
}

func TestIssueLinkUnmarshalFlattensJiraWire(t *testing.T) {
	wire := `{
		"id": "10500",
		"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
		"outwardIssue": {"key": "DEX-2", "fields": {"summary": "Downstream", "status": {"name": "To Do"}}}
	}`
	var link IssueLink
	if err := json.Unmarshal([]byte(wire), &link); err != nil {
		t.Fatalf("unmarshal wire: %v", err)
	}
	if link.Type != "Blocks" || link.Verb != "blocks" || link.OtherKey != "DEX-2" || link.OtherSummary != "Downstream" || link.OtherStatus != "To Do" {
		t.Fatalf("link = %#v", link)
	}
	// Inward side reads through the inward verb.
	inward := `{"type": {"name": "Blocks", "inward": "is blocked by", "outward": "blocks"}, "inwardIssue": {"key": "DEX-1"}}`
	var blocked IssueLink
	if err := json.Unmarshal([]byte(inward), &blocked); err != nil {
		t.Fatalf("unmarshal inward: %v", err)
	}
	if blocked.Verb != "is blocked by" || blocked.OtherKey != "DEX-1" {
		t.Fatalf("inward link = %#v", blocked)
	}
	// The flattened output round-trips through its own shape.
	flattened, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}
	var again IssueLink
	if err := json.Unmarshal(flattened, &again); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if again != link {
		t.Fatalf("round-trip mismatch: %#v != %#v", again, link)
	}
}

func TestIssueLinkAddVerifiesReadBack(t *testing.T) {
	client := &fakeClient{issue: Issue{Key: "DEX-1", Fields: IssueFields{
		IssueLinks: []IssueLink{{Type: "Blocks", Verb: "blocks", OtherKey: "DEX-2"}},
	}}}
	plugin := testPlugin(client)
	out := plugintest.RunOK[IssueLinkAddResult](t, plugin, OperationIssueLinkAdd, map[string]any{"key": "DEX-1", "to_key": "DEX-2", "type": "Blocks"})
	if !out.OK || len(out.Links) != 1 || out.Links[0].OtherKey != "DEX-2" {
		t.Fatalf("out = %#v", out)
	}
	if len(client.linkRequests) != 1 || client.linkRequests[0].OutwardKey != "DEX-1" || client.linkRequests[0].InwardKey != "DEX-2" {
		t.Fatalf("link requests = %#v", client.linkRequests)
	}
}

func TestIssueLinkAddFailsWhenLinkNotVisible(t *testing.T) {
	// Jira accepted the POST but the read-back shows no link: the silent
	// write failure must surface as an error, not ok:true.
	client := &fakeClient{issue: Issue{Key: "DEX-1"}}
	plugin := testPlugin(client)
	perr := plugintest.RunError(t, plugin, OperationIssueLinkAdd, map[string]any{"key": "DEX-1", "to_key": "DEX-2", "type": "Blocks"})
	if !strings.Contains(perr.Message, "no link to DEX-2 is visible") {
		t.Fatalf("error = %#v", perr)
	}
}

func TestIssueLinkAddUnknownTypeListsAvailableTypes(t *testing.T) {
	client := &fakeClient{
		linkErr:   fmt.Errorf("No issue link type with name 'Blcks' found"),
		linkTypes: []IssueLinkType{{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}},
	}
	plugin := testPlugin(client)
	perr := plugintest.RunError(t, plugin, OperationIssueLinkAdd, map[string]any{"key": "DEX-1", "to_key": "DEX-2", "type": "Blcks"})
	if len(perr.Details) == 0 || !strings.Contains(perr.Details[0], "Blocks") {
		t.Fatalf("error details = %#v", perr)
	}
}

func TestCommentAddSurfacesCommentID(t *testing.T) {
	client := &fakeClient{commentResult: CommentResult{OK: true, Comment: Comment{ID: "10042", Body: "done"}}}
	plugin := testPlugin(client)
	out := plugintest.RunOK[CommentResult](t, plugin, OperationCommentAdd, map[string]any{"key": "DEX-1", "body_markdown": "done"})
	if out.CommentID != "10042" {
		t.Fatalf("comment_id = %q (out = %#v)", out.CommentID, out)
	}
}

func TestTransitionRunFailureDisclosesAppliedTransitions(t *testing.T) {
	// One transition applies, then the walker dead-ends: the error must say
	// the issue was mutated and where it stands.
	client := &fakeClient{
		transitions: []IssueTransitionListResult{
			{IssueKey: "DEX-9", CurrentStatus: NamedValue{Name: "Backlog"}, Transitions: []IssueTransition{{ID: "11", Name: "Start progress", To: NamedValue{Name: "In Progress"}}}},
			{IssueKey: "DEX-9", CurrentStatus: NamedValue{Name: "In Progress"}, Transitions: nil},
		},
		transitionResults: []IssueMutationResult{
			{OK: true, Key: "DEX-9", Issue: &Issue{Key: "DEX-9", Fields: IssueFields{Status: NamedValue{Name: "In Progress"}}}},
		},
	}
	plugin := testPlugin(client)
	perr := plugintest.RunError(t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done", "auto_transition": true})
	if len(perr.Details) < 2 || !strings.Contains(perr.Details[0], "WAS mutated") || !strings.Contains(perr.Details[0], "Start progress") {
		t.Fatalf("error details = %#v", perr)
	}
	if !strings.Contains(perr.Details[1], `"In Progress"`) || !strings.Contains(perr.Details[1], `"Backlog"`) {
		t.Fatalf("status detail = %#v", perr.Details)
	}
}

func TestIssueShowAcceptsIssueKeyAlias(t *testing.T) {
	client := &fakeClient{issue: Issue{Key: "DEX-1", Fields: IssueFields{Summary: "Hello"}}}
	plugin := testPlugin(client)
	out := plugintest.RunOK[pluginbinding.ShowResult[Issue]](t, plugin, OperationIssueShow, map[string]any{"issue_key": "DEX-1"})
	if out.Record.Key != "DEX-1" || client.issueKey != "DEX-1" {
		t.Fatalf("out = %#v issueKey = %q", out, client.issueKey)
	}
}

func TestIssueCreateAcceptsProjectAlias(t *testing.T) {
	client := &fakeClient{createResult: IssueMutationResult{OK: true, Key: "DEX-10"}}
	plugin := testPlugin(client)
	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueCreate, map[string]any{"project": "DEX", "issue_type": "Task", "summary": "From alias"})
	if !out.OK {
		t.Fatalf("out = %#v", out)
	}
	project, _ := client.createRequest.Fields["project"].(map[string]string)
	if project["key"] != "DEX" {
		t.Fatalf("create request project = %#v", client.createRequest.Fields["project"])
	}
}

func TestIssueEditWarnsWhenRawIssueLinksVanish(t *testing.T) {
	client := &fakeClient{editResult: IssueMutationResult{OK: true, Key: "DEX-1", Issue: &Issue{Key: "DEX-1"}}}
	plugin := testPlugin(client)
	out := plugintest.RunOK[IssueMutationResult](t, plugin, OperationIssueEdit, map[string]any{
		"key":    "DEX-1",
		"update": map[string]any{"issuelinks": []map[string]any{{"add": map[string]any{"type": map[string]any{"name": "Blocks"}}}}},
	})
	if !strings.Contains(out.Warning, "issuelinks") || !strings.Contains(out.Warning, "jira.issue.link.add") {
		t.Fatalf("warning = %q", out.Warning)
	}
}

func TestTransitionRunResultCollectionsAreNeverNull(t *testing.T) {
	client := &fakeClient{
		transitions: []IssueTransitionListResult{
			{IssueKey: "DEX-9", CurrentStatus: NamedValue{Name: "Done"}, Transitions: nil},
		},
	}
	plugin := testPlugin(client)
	run := plugintest.RunOK[IssueTransitionRunResult](t, plugin, OperationTransitionRun, map[string]any{"key": "DEX-9", "target_status": "Done"})
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"applied_transitions":null`) || !strings.Contains(string(raw), `"applied_transitions":[`) {
		t.Fatalf("applied_transitions must serialize as []: %s", raw)
	}
}
