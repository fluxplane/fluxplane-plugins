package slack

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

func TestSlackScopeHeaderParsesCommaSeparatedScopes(t *testing.T) {
	scopes := slackScopeHeader(map[string][]string{
		"X-OAuth-Scopes": {"chat:write, bookmarks:read", "bookmarks:write"},
		"x-oauth-scopes": {"chat:write"},
	}, "X-Oauth-Scopes")
	if len(scopes) != 3 || scopes[0] != "chat:write" || scopes[1] != "bookmarks:read" || scopes[2] != "bookmarks:write" {
		t.Fatalf("scopes = %#v", scopes)
	}
}

// uploadRecordingHost answers the three-step external upload flow
// (files.getUploadURLExternal, the upload URL, files.completeUploadExternal)
// plus files.info, and records the form sent to files.getUploadURLExternal.
type uploadRecordingHost struct {
	fakeHostClient
	getUploadURLForm url.Values
}

func (h *uploadRecordingHost) HTTP(request pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	switch {
	case strings.HasSuffix(request.URL, "/files.getUploadURLExternal"):
		form, err := url.ParseQuery(string(request.Body))
		if err != nil {
			return pluginbinding.HTTPResponse{}, err
		}
		h.getUploadURLForm = form
		return jsonResponse(`{"ok":true,"upload_url":"https://files.example.com/upload/v1/F1","file_id":"F1"}`), nil
	case strings.HasPrefix(request.URL, "https://files.example.com/upload/"):
		return pluginbinding.HTTPResponse{StatusCode: 200, Body: []byte("OK - 5")}, nil
	case strings.HasSuffix(request.URL, "/files.completeUploadExternal"):
		return jsonResponse(`{"ok":true,"files":[{"id":"F1","title":"report.html"}]}`), nil
	case strings.HasSuffix(request.URL, "/files.info"):
		return jsonResponse(`{"ok":true,"file":{"id":"F1","permalink":"https://example.slack.com/files/U1/F1/report.html"}}`), nil
	}
	return pluginbinding.HTTPResponse{StatusCode: 404, Body: []byte("unexpected " + request.URL)}, nil
}

func jsonResponse(body string) pluginbinding.HTTPResponse {
	return pluginbinding.HTTPResponse{StatusCode: 200, Headers: map[string][]string{"Content-Type": {"application/json"}}, Body: []byte(body)}
}

func uploadThroughLiveClient(t *testing.T, request FileUploadRequest) url.Values {
	t.Helper()
	host := &uploadRecordingHost{}
	client, err := NewLiveClient(pluginbinding.Context{Host: host}, AuthPurposeBot)
	if err != nil {
		t.Fatalf("NewLiveClient: %v", err)
	}
	if _, err := client.UploadFile(context.Background(), request); err != nil {
		t.Fatalf("UploadFile: %v", err)
	}
	if host.getUploadURLForm == nil {
		t.Fatal("files.getUploadURLExternal was not called")
	}
	return host.getUploadURLForm
}

func TestLiveClientUploadFileSendsSnippetTypeForHTMLFilename(t *testing.T) {
	form := uploadThroughLiveClient(t, FileUploadRequest{Channel: "C1", Filename: "report.html", Content: []byte("<html></html>")})
	if got := form.Get("snippet_type"); got != "html" {
		t.Fatalf("files.getUploadURLExternal snippet_type = %q, want %q (form %v)", got, "html", form)
	}
}
