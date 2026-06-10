package asterisk

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// amiSession is an authenticated Asterisk Manager Interface connection dialed
// through the host conn capability. It speaks the AMI line protocol directly:
// actions go out as "Key: Value" blocks, responses and action-scoped events
// come back the same way.
type amiSession struct {
	conn     net.Conn
	reader   *bufio.Reader
	greeting string
	timeout  time.Duration
	actionID int
}

// dialAMISession connects and logs in, sending "Events: off" so the session
// only receives action responses and action-triggered event lists, never the
// unsolicited event firehose.
func dialAMISession(ctx pluginbinding.Context, target AMITargetInput, timeout time.Duration) (*amiSession, error) {
	targetURL, err := amiTargetURL(ctx, target)
	if err != nil {
		return nil, err
	}
	creds := amiCredentials(ctx, targetURL)
	dialer, ok := ctx.Host.(pluginbinding.ConnDialer)
	if !ok {
		return nil, pluginbinding.Fail("host_unavailable", "host does not support the conn dial capability required by asterisk")
	}
	network, address, err := amiDialAddress(targetURL)
	if err != nil {
		return nil, pluginbinding.Errorf("bad_input", "%s", err)
	}
	conn, err := pluginbinding.DialHostConn(context.Background(), dialer, pluginbinding.ConnDialRequest{
		Network:   network,
		Address:   address,
		TimeoutMS: int(timeout / time.Millisecond),
	})
	if err != nil {
		return nil, pluginbinding.Errorf("asterisk", "%s", err)
	}
	session := &amiSession{conn: conn, reader: bufio.NewReader(conn), timeout: timeout}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	greeting, err := session.reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, pluginbinding.Errorf("asterisk", "read AMI greeting: %s", err)
	}
	session.greeting = strings.TrimSpace(greeting)
	login, err := session.do(map[string]string{
		"Action":   "Login",
		"Username": creds.username,
		"Secret":   creds.secret,
		"Events":   "off",
	})
	if err != nil {
		_ = conn.Close()
		return nil, pluginbinding.Errorf("asterisk", "%s", err)
	}
	if !strings.EqualFold(login["Response"], "Success") {
		_ = conn.Close()
		return nil, pluginbinding.Errorf("asterisk", "AMI login failed: %s", firstNonEmpty(login["Message"], login["Response"]))
	}
	return session, nil
}

// Close logs off politely and closes the stream.
func (s *amiSession) Close() {
	if s == nil || s.conn == nil {
		return
	}
	_ = s.write(map[string]string{"Action": "Logoff"}, "")
	_ = s.conn.Close()
}

// do sends one action and returns its (single) response message.
func (s *amiSession) do(fields map[string]string) (map[string]string, error) {
	actionID := s.nextActionID()
	if err := s.write(fields, actionID); err != nil {
		return nil, err
	}
	for {
		message, err := s.readMessage()
		if err != nil {
			return nil, err
		}
		if id := message["ActionID"]; id != "" && id != actionID {
			continue // stale message from a previous action
		}
		if message["Response"] == "" && message["Event"] != "" {
			continue // unsolicited event (e.g. FullyBooted right after login)
		}
		return message, nil
	}
}

// collect sends one action whose answer is an event list: it returns the
// initial response plus every event up to (excluding) the named complete
// event. AMI flags list responses with "EventList: start".
func (s *amiSession) collect(fields map[string]string, completeEvents ...string) (map[string]string, []map[string]string, error) {
	actionID := s.nextActionID()
	if err := s.write(fields, actionID); err != nil {
		return nil, nil, err
	}
	complete := map[string]bool{}
	for _, name := range completeEvents {
		complete[strings.ToLower(name)] = true
	}
	var response map[string]string
	var events []map[string]string
	for {
		message, err := s.readMessage()
		if err != nil {
			return response, events, err
		}
		if id := message["ActionID"]; id != "" && id != actionID {
			continue
		}
		event := strings.ToLower(message["Event"])
		switch {
		case response == nil && message["Response"] != "":
			if !strings.EqualFold(message["Response"], "Success") {
				return message, nil, fmt.Errorf("AMI %s failed: %s", fields["Action"], firstNonEmpty(message["Message"], message["Response"]))
			}
			response = message
		case complete[event]:
			return response, events, nil
		case event != "":
			events = append(events, message)
		}
	}
}

// command runs an Asterisk CLI command and returns its raw output. Modern
// Asterisk answers with repeated "Output:" headers; pre-13 versions answer
// "Response: Follows" with raw text terminated by "--END COMMAND--", which
// readMessage folds into the synthetic "CommandOutput" key.
func (s *amiSession) command(commandLine string) (string, error) {
	response, err := s.do(map[string]string{"Action": "Command", "Command": commandLine})
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(response["Response"], "Success") && !strings.EqualFold(response["Response"], "Follows") {
		return "", fmt.Errorf("AMI command failed: %s", firstNonEmpty(response["Message"], response["Response"]))
	}
	return firstNonEmpty(response["Output"], response["CommandOutput"]), nil
}

// write sends one action block. Action goes first and ActionID second so the
// block reads naturally in AMI logs; remaining keys follow sorted.
func (s *amiSession) write(fields map[string]string, actionID string) error {
	_ = s.conn.SetWriteDeadline(time.Now().Add(s.timeout))
	var b strings.Builder
	writeField := func(key, value string) {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(value)
		b.WriteString("\r\n")
	}
	writeField("Action", fields["Action"])
	if actionID != "" {
		writeField("ActionID", actionID)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key == "Action" || key == "ActionID" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := fields[key]; strings.TrimSpace(value) != "" {
			writeField(key, value)
		}
	}
	b.WriteString("\r\n")
	_, err := s.conn.Write([]byte(b.String()))
	return err
}

// readMessage reads one "Key: Value" block up to a blank line. Repeated keys
// (Command output) are joined with newlines; "Response: Follows" raw output is
// captured under the synthetic "CommandOutput" key.
func (s *amiSession) readMessage() (map[string]string, error) {
	_ = s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	out := map[string]string{}
	var follows, followsBody bool
	var followsOutput []string
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return out, err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			if follows && out["CommandOutput"] == "" {
				out["CommandOutput"] = strings.Join(followsOutput, "\n")
			}
			return out, nil
		}
		if follows {
			if strings.TrimSpace(line) == "--END COMMAND--" {
				out["CommandOutput"] = strings.Join(followsOutput, "\n")
				followsBody = false
				continue
			}
			// After the known headers everything is raw command output, even
			// lines that happen to contain colons.
			if followsBody || !isFollowsHeader(line) {
				followsBody = true
				followsOutput = append(followsOutput, line)
				continue
			}
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "Response" && strings.EqualFold(value, "Follows") {
			follows = true
		}
		if existing, exists := out[key]; exists {
			out[key] = existing + "\n" + value
		} else {
			out[key] = value
		}
	}
}

// isFollowsHeader reports whether a line inside a "Response: Follows" block is
// still one of the protocol headers rather than raw command output.
func isFollowsHeader(line string) bool {
	key, _, ok := strings.Cut(line, ":")
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "actionid", "privilege", "message":
		return true
	default:
		return false
	}
}

func (s *amiSession) nextActionID() string {
	s.actionID++
	return fmt.Sprintf("fp-%d", s.actionID)
}
