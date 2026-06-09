package asterisk

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// runAMIPing connects to an Asterisk Manager Interface over a stream dialed
// through the host conn capability, authenticates, and pings — so the plugin
// speaks the AMI line protocol itself while performing no direct network IO.
func runAMIPing(ctx pluginbinding.Context, input AMIPingInput) (AMIPingResult, error) {
	timeout, err := amiDuration(input.Timeout, 10*time.Second)
	if err != nil {
		return AMIPingResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	targetURL, err := amiTargetURL(ctx, input.AMITargetInput)
	if err != nil {
		return AMIPingResult{}, err
	}
	creds := amiCredentials(ctx, targetURL)
	out := AMIPingResult{EndpointRef: input.EndpointRef, URL: amiRedactURL(targetURL)}
	dialer, ok := ctx.Host.(pluginbinding.ConnDialer)
	if !ok {
		return out, pluginbinding.Fail("host_unavailable", "host does not support the conn dial capability required by asterisk")
	}
	network, address, err := amiDialAddress(targetURL)
	if err != nil {
		return out, pluginbinding.Errorf("bad_input", "%s", err)
	}
	start := time.Now()
	conn, err := pluginbinding.DialHostConn(context.Background(), dialer, pluginbinding.ConnDialRequest{
		Network:   network,
		Address:   address,
		TimeoutMS: int(timeout / time.Millisecond),
	})
	if err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	reader := bufio.NewReader(conn)
	greeting, err := reader.ReadString('\n')
	if err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "read AMI greeting: %s", err)
	}
	out.Greeting = strings.TrimSpace(greeting)

	if err := amiWriteAction(conn, map[string]string{
		"Action":   "Login",
		"Username": creds.username,
		"Secret":   creds.secret,
		"ActionID": "fp-login",
	}); err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	login, err := amiReadMessage(reader)
	if err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	out.Response = login["Response"]
	out.Message = login["Message"]
	if !strings.EqualFold(login["Response"], "Success") {
		out.DurationMS = time.Since(start).Milliseconds()
		return out, pluginbinding.Errorf("asterisk", "AMI login failed: %s", firstNonEmpty(login["Message"], login["Response"]))
	}
	out.Authenticated = true

	if err := amiWriteAction(conn, map[string]string{"Action": "Ping", "ActionID": "fp-ping"}); err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	pong, err := amiReadMessage(reader)
	if err != nil {
		out.Error = err.Error()
		return out, pluginbinding.Errorf("asterisk", "%s", err)
	}
	out.Response = pong["Response"]
	out.Message = firstNonEmpty(pong["Ping"], pong["Message"])
	out.Pong = strings.EqualFold(pong["Response"], "Success") && strings.EqualFold(pong["Ping"], "Pong")
	_ = amiWriteAction(conn, map[string]string{"Action": "Logoff", "ActionID": "fp-logoff"})
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

func amiWriteAction(conn net.Conn, fields map[string]string) error {
	for _, key := range []string{"Action", "Username", "Secret", "ActionID"} {
		if value := strings.TrimSpace(fields[key]); value != "" {
			if _, err := fmt.Fprintf(conn, "%s: %s\r\n", key, value); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprint(conn, "\r\n")
	return err
}

func amiReadMessage(reader *bufio.Reader) (map[string]string, error) {
	out := map[string]string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return out, err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			return out, nil
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
}
