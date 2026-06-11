package gitlab

import (
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// CI/CD and repository read operations (field reports 2+3, issue #5):
// pipelines, jobs, environments, deployments, releases, tags, and commits.
// All are bounded list reads — `count`/`has_more` follow the shared
// ListResult shape used by the other gitlab list operations.

// cicdProjectInput is the shared project + limit input for project-scoped
// CI/CD list reads.
type cicdProjectInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum records to return. Defaults to 20\\, capped at 200. has_more is set when the cap was hit.,minimum=0"`
}

func (i cicdProjectInput) project() string {
	return strings.TrimSpace(firstNonEmpty(i.Project, i.ProjectID, i.Path))
}

type PipelineListInput struct {
	cicdProjectInput
	Status   string `json:"status,omitempty" jsonschema:"description=Filter by pipeline status,enum=created,enum=waiting_for_resource,enum=preparing,enum=pending,enum=running,enum=success,enum=failed,enum=canceled,enum=skipped,enum=manual,enum=scheduled"`
	Ref      string `json:"ref,omitempty" jsonschema:"description=Filter by git ref (branch or tag)"`
	Source   string `json:"source,omitempty" jsonschema:"description=Filter by trigger source (e.g. push\\, schedule\\, merge_request_event)"`
	Username string `json:"username,omitempty" jsonschema:"description=Filter by the user who triggered the pipeline"`
}

type PipelineListOptions struct {
	Status   string
	Ref      string
	Source   string
	Username string
	Limit    int
}

// PipelineList lists a project's pipelines, newest first.
func (s Service) PipelineList(ctx pluginbinding.Context, input PipelineListInput) (pluginbinding.ListResult[Pipeline], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[Pipeline]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[Pipeline]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	pipelines, truncated, err := client.ListProjectPipelines(projectID(project), PipelineListOptions{
		Status:   strings.TrimSpace(input.Status),
		Ref:      strings.TrimSpace(input.Ref),
		Source:   strings.TrimSpace(input.Source),
		Username: strings.TrimSpace(input.Username),
		Limit:    clampInt(input.Limit, 20, 200),
	})
	if err != nil {
		return pluginbinding.ListResult[Pipeline]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(pipelines)
	result.HasMore = truncated
	return result, nil
}

type JobListInput struct {
	cicdProjectInput
	PipelineID int64    `json:"pipeline_id,omitempty" jsonschema:"description=Pipeline id whose jobs to list"`
	Scope      []string `json:"scope,omitempty" jsonschema:"description=Filter by job status (e.g. failed\\, success\\, running)"`
}

type JobInfo struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name,omitempty"`
	Stage         string   `json:"stage,omitempty"`
	Status        string   `json:"status,omitempty"`
	FailureReason string   `json:"failure_reason,omitempty"`
	Ref           string   `json:"ref,omitempty"`
	WebURL        string   `json:"web_url,omitempty"`
	Duration      float64  `json:"duration,omitempty"`
	QueuedSeconds float64  `json:"queued_seconds,omitempty"`
	AllowFailure  bool     `json:"allow_failure,omitempty"`
	TagList       []string `json:"tag_list,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	FinishedAt    string   `json:"finished_at,omitempty"`
}

// JobList lists one pipeline's jobs with status, stage, and failure reasons.
func (s Service) JobList(ctx pluginbinding.Context, input JobListInput) (pluginbinding.ListResult[JobInfo], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[JobInfo]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[JobInfo]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if input.PipelineID <= 0 {
		return pluginbinding.ListResult[JobInfo]{}, pluginbinding.Fail("bad_input", "pipeline_id must be a positive integer")
	}
	jobs, truncated, err := client.ListPipelineJobs(projectID(project), input.PipelineID, input.Scope, clampInt(input.Limit, 50, 200))
	if err != nil {
		return pluginbinding.ListResult[JobInfo]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(jobs)
	result.HasMore = truncated
	return result, nil
}

type EnvironmentListInput struct {
	cicdProjectInput
	Search string `json:"search,omitempty" jsonschema:"description=Filter environments by name fragment"`
	States string `json:"states,omitempty" jsonschema:"description=Filter by state,enum=available,enum=stopping,enum=stopped"`
}

type EnvironmentInfo struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name,omitempty"`
	Slug           string          `json:"slug,omitempty"`
	State          string          `json:"state,omitempty"`
	Tier           string          `json:"tier,omitempty"`
	ExternalURL    string          `json:"external_url,omitempty"`
	KubeNamespace  string          `json:"kubernetes_namespace,omitempty"`
	LastDeployment *DeploymentInfo `json:"last_deployment,omitempty"`
	CreatedAt      string          `json:"created_at,omitempty"`
	UpdatedAt      string          `json:"updated_at,omitempty"`
}

