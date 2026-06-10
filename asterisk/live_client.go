package asterisk

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// runAMIPing opens an authenticated AMI session (dialed through the host conn
// capability) and pings. The session logs in with "Events: off" and matches
// responses by ActionID, so the unsolicited FullyBooted/SuccessfulAuth events
// modern Asterisk pushes right after login never masquerade as the pong.
func runAMIPing(ctx pluginbinding.Context, session *amiSession, sessionErr error, input AMIPingInput, start time.Time) (AMIPingResult, error) {
	out := AMIPingResult{EndpointRef: input.EndpointRef}
	if targetURL, err := amiTargetURL(ctx, input.AMITargetInput); err == nil {
		out.URL = amiRedactURL(targetURL)
	}
	if sessionErr != nil {
		out.Error = sessionErr.Error()
		return out, sessionErr
	}
	defer session.Close()
	out.Greeting = session.greeting
	out.Authenticated = true
	pong, err := session.do(map[string]string{"Action": "Ping"})
	if err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	out.Response = pong["Response"]
	out.Message = firstNonEmpty(pong["Ping"], pong["Message"])
	out.Pong = strings.EqualFold(pong["Response"], "Success") && strings.EqualFold(pong["Ping"], "Pong")
	out.OK = out.Pong
	out.DurationMS = time.Since(start).Milliseconds()
	if !out.Pong {
		return out, pluginbinding.Errorf("asterisk", "AMI ping failed: %s", firstNonEmpty(out.Message, out.Response))
	}
	return out, nil
}

type amiCreds struct {
	username string
	secret   string
}

// amiCredentials resolves AMI credentials from the endpoint URL userinfo, then
// falls back to host-resolved secrets (username/secret purposes).
func amiCredentials(ctx pluginbinding.Context, rawURL string) amiCreds {
	creds := amiCreds{}
	if parsed, err := url.Parse(rawURL); err == nil && parsed.User != nil {
		creds.username = parsed.User.Username()
		if pass, ok := parsed.User.Password(); ok {
			creds.secret = pass
		}
	}
	if ctx.Host == nil {
		return creds
	}
	if creds.username == "" {
		if material, err := ctx.Host.Secret("username"); err == nil {
			creds.username = strings.TrimSpace(material.Value)
		}
	}
	if creds.secret == "" {
		if material, err := ctx.Host.Secret("secret"); err == nil {
			creds.secret = strings.TrimSpace(material.Value)
		} else if material, err := ctx.Host.Secret("password"); err == nil {
			creds.secret = strings.TrimSpace(material.Value)
		}
	}
	return creds
}

func amiTargetURL(ctx pluginbinding.Context, input AMITargetInput) (string, error) {
	if raw := strings.TrimSpace(input.URL); raw != "" {
		return normalizeAMIURL(raw), nil
	}
	if ctx.Host == nil {
		return "", pluginbinding.Fail("host_unavailable", "host client is unavailable")
	}
	endpoint, err := ctx.Host.ResolveEndpoint(strings.TrimSpace(input.EndpointRef))
	if err != nil {
		return "", pluginbinding.Errorf("asterisk", "resolve endpoint: %s", err)
	}
	if strings.TrimSpace(endpoint.URL) == "" {
		return "", pluginbinding.Fail("bad_input", "endpoint has no url")
	}
	return normalizeAMIURL(endpoint.URL), nil
}

func amiDialAddress(rawURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", err
	}
	if parsed.Scheme != "ami" && parsed.Scheme != "tcp" {
		return "", "", fmt.Errorf("unsupported AMI endpoint scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", "", fmt.Errorf("AMI endpoint host is required")
	}
	port := parsed.Port()
	if port == "" {
		port = "5038"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", "", fmt.Errorf("invalid AMI endpoint port %q", port)
	}
	return "tcp", net.JoinHostPort(host, port), nil
}

func normalizeAMIURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "ami://" + rawURL
	}
	return rawURL
}

// amiRedactURL strips any password from the URL userinfo for safe reporting.
func amiRedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.User == nil {
		return rawURL
	}
	if _, ok := parsed.User.Password(); ok {
		parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	}
	return parsed.String()
}

func amiDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(strings.TrimSpace(value))
}
