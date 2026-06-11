package jira

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe` and treats an example-bearing op as having
// conditional (one-of) input during local `--dry-run` validation. Kept local to
// the jira plugin — the only current consumer — rather than promoted to the SDK.
func withInputExamples(spec core.OperationSpec, examples ...map[string]any) core.OperationSpec {
	if len(examples) == 0 || len(spec.Input) == 0 {
		return spec
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Input, &schema); err != nil {
		return spec
	}
	arr := make([]any, 0, len(examples))
	for _, example := range examples {
		arr = append(arr, example)
	}
	schema["examples"] = arr
	if raw, err := json.Marshal(schema); err == nil {
		spec.Input = raw
	}
	return spec
}

const (
	PluginName        = "jira"
	PluginVersion     = "0.22.0"
	PluginDescription = "Jira Cloud issue operations, comments, attachments, transitions, datasources, indexes, and reverse lookups."

	AuthMethodAtlassianCloud = "atlassian_cloud_basic"
	AuthPurposeAPIToken      = "api_token"
	AuthPurposeCloudID       = "cloud_id"
	AuthPurposeSiteURL       = "site_url"

	EnvAtlassianAPIToken = "ATLASSIAN_API_TOKEN"
	EnvJiraAPIToken      = "JIRA_API_TOKEN"
	EnvAtlassianURL      = "ATLASSIAN_URL"
	EnvAtlassianSiteURL  = "ATLASSIAN_SITE_URL"
	EnvJiraURL           = "JIRA_URL"

	OperationAuthTest         = "jira.auth.test"
	OperationIndexBuild       = "jira.index.build"
	OperationCreateMeta       = "jira.issue.create_meta"
	OperationEditMeta         = "jira.issue.edit_meta"
	OperationTransitionList   = "jira.issue.transition.list"
	OperationTransitionRun    = "jira.issue.transition.run"
	OperationCommentAdd       = "jira.issue.comment.add"
	OperationCommentEdit      = "jira.issue.comment.edit"
	OperationCommentDelete    = "jira.issue.comment.delete"
	OperationCommentList      = "jira.issue.comment.list"
	OperationAttachmentAdd    = "jira.issue.attachment.add"
	OperationAttachmentList   = "jira.issue.attachment.list"
	OperationAttachmentGet    = "jira.issue.attachment.get"
	OperationAttachmentDelete = "jira.issue.attachment.delete"
	OperationIssueCreate      = "jira.issue.create"
	OperationIssueEdit        = "jira.issue.edit"
	OperationIssueDelete      = "jira.issue.delete"
	OperationIssueSearch      = "jira.issue.search"
	OperationIssueShow        = "jira.issue.show"
	OperationUserSearch       = "jira.user.search"

	DatasourceIssues = "jira.issues"
	DatasourceUsers  = "jira.users"

	EntityIssue = "jira.issue"
	EntityUser  = "jira.user"

	EndpointName = "jira.endpoint"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"jr", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodAtlassianCloud,
			Kind:        "bearer_token",
			Description: "Atlassian Cloud API token resolved by the host for endpoint-ref HTTP calls.",
			Env:         []string{EnvJiraAPIToken, EnvAtlassianAPIToken},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIToken, "Atlassian API token", true, true, EnvJiraAPIToken, EnvAtlassianAPIToken),
				pluginbinding.AuthField(AuthPurposeCloudID, "Atlassian Cloud ID", false, true, "ATLASSIAN_CLOUD_ID", "JIRA_CLOUD_ID"),
				pluginbinding.AuthField(AuthPurposeSiteURL, "Atlassian site URL (https://<site>.atlassian.net) used for human browse links on issue outputs", false, false, EnvAtlassianSiteURL, EnvJiraURL),
			},
		}},
		Endpoints: []core.EndpointSpec{{
			Name:        EndpointName,
			Description: "Configured Jira API endpoint.",
			Products:    []string{PluginName, "atlassian"},
			Env:         []string{EnvJiraURL, EnvAtlassianURL, EnvAtlassianSiteURL},
		}},
		Operations: operationSpecs(),
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasourceWithOptions(DatasourceIssues, EntityIssue, "Search Jira issues.", "Jira issue reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[IssueRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "key", TitleField: "summary"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
				pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceUsers, EntityUser, "Search Jira users.", "Jira user reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[UserRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "account_id", TitleField: "display_name"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
				pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
			),
		},
	}
}

func operationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		authTestSpec(),
		indexBuildSpec(),
		createMetaSpec(),
		editMetaSpec(),
		transitionListSpec(),
		transitionRunSpec(),
		commentAddSpec(),
		commentEditSpec(),
		commentDeleteSpec(),
		commentListSpec(),
		attachmentAddSpec(),
		attachmentListSpec(),
		attachmentGetSpec(),
		attachmentDeleteSpec(),
		issueCreateSpec(),
		issueEditSpec(),
		issueDeleteSpec(),
		issueSearchSpec(),
		issueShowSpec(),
		userSearchSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return jiraReadOperation[AuthTestInput, AuthTestResult](OperationAuthTest, "Test Jira authentication by fetching the current user.")
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](OperationIndexBuild, "Build Jira issue and user index records.", jiraReadOptions(core.OperationConditional)...)
}

func createMetaSpec() core.OperationSpec {
	return jiraReadOperation[IssueCreateMetaInput, IssueMetaResult](OperationCreateMeta, "Show Jira issue create metadata.")
}

func editMetaSpec() core.OperationSpec {
	return jiraReadOperation[IssueEditMetaInput, IssueMetaResult](OperationEditMeta, "Show Jira issue edit metadata.")
}

func transitionListSpec() core.OperationSpec {
	return jiraReadOperation[IssueTransitionListInput, IssueTransitionListResult](OperationTransitionList, "Show a Jira issue's current status and currently available transitions.")
}

func transitionRunSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[IssueTransitionRunInput, IssueTransitionRunResult](OperationTransitionRun, "Run a Jira issue transition. Provide exactly one of transition_id, transition_name, or target_status (these are flat top-level keys, not a nested transition object). Run jira.issue.transition.list first to see the available transition IDs and names. With auto_transition, walks intermediate transitions until target_status is reached.", jiraWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"key": "DEV-123", "target_status": "In Progress", "auto_transition": true},
	)
}

func commentAddSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CommentAddInput, CommentResult](OperationCommentAdd, "Add a Markdown comment to a Jira issue.", jiraFilesystemWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"key": "DEV-123", "body_markdown": "Investigated — root cause is `worker.go`. See **logs**."},
	)
}

func commentEditSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[CommentEditInput, CommentResult](OperationCommentEdit, "Edit a Jira issue comment with Markdown.", jiraFilesystemWriteOptions(core.OperationNonIdempotent)...)
}

func commentDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[CommentDeleteInput, CommentMutationResult](OperationCommentDelete, "Delete a Jira issue comment.", jiraWriteOptions(core.OperationNonIdempotent)...)
}

func commentListSpec() core.OperationSpec {
	return jiraReadOperation[CommentListInput, CommentListResult](OperationCommentList, "List comments on a Jira issue as Markdown, with raw ADF available via body_format.")
}

func attachmentAddSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[AttachmentAddInput, AttachmentUploadResult](OperationAttachmentAdd, "Upload an attachment to a Jira issue. Provide exactly one of blob_ref or content_bytes.", jiraFilesystemWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"key": "DEV-123", "blob_ref": "<host blob ref>", "filename": "report.pdf"},
	)
}

func attachmentListSpec() core.OperationSpec {
	return jiraReadOperation[AttachmentListInput, AttachmentListResult](OperationAttachmentList, "List Jira issue attachments.")
}

func attachmentGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentGetInput, AttachmentGetResult](OperationAttachmentGet, "Download or return a Jira attachment.", jiraFilesystemReadOptions(core.OperationIdempotent)...)
}

func attachmentDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentDeleteInput, AttachmentDeleteResult](OperationAttachmentDelete, "Delete a Jira issue attachment.", jiraDeleteOptions(core.OperationNonIdempotent)...)
}

func issueCreateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[IssueCreateInput, IssueMutationResult](OperationIssueCreate, "Create a Jira issue from structured fields and Markdown. Typed fields (summary, parent_key, assignee) are verified against the created issue and any that Jira silently dropped are reported in a warning.", jiraFilesystemWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"project_key": "DEV", "issue_type": "Task", "summary": "Investigate flaky transition test", "description_markdown": "Steps:\n\n1. Run the suite\n2. Observe the retry on `transition.run`", "labels": []string{"ai"}},
	)
}

func issueEditSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[IssueEditInput, IssueMutationResult](OperationIssueEdit, "Edit a Jira issue from structured fields and Markdown, including reparenting via parent_key. Typed fields (summary, parent_key, assignee) are verified against the updated issue and any that Jira silently dropped are reported in a warning.", jiraFilesystemWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"key": "DEV-123", "parent_key": "DEV-100", "labels": []string{"triaged"}},
	)
}

func issueDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IssueDeleteInput, IssueMutationResult](OperationIssueDelete, "Delete a Jira issue.", jiraDeleteOptions(core.OperationNonIdempotent)...)
}

func issueSearchSpec() core.OperationSpec {
	return jiraCompactReadOperation[IssueSearchInput, IssueSearchResult](OperationIssueSearch, "Search Jira issues.")
}

func issueShowSpec() core.OperationSpec {
	return jiraReadOperation[IssueShowInput, pluginbinding.ShowResult[Issue]](OperationIssueShow, "Show one Jira issue.")
}

func userSearchSpec() core.OperationSpec {
	return jiraCompactReadOperation[UserSearchInput, UserSearchResult](OperationUserSearch, "Search Jira users.")
}

func jiraReadOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, jiraReadOptions(core.OperationIdempotent)...)
}

func jiraCompactReadOperation[I any, O any](name, description string) core.OperationSpec {
	options := append(jiraReadOptions(core.OperationIdempotent), pluginbinding.Compact())
	return pluginbinding.TypedOperationSpec[I, O](name, description, options...)
}

func jiraReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func jiraWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func jiraFilesystemWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func jiraFilesystemReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func jiraDeleteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskDestructive),
		pluginbinding.Idempotency(idempotency),
	}
}

func jiraIssuesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[IssueSearchInput, IssueDatasourceResult](
		DatasourceIssues,
		EntityIssue,
		"Search Jira issues.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[IssueRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "key", TitleField: "summary"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Jira issue fields.", "key", "summary", "project_key", "status", "assignee_display_name", "reporter_display_name"),
	)
}

func jiraUsersDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[UserSearchInput, UserDatasourceResult](
		DatasourceUsers,
		EntityUser,
		"Search Jira users.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[UserRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "account_id", TitleField: "display_name"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Jira user fields.", "account_id", "display_name", "email"),
	)
}

func jiraIssuesLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](
		DatasourceIssues,
		EntityIssue,
		"Lookup Jira issues.",
		[]string{pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
	)
}

func jiraUsersLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](
		DatasourceUsers,
		EntityUser,
		"Lookup Jira users.",
		[]string{pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
	)
}

func atlassianAuthPurposes() []string {
	return []string{AuthPurposeAPIToken, AuthPurposeCloudID, AuthPurposeSiteURL}
}
