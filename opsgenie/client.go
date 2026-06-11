package opsgenie

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

// request performs one Opsgenie API call. Auth is the GenieKey scheme, which
// the host's bearer injection cannot compose — the key is read from the
// persisted secret store and set as the Authorization header here (never from
// the environment at invoke time).
func (c Client) request(ctx context.Context, method, path string, values url.Values, body any, out any) error {
	_ = ctx
	if c.Host == nil {
		return fmt.Errorf("host client is unavailable")
	}
	material, err := c.Host.Secret(AuthPurposeAPIKey)
	if err != nil {
		return fmt.Errorf("opsgenie api key is not connected — run: fluxplane-plugin auth connect opsgenie --field api_key=<key> (%s)", err)
	}
	key := strings.TrimSpace(material.Value)
	if key == "" {
		return fmt.Errorf("opsgenie api key is not connected — run: fluxplane-plugin auth connect opsgenie --field api_key=<key>")
	}
	var payload []byte
	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": "GenieKey " + key,
	}
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = data
		headers["Content-Type"] = "application/json"
	}
	request := pluginbinding.HTTPRequest{
		Path:      path,
		Query:     map[string][]string(values),
		Method:    method,
		Headers:   headers,
		Body:      payload,
		TimeoutMS: 30000,
		MaxBytes:  16 * 1024 * 1024,
		UserAgent: "fluxplane-plugin/0.1",
	}
	if endpointRef := strings.TrimSpace(c.EndpointRef); endpointRef != "" {
		request.EndpointRef = endpointRef
	} else {
		request.URL = DefaultAPIURL
	}
	resp, err := c.Host.HTTP(request)
	if err != nil {
		return err
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("opsgenie rejected the api key (status %d): %s — check the key's permissions or reconnect with auth connect opsgenie", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("opsgenie returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(resp.Body, out)
}

func (c Client) get(ctx context.Context, path string, values url.Values, out any) error {
	return c.request(ctx, "GET", path, values, nil, out)
}
