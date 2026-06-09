// Package confluence implements a Fluxplane plugin against the Atlassian Cloud
// Confluence REST API.
//
// All HTTP paths in this file target the v1 API surface
// (`/wiki/rest/api/...`). Atlassian has announced sunset plans for several
// v1 content endpoints in favor of v2 (`/wiki/api/v2/...`). Migrating is
// tracked as a follow-up; the v1 endpoints used here are still supported on
// Confluence Cloud at the time of writing.
package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Client interface {
	CurrentUser(context.Context) (User, error)
	SearchPages(context.Context, PageSearchOptions) ([]Page, error)
	ListPages(context.Context, PageSearchOptions) (PageList, error)
	GetPage(context.Context, string) (Page, error)
	CreatePage(context.Context, PageCreateRequest) (PageMutationResult, error)
	UpdatePage(context.Context, string, PageUpdateRequest) (PageMutationResult, error)
	DeletePage(context.Context, string) (PageMutationResult, error)
	ListComments(context.Context, string, CommentListOptions) (CommentList, error)
	CreateComment(context.Context, string, string) (Comment, error)
	UploadPageAttachment(context.Context, string, AttachmentUploadRequest) (AttachmentUploadResult, error)
	ListPageAttachments(context.Context, string) (AttachmentListResult, error)
	GetAttachment(context.Context, string, string, bool) (AttachmentGetResult, error)
	DeleteAttachment(context.Context, string) (AttachmentDeleteResult, error)
	SearchUsers(context.Context, UserSearchOptions) ([]User, error)
	GetUser(context.Context, string) (User, error)
}

type ClientFactory func(pluginbinding.Context, string) (Client, error)

func NewLiveClient(ctx pluginbinding.Context, endpointRef string) (Client, error) {
	endpointRef = strings.TrimSpace(endpointRef)
	client := liveClient{endpointRef: endpointRef, host: ctx.Host}
	if ctx.Host != nil {
		if material, err := ctx.Host.Secret(AuthPurposeCloudID); err == nil {
			if cloudID := strings.TrimSpace(material.Value); cloudID != "" {
				client.baseURL = "https://api.atlassian.com/ex/confluence/" + url.PathEscape(cloudID)
			}
		}
	}
	if endpointRef == "" && strings.TrimSpace(client.baseURL) == "" {
		return nil, fmt.Errorf("confluence endpoint_ref or cloud_id is required")
	}
	return client, nil
}

type liveClient struct {
	endpointRef string
	baseURL     string
	host        pluginbinding.HostClient
}

func (c liveClient) CurrentUser(ctx context.Context) (User, error) {
	var out User
	err := c.getJSON(ctx, "/wiki/rest/api/user/current", nil, &out)
	return out, err
}

func (c liveClient) SearchPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	if shouldUsePageList(input) {
		return c.listPages(ctx, input)
	}
	return c.searchPages(ctx, input)
}

func (c liveClient) listPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("status", defaultString(input.Status, "current"))
	query.Set("type", "page")
	if input.Title != "" {
		query.Set("title", input.Title)
	}
	if input.SpaceKey != "" {
		query.Set("spaceKey", input.SpaceKey)
	}
	if input.Cursor != "" {
		query.Set("start", input.Cursor)
	}
	var out []Page
	for {
		var page pageListResponse
		if err := c.getJSON(ctx, "/wiki/rest/api/content", query, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Results...)
		next := startFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("start", next)
	}
}

func (c liveClient) searchPages(ctx context.Context, input PageSearchOptions) ([]Page, error) {
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("cql", pageCQL(input))
	query.Set("limit", strconv.Itoa(limit))
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var out []Page
	for {
		var page searchResponse
		if err := c.getJSON(ctx, "/wiki/rest/api/search", query, &page); err != nil {
			return nil, err
		}
		out = append(out, pagesFromSearch(page.Results)...)
		next := cursorFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("cursor", next)
	}
}

