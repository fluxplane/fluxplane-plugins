package gitlab

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Release-management specs: the release lifecycle, changelog
// generation/commit, release asset links, and tag show/delete. Reads are low
// risk; creates/updates are medium-risk writes; deletes are destructive.

func registerReleaseOperations(service Service) []pluginbinding.PluginOption {
	return []pluginbinding.PluginOption{
		pluginbinding.RegisterOperation(releaseCreateSpec(), service.ReleaseCreate),
		pluginbinding.RegisterOperation(releaseShowSpec(), service.ReleaseShow),
		pluginbinding.RegisterOperation(releaseUpdateSpec(), service.ReleaseUpdate),
		pluginbinding.RegisterOperation(releaseDeleteSpec(), service.ReleaseDelete),
		pluginbinding.RegisterOperation(changelogGenerateSpec(), service.ChangelogGenerate),
		pluginbinding.RegisterOperation(changelogAddSpec(), service.ChangelogAdd),
		pluginbinding.RegisterOperation(tagShowSpec(), service.TagShow),
		pluginbinding.RegisterOperation(tagDeleteSpec(), service.TagDelete),
		pluginbinding.RegisterOperation(releaseLinkListSpec(), service.ReleaseLinkList),
		pluginbinding.RegisterOperation(releaseLinkCreateSpec(), service.ReleaseLinkCreate),
		pluginbinding.RegisterOperation(releaseLinkUpdateSpec(), service.ReleaseLinkUpdate),
		pluginbinding.RegisterOperation(releaseLinkDeleteSpec(), service.ReleaseLinkDelete),
	}
}

func releaseOperationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		releaseCreateSpec(),
		releaseShowSpec(),
		releaseUpdateSpec(),
		releaseDeleteSpec(),
		changelogGenerateSpec(),
		changelogAddSpec(),
		tagShowSpec(),
		tagDeleteSpec(),
		releaseLinkListSpec(),
		releaseLinkCreateSpec(),
		releaseLinkUpdateSpec(),
		releaseLinkDeleteSpec(),
	}
}

func releaseCreateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ReleaseCreateInput, ReleaseDetail](
		OperationReleaseCreate,
		"Create a GitLab release for a tag, cutting the tag from ref when it does not yet exist — pass changelog notes as the description.",
		core.OperationNonIdempotent),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0", "ref": "main", "name": "1.2.0", "description": "## Changelog\n- Fixed a bug"},
	)
}

func releaseShowSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[ReleaseShowInput, ReleaseDetail](
		OperationReleaseShow,
		"Show one GitLab release with its description, milestones, and asset links."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0"},
	)
}

func releaseUpdateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ReleaseUpdateInput, ReleaseDetail](
		OperationReleaseUpdate,
		"Update a GitLab release's title, notes, milestones, or release date.",
		core.OperationIdempotent),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0", "description": "## Changelog\n- Updated notes"},
	)
}

func releaseDeleteSpec() core.OperationSpec {
	return withInputExamples(gitlabDestructiveOperation[ReleaseDeleteInput, ReleaseActionResult](
		OperationReleaseDelete,
		"Delete a GitLab release. The underlying git tag is left in place."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0"},
	)
}

func changelogGenerateSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[ChangelogGenerateInput, ChangelogNotes](
		OperationChangelogGenerate,
		"Generate Markdown release notes from the commits between two refs without committing — feed the result into gitlab.release.create's description."),
		map[string]any{"project": "group/app", "version": "1.2.0"},
	)
}

func changelogAddSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ChangelogAddInput, ChangelogAddResult](
		OperationChangelogAdd,
		"Generate a changelog section and commit it into the repository's changelog file (default CHANGELOG.md).",
		core.OperationNonIdempotent),
		map[string]any{"project": "group/app", "version": "1.2.0", "branch": "main"},
	)
}

func tagShowSpec() core.OperationSpec {
	return withInputExamples(gitlabReadOperation[TagShowInput, RepositoryTag](
		OperationTagShow,
		"Show one git tag with its target commit and any annotation message."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0"},
	)
}

func tagDeleteSpec() core.OperationSpec {
	return withInputExamples(gitlabDestructiveOperation[TagDeleteInput, TagActionResult](
		OperationTagDelete,
		"Delete a git tag from a project."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0"},
	)
}

func releaseLinkListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[ReleaseLinkListInput, pluginbinding.ListResult[ReleaseLink]](
		OperationReleaseLinkList,
		"List the asset links attached to a release."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0"},
	)
}

func releaseLinkCreateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ReleaseLinkCreateInput, ReleaseLink](
		OperationReleaseLinkCreate,
		"Attach a new asset link (a download or related URL) to a release.",
		core.OperationNonIdempotent),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0", "name": "Binary", "url": "https://example.com/app-1.2.0.zip", "link_type": "package"},
	)
}

func releaseLinkUpdateSpec() core.OperationSpec {
	return withInputExamples(gitlabWriteOperation[ReleaseLinkUpdateInput, ReleaseLink](
		OperationReleaseLinkUpdate,
		"Edit an existing release asset link.",
		core.OperationIdempotent),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0", "link_id": 7, "name": "Binary (signed)"},
	)
}

func releaseLinkDeleteSpec() core.OperationSpec {
	return withInputExamples(gitlabDestructiveOperation[ReleaseLinkDeleteInput, ReleaseLinkActionResult](
		OperationReleaseLinkDelete,
		"Remove an asset link from a release."),
		map[string]any{"project": "group/app", "tag_name": "v1.2.0", "link_id": 7},
	)
}
