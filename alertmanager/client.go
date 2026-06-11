package alertmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Client struct {
	EndpointRef string
	Host        pluginbinding.HostClient
}

func (c Client) request(ctx context.Context, method, path string, values url.Values, body any, out any) error {
	_ = ctx
	endpointRef := strings.TrimSpace(c.EndpointRef)
	if endpointRef == "" {
		return fmt.Errorf("alertmanager endpoint_ref is required")
	}
	var payload []byte
	headers := map[string]string{"Accept": "application/json"}
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = data
		headers["Content-Type"] = "application/json"
	}
	resp, err := c.Host.HTTP(pluginbinding.HTTPRequest{
		EndpointRef: endpointRef,
		Path:        path,
		Query:       map[string][]string(values),
		Method:      method,
		Headers:     headers,
		Body:        payload,
		Auth: &pluginbinding.HTTPAuthRequest{
			UsernamePurpose: AuthPurposeBasicUsername,
			PasswordPurpose: AuthPurposeBasicPassword,
		},
		TimeoutMS: 30000,
		MaxBytes:  32 * 1024 * 1024,
		UserAgent: "fluxplane-plugin/0.1",
	})
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("alertmanager returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		if resp.Truncated {
			return fmt.Errorf("response exceeded the 32MB cap and was truncated — narrow the request with filter matchers")
		}
		return err
	}
	return nil
}

func (c Client) get(ctx context.Context, path string, values url.Values, out any) error {
	return c.request(ctx, "GET", path, values, nil, out)
}