// ListPages returns a single page of results (unlike listPages, which
// aggregates when All is set) so callers can surface pagination signals.
func (c liveClient) ListPages(ctx context.Context, input PageSearchOptions) (PageList, error) {
	limit := clamp(input.Limit, 25, 100)
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("status", defaultString(input.Status, "current"))
	query.Set("type", "page")
	query.Set("expand", "version,space")
	if input.Title != "" {
		query.Set("title", input.Title)
	}
	if input.SpaceKey != "" {
		query.Set("spaceKey", input.SpaceKey)
	}
	if input.Cursor != "" {
		query.Set("start", input.Cursor)
	}
	var out pageListResponse
	if err := c.getJSON(ctx, "/wiki/rest/api/content", query, &out); err != nil {
		return PageList{}, err
	}
	return PageList{Pages: out.Results, NextStart: startFromNext(out.Links.Next)}, nil
}

func (c liveClient) GetPage(ctx context.Context, id string) (Page, error) {
	query := url.Values{}
	query.Set("expand", "body.storage,version,space,ancestors")
	var out Page
	err := c.getJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(strings.TrimSpace(id)), query, &out)
	return out, err
}

func (c liveClient) CreatePage(ctx context.Context, request PageCreateRequest) (PageMutationResult, error) {
	payload := map[string]any{
		"type":  "page",
		"title": strings.TrimSpace(request.Title),
		"space": map[string]string{"key": strings.TrimSpace(request.SpaceKey)},
		"body": map[string]any{
			"storage": map[string]string{
				"value":          request.BodyStorage,
				"representation": "storage",
			},
		},
	}
	if parentID := strings.TrimSpace(request.ParentID); parentID != "" {
		payload["ancestors"] = []map[string]string{{"id": parentID}}
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return PageMutationResult{}, err
	}
	var created Page
	if err := c.doJSON(ctx, "POST", "/wiki/rest/api/content", nil, bytes.NewReader(data), &created); err != nil {
		return PageMutationResult{}, err
	}
	out := PageMutationResult{OK: true, ID: created.ID, Page: &created}
	if page, err := c.GetPage(ctx, created.ID); err == nil {
		out.Page = &page
	}
	return out, nil
}

