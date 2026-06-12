package confluence

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
// the confluence plugin rather than promoted to the SDK.
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
	PluginName        = "confluence"
	PluginVersion     = "0.20.0"
	PluginDescription = "Confluence Cloud page/user operations, comments, attachments, datasources, indexes, and reverse lookups."

	AuthMethodAtlassianCloud = "atlassian_cloud_basic"
	AuthPurposeAPIToken      = "api_token"
	AuthPurposeCloudID       = "cloud_id"

	EnvAtlassianAPIToken  = "ATLASSIAN_API_TOKEN"
	EnvConfluenceAPIToken = "CONFLUENCE_API_TOKEN"
	EnvAtlassianURL       = "ATLASSIAN_URL"
	EnvAtlassianSiteURL   = "ATLASSIAN_SITE_URL"
	EnvConfluenceURL      = "CONFLUENCE_URL"

	OperationTest             = "confluence.test"
	OperationIndexBuild       = "confluence.index.build"
	OperationAttachmentAdd    = "confluence.page.attachment.add"
	OperationAttachmentList   = "confluence.page.attachment.list"
	OperationAttachmentGet    = "confluence.attachment.get"
	OperationAttachmentDelete = "confluence.attachment.delete"
	OperationPageCreate       = "confluence.page.create"
	OperationPageUpdate       = "confluence.page.update"
	OperationPageDelete       = "confluence.page.delete"
	OperationPageSearch       = "confluence.page.search"
	OperationPageList         = "confluence.page.list"
	OperationPageShow         = "confluence.page.show"
	OperationCommentList      = "confluence.page.comment.list"
	OperationCommentAdd       = "confluence.page.comment.add"
	OperationUserSearch       = "confluence.user.search"

	DatasourcePages = "confluence.pages"
	DatasourceUsers = "confluence.users"

	EntityPage = "confluence.page"
	EntityUser = "confluence.user"

	EndpointName = "confluence.endpoint"
)

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{"conf", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodAtlassianCloud,
			Kind:        "bearer_token",
			Description: "Atlassian Cloud API token resolved by the host for endpoint-ref HTTP calls.",
			Env:         []string{EnvConfluenceAPIToken, EnvAtlassianAPIToken},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeAPIToken, "Atlassian API token", true, true, EnvConfluenceAPIToken, EnvAtlassianAPIToken),
				pluginbinding.AuthField(AuthPurposeCloudID, "Atlassian Cloud ID", false, true, "ATLASSIAN_CLOUD_ID", "CONFLUENCE_CLOUD_ID"),
			},
		}},
		Endpoints: []core.EndpointSpec{{
			Name:        EndpointName,
			Description: "Configured Confluence API endpoint.",
			Products:    []string{PluginName, "atlassian"},
			Env:         []string{EnvConfluenceURL, EnvAtlassianURL, EnvAtlassianSiteURL},
		}},
		Operations: operationSpecs(),
		IndexedDatasources: []pluginbinding.IndexedDatasourceSpec{
			pluginbinding.IndexedDatasourceWithOptions(DatasourcePages, EntityPage, "Search Confluence pages.", "Confluence page reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
				pluginbinding.EntitySchemaFor[PageRecord](),
				pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "page_id", TitleField: "title"}),
				pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
				pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
			),
			pluginbinding.IndexedDatasourceWithOptions(DatasourceUsers, EntityUser, "Search Confluence users.", "Confluence user reverse lookup index.", pluginbinding.SearchableIndexCapabilities(),
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
		attachmentAddSpec(),
		attachmentListSpec(),
		attachmentGetSpec(),
		attachmentDeleteSpec(),
		pageCreateSpec(),
		pageUpdateSpec(),
		pageDeleteSpec(),
		pageSearchSpec(),
		pageListSpec(),
		pageShowSpec(),
		commentListSpec(),
		commentAddSpec(),
		userSearchSpec(),
	}
}

func authTestSpec() core.OperationSpec {
	return confluenceReadOperation[AuthTestInput, AuthTestResult](OperationTest, "Test Confluence authentication by fetching the current user.")
}

func indexBuildSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[IndexBuildInput, pluginbinding.IndexBuildResult](OperationIndexBuild, "Build Confluence page and user index records.", confluenceReadOptions(core.OperationConditional)...)
}

func attachmentAddSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentAddInput, AttachmentUploadResult](OperationAttachmentAdd, "Upload an attachment to a Confluence page.", confluenceFilesystemWriteOptions(core.OperationNonIdempotent)...)
}

func attachmentListSpec() core.OperationSpec {
	return confluenceReadOperation[AttachmentListInput, AttachmentListResult](OperationAttachmentList, "List Confluence page attachments.")
}

func attachmentGetSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentGetInput, AttachmentGetResult](OperationAttachmentGet, "Download or return a Confluence attachment.", confluenceFilesystemReadOptions(core.OperationIdempotent)...)
}

func attachmentDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[AttachmentDeleteInput, AttachmentDeleteResult](OperationAttachmentDelete, "Delete a Confluence attachment.", confluenceDeleteOptions(core.OperationNonIdempotent)...)
}

func pageCreateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PageCreateInput, PageMutationResult](OperationPageCreate, "Create a Confluence page from Markdown (body_markdown, preferred) or storage-format XHTML (body_storage).", confluenceWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"space_key": "DEV", "title": "Release notes", "body_markdown": "## Summary\n\n- shipped **v2**\n- fixed `worker.go`"},
	)
}

func pageUpdateSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[PageUpdateInput, PageMutationResult](OperationPageUpdate, "Update a Confluence page's title and/or body (Markdown preferred), incrementing the page version. The body replaces the whole page content.", confluenceWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"page_id": "123456", "body_markdown": "## Updated\n\nNew **content**."},
	)
}

func pageDeleteSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[PageDeleteInput, PageMutationResult](OperationPageDelete, "Delete a Confluence page.", confluenceDeleteOptions(core.OperationNonIdempotent)...)
}

func pageSearchSpec() core.OperationSpec {
	return confluenceCompactReadOperation[PageSearchInput, PageSearchResult](OperationPageSearch, "Search Confluence pages.")
}

func pageListSpec() core.OperationSpec {
	return confluenceCompactReadOperation[PageListInput, pluginbinding.ListResult[Page]](OperationPageList, "List Confluence pages, filterable by space and title, with offset pagination via next_page_token.")
}

func pageShowSpec() core.OperationSpec {
	return confluenceReadOperation[PageShowInput, pluginbinding.ShowResult[Page]](OperationPageShow, "Show one Confluence page with its body as Markdown (body_format selects markdown/storage/both).")
}

func commentListSpec() core.OperationSpec {
	return confluenceReadOperation[CommentListInput, CommentListResult](OperationCommentList, "List comments on a Confluence page as Markdown, with raw storage XHTML available via body_format.")
}

func commentAddSpec() core.OperationSpec {
	return withInputExamples(
		pluginbinding.TypedOperationSpec[CommentAddInput, Comment](OperationCommentAdd, "Add a Markdown comment to a Confluence page.", confluenceWriteOptions(core.OperationNonIdempotent)...),
		map[string]any{"page_id": "123456", "body_markdown": "Reviewed — root cause is `worker.go`. See **notes**."},
	)
}

func userSearchSpec() core.OperationSpec {
	return confluenceCompactReadOperation[UserSearchInput, UserSearchResult](OperationUserSearch, "Search Confluence users.")
}

func confluenceReadOperation[I any, O any](name, description string) core.OperationSpec {
	return pluginbinding.TypedOperationSpec[I, O](name, description, confluenceReadOptions(core.OperationIdempotent)...)
}

func confluenceCompactReadOperation[I any, O any](name, description string) core.OperationSpec {
	options := append(confluenceReadOptions(core.OperationIdempotent), pluginbinding.Compact())
	return pluginbinding.TypedOperationSpec[I, O](name, description, options...)
}

func confluenceReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceFilesystemReadOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceFilesystemWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectWrite, core.OperationEffectNetwork, core.OperationEffectFilesystem),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork, core.OperationAccessFilesystem),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceWriteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskMedium),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluenceDeleteOptions(idempotency core.OperationIdempotency) []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.SecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.Effects(core.OperationEffectWrite, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskDestructive),
		pluginbinding.Idempotency(idempotency),
	}
}

func confluencePagesDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[PageSearchInput, PageDatasourceResult](
		DatasourcePages,
		EntityPage,
		"Search Confluence pages.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[PageRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "page_id", TitleField: "title"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Confluence page fields.", "page_id", "title", "space_id", "space_key", "author_id"),
	)
}

func confluenceUsersDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[UserSearchInput, UserDatasourceResult](
		DatasourceUsers,
		EntityUser,
		"Search Confluence users.",
		[]string{pluginbinding.CapabilitySearch, pluginbinding.CapabilityLookup},
		pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...),
		pluginbinding.EntitySchemaFor[UserRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "account_id", TitleField: "display_name"}),
		pluginbinding.Fallback(core.DatasourceFallbackHostIndexFirst),
		pluginbinding.Completion("Confluence user fields.", "account_id", "display_name", "email"),
	)
}

func confluencePagesLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourcePages, EntityPage, "Lookup Confluence pages.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...))
}

func confluenceUsersLookupSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[LookupInput, LookupResult](DatasourceUsers, EntityUser, "Lookup Confluence users.", []string{pluginbinding.CapabilityLookup}, pluginbinding.DatasourceSecretPurposes(atlassianAuthPurposes()...))
}

func atlassianAuthPurposes() []string {
	return []string{AuthPurposeAPIToken, AuthPurposeCloudID}
}
