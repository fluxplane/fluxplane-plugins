package gitlab

import (
	"fmt"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Release management: the full release lifecycle plus changelog
// generation, release asset links, and tag show/delete. Reads stay low risk;
// creates/updates are medium-risk writes and deletes are destructive. All are
// project-scoped and accept the shared project/project_id/path aliases.

// releaseProjectInput is the shared project selector for release-scoped
// operations that do not page (no limit field, unlike cicdProjectInput).
type releaseProjectInput struct {
	Project   string `json:"project,omitempty" jsonschema:"description=Project path or numeric ID"`
	ProjectID string `json:"project_id,omitempty" jsonschema:"description=Alias for project"`
	Path      string `json:"path,omitempty" jsonschema:"description=Alias for project"`
}

func (i releaseProjectInput) project() string {
	return strings.TrimSpace(firstNonEmpty(i.Project, i.ProjectID, i.Path))
}

// ReleaseDetail is the rich single-release shape returned by show/create/update.
// ReleaseList keeps the compact ReleaseInfo; this adds web URL, milestones, and
// asset links.
type ReleaseDetail struct {
	TagName     string        `json:"tag_name,omitempty"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	WebURL      string        `json:"web_url,omitempty"`
	Author      string        `json:"author,omitempty"`
	CommitSHA   string        `json:"commit_sha,omitempty"`
	CreatedAt   string        `json:"created_at,omitempty"`
	ReleasedAt  string        `json:"released_at,omitempty"`
	Upcoming    bool          `json:"upcoming_release,omitempty"`
	Milestones  []string      `json:"milestones"`
	AssetLinks  []ReleaseLink `json:"assets_links"`
}

// ReleaseLink is a release asset link (a download or related URL attached to a
// release).
type ReleaseLink struct {
	ID             int64  `json:"id"`
	Name           string `json:"name,omitempty"`
	URL            string `json:"url,omitempty"`
	DirectAssetURL string `json:"direct_asset_url,omitempty"`
	LinkType       string `json:"link_type,omitempty"`
	External       bool   `json:"external,omitempty"`
}

// ReleaseLinkInput is an asset link supplied when creating a release.
type ReleaseLinkInput struct {
	Name            string `json:"name,omitempty" jsonschema:"description=Link label"`
	URL             string `json:"url,omitempty" jsonschema:"description=Link URL"`
	DirectAssetPath string `json:"direct_asset_path,omitempty" jsonschema:"description=Permalink path served under the project's releases (e.g. /bin/asset.zip)"`
	LinkType        string `json:"link_type,omitempty" jsonschema:"description=Link category,enum=other,enum=runbook,enum=image,enum=package"`
}

// ReleaseLinkOption is the internal asset-link option passed to the client.
type ReleaseLinkOption struct {
	Name            string
	URL             string
	DirectAssetPath string
	LinkType        string
}

type ReleaseCreateInput struct {
	releaseProjectInput
	TagName     string             `json:"tag_name,omitempty" jsonschema:"description=Tag the release is bound to. Created from ref when it does not yet exist"`
	Ref         string             `json:"ref,omitempty" jsonschema:"description=Commit SHA\\, branch\\, or tag to create tag_name from when the tag does not yet exist"`
	Name        string             `json:"name,omitempty" jsonschema:"description=Release title. Defaults to the tag name"`
	Description string             `json:"description,omitempty" jsonschema:"description=Release notes in Markdown. Pass the output of gitlab.repository.changelog.generate here"`
	TagMessage  string             `json:"tag_message,omitempty" jsonschema:"description=Annotation message used when tag_name is created from ref (makes an annotated tag)"`
	Milestones  []string           `json:"milestones,omitempty" jsonschema:"description=Titles of milestones to associate with the release"`
	ReleasedAt  string             `json:"released_at,omitempty" jsonschema:"description=Release timestamp (RFC3339). Defaults to now"`
	AssetLinks  []ReleaseLinkInput `json:"assets_links,omitempty" jsonschema:"description=Asset links to attach to the release"`
}

type ReleaseCreateOptions struct {
	TagName     string
	Ref         string
	Name        string
	Description string
	TagMessage  string
	Milestones  []string
	ReleasedAt  *time.Time
	Links       []ReleaseLinkOption
}

// ReleaseCreate creates a release for a tag, cutting the tag from ref when it
// does not yet exist.
func (s Service) ReleaseCreate(ctx pluginbinding.Context, input ReleaseCreateInput) (ReleaseDetail, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(input.TagName)
	if project == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	releasedAt, err := parseReleaseTime(input.ReleasedAt)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	options := ReleaseCreateOptions{
		TagName:     tag,
		Ref:         strings.TrimSpace(input.Ref),
		Name:        strings.TrimSpace(input.Name),
		Description: input.Description,
		TagMessage:  strings.TrimSpace(input.TagMessage),
		Milestones:  input.Milestones,
		ReleasedAt:  releasedAt,
		Links:       releaseLinkOptions(input.AssetLinks),
	}
	release, err := client.CreateRelease(projectID(project), options)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return release, nil
}

type ReleaseShowInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag of the release to show"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
}

// ReleaseShow returns one release with its description, milestones, and asset
// links.
func (s Service) ReleaseShow(ctx pluginbinding.Context, input ReleaseShowInput) (ReleaseDetail, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	release, err := client.GetRelease(projectID(project), tag)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return release, nil
}

type ReleaseUpdateInput struct {
	releaseProjectInput
	TagName     string    `json:"tag_name,omitempty" jsonschema:"description=Tag of the release to update"`
	Tag         string    `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	Name        *string   `json:"name,omitempty" jsonschema:"description=New release title"`
	Description *string   `json:"description,omitempty" jsonschema:"description=New release notes (Markdown)"`
	Milestones  *[]string `json:"milestones,omitempty" jsonschema:"description=Replacement milestone titles. Pass [] to clear all"`
	ReleasedAt  *string   `json:"released_at,omitempty" jsonschema:"description=New release timestamp (RFC3339)"`
}

type ReleaseUpdateOptions struct {
	Name        *string
	Description *string
	Milestones  *[]string
	ReleasedAt  *time.Time
}

// ReleaseUpdate edits a release's title, notes, milestones, or release date.
func (s Service) ReleaseUpdate(ctx pluginbinding.Context, input ReleaseUpdateInput) (ReleaseDetail, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseDetail{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	options := ReleaseUpdateOptions{
		Name:        input.Name,
		Description: input.Description,
		Milestones:  input.Milestones,
	}
	if input.ReleasedAt != nil {
		releasedAt, err := parseReleaseTime(*input.ReleasedAt)
		if err != nil {
			return ReleaseDetail{}, pluginbinding.Errorf("bad_input", "%s", err)
		}
		options.ReleasedAt = releasedAt
	}
	release, err := client.UpdateRelease(projectID(project), tag, options)
	if err != nil {
		return ReleaseDetail{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return release, nil
}

type ReleaseDeleteInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag of the release to delete. The tag itself is not removed"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
}

// ReleaseActionResult reports the outcome of a release mutation that returns no
// body of its own.
type ReleaseActionResult struct {
	Project string `json:"project,omitempty"`
	TagName string `json:"tag_name,omitempty"`
	Message string `json:"message,omitempty"`
}

// ReleaseDelete removes a release. The underlying git tag is left in place.
func (s Service) ReleaseDelete(ctx pluginbinding.Context, input ReleaseDeleteInput) (ReleaseActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return ReleaseActionResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseActionResult{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	if err := client.DeleteRelease(projectID(project), tag); err != nil {
		return ReleaseActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return ReleaseActionResult{Project: project, TagName: tag, Message: "release deleted"}, nil
}

type ChangelogGenerateInput struct {
	releaseProjectInput
	Version    string `json:"version,omitempty" jsonschema:"description=Semantic version the changelog is generated for (e.g. 1.2.0)"`
	From       string `json:"from,omitempty" jsonschema:"description=Start commit SHA or tag (exclusive). Defaults to the previous tag"`
	To         string `json:"to,omitempty" jsonschema:"description=End commit SHA or tag. Defaults to the default branch HEAD"`
	Date       string `json:"date,omitempty" jsonschema:"description=Release date (RFC3339). Defaults to now"`
	Trailer    string `json:"trailer,omitempty" jsonschema:"description=Git trailer that marks commits for the changelog. Defaults to Changelog"`
	ConfigFile string `json:"config_file,omitempty" jsonschema:"description=Path to the changelog config in the repo. Defaults to .gitlab/changelog_config.yml"`
}

type ChangelogGenerateOptions struct {
	Version    string
	From       string
	To         string
	Date       *time.Time
	Trailer    string
	ConfigFile string
}

// ChangelogNotes is the generated Markdown changelog, ready to drop into a
// release description.
type ChangelogNotes struct {
	Notes string `json:"notes,omitempty"`
}

// ChangelogGenerate builds Markdown release notes from the commits between two
// refs, without committing anything.
func (s Service) ChangelogGenerate(ctx pluginbinding.Context, input ChangelogGenerateInput) (ChangelogNotes, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ChangelogNotes{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	version := strings.TrimSpace(input.Version)
	if project == "" {
		return ChangelogNotes{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if version == "" {
		return ChangelogNotes{}, pluginbinding.Fail("bad_input", "version is required")
	}
	date, err := parseReleaseTime(input.Date)
	if err != nil {
		return ChangelogNotes{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	notes, err := client.GenerateChangelog(projectID(project), ChangelogGenerateOptions{
		Version:    version,
		From:       strings.TrimSpace(input.From),
		To:         strings.TrimSpace(input.To),
		Date:       date,
		Trailer:    strings.TrimSpace(input.Trailer),
		ConfigFile: strings.TrimSpace(input.ConfigFile),
	})
	if err != nil {
		return ChangelogNotes{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return ChangelogNotes{Notes: notes}, nil
}

type ChangelogAddInput struct {
	releaseProjectInput
	Version    string `json:"version,omitempty" jsonschema:"description=Semantic version the changelog section is generated for (e.g. 1.2.0)"`
	Branch     string `json:"branch,omitempty" jsonschema:"description=Branch to commit the changelog to. Defaults to the default branch"`
	File       string `json:"file,omitempty" jsonschema:"description=Changelog file to update. Defaults to CHANGELOG.md"`
	From       string `json:"from,omitempty" jsonschema:"description=Start commit SHA or tag (exclusive). Defaults to the previous tag"`
	To         string `json:"to,omitempty" jsonschema:"description=End commit SHA or tag. Defaults to the default branch HEAD"`
	Date       string `json:"date,omitempty" jsonschema:"description=Release date (RFC3339). Defaults to now"`
	Message    string `json:"message,omitempty" jsonschema:"description=Commit message. Defaults to a generated message"`
	Trailer    string `json:"trailer,omitempty" jsonschema:"description=Git trailer that marks commits for the changelog. Defaults to Changelog"`
	ConfigFile string `json:"config_file,omitempty" jsonschema:"description=Path to the changelog config in the repo. Defaults to .gitlab/changelog_config.yml"`
}

type ChangelogAddOptions struct {
	Version    string
	Branch     string
	File       string
	From       string
	To         string
	Date       *time.Time
	Message    string
	Trailer    string
	ConfigFile string
}

// ChangelogAddResult reports the changelog commit that was requested. The
// GitLab endpoint returns no body, so the version/branch/file are echoed back.
type ChangelogAddResult struct {
	Project string `json:"project,omitempty"`
	Version string `json:"version,omitempty"`
	Branch  string `json:"branch,omitempty"`
	File    string `json:"file,omitempty"`
	Message string `json:"message,omitempty"`
}

// ChangelogAdd generates a changelog section and commits it into the repo's
// changelog file (default CHANGELOG.md).
func (s Service) ChangelogAdd(ctx pluginbinding.Context, input ChangelogAddInput) (ChangelogAddResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ChangelogAddResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	version := strings.TrimSpace(input.Version)
	if project == "" {
		return ChangelogAddResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if version == "" {
		return ChangelogAddResult{}, pluginbinding.Fail("bad_input", "version is required")
	}
	date, err := parseReleaseTime(input.Date)
	if err != nil {
		return ChangelogAddResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	file := strings.TrimSpace(input.File)
	branch := strings.TrimSpace(input.Branch)
	if err := client.AddChangelog(projectID(project), ChangelogAddOptions{
		Version:    version,
		Branch:     branch,
		File:       file,
		From:       strings.TrimSpace(input.From),
		To:         strings.TrimSpace(input.To),
		Date:       date,
		Message:    strings.TrimSpace(input.Message),
		Trailer:    strings.TrimSpace(input.Trailer),
		ConfigFile: strings.TrimSpace(input.ConfigFile),
	}); err != nil {
		return ChangelogAddResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	if file == "" {
		file = "CHANGELOG.md"
	}
	return ChangelogAddResult{Project: project, Version: version, Branch: branch, File: file, Message: "changelog committed"}, nil
}

type TagShowInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag to show"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	Name    string `json:"name,omitempty" jsonschema:"description=Alias for tag_name"`
}

// TagShow returns one git tag with its target commit and any annotation message.
func (s Service) TagShow(ctx pluginbinding.Context, input TagShowInput) (RepositoryTag, error) {
	client, err := s.client(ctx)
	if err != nil {
		return RepositoryTag{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag, input.Name))
	if project == "" {
		return RepositoryTag{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return RepositoryTag{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	result, err := client.GetRepositoryTag(projectID(project), tag)
	if err != nil {
		return RepositoryTag{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return result, nil
}

type TagDeleteInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag to delete"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	Name    string `json:"name,omitempty" jsonschema:"description=Alias for tag_name"`
}

// TagActionResult reports the outcome of a tag mutation that returns no body.
type TagActionResult struct {
	Project string `json:"project,omitempty"`
	TagName string `json:"tag_name,omitempty"`
	Message string `json:"message,omitempty"`
}

// TagDelete removes a git tag from a project.
func (s Service) TagDelete(ctx pluginbinding.Context, input TagDeleteInput) (TagActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return TagActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag, input.Name))
	if project == "" {
		return TagActionResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return TagActionResult{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	if err := client.DeleteRepositoryTag(projectID(project), tag); err != nil {
		return TagActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return TagActionResult{Project: project, TagName: tag, Message: "tag deleted"}, nil
}

type ReleaseLinkListInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag of the release whose links to list"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum records to return. Defaults to 20\\, capped at 200. has_more is set when the cap was hit.,minimum=0"`
}

// ReleaseLinkList lists the asset links attached to a release.
func (s Service) ReleaseLinkList(ctx pluginbinding.Context, input ReleaseLinkListInput) (pluginbinding.ListResult[ReleaseLink], error) {
	client, err := s.client(ctx)
	if err != nil {
		return pluginbinding.ListResult[ReleaseLink]{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return pluginbinding.ListResult[ReleaseLink]{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return pluginbinding.ListResult[ReleaseLink]{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	links, truncated, err := client.ListReleaseLinks(projectID(project), tag, clampInt(input.Limit, 20, 200))
	if err != nil {
		return pluginbinding.ListResult[ReleaseLink]{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	result := pluginbinding.NewListResult(links)
	result.HasMore = truncated
	return result, nil
}

type ReleaseLinkCreateInput struct {
	releaseProjectInput
	TagName         string `json:"tag_name,omitempty" jsonschema:"description=Tag of the release to attach the link to"`
	Tag             string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	Name            string `json:"name,omitempty" jsonschema:"description=Link label"`
	URL             string `json:"url,omitempty" jsonschema:"description=Link URL"`
	DirectAssetPath string `json:"direct_asset_path,omitempty" jsonschema:"description=Permalink path served under the project's releases (e.g. /bin/asset.zip)"`
	LinkType        string `json:"link_type,omitempty" jsonschema:"description=Link category,enum=other,enum=runbook,enum=image,enum=package"`
}

type ReleaseLinkCreateOptions struct {
	Name            string
	URL             string
	DirectAssetPath string
	LinkType        string
}

// ReleaseLinkCreate attaches a new asset link to a release.
func (s Service) ReleaseLinkCreate(ctx pluginbinding.Context, input ReleaseLinkCreateInput) (ReleaseLink, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseLink{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	name := strings.TrimSpace(input.Name)
	url := strings.TrimSpace(input.URL)
	if project == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	if name == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "name is required")
	}
	if url == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "url is required")
	}
	link, err := client.CreateReleaseLink(projectID(project), tag, ReleaseLinkCreateOptions{
		Name:            name,
		URL:             url,
		DirectAssetPath: strings.TrimSpace(input.DirectAssetPath),
		LinkType:        strings.TrimSpace(input.LinkType),
	})
	if err != nil {
		return ReleaseLink{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return link, nil
}

type ReleaseLinkUpdateInput struct {
	releaseProjectInput
	TagName         string  `json:"tag_name,omitempty" jsonschema:"description=Tag of the release the link belongs to"`
	Tag             string  `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	LinkID          int64   `json:"link_id,omitempty" jsonschema:"description=ID of the link to update"`
	Name            *string `json:"name,omitempty" jsonschema:"description=New link label"`
	URL             *string `json:"url,omitempty" jsonschema:"description=New link URL"`
	DirectAssetPath *string `json:"direct_asset_path,omitempty" jsonschema:"description=New permalink path served under the project's releases"`
	LinkType        *string `json:"link_type,omitempty" jsonschema:"description=New link category,enum=other,enum=runbook,enum=image,enum=package"`
}

type ReleaseLinkUpdateOptions struct {
	Name            *string
	URL             *string
	DirectAssetPath *string
	LinkType        *string
}

// ReleaseLinkUpdate edits an existing release asset link.
func (s Service) ReleaseLinkUpdate(ctx pluginbinding.Context, input ReleaseLinkUpdateInput) (ReleaseLink, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseLink{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	if input.LinkID <= 0 {
		return ReleaseLink{}, pluginbinding.Fail("bad_input", "link_id must be a positive integer")
	}
	link, err := client.UpdateReleaseLink(projectID(project), tag, input.LinkID, ReleaseLinkUpdateOptions{
		Name:            input.Name,
		URL:             input.URL,
		DirectAssetPath: input.DirectAssetPath,
		LinkType:        input.LinkType,
	})
	if err != nil {
		return ReleaseLink{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return link, nil
}

type ReleaseLinkDeleteInput struct {
	releaseProjectInput
	TagName string `json:"tag_name,omitempty" jsonschema:"description=Tag of the release the link belongs to"`
	Tag     string `json:"tag,omitempty" jsonschema:"description=Alias for tag_name"`
	LinkID  int64  `json:"link_id,omitempty" jsonschema:"description=ID of the link to delete"`
}

// ReleaseLinkActionResult reports the outcome of a release-link mutation that
// returns no body.
type ReleaseLinkActionResult struct {
	Project string `json:"project,omitempty"`
	TagName string `json:"tag_name,omitempty"`
	LinkID  int64  `json:"link_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// ReleaseLinkDelete removes an asset link from a release.
func (s Service) ReleaseLinkDelete(ctx pluginbinding.Context, input ReleaseLinkDeleteInput) (ReleaseLinkActionResult, error) {
	client, err := s.client(ctx)
	if err != nil {
		return ReleaseLinkActionResult{}, pluginbinding.Errorf("secret", "%s", err)
	}
	project := input.project()
	tag := strings.TrimSpace(firstNonEmpty(input.TagName, input.Tag))
	if project == "" {
		return ReleaseLinkActionResult{}, pluginbinding.Fail("bad_input", "project is required")
	}
	if tag == "" {
		return ReleaseLinkActionResult{}, pluginbinding.Fail("bad_input", "tag_name is required")
	}
	if input.LinkID <= 0 {
		return ReleaseLinkActionResult{}, pluginbinding.Fail("bad_input", "link_id must be a positive integer")
	}
	if err := client.DeleteReleaseLink(projectID(project), tag, input.LinkID); err != nil {
		return ReleaseLinkActionResult{}, pluginbinding.Errorf("gitlab", "%s", err)
	}
	return ReleaseLinkActionResult{Project: project, TagName: tag, LinkID: input.LinkID, Message: "release link deleted"}, nil
}

// releaseLinkOptions maps create-release asset link inputs to client options.
func releaseLinkOptions(inputs []ReleaseLinkInput) []ReleaseLinkOption {
	if len(inputs) == 0 {
		return nil
	}
	out := make([]ReleaseLinkOption, 0, len(inputs))
	for _, link := range inputs {
		out = append(out, ReleaseLinkOption{
			Name:            strings.TrimSpace(link.Name),
			URL:             strings.TrimSpace(link.URL),
			DirectAssetPath: strings.TrimSpace(link.DirectAssetPath),
			LinkType:        strings.TrimSpace(link.LinkType),
		})
	}
	return out
}

// parseReleaseTime parses an optional RFC3339 timestamp; empty yields nil.
func parseReleaseTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp %q: expected RFC3339 (e.g. 2026-01-02T15:04:05Z)", value)
	}
	return &parsed, nil
}
