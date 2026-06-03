package openapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/getkin/kin-openapi/openapi3"
)

type OperationResult struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

func runOperation(ctx pluginbinding.Context, def operationDefinition, input map[string]any) (OperationResult, error) {
	req, err := bindRequestInput(input)
	if err != nil {
		return OperationResult{}, pluginbinding.Fail("invalid_"+def.Name+"_input", err.Error())
	}
	target, err := requestURL(def, req)
	if err != nil {
		return OperationResult{}, pluginbinding.Fail(def.Name+"_failed", err.Error())
	}
	body, contentType, err := requestBody(req.Body)
	if err != nil {
		return OperationResult{}, pluginbinding.Fail(def.Name+"_failed", err.Error())
	}
	headers := map[string]string{}
	for key, value := range req.Headers {
		if key = strings.TrimSpace(key); key != "" {
			headers[key] = scalarString(value)
		}
	}
	if contentType != "" && headers["Content-Type"] == "" {
		headers["Content-Type"] = contentType
	}
	if len(req.Cookies) > 0 {
		headers["Cookie"] = cookieHeader(req.Cookies)
	}
	authReq, err := applyAuth(ctx, def, &target, headers)
	if err != nil {
		return OperationResult{}, pluginbinding.Fail(def.Name+"_failed", err.Error())
	}
	resp, err := ctx.Host.HTTP(pluginbinding.HTTPRequest{
		URL:       target,
		Method:    def.Method,
		Headers:   headers,
		Body:      body,
		Auth:      authReq,
		MaxBytes:  10 * 1024 * 1024,
		UserAgent: "fluxplane-openapi-plugin/0.1",
	})
	if err != nil {
		return OperationResult{}, pluginbinding.Fail(def.Name+"_failed", err.Error())
	}
	return OperationResult{
		Status:  resp.StatusCode,
		Headers: responseHeaders(resp.Headers),
		Body:    decodeResponseBody(resp.ContentType, resp.Body),
	}, nil
}

func requestURL(def operationDefinition, in requestInput) (string, error) {
	if strings.TrimSpace(def.Server) == "" {
		return "", fmt.Errorf("openapi operation %s has no server URL", def.Name)
	}
	baseURL := strings.TrimRight(def.Server, "/") + "/"
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	path := def.Path
	for key, value := range in.Path {
		path = strings.ReplaceAll(path, "{"+key+"}", url.PathEscape(scalarString(value)))
	}
	rel, err := url.Parse(strings.TrimLeft(path, "/"))
	if err != nil {
		return "", err
	}
	target := base.ResolveReference(rel)
	q := target.Query()
	for key, value := range in.Query {
		q.Set(key, scalarString(value))
	}
	target.RawQuery = q.Encode()
	return target.String(), nil
}