// UpdatePage reads the current page to learn the version (and to preserve
// title or body when the caller leaves them empty), then PUTs the next
// version. Confluence v1 replaces the whole content on PUT, so the body is
// always resent.
func (c liveClient) UpdatePage(ctx context.Context, id string, request PageUpdateRequest) (PageMutationResult, error) {
	id = strings.TrimSpace(id)
	current, err := c.GetPage(ctx, id)
	if err != nil {
		return PageMutationResult{}, err
	}
	body := request.BodyStorage
	if strings.TrimSpace(body) == "" && current.Body != nil && current.Body.Storage != nil {
		body = current.Body.Storage.Value
	}
	payload := map[string]any{
		"id":      id,
		"type":    "page",
		"title":   firstNonEmpty(request.Title, current.Title),
		"version": map[string]any{"number": current.Version.Number + 1},
		"body": map[string]any{
			"storage": map[string]string{
				"value":          body,
				"representation": "storage",
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return PageMutationResult{}, err
	}
	var updated Page
	if err := c.doJSON(ctx, "PUT", "/wiki/rest/api/content/"+url.PathEscape(id), nil, bytes.NewReader(data), &updated); err != nil {
		return PageMutationResult{}, err
	}
	out := PageMutationResult{OK: true, ID: id, Page: &updated}
	if page, err := c.GetPage(ctx, id); err == nil {
		out.Page = &page
	}
	return out, nil
}

func (c liveClient) DeletePage(ctx context.Context, id string) (PageMutationResult, error) {
	id = strings.TrimSpace(id)
	if err := c.doJSON(ctx, "DELETE", "/wiki/rest/api/content/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return PageMutationResult{}, err
	}
	return PageMutationResult{OK: true, ID: id}, nil
}

func (c liveClient) UploadPageAttachment(ctx context.Context, pageID string, request AttachmentUploadRequest) (AttachmentUploadResult, error) {
	pageID = strings.TrimSpace(pageID)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", firstNonEmpty(request.Filename, "attachment"))
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	if _, err := part.Write(request.Data); err != nil {
		return AttachmentUploadResult{}, err
	}
	if err := writer.Close(); err != nil {
		return AttachmentUploadResult{}, err
	}
	var out attachmentUploadResponse
	err = c.do(ctx, "POST", "/wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/attachment", nil, &body, map[string]string{
		"Accept":            "application/json",
		"Content-Type":      writer.FormDataContentType(),
		"X-Atlassian-Token": "no-check",
	}, &out)
	if err != nil {
		return AttachmentUploadResult{}, err
	}
	return AttachmentUploadResult{OK: true, PageID: pageID, Attachments: out.Results}, nil
}

func (c liveClient) ListPageAttachments(ctx context.Context, pageID string) (AttachmentListResult, error) {
	pageID = strings.TrimSpace(pageID)
	query := url.Values{}
	query.Set("limit", "100")
	var out attachmentListResponse
	err := c.getJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/attachment", query, &out)
	if err != nil {
		return AttachmentListResult{}, err
	}
	return AttachmentListResult{PageID: pageID, Count: len(out.Results), Attachments: out.Results}, nil
}

func (c liveClient) GetAttachment(ctx context.Context, id, pageID string, downloadContent bool) (AttachmentGetResult, error) {
	id = strings.TrimSpace(id)
	var attachment Attachment
	if err := c.getJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(id), nil, &attachment); err != nil {
		return AttachmentGetResult{}, err
	}
	download := confluenceDownloadURL(attachment)
	if download == "" || !downloadContent {
		return AttachmentGetResult{ID: id, Filename: firstNonEmpty(attachment.Filename, attachment.Title), MimeType: attachment.MediaType, Attachment: attachment}, nil
	}
	if parentID := firstNonEmpty(pageID, confluenceAttachmentPageID(attachment)); parentID != "" {
		download = "/wiki/rest/api/content/" + url.PathEscape(parentID) + "/child/attachment/" + url.PathEscape(id) + "/download"
	}
	data, contentType, err := c.getBytes(ctx, download, nil)
	if err != nil {
		return AttachmentGetResult{}, err
	}
	return AttachmentGetResult{ID: id, Filename: firstNonEmpty(attachment.Filename, attachment.Title), MimeType: firstNonEmpty(attachment.MediaType, contentType), Size: len(data), ContentBytes: data, Attachment: attachment}, nil
}

func (c liveClient) DeleteAttachment(ctx context.Context, id string) (AttachmentDeleteResult, error) {
	id = strings.TrimSpace(id)
	if err := c.doJSON(ctx, "DELETE", "/wiki/rest/api/content/"+url.PathEscape(id), nil, nil, nil); err != nil {
		return AttachmentDeleteResult{}, err
	}
	return AttachmentDeleteResult{OK: true, AttachmentID: id}, nil
}

// ListComments returns one page of a page's comments (footer and inline,
// including replies via depth=all), oldest first.
func (c liveClient) ListComments(ctx context.Context, pageID string, options CommentListOptions) (CommentList, error) {
	pageID = strings.TrimSpace(pageID)
	limit := clamp(options.Limit, 25, 100)
	query := url.Values{}
	query.Set("limit", strconv.Itoa(limit))
	query.Set("depth", "all")
	query.Set("expand", "body.storage,version,history,extensions.location")
	if start := strings.TrimSpace(options.Start); start != "" {
		query.Set("start", start)
	}
	var out commentListResponse
	if err := c.getJSON(ctx, "/wiki/rest/api/content/"+url.PathEscape(pageID)+"/child/comment", query, &out); err != nil {
		return CommentList{}, err
	}
	comments := make([]Comment, 0, len(out.Results))
	for _, raw := range out.Results {
		comments = append(comments, commentFromAPI(raw))
	}
	return CommentList{Comments: comments, NextStart: startFromNext(out.Links.Next)}, nil
}

func (c liveClient) CreateComment(ctx context.Context, pageID, bodyStorage string) (Comment, error) {
	pageID = strings.TrimSpace(pageID)
	payload := map[string]any{
		"type":      "comment",
		"container": map[string]string{"id": pageID, "type": "page"},
		"body": map[string]any{
			"storage": map[string]string{
				"value":          bodyStorage,
				"representation": "storage",
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Comment{}, err
	}
	var created apiComment
	if err := c.doJSON(ctx, "POST", "/wiki/rest/api/content", nil, bytes.NewReader(data), &created); err != nil {
		return Comment{}, err
	}
	return commentFromAPI(created), nil
}

func (c liveClient) GetUser(ctx context.Context, accountID string) (User, error) {
	query := url.Values{}
	query.Set("accountId", strings.TrimSpace(accountID))
	var out User
	err := c.getJSON(ctx, "/wiki/rest/api/user", query, &out)
	return out, err
}

func (c liveClient) SearchUsers(ctx context.Context, input UserSearchOptions) ([]User, error) {
	if strings.TrimSpace(input.Query) == "" && strings.TrimSpace(input.CQL) == "" {
		user, err := c.CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		return []User{user}, nil
	}
	limit := clamp(input.Limit, 20, 100)
	query := url.Values{}
	query.Set("cql", userCQL(input))
	query.Set("limit", strconv.Itoa(limit))
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	var out []User
	for {
		var page searchResponse
		if err := c.getJSON(ctx, "/wiki/rest/api/search", query, &page); err != nil {
			return nil, err
		}
		out = append(out, usersFromSearch(page.Results)...)
		next := cursorFromNext(page.Links.Next)
		if !input.All || next == "" {
			return out, nil
		}
		query.Set("cursor", next)
	}
}

func (c liveClient) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.doJSON(ctx, "GET", path, query, nil, out)
}

func (c liveClient) doJSON(ctx context.Context, method, path string, query url.Values, body io.Reader, out any) error {
	headers := map[string]string{"Accept": "application/json"}
	if body != nil {
		headers["Content-Type"] = "application/json"
	}
	return c.do(ctx, method, path, query, body, headers, out)
}

func (c liveClient) do(ctx context.Context, method, path string, query url.Values, body io.Reader, headers map[string]string, out any) error {
	_ = ctx
	if c.host == nil {
		return fmt.Errorf("host client is unavailable")
	}
	var payload []byte
	if body != nil {
		data, err := io.ReadAll(io.LimitReader(body, 256*1024*1024))
		if err != nil {
			return err
		}
		payload = data
	}
	requestURL := ""
	endpointRef := c.endpointRef
	if strings.TrimSpace(c.baseURL) != "" {
		requestURL = strings.TrimRight(c.baseURL, "/")
		endpointRef = ""
		path = "/" + strings.TrimLeft(path, "/")
	}
	resp, err := c.host.HTTP(pluginbinding.HTTPRequest{
		URL:         requestURL,
		EndpointRef: endpointRef,
		Path:        path,
		Query:       map[string][]string(query),
		Method:      method,
		Headers:     headers,
		Body:        payload,
		Auth: &pluginbinding.HTTPAuthRequest{
			BearerTokenPurpose: AuthPurposeAPIToken,
		},
		TimeoutMS: 30000,
		MaxBytes:  64 * 1024 * 1024,
		UserAgent: "fluxplane-plugin/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return confluenceHTTPError(resp.StatusCode, resp.Body)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Body, out)
}

func (c liveClient) getBytes(ctx context.Context, path string, query url.Values) ([]byte, string, error) {
	_ = ctx
	if c.host == nil {
		return nil, "", fmt.Errorf("host client is unavailable")
	}
	resp, err := c.host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: c.endpointRef,
		Path:        path,
		Query:       map[string][]string(query),
		Method:      "GET",
		Auth: &pluginbinding.HTTPAuthRequest{
			BearerTokenPurpose: AuthPurposeAPIToken,
		},
		TimeoutMS: 30000,
		MaxBytes:  256 * 1024 * 1024,
		UserAgent: "fluxplane-plugin/0.1",
	})
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", confluenceHTTPError(resp.StatusCode, resp.Body)
	}
	return resp.Body, resp.ContentType, nil
}

func confluenceHTTPError(status int, data []byte) error {
	message := strings.TrimSpace(string(data))
	var payload struct {
		Message       string   `json:"message"`
		ErrorMessages []string `json:"errorMessages"`
	}
	if err := json.Unmarshal(data, &payload); err == nil {
		switch {
		case strings.TrimSpace(payload.Message) != "":
			message = strings.TrimSpace(payload.Message)
		case len(payload.ErrorMessages) > 0:
			message = strings.Join(payload.ErrorMessages, "; ")
		}
	}
	if message == "" {
		message = "request failed"
	}
	return fmt.Errorf("confluence returned status %d: %s", status, message)
}

func shouldUsePageList(input PageSearchOptions) bool {
	return strings.TrimSpace(input.Query) == "" && strings.TrimSpace(input.CQL) == ""
}

func clamp(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		return max
	}
	return value
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
