package slack

import "testing"

func TestSlackScopeHeaderParsesCommaSeparatedScopes(t *testing.T) {
	scopes := slackScopeHeader(map[string][]string{
		"X-OAuth-Scopes": {"chat:write, bookmarks:read", "bookmarks:write"},
		"x-oauth-scopes": {"chat:write"},
	}, "X-Oauth-Scopes")
	if len(scopes) != 3 || scopes[0] != "chat:write" || scopes[1] != "bookmarks:read" || scopes[2] != "bookmarks:write" {
		t.Fatalf("scopes = %#v", scopes)
	}
}