func applyAuth(ctx pluginbinding.Context, def operationDefinition, target *string, headers map[string]string) (*pluginbinding.HTTPAuthRequest, error) {
	if len(def.Security) == 0 {
		return nil, nil
	}
	schemeNames := configuredSecuritySchemes(def.Security, def.AuthByScheme)
	if len(schemeNames) == 0 {
		return nil, fmt.Errorf("openapi operation %s requires auth but no configured security scheme matches", def.Name)
	}
	authReq := &pluginbinding.HTTPAuthRequest{HeaderPurposes: map[string]string{}}
	for _, schemeName := range schemeNames {
		scheme := def.SecuritySchemes[schemeName]
		if scheme == nil {
			continue
		}
		switch {
		case strings.EqualFold(scheme.Type, "http") && strings.EqualFold(scheme.Scheme, "bearer"):
			authReq.BearerTokenPurpose = schemeName
		case strings.EqualFold(scheme.Type, "http") && strings.EqualFold(scheme.Scheme, "basic"):
			authReq.PasswordPurpose = schemeName
		case strings.EqualFold(scheme.Type, "oauth2") || strings.EqualFold(scheme.Type, "openIdConnect"):
			authReq.BearerTokenPurpose = schemeName
		case strings.EqualFold(scheme.Type, "apiKey") && strings.EqualFold(scheme.In, "header"):
			authReq.HeaderPurposes[scheme.Name] = schemeName
		case strings.EqualFold(scheme.Type, "apiKey") && strings.EqualFold(scheme.In, "query"):
			value, err := secretValue(ctx, schemeName)
			if err != nil {
				return nil, err
			}
			if target == nil {
				return nil, fmt.Errorf("openapi operation %s cannot apply query auth without URL", def.Name)
			}
			u, err := url.Parse(*target)
			if err != nil {
				return nil, err
			}
			q := u.Query()
			q.Set(scheme.Name, value)
			u.RawQuery = q.Encode()
			*target = u.String()
		case strings.EqualFold(scheme.Type, "apiKey") && strings.EqualFold(scheme.In, "cookie"):
			value, err := secretValue(ctx, schemeName)
			if err != nil {
				return nil, err
			}
			if headers["Cookie"] != "" {
				headers["Cookie"] += "; "
			}
			headers["Cookie"] += (&http.Cookie{Name: scheme.Name, Value: value}).String()
		}
	}
	if len(authReq.HeaderPurposes) == 0 {
		authReq.HeaderPurposes = nil
	}
	if authReq.BearerTokenPurpose == "" && authReq.PasswordPurpose == "" && authReq.UsernamePurpose == "" && len(authReq.HeaderPurposes) == 0 {
		return nil, nil
	}
	return authReq, nil
}

func secretValue(ctx pluginbinding.Context, purpose string) (string, error) {
	material, err := ctx.Host.Secret(purpose)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(material.Material().String())
	if value == "" {
		value = strings.TrimSpace(material.Value)
	}
	if value == "" {
		return "", fmt.Errorf("openapi auth secret is not configured for scheme %s", purpose)
	}
	return value, nil
}

type requestInput struct {
	Path    map[string]any `json:"path,omitempty"`
	Query   map[string]any `json:"query,omitempty"`
	Headers map[string]any `json:"headers,omitempty"`
	Cookies map[string]any `json:"cookies,omitempty"`
	Body    any            `json:"body,omitempty"`
}

func bindRequestInput(input any) (requestInput, error) {
	var out requestInput
	if input == nil {
		return out, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return out, err
	}
	return out, nil
}

func requestBody(value any) ([]byte, string, error) {
	if value == nil {
		return nil, "", nil
	}
	switch typed := value.(type) {
	case string:
		return []byte(typed), "text/plain", nil
	case []byte:
		return typed, "application/octet-stream", nil
	default:
		data, err := json.Marshal(value)
		return data, "application/json", err
	}
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	case float32:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func configuredSecuritySchemes(requirements openapi3.SecurityRequirements, methods map[string]authBinding) []string {
	for _, requirement := range requirements {
		if len(requirement) == 0 {
			return nil
		}
		names := make([]string, 0, len(requirement))
		for name := range requirement {
			names = append(names, name)
		}
		for _, name := range names {
			if _, ok := methods[name]; !ok {
				return nil
			}
		}
		sort.Strings(names)
		return names
	}
	return nil
}

func responseHeaders(headers map[string][]string) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

func decodeResponseBody(contentType string, body []byte) any {
	media, _, err := mime.ParseMediaType(contentType)
	if err == nil && (media == "application/json" || strings.HasSuffix(media, "+json")) {
		var out any
		if json.Unmarshal(body, &out) == nil {
			return out
		}
	}
	if isText(contentType, body) {
		return string(body)
	}
	return base64.StdEncoding.EncodeToString(body)
}

func isText(contentType string, body []byte) bool {
	media, _, _ := mime.ParseMediaType(contentType)
	if strings.HasPrefix(media, "text/") {
		return true
	}
	return bytes.IndexByte(body, 0) < 0
}

func cookieHeader(cookies map[string]any) string {
	var values []string
	for key, value := range cookies {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values = append(values, (&http.Cookie{Name: key, Value: scalarString(value)}).String())
	}
	sort.Strings(values)
	return strings.Join(values, "; ")
}
