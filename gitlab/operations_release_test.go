package gitlab

import (
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

// Release-management fakes. These live on the base *fakeClient so
// the whole Client interface stays satisfied for every test in the package.

func (c *fakeClient) GetRelease(project any, tag string) (ReleaseDetail, error) {
	c.releaseTag = tag
	return c.releaseDetail, nil
}

func (c *fakeClient) CreateRelease(project any, input ReleaseCreateOptions) (ReleaseDetail, error) {
	c.releaseCreateProject = project
	c.releaseCreateOptions = input
	return c.releaseDetail, nil
}

func (c *fakeClient) UpdateRelease(project any, tag string, input ReleaseUpdateOptions) (ReleaseDetail, error) {
	c.releaseTag = tag
	c.releaseUpdateOptions = input
	return c.releaseDetail, nil
}

func (c *fakeClient) DeleteRelease(project any, tag string) error {
	c.releaseDeletedTag = tag
	return nil
}

func (c *fakeClient) GetRepositoryTag(project any, tag string) (RepositoryTag, error) {
	c.tagShowName = tag
	return c.repositoryTag, nil
}

func (c *fakeClient) DeleteRepositoryTag(project any, tag string) error {
	c.tagDeletedName = tag
	return nil
}

func (c *fakeClient) GenerateChangelog(project any, input ChangelogGenerateOptions) (string, error) {
	c.changelogGenerateOptions = input
	return c.changelogNotes, nil
}

func (c *fakeClient) AddChangelog(project any, input ChangelogAddOptions) error {
	c.changelogAddOptions = input
	return nil
}

func (c *fakeClient) ListReleaseLinks(project any, tag string, limit int) ([]ReleaseLink, bool, error) {
	c.releaseLinkTag = tag
	return c.releaseLinks, c.cicdTruncated, nil
}

func (c *fakeClient) CreateReleaseLink(project any, tag string, input ReleaseLinkCreateOptions) (ReleaseLink, error) {
	c.releaseLinkTag = tag
	c.releaseLinkCreateOptions = input
	return c.releaseLink, nil
}

func (c *fakeClient) UpdateReleaseLink(project any, tag string, linkID int64, input ReleaseLinkUpdateOptions) (ReleaseLink, error) {
	c.releaseLinkTag = tag
	c.releaseLinkUpdateID = linkID
	c.releaseLinkUpdateOptions = input
	return c.releaseLink, nil
}

func (c *fakeClient) DeleteReleaseLink(project any, tag string, linkID int64) error {
	c.releaseLinkTag = tag
	c.releaseLinkDeletedID = linkID
	return nil
}

func TestServiceReleaseCreate(t *testing.T) {
	client := &fakeClient{releaseDetail: ReleaseDetail{TagName: "v1.2.0", Name: "1.2.0", WebURL: "https://gitlab.example.com/group/app/-/releases/v1.2.0"}}
	plugin := testPlugin(client)

	out := plugintest.RunOK[ReleaseDetail](t, plugin, OperationReleaseCreate, map[string]any{
		"project":     "group/app",
		"tag_name":    "v1.2.0",
		"ref":         "main",
		"name":        "1.2.0",
		"description": "## Changelog\n- Fixed a bug",
		"milestones":  []any{"Sprint 1"},
		"assets_links": []any{
			map[string]any{"name": "Binary", "url": "https://example.com/app.zip", "link_type": "package"},
		},
	})
	if out.TagName != "v1.2.0" || out.WebURL == "" {
		t.Fatalf("release create = %#v", out)
	}
	opts := client.releaseCreateOptions
	if opts.TagName != "v1.2.0" || opts.Ref != "main" || opts.Name != "1.2.0" {
		t.Fatalf("create options = %#v", opts)
	}
	if len(opts.Milestones) != 1 || opts.Milestones[0] != "Sprint 1" {
		t.Fatalf("create milestones = %#v", opts.Milestones)
	}
	if len(opts.Links) != 1 || opts.Links[0].Name != "Binary" || opts.Links[0].LinkType != "package" {
		t.Fatalf("create links = %#v", opts.Links)
	}
}

func TestServiceReleaseShowUpdateDelete(t *testing.T) {
	client := &fakeClient{releaseDetail: ReleaseDetail{TagName: "v1.2.0", Name: "1.2.0"}}
	plugin := testPlugin(client)

	show := plugintest.RunOK[ReleaseDetail](t, plugin, OperationReleaseShow, map[string]any{"project": "group/app", "tag_name": "v1.2.0"})
	if show.TagName != "v1.2.0" || client.releaseTag != "v1.2.0" {
		t.Fatalf("release show = %#v tag=%q", show, client.releaseTag)
	}

	plugintest.RunOK[ReleaseDetail](t, plugin, OperationReleaseUpdate, map[string]any{
		"project": "group/app", "tag": "v1.2.0", "description": "## Changelog\n- Updated",
	})
	if client.releaseUpdateOptions.Description == nil || *client.releaseUpdateOptions.Description != "## Changelog\n- Updated" {
		t.Fatalf("update options = %#v", client.releaseUpdateOptions)
	}

	del := plugintest.RunOK[ReleaseActionResult](t, plugin, OperationReleaseDelete, map[string]any{"project": "group/app", "tag_name": "v1.2.0"})
	if del.TagName != "v1.2.0" || client.releaseDeletedTag != "v1.2.0" {
		t.Fatalf("release delete = %#v deleted=%q", del, client.releaseDeletedTag)
	}
}

func TestServiceChangelogOperations(t *testing.T) {
	client := &fakeClient{changelogNotes: "## 1.2.0\n- Fixed a bug"}
	plugin := testPlugin(client)

	notes := plugintest.RunOK[ChangelogNotes](t, plugin, OperationChangelogGenerate, map[string]any{
		"project": "group/app", "version": "1.2.0", "from": "v1.1.0", "to": "main",
	})
	if notes.Notes != "## 1.2.0\n- Fixed a bug" {
		t.Fatalf("changelog notes = %#v", notes)
	}
	if client.changelogGenerateOptions.Version != "1.2.0" || client.changelogGenerateOptions.From != "v1.1.0" {
		t.Fatalf("generate options = %#v", client.changelogGenerateOptions)
	}

	added := plugintest.RunOK[ChangelogAddResult](t, plugin, OperationChangelogAdd, map[string]any{
		"project": "group/app", "version": "1.2.0", "branch": "main",
	})
	if added.Version != "1.2.0" || added.Branch != "main" || added.File != "CHANGELOG.md" {
		t.Fatalf("changelog add = %#v", added)
	}
	if client.changelogAddOptions.Version != "1.2.0" || client.changelogAddOptions.Branch != "main" {
		t.Fatalf("add options = %#v", client.changelogAddOptions)
	}
}

func TestServiceTagShowDelete(t *testing.T) {
	client := &fakeClient{repositoryTag: RepositoryTag{Name: "v1.2.0", Target: "abc123"}}
	plugin := testPlugin(client)

	show := plugintest.RunOK[RepositoryTag](t, plugin, OperationTagShow, map[string]any{"project": "group/app", "tag_name": "v1.2.0"})
	if show.Name != "v1.2.0" || client.tagShowName != "v1.2.0" {
		t.Fatalf("tag show = %#v name=%q", show, client.tagShowName)
	}

	del := plugintest.RunOK[TagActionResult](t, plugin, OperationTagDelete, map[string]any{"project": "group/app", "name": "v1.2.0"})
	if del.TagName != "v1.2.0" || client.tagDeletedName != "v1.2.0" {
		t.Fatalf("tag delete = %#v deleted=%q", del, client.tagDeletedName)
	}
}

func TestServiceReleaseLinkOperations(t *testing.T) {
	client := &fakeClient{
		releaseLinks: []ReleaseLink{{ID: 7, Name: "Binary", URL: "https://example.com/app.zip", LinkType: "package"}},
		releaseLink:  ReleaseLink{ID: 7, Name: "Binary"},
	}
	plugin := testPlugin(client)

	list := plugintest.RunOK[pluginbinding.ListResult[ReleaseLink]](t, plugin, OperationReleaseLinkList, map[string]any{"project": "group/app", "tag_name": "v1.2.0"})
	if list.Count != 1 || list.Items[0].LinkType != "package" || client.releaseLinkTag != "v1.2.0" {
		t.Fatalf("link list = %#v tag=%q", list, client.releaseLinkTag)
	}

	created := plugintest.RunOK[ReleaseLink](t, plugin, OperationReleaseLinkCreate, map[string]any{
		"project": "group/app", "tag_name": "v1.2.0", "name": "Binary", "url": "https://example.com/app.zip", "link_type": "package",
	})
	if created.ID != 7 || client.releaseLinkCreateOptions.URL != "https://example.com/app.zip" {
		t.Fatalf("link create = %#v opts=%#v", created, client.releaseLinkCreateOptions)
	}

	plugintest.RunOK[ReleaseLink](t, plugin, OperationReleaseLinkUpdate, map[string]any{
		"project": "group/app", "tag_name": "v1.2.0", "link_id": 7, "name": "Binary (signed)",
	})
	if client.releaseLinkUpdateID != 7 || client.releaseLinkUpdateOptions.Name == nil || *client.releaseLinkUpdateOptions.Name != "Binary (signed)" {
		t.Fatalf("link update = id %d opts %#v", client.releaseLinkUpdateID, client.releaseLinkUpdateOptions)
	}

	del := plugintest.RunOK[ReleaseLinkActionResult](t, plugin, OperationReleaseLinkDelete, map[string]any{"project": "group/app", "tag_name": "v1.2.0", "link_id": 7})
	if del.LinkID != 7 || client.releaseLinkDeletedID != 7 {
		t.Fatalf("link delete = %#v deleted=%d", del, client.releaseLinkDeletedID)
	}
}

func TestServiceReleaseValidation(t *testing.T) {
	plugin := testPlugin(&fakeClient{})

	// project is required everywhere.
	plugintest.RunError(t, plugin, OperationReleaseCreate, map[string]any{"tag_name": "v1.2.0"})
	// tag_name is required for release create.
	plugintest.RunError(t, plugin, OperationReleaseCreate, map[string]any{"project": "group/app"})
	// version is required for changelog generate.
	plugintest.RunError(t, plugin, OperationChangelogGenerate, map[string]any{"project": "group/app"})
	// link create needs name and url.
	plugintest.RunError(t, plugin, OperationReleaseLinkCreate, map[string]any{"project": "group/app", "tag_name": "v1.2.0", "name": "Binary"})
	// link update needs a positive link_id.
	plugintest.RunError(t, plugin, OperationReleaseLinkUpdate, map[string]any{"project": "group/app", "tag_name": "v1.2.0", "name": "x"})
	// invalid released_at timestamp is rejected.
	plugintest.RunError(t, plugin, OperationReleaseCreate, map[string]any{"project": "group/app", "tag_name": "v1.2.0", "released_at": "not-a-time"})
}