// EnvironmentList lists a project's environments with their last deployment.
func (s Service) EnvironmentList(ctx pluginbinding.Context, input EnvironmentListInput) (pluginbinding.ListResult[EnvironmentInfo], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[EnvironmentInfo]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[EnvironmentInfo]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	environments, truncated, err := client.ListEnvironments(projectID(project), strings.TrimSpace(input.Search), strings.TrimSpace(input.States), clampInt(input.Limit, 20, 200))
	if err != nil {
		return pluginbinding.ListResult[EnvironmentInfo]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(environments)
	result.HasMore = truncated
	return result, nil
}

type DeploymentListInput struct {
	cicdProjectInput
	Environment string `json:"environment,omitempty" jsonschema:"description=Filter by environment name"`
	Status      string `json:"status,omitempty" jsonschema:"description=Filter by deployment status,enum=created,enum=running,enum=success,enum=failed,enum=canceled,enum=blocked"`
}

type DeploymentInfo struct {
	ID          int64  `json:"id"`
	IID         int64  `json:"iid,omitempty"`
	Ref         string `json:"ref,omitempty"`
	SHA         string `json:"sha,omitempty"`
	Status      string `json:"status,omitempty"`
	Environment string `json:"environment,omitempty"`
	Username    string `json:"username,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// DeploymentList lists a project's deployments, newest first.
func (s Service) DeploymentList(ctx pluginbinding.Context, input DeploymentListInput) (pluginbinding.ListResult[DeploymentInfo], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[DeploymentInfo]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[DeploymentInfo]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	deployments, truncated, err := client.ListProjectDeployments(projectID(project), strings.TrimSpace(input.Environment), strings.TrimSpace(input.Status), clampInt(input.Limit, 20, 200))
	if err != nil {
		return pluginbinding.ListResult[DeploymentInfo]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(deployments)
	result.HasMore = truncated
	return result, nil
}

type ReleaseListInput struct {
	cicdProjectInput
}

type ReleaseInfo struct {
	TagName     string `json:"tag_name,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	CommitSHA   string `json:"commit_sha,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	ReleasedAt  string `json:"released_at,omitempty"`
	Upcoming    bool   `json:"upcoming_release,omitempty"`
}

// ReleaseList lists a project's releases, newest first.
func (s Service) ReleaseList(ctx pluginbinding.Context, input ReleaseListInput) (pluginbinding.ListResult[ReleaseInfo], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[ReleaseInfo]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[ReleaseInfo]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	releases, truncated, err := client.ListReleases(projectID(project), clampInt(input.Limit, 20, 200))
	if err != nil {
		return pluginbinding.ListResult[ReleaseInfo]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(releases)
	result.HasMore = truncated
	return result, nil
}

type TagListInput struct {
	cicdProjectInput
	Search string `json:"search,omitempty" jsonschema:"description=Filter tags by name fragment"`
}

// TagList lists a project's git tags, newest first.
func (s Service) TagList(ctx pluginbinding.Context, input TagListInput) (pluginbinding.ListResult[RepositoryTag], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[RepositoryTag]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[RepositoryTag]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	tags, truncated, err := client.ListRepositoryTags(projectID(project), strings.TrimSpace(input.Search), clampInt(input.Limit, 20, 200))
	if err != nil {
		return pluginbinding.ListResult[RepositoryTag]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(tags)
	result.HasMore = truncated
	return result, nil
}

type CommitListInput struct {
	cicdProjectInput
	Ref      string `json:"ref,omitempty" jsonschema:"description=Branch\\, tag\\, or commit SHA to list history for. Defaults to the default branch"`
	FilePath string `json:"file_path,omitempty" jsonschema:"description=Only commits touching this file or directory path"`
	Author   string `json:"author,omitempty" jsonschema:"description=Filter by commit author name or email"`
	Since    string `json:"since,omitempty" jsonschema:"description=Only commits after this RFC3339 timestamp"`
	Until    string `json:"until,omitempty" jsonschema:"description=Only commits before this RFC3339 timestamp"`
}

type CommitListOptions struct {
	Ref      string
	FilePath string
	Author   string
	Since    string
	Until    string
	Limit    int
}

// CommitList lists a ref's commit history, newest first.
func (s Service) CommitList(ctx pluginbinding.Context, input CommitListInput) (pluginbinding.ListResult[Commit], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[Commit]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	if project == "" {
		return pluginbinding.ListResult[Commit]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	commits, truncated, err := client.ListCommits(projectID(project), CommitListOptions{
		Ref:      strings.TrimSpace(input.Ref),
		FilePath: strings.TrimSpace(input.FilePath),
		Author:   strings.TrimSpace(input.Author),
		Since:    strings.TrimSpace(input.Since),
		Until:    strings.TrimSpace(input.Until),
		Limit:    clampInt(input.Limit, 20, 200),
	})
	if err != nil {
		return pluginbinding.ListResult[Commit]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(commits)
	result.HasMore = truncated
	return result, nil
}
