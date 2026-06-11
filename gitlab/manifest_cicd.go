package gitlab

import (
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// CI/CD + repository read specs (issue #5). All are bounded list reads:
// limit defaults/caps live in the input docs and `has_more` flags a hit cap.

func registerCICDOperations(service Service) []pluginbinding.PluginOption {
	return []pluginbinding.PluginOption{
		pluginbinding.RegisterOperation(pipelineListSpec(), service.PipelineList),
		pluginbinding.RegisterOperation(jobListSpec(), service.JobList),
		pluginbinding.RegisterOperation(environmentListSpec(), service.EnvironmentList),
		pluginbinding.RegisterOperation(deploymentListSpec(), service.DeploymentList),
		pluginbinding.RegisterOperation(releaseListSpec(), service.ReleaseList),
		pluginbinding.RegisterOperation(tagListSpec(), service.TagList),
		pluginbinding.RegisterOperation(commitListSpec(), service.CommitList),
	}
}

func cicdOperationSpecs() []core.OperationSpec {
	return []core.OperationSpec{
		pipelineListSpec(),
		jobListSpec(),
		environmentListSpec(),
		deploymentListSpec(),
		releaseListSpec(),
		tagListSpec(),
		commitListSpec(),
	}
}

func pipelineListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[PipelineListInput, pluginbinding.ListResult[Pipeline]](
		OperationPipelineList,
		"List a project's CI pipelines, newest first, with status/ref/source/username filters — the entry point for \"is CI green?\" and \"what ran on this branch?\"."),
		map[string]any{"project": "group/app", "ref": "main", "limit": 5},
	)
}

func jobListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[JobListInput, pluginbinding.ListResult[JobInfo]](
		OperationJobList,
		"List one pipeline's jobs with stage, status, duration, and failure_reason — follow a failed pipeline down to the failing job."),
		map[string]any{"project": "group/app", "pipeline_id": 12345, "scope": []string{"failed"}},
	)
}

func environmentListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[EnvironmentListInput, pluginbinding.ListResult[EnvironmentInfo]](
		OperationEnvironmentList,
		"List a project's environments with state, tier, external URL, and last deployment."),
		map[string]any{"project": "group/app"},
	)
}

func deploymentListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[DeploymentListInput, pluginbinding.ListResult[DeploymentInfo]](
		OperationDeploymentList,
		"List a project's deployments, newest first, filterable by environment and status — \"what shipped to production and when?\"."),
		map[string]any{"project": "group/app", "environment": "production", "limit": 10},
	)
}

func releaseListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[ReleaseListInput, pluginbinding.ListResult[ReleaseInfo]](
		OperationReleaseList,
		"List a project's releases (tag, name, author, released_at), newest first."),
		map[string]any{"project": "group/app", "limit": 10},
	)
}

func tagListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[TagListInput, pluginbinding.ListResult[RepositoryTag]](
		OperationTagList,
		"List a project's git tags with their target commits, newest first, filterable by name fragment."),
		map[string]any{"project": "group/app", "search": "v1."},
	)
}

func commitListSpec() core.OperationSpec {
	return withInputExamples(gitlabCompactReadOperation[CommitListInput, pluginbinding.ListResult[Commit]](
		OperationCommitList,
		"List a ref's commit history, newest first; filter by file path, author, or a since/until time window — repository archaeology without cloning."),
		map[string]any{"project": "group/app", "ref": "main", "file_path": "src/billing/", "limit": 20},
	)
}
