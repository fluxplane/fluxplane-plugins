package gitlab

import (
	"strings"

	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

// Release-management client calls: releases, release asset links,
// changelog generation/commit, and tag show/delete. These wrap the GitLab SDK
// services the same way client_cicd.go wraps the read surface.

func (c liveClient) GetRelease(project any, tag string) (ReleaseDetail, error) {
	release, _, err := c.client.Releases.GetRelease(project, tag)
	if err != nil {
		return ReleaseDetail{}, err
	}
	return releaseDetailFromAPI(release), nil
}

func (c liveClient) CreateRelease(project any, input ReleaseCreateOptions) (ReleaseDetail, error) {
	opt := &gitlabapi.CreateReleaseOptions{
		TagName: gitlabapi.Ptr(input.TagName),
	}
	if input.Ref != "" {
		opt.Ref = gitlabapi.Ptr(input.Ref)
	}
	if input.Name != "" {
		opt.Name = gitlabapi.Ptr(input.Name)
	}
	if input.Description != "" {
		opt.Description = gitlabapi.Ptr(input.Description)
	}
	if input.TagMessage != "" {
		opt.TagMessage = gitlabapi.Ptr(input.TagMessage)
	}
	if len(input.Milestones) > 0 {
		milestones := append([]string(nil), input.Milestones...)
		opt.Milestones = &milestones
	}
	if input.ReleasedAt != nil {
		opt.ReleasedAt = input.ReleasedAt
	}
	if len(input.Links) > 0 {
		links := make([]*gitlabapi.ReleaseAssetLinkOptions, 0, len(input.Links))
		for _, link := range input.Links {
			links = append(links, releaseAssetLinkOptions(link))
		}
		opt.Assets = &gitlabapi.ReleaseAssetsOptions{Links: links}
	}
	release, _, err := c.client.Releases.CreateRelease(project, opt)
	if err != nil {
		return ReleaseDetail{}, err
	}
	return releaseDetailFromAPI(release), nil
}

func (c liveClient) UpdateRelease(project any, tag string, input ReleaseUpdateOptions) (ReleaseDetail, error) {
	opt := &gitlabapi.UpdateReleaseOptions{}
	if input.Name != nil {
		opt.Name = input.Name
	}
	if input.Description != nil {
		opt.Description = input.Description
	}
	if input.Milestones != nil {
		milestones := append([]string(nil), *input.Milestones...)
		opt.Milestones = &milestones
	}
	if input.ReleasedAt != nil {
		opt.ReleasedAt = input.ReleasedAt
	}
	release, _, err := c.client.Releases.UpdateRelease(project, tag, opt)
	if err != nil {
		return ReleaseDetail{}, err
	}
	return releaseDetailFromAPI(release), nil
}

func (c liveClient) DeleteRelease(project any, tag string) error {
	_, _, err := c.client.Releases.DeleteRelease(project, tag)
	return err
}

func (c liveClient) GetRepositoryTag(project any, tag string) (RepositoryTag, error) {
	result, _, err := c.client.Tags.GetTag(project, tag)
	if err != nil {
		return RepositoryTag{}, err
	}
	return repositoryTagFromAPI(result), nil
}

func (c liveClient) DeleteRepositoryTag(project any, tag string) error {
	_, err := c.client.Tags.DeleteTag(project, tag)
	return err
}

func (c liveClient) GenerateChangelog(project any, input ChangelogGenerateOptions) (string, error) {
	opt := gitlabapi.GenerateChangelogDataOptions{
		Version: gitlabapi.Ptr(input.Version),
	}
	if input.From != "" {
		opt.From = gitlabapi.Ptr(input.From)
	}
	if input.To != "" {
		opt.To = gitlabapi.Ptr(input.To)
	}
	if input.Date != nil {
		opt.Date = gitlabapi.Ptr(gitlabapi.ISOTime(*input.Date))
	}
	if input.Trailer != "" {
		opt.Trailer = gitlabapi.Ptr(input.Trailer)
	}
	if input.ConfigFile != "" {
		opt.ConfigFile = gitlabapi.Ptr(input.ConfigFile)
	}
	data, _, err := c.client.Repositories.GenerateChangelogData(project, opt)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}
	return data.Notes, nil
}

func (c liveClient) AddChangelog(project any, input ChangelogAddOptions) error {
	opt := &gitlabapi.AddChangelogOptions{
		Version: gitlabapi.Ptr(input.Version),
	}
	if input.Branch != "" {
		opt.Branch = gitlabapi.Ptr(input.Branch)
	}
	if input.File != "" {
		opt.File = gitlabapi.Ptr(input.File)
	}
	if input.From != "" {
		opt.From = gitlabapi.Ptr(input.From)
	}
	if input.To != "" {
		opt.To = gitlabapi.Ptr(input.To)
	}
	if input.Date != nil {
		opt.Date = gitlabapi.Ptr(gitlabapi.ISOTime(*input.Date))
	}
	if input.Message != "" {
		opt.Message = gitlabapi.Ptr(input.Message)
	}
	if input.Trailer != "" {
		opt.Trailer = gitlabapi.Ptr(input.Trailer)
	}
	if input.ConfigFile != "" {
		opt.ConfigFile = gitlabapi.Ptr(input.ConfigFile)
	}
	_, err := c.client.Repositories.AddChangelog(project, opt)
	return err
}

