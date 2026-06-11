package gitlab

import (
	"strings"
	"time"

	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

// CI/CD and repository read calls (issue #5). Each pages through the API up
// to the caller's limit and reports truncation, mirroring ListMergeRequestDiffs.

func (c liveClient) ListProjectPipelines(project any, input PipelineListOptions) ([]Pipeline, bool, error) {
	opt := &gitlabapi.ListProjectPipelinesOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if input.Status != "" {
		opt.Status = gitlabapi.Ptr(gitlabapi.BuildStateValue(input.Status))
	}
	if input.Ref != "" {
		opt.Ref = gitlabapi.Ptr(input.Ref)
	}
	if input.Source != "" {
		opt.Source = gitlabapi.Ptr(input.Source)
	}
	if input.Username != "" {
		opt.Username = gitlabapi.Ptr(input.Username)
	}
	var out []Pipeline
	for {
		pipelines, resp, err := c.client.Pipelines.ListProjectPipelines(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, pipeline := range pipelines {
			if input.Limit > 0 && len(out) >= input.Limit {
				return out, true, nil
			}
			out = append(out, pipelineInfoFromAPI(pipeline))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListPipelineJobs(project any, pipelineID int64, scope []string, limit int) ([]JobInfo, bool, error) {
	opt := &gitlabapi.ListJobsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 50))
	opt.Page = 1
	if len(scope) > 0 {
		states := make([]gitlabapi.BuildStateValue, 0, len(scope))
		for _, state := range scope {
			if state = strings.TrimSpace(state); state != "" {
				states = append(states, gitlabapi.BuildStateValue(state))
			}
		}
		if len(states) > 0 {
			opt.Scope = &states
		}
	}
	var out []JobInfo
	for {
		jobs, resp, err := c.client.Jobs.ListPipelineJobs(project, pipelineID, opt)
		if err != nil {
			return nil, false, err
		}
		for _, job := range jobs {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, jobInfoFromAPI(job))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListEnvironments(project any, search, states string, limit int) ([]EnvironmentInfo, bool, error) {
	opt := &gitlabapi.ListEnvironmentsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	if search != "" {
		opt.Search = gitlabapi.Ptr(search)
	}
	if states != "" {
		opt.States = gitlabapi.Ptr(states)
	}
	var out []EnvironmentInfo
	for {
		environments, resp, err := c.client.Environments.ListEnvironments(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, environment := range environments {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, environmentInfoFromAPI(environment))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListProjectDeployments(project any, environment, status string, limit int) ([]DeploymentInfo, bool, error) {
	opt := &gitlabapi.ListProjectDeploymentsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	opt.OrderBy = gitlabapi.Ptr("created_at")
	opt.Sort = gitlabapi.Ptr("desc")
	if environment != "" {
		opt.Environment = gitlabapi.Ptr(environment)
	}
	if status != "" {
		opt.Status = gitlabapi.Ptr(status)
	}
	var out []DeploymentInfo
	for {
		deployments, resp, err := c.client.Deployments.ListProjectDeployments(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, deployment := range deployments {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, deploymentInfoFromAPI(deployment))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListReleases(project any, limit int) ([]ReleaseInfo, bool, error) {
	opt := &gitlabapi.ListReleasesOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	var out []ReleaseInfo
	for {
		releases, resp, err := c.client.Releases.ListReleases(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, release := range releases {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, releaseInfoFromAPI(release))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListRepositoryTags(project any, search string, limit int) ([]RepositoryTag, bool, error) {
	opt := &gitlabapi.ListTagsOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	if search != "" {
		opt.Search = gitlabapi.Ptr(search)
	}
	var out []RepositoryTag
	for {
		tags, resp, err := c.client.Tags.ListTags(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, tag := range tags {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, repositoryTagFromAPI(tag))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) ListCommits(project any, input CommitListOptions) ([]Commit, bool, error) {
	opt := &gitlabapi.ListCommitsOptions{}
	opt.PerPage = int64(clampProjectPageSize(input.Limit, 20))
	opt.Page = 1
	if input.Ref != "" {
		opt.RefName = gitlabapi.Ptr(input.Ref)
	}
	if input.FilePath != "" {
		opt.Path = gitlabapi.Ptr(input.FilePath)
	}
	if input.Author != "" {
		opt.Author = gitlabapi.Ptr(input.Author)
	}
	if since, err := time.Parse(time.RFC3339, input.Since); err == nil && input.Since != "" {
		opt.Since = gitlabapi.Ptr(since)
	}
	if until, err := time.Parse(time.RFC3339, input.Until); err == nil && input.Until != "" {
		opt.Until = gitlabapi.Ptr(until)
	}
	var out []Commit
	for {
		commits, resp, err := c.client.Commits.ListCommits(project, opt)
		if err != nil {
			return nil, false, err
		}
		for _, commit := range commits {
			if input.Limit > 0 && len(out) >= input.Limit {
				return out, true, nil
			}
			out = append(out, commitFromAPI(commit))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func pipelineInfoFromAPI(pipeline *gitlabapi.PipelineInfo) Pipeline {
	if pipeline == nil {
		return Pipeline{}
	}
	return Pipeline{
		ID:        pipeline.ID,
		ProjectID: pipeline.ProjectID,
		Status:    pipeline.Status,
		Ref:       pipeline.Ref,
		SHA:       pipeline.SHA,
		WebURL:    pipeline.WebURL,
		Source:    pipeline.Source,
		CreatedAt: formatTime(pipeline.CreatedAt),
		UpdatedAt: formatTime(pipeline.UpdatedAt),
	}
}

func jobInfoFromAPI(job *gitlabapi.Job) JobInfo {
	if job == nil {
		return JobInfo{}
	}
	return JobInfo{
		ID:            job.ID,
		Name:          job.Name,
		Stage:         job.Stage,
		Status:        job.Status,
		FailureReason: job.FailureReason,
		Ref:           job.Ref,
		WebURL:        job.WebURL,
		Duration:      job.Duration,
		QueuedSeconds: job.QueuedDuration,
		AllowFailure:  job.AllowFailure,
		TagList:       job.TagList,
		CreatedAt:     formatTime(job.CreatedAt),
		StartedAt:     formatTime(job.StartedAt),
		FinishedAt:    formatTime(job.FinishedAt),
	}
}

func environmentInfoFromAPI(environment *gitlabapi.Environment) EnvironmentInfo {
	if environment == nil {
		return EnvironmentInfo{}
	}
	out := EnvironmentInfo{
		ID:            environment.ID,
		Name:          environment.Name,
		Slug:          environment.Slug,
		State:         environment.State,
		Tier:          environment.Tier,
		ExternalURL:   environment.ExternalURL,
		KubeNamespace: environment.KubernetesNamespace,
		CreatedAt:     formatTime(environment.CreatedAt),
		UpdatedAt:     formatTime(environment.UpdatedAt),
	}
	if environment.LastDeployment != nil {
		last := deploymentInfoFromAPI(environment.LastDeployment)
		out.LastDeployment = &last
	}
	return out
}

func deploymentInfoFromAPI(deployment *gitlabapi.Deployment) DeploymentInfo {
	if deployment == nil {
		return DeploymentInfo{}
	}
	out := DeploymentInfo{
		ID:        deployment.ID,
		IID:       deployment.IID,
		Ref:       deployment.Ref,
		SHA:       deployment.SHA,
		Status:    deployment.Status,
		CreatedAt: formatTime(deployment.CreatedAt),
		UpdatedAt: formatTime(deployment.UpdatedAt),
	}
	if deployment.Environment != nil {
		out.Environment = deployment.Environment.Name
	}
	if deployment.User != nil {
		out.Username = deployment.User.Username
	}
	return out
}

func releaseInfoFromAPI(release *gitlabapi.Release) ReleaseInfo {
	if release == nil {
		return ReleaseInfo{}
	}
	return ReleaseInfo{
		TagName:     release.TagName,
		Name:        release.Name,
		Description: release.Description,
		Author:      release.Author.Username,
		CommitSHA:   release.Commit.ID,
		CreatedAt:   formatTime(release.CreatedAt),
		ReleasedAt:  formatTime(release.ReleasedAt),
		Upcoming:    release.UpcomingRelease,
	}
}
