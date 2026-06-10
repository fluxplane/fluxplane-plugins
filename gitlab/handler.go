package gitlab

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func NewPlugin() *pluginbinding.Plugin {
	return NewPluginWithService(NewService())
}

func NewPluginWithService(service Service) *pluginbinding.Plugin {
	options := []pluginbinding.PluginOption{
		pluginbinding.WithAuthTestOperation(OperationAuthTest),
		pluginbinding.WithIndexBuildOperation(OperationIndexBuild),
		pluginbinding.WithHostOwnedIndexStatus("GitLab"),
		pluginbinding.RegisterOperation(authTestSpec(), service.AuthTest),
		pluginbinding.RegisterOperation(indexBuildSpec(), service.IndexBuild),
		pluginbinding.RegisterOperation(projectListSpec(), service.ProjectList),
		pluginbinding.RegisterOperation(projectShowSpec(), service.ProjectShow),
		pluginbinding.RegisterOperation(mergeRequestListSpec(), service.MergeRequestList),
		pluginbinding.RegisterOperation(mergeRequestShowSpec(), service.MergeRequestShow),
		pluginbinding.RegisterOperation(mergeRequestCreateSpec(), service.MergeRequestCreate),
		pluginbinding.RegisterOperation(mergeRequestApproveSpec(), service.MergeRequestApprove),
		pluginbinding.RegisterOperation(mergeRequestMergeSpec(), service.MergeRequestMerge),
		pluginbinding.RegisterOperation(repositoryTagCreateSpec(), service.RepositoryTagCreate),
		pluginbinding.RegisterOperation(branchCreateSpec(), service.BranchCreate),
		pluginbinding.RegisterOperation(branchDeleteSpec(), service.BranchDelete),
		pluginbinding.RegisterOperation(branchDeleteMergedSpec(), service.BranchDeleteMerged),
		pluginbinding.RegisterOperation(repoFileCreateSpec(), service.RepoFileCreate),
		pluginbinding.RegisterOperation(repoFileUpdateSpec(), service.RepoFileUpdate),
		pluginbinding.RegisterOperation(repoFileDeleteSpec(), service.RepoFileDelete),
		pluginbinding.RegisterOperation(commitCreateSpec(), service.CommitCreate),
		pluginbinding.RegisterOperation(ciVariableCreateSpec(), service.CIVariableCreate),
		pluginbinding.RegisterOperation(ciVariableUpdateSpec(), service.CIVariableUpdate),
		pluginbinding.RegisterOperation(ciVariableDeleteSpec(), service.CIVariableDelete),
		pluginbinding.RegisterOperation(pipelineCreateSpec(), service.PipelineCreate),
		pluginbinding.RegisterOperation(pipelineRetrySpec(), service.PipelineRetry),
		pluginbinding.RegisterOperation(pipelineCancelSpec(), service.PipelineCancel),
		pluginbinding.RegisterOperation(snippetCreateSpec(), service.SnippetCreate),
		pluginbinding.RegisterOperation(snippetDeleteSpec(), service.SnippetDelete),
		pluginbinding.RegisterOperation(issueListSpec(), service.IssueList),
		pluginbinding.RegisterOperation(issueShowSpec(), service.IssueShow),
		pluginbinding.RegisterOperation(issueCreateSpec(), service.IssueCreate),
		pluginbinding.RegisterOperation(issueUpdateSpec(), service.IssueUpdate),
		pluginbinding.RegisterOperation(issueNoteListSpec(), service.IssueNoteList),
		pluginbinding.RegisterOperation(issueNoteCreateSpec(), service.IssueNoteCreate),
		pluginbinding.RegisterContextProvider(pluginbinding.ContextSpec(ContextName, "GitLab context blocks.", pluginbinding.ContextKindText, pluginbinding.ContextKindReference), BuildContext),
		pluginbinding.RegisterDatasourceSearch(gitlabProjectsDatasourceSpec(), service.ProjectDatasourceSearch),
		pluginbinding.RegisterDatasourceList(gitlabProjectsDatasourceSpec(), service.DatasourceList),
		pluginbinding.RegisterDatasourceGet(gitlabProjectsDatasourceGetSpec(), service.ProjectDatasourceGet),
		pluginbinding.RegisterDatasourceBatchGet(gitlabProjectsDatasourceSpec(), service.DatasourceBatchGet),
		pluginbinding.RegisterDatasourceLookup(gitlabProjectsLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceSearch(gitlabUsersDatasourceSpec(), service.UserDatasourceSearch),
		pluginbinding.RegisterDatasourceList(gitlabUsersDatasourceSpec(), service.DatasourceList),
		pluginbinding.RegisterDatasourceGet(gitlabUsersDatasourceGetSpec(), service.UserDatasourceGet),
		pluginbinding.RegisterDatasourceBatchGet(gitlabUsersDatasourceSpec(), service.DatasourceBatchGet),
		pluginbinding.RegisterDatasourceLookup(gitlabUsersLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceSearch(gitlabGroupsDatasourceSpec(), service.GroupDatasourceSearch),
		pluginbinding.RegisterDatasourceList(gitlabGroupsDatasourceSpec(), service.DatasourceList),
		pluginbinding.RegisterDatasourceGet(gitlabGroupsDatasourceGetSpec(), service.GroupDatasourceGet),
		pluginbinding.RegisterDatasourceBatchGet(gitlabGroupsDatasourceSpec(), service.DatasourceBatchGet),
		pluginbinding.RegisterDatasourceLookup(gitlabGroupsLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceSearch(gitlabIssuesDatasourceSpec(), service.IssueDatasourceSearch),
		pluginbinding.RegisterDatasourceList(gitlabIssuesDatasourceSpec(), service.DatasourceList),
		pluginbinding.RegisterDatasourceGet(gitlabIssuesDatasourceGetSpec(), service.IssueDatasourceGet),
		pluginbinding.RegisterDatasourceBatchGet(gitlabIssuesDatasourceSpec(), service.DatasourceBatchGet),
		pluginbinding.RegisterDatasourceLookup(gitlabIssuesLookupSpec(), service.Lookup),
		pluginbinding.RegisterDatasourceSearch(gitlabMergeRequestsDatasourceSpec(), service.MergeRequestDatasourceSearch),
		pluginbinding.RegisterDatasourceList(gitlabMergeRequestsDatasourceSpec(), service.DatasourceList),
		pluginbinding.RegisterDatasourceGet(gitlabMergeRequestsDatasourceGetSpec(), service.MergeRequestDatasourceGet),
		pluginbinding.RegisterDatasourceBatchGet(gitlabMergeRequestsDatasourceSpec(), service.DatasourceBatchGet),
		pluginbinding.RegisterDatasourceLookup(gitlabMergeRequestsLookupSpec(), service.Lookup),
	}
	options = append(options, registerReviewOperations(service)...)
	return pluginbinding.Define(manifestSpec(), options...)
}

func Handle(req protocol.Request) protocol.Response {
	return NewPlugin().Handle(req)
}