func (c liveClient) ListReleaseLinks(project any, tag string, limit int) ([]ReleaseLink, bool, error) {
	opt := &gitlabapi.ListReleaseLinksOptions{}
	opt.PerPage = int64(clampProjectPageSize(limit, 20))
	opt.Page = 1
	var out []ReleaseLink
	for {
		links, resp, err := c.client.ReleaseLinks.ListReleaseLinks(project, tag, opt)
		if err != nil {
			return nil, false, err
		}
		for _, link := range links {
			if limit > 0 && len(out) >= limit {
				return out, true, nil
			}
			out = append(out, releaseLinkFromAPI(link))
		}
		if resp == nil || resp.NextPage == 0 {
			return out, false, nil
		}
		opt.Page = resp.NextPage
	}
}

func (c liveClient) CreateReleaseLink(project any, tag string, input ReleaseLinkCreateOptions) (ReleaseLink, error) {
	opt := &gitlabapi.CreateReleaseLinkOptions{
		Name: gitlabapi.Ptr(input.Name),
		URL:  gitlabapi.Ptr(input.URL),
	}
	if input.DirectAssetPath != "" {
		opt.DirectAssetPath = gitlabapi.Ptr(input.DirectAssetPath)
	}
	if linkType := linkTypePtr(input.LinkType); linkType != nil {
		opt.LinkType = linkType
	}
	link, _, err := c.client.ReleaseLinks.CreateReleaseLink(project, tag, opt)
	if err != nil {
		return ReleaseLink{}, err
	}
	return releaseLinkFromAPI(link), nil
}

func (c liveClient) UpdateReleaseLink(project any, tag string, linkID int64, input ReleaseLinkUpdateOptions) (ReleaseLink, error) {
	opt := &gitlabapi.UpdateReleaseLinkOptions{}
	if input.Name != nil {
		opt.Name = input.Name
	}
	if input.URL != nil {
		opt.URL = input.URL
	}
	if input.DirectAssetPath != nil {
		opt.DirectAssetPath = input.DirectAssetPath
	}
	if input.LinkType != nil {
		if linkType := linkTypePtr(*input.LinkType); linkType != nil {
			opt.LinkType = linkType
		}
	}
	link, _, err := c.client.ReleaseLinks.UpdateReleaseLink(project, tag, linkID, opt)
	if err != nil {
		return ReleaseLink{}, err
	}
	return releaseLinkFromAPI(link), nil
}

func (c liveClient) DeleteReleaseLink(project any, tag string, linkID int64) error {
	_, _, err := c.client.ReleaseLinks.DeleteReleaseLink(project, tag, linkID)
	return err
}

func releaseAssetLinkOptions(link ReleaseLinkOption) *gitlabapi.ReleaseAssetLinkOptions {
	opt := &gitlabapi.ReleaseAssetLinkOptions{
		Name: gitlabapi.Ptr(link.Name),
		URL:  gitlabapi.Ptr(link.URL),
	}
	if link.DirectAssetPath != "" {
		opt.DirectAssetPath = gitlabapi.Ptr(link.DirectAssetPath)
	}
	if linkType := linkTypePtr(link.LinkType); linkType != nil {
		opt.LinkType = linkType
	}
	return opt
}

func linkTypePtr(value string) *gitlabapi.LinkTypeValue {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	linkType := gitlabapi.LinkTypeValue(value)
	return &linkType
}

func releaseDetailFromAPI(release *gitlabapi.Release) ReleaseDetail {
	if release == nil {
		return ReleaseDetail{}
	}
	milestones := make([]string, 0, len(release.Milestones))
	for _, milestone := range release.Milestones {
		if milestone != nil {
			milestones = append(milestones, milestone.Title)
		}
	}
	links := make([]ReleaseLink, 0, len(release.Assets.Links))
	for _, link := range release.Assets.Links {
		links = append(links, releaseLinkFromAPI(link))
	}
	return ReleaseDetail{
		TagName:     release.TagName,
		Name:        release.Name,
		Description: release.Description,
		WebURL:      release.Links.Self,
		Author:      release.Author.Username,
		CommitSHA:   release.Commit.ID,
		CreatedAt:   formatTime(release.CreatedAt),
		ReleasedAt:  formatTime(release.ReleasedAt),
		Upcoming:    release.UpcomingRelease,
		Milestones:  milestones,
		AssetLinks:  links,
	}
}

func releaseLinkFromAPI(link *gitlabapi.ReleaseLink) ReleaseLink {
	if link == nil {
		return ReleaseLink{}
	}
	return ReleaseLink{
		ID:             link.ID,
		Name:           link.Name,
		URL:            link.URL,
		DirectAssetURL: link.DirectAssetURL,
		LinkType:       string(link.LinkType),
		External:       link.External,
	}
}
