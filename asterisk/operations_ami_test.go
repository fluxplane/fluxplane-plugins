package asterisk

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

// testAMIService returns a Service whose AMI sessions speak to an in-memory
// scripted server: respond receives the parsed action fields and returns the
// raw wire response ({{ACTIONID}} is substituted).
func testAMIService(respond func(action map[string]string) string) Service {
	return Service{DialAMI: func(_ pluginbinding.Context, _ AMITargetInput, timeout time.Duration) (*amiSession, error) {
		server, client := net.Pipe()
		go func() {
			defer func() { _ = server.Close() }()
			reader := bufio.NewReader(server)
			for {
				fields := map[string]string{}
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					line = strings.TrimRight(line, "\r\n")
					if strings.TrimSpace(line) == "" {
						break
					}
					if key, value, ok := strings.Cut(line, ":"); ok {
						fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
					}
				}
				if strings.EqualFold(fields["Action"], "Logoff") {
					return
				}
				response := strings.ReplaceAll(respond(fields), "{{ACTIONID}}", fields["ActionID"])
				if _, err := server.Write([]byte(response)); err != nil {
					return
				}
			}
		}()
		return &amiSession{conn: client, reader: bufio.NewReader(client), timeout: timeout}, nil
	}}
}

func wire(lines ...string) string {
	return strings.Join(lines, "\r\n") + "\r\n"
}

func TestChannelListParsesCoreShowChannels(t *testing.T) {
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		if action["Action"] != "CoreShowChannels" {
			t.Fatalf("unexpected action %q", action["Action"])
		}
		return wire(
			"Response: Success", "ActionID: {{ACTIONID}}", "EventList: start", "Message: Channels will follow", "",
			"Event: CoreShowChannel", "ActionID: {{ACTIONID}}", "Channel: PJSIP/agent-7-00000123", "Uniqueid: 1717920000.123",
			"ChannelStateDesc: Up", "CallerIDNum: 7000", "CallerIDName: Agent Seven", "ConnectedLineNum: 4930123456",
			"Context: from-internal", "Exten: 100", "Application: Queue", "ApplicationData: support",
			"Duration: 00:02:13", "BridgeId: b-1", "AccountCode: acme", "",
			"Event: CoreShowChannelsComplete", "ActionID: {{ACTIONID}}", "EventList: Complete", "ListItems: 1", "",
		)
	}))

	out := plugintest.RunOK[ChannelListResult](t, plugin, OperationChannelList, map[string]any{"url": "ami://pbx:5038"})
	if out.Count != 1 {
		t.Fatalf("channel list = %#v", out)
	}
	channel := out.Channels[0]
	if channel.Channel != "PJSIP/agent-7-00000123" || channel.State != "Up" || channel.Application != "Queue" || channel.Duration != "00:02:13" {
		t.Fatalf("channel = %#v", channel)
	}
}

func TestPeerListParsesPJSIPAndSIP(t *testing.T) {
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		switch action["Action"] {
		case "PJSIPShowEndpoints":
			return wire(
				"Response: Success", "ActionID: {{ACTIONID}}", "EventList: start", "",
				"Event: EndpointList", "ActionID: {{ACTIONID}}", "ObjectType: endpoint", "ObjectName: agent-7",
				"Contacts: agent-7/sip:agent-7@10.0.0.9:5060", "DeviceState: Not in use", "ActiveChannels: 0", "",
				"Event: EndpointListComplete", "ActionID: {{ACTIONID}}", "EventList: Complete", "ListItems: 1", "",
			)
		case "SIPpeers":
			return wire(
				"Response: Success", "ActionID: {{ACTIONID}}", "Message: Peer status list will follow", "",
				"Event: PeerEntry", "ActionID: {{ACTIONID}}", "Channeltype: SIP", "ObjectName: trunk-out",
				"IPaddress: 192.0.2.10", "IPport: 5060", "Dynamic: no", "Status: OK (12 ms)", "",
				"Event: PeerlistComplete", "ActionID: {{ACTIONID}}", "ListItems: 1", "",
			)
		default:
			t.Fatalf("unexpected action %q", action["Action"])
			return ""
		}
	}))

	pjsip := plugintest.RunOK[PeerListResult](t, plugin, OperationPeerList, map[string]any{"url": "ami://pbx:5038"})
	if pjsip.Technology != "pjsip" || pjsip.Count != 1 || pjsip.Peers[0].Name != "agent-7" || pjsip.Peers[0].Status != "Not in use" {
		t.Fatalf("pjsip peers = %#v", pjsip)
	}

	sip := plugintest.RunOK[PeerListResult](t, plugin, OperationPeerList, map[string]any{"url": "ami://pbx:5038", "technology": "sip"})
	if sip.Count != 1 || sip.Peers[0].Address != "192.0.2.10:5060" || sip.Peers[0].Dynamic || sip.Peers[0].Status != "OK (12 ms)" {
		t.Fatalf("sip peers = %#v", sip)
	}
}

func TestQueueStatusAggregatesMembersAndCallers(t *testing.T) {
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		if action["Action"] != "QueueStatus" || action["Queue"] != "support" {
			t.Fatalf("unexpected action %#v", action)
		}
		return wire(
			"Response: Success", "ActionID: {{ACTIONID}}", "EventList: start", "",
			"Event: QueueParams", "ActionID: {{ACTIONID}}", "Queue: support", "Strategy: rrmemory",
			"Calls: 2", "Holdtime: 45", "TalkTime: 180", "Completed: 17", "Abandoned: 3", "ServiceLevel: 60", "",
			"Event: QueueMember", "ActionID: {{ACTIONID}}", "Queue: support", "MemberName: Agent Seven",
			"StateInterface: PJSIP/agent-7", "Membership: dynamic", "Penalty: 1", "CallsTaken: 9",
			"LastCall: 1765300000", "Status: 2", "Paused: 1", "InCall: 1", "",
			"Event: QueueEntry", "ActionID: {{ACTIONID}}", "Queue: support", "Position: 1",
			"Channel: PJSIP/caller-0000007b", "CallerIDNum: 4930123456", "CallerIDName: Customer", "Wait: 37", "",
			"Event: QueueStatusComplete", "ActionID: {{ACTIONID}}", "EventList: Complete", "",
		)
	}))

	out := plugintest.RunOK[QueueStatusResult](t, plugin, OperationQueueStatus, map[string]any{"url": "ami://pbx:5038", "queue": "support"})
	if out.Count != 1 {
		t.Fatalf("queues = %#v", out)
	}
	queue := out.Queues[0]
	if queue.Strategy != "rrmemory" || queue.Calls != 2 || queue.Abandoned != 3 {
		t.Fatalf("queue = %#v", queue)
	}
	if len(queue.Members) != 1 || queue.Members[0].Status != "in_use" || !queue.Members[0].Paused || queue.Members[0].LastCall == "" {
		t.Fatalf("members = %#v", queue.Members)
	}
	if len(queue.Callers) != 1 || queue.Callers[0].Position != 1 || queue.Callers[0].WaitSeconds != 37 {
		t.Fatalf("callers = %#v", queue.Callers)
	}
}

func TestDeviceStateListFilters(t *testing.T) {
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		return wire(
			"Response: Success", "ActionID: {{ACTIONID}}", "EventList: start", "",
			"Event: DeviceStateChange", "ActionID: {{ACTIONID}}", "Device: PJSIP/agent-7", "State: NOT_INUSE", "",
			"Event: DeviceStateChange", "ActionID: {{ACTIONID}}", "Device: PJSIP/agent-9", "State: RINGING", "",
			"Event: DeviceStateListComplete", "ActionID: {{ACTIONID}}", "EventList: Complete", "",
		)
	}))

	out := plugintest.RunOK[DeviceStateListResult](t, plugin, OperationDeviceStateList, map[string]any{"url": "ami://pbx:5038", "device": "agent-9"})
	if out.Count != 1 || out.States[0].Device != "PJSIP/agent-9" || out.States[0].State != "RINGING" {
		t.Fatalf("device states = %#v", out)
	}
}

func TestCommandHandlesModernAndLegacyOutput(t *testing.T) {
	modern := NewPluginWithService(testAMIService(func(action map[string]string) string {
		if action["Action"] != "Command" || action["Command"] != "core show uptime" {
			t.Fatalf("unexpected action %#v", action)
		}
		return wire("Response: Success", "ActionID: {{ACTIONID}}", "Output: System uptime: 3 weeks", "Output: Last reload: 2 days", "")
	}))
	out := plugintest.RunOK[CommandResult](t, modern, OperationCommand, map[string]any{"url": "ami://pbx:5038", "command": "core show uptime"})
	if out.Output != "System uptime: 3 weeks\nLast reload: 2 days" || len(out.Lines) != 2 {
		t.Fatalf("modern command = %#v", out)
	}

	legacy := NewPluginWithService(testAMIService(func(map[string]string) string {
		return wire("Response: Follows", "ActionID: {{ACTIONID}}", "Privilege: Command", "System uptime: 3 weeks", "--END COMMAND--", "")
	}))
	out = plugintest.RunOK[CommandResult](t, legacy, OperationCommand, map[string]any{"url": "ami://pbx:5038", "command": "core show uptime"})
	if out.Output != "System uptime: 3 weeks" {
		t.Fatalf("legacy command = %#v", out)
	}

	if err := plugintest.RunError(t, modern, OperationCommand, map[string]any{"url": "ami://pbx:5038"}); err.Code != "bad_input" {
		t.Fatalf("command without command should fail: %#v", err)
	}
}

func TestOriginateBuildsActionAndValidates(t *testing.T) {
	var captured map[string]string
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		captured = action
		return wire("Response: Success", "ActionID: {{ACTIONID}}", "Message: Originate successfully queued", "Uniqueid: 1717920000.55", "")
	}))

	out := plugintest.RunOK[OriginateResult](t, plugin, OperationOriginate, map[string]any{
		"url": "ami://pbx:5038", "channel": "PJSIP/agent-7", "exten": "100", "context": "from-internal",
		"caller_id": "Fluxplane <7000>", "variables": map[string]string{"CAMPAIGN": "q2"},
	})
	if !out.OK || out.UniqueID != "1717920000.55" {
		t.Fatalf("originate = %#v", out)
	}
	if captured["Channel"] != "PJSIP/agent-7" || captured["Exten"] != "100" || captured["Context"] != "from-internal" ||
		captured["Priority"] != "1" || captured["Timeout"] != "30000" || captured["Async"] != "true" ||
		captured["Variable"] != "CAMPAIGN=q2" || captured["CallerID"] != "Fluxplane <7000>" {
		t.Fatalf("originate action = %#v", captured)
	}

	for name, input := range map[string]map[string]any{
		"missing channel":          {"url": "ami://pbx:5038", "exten": "100", "context": "from-internal"},
		"missing target":           {"channel": "PJSIP/agent-7", "exten": "100", "context": "from-internal"},
		"exten without context":    {"url": "ami://pbx:5038", "channel": "PJSIP/agent-7", "exten": "100"},
		"exten and application":    {"url": "ami://pbx:5038", "channel": "PJSIP/agent-7", "exten": "100", "context": "c", "application": "Playback"},
		"no exten, no application": {"url": "ami://pbx:5038", "channel": "PJSIP/agent-7"},
	} {
		if err := plugintest.RunError(t, plugin, OperationOriginate, input); err.Code != "bad_input" {
			t.Fatalf("%s should fail with bad_input: %#v", name, err)
		}
	}
}

func TestHangupReportsFailure(t *testing.T) {
	plugin := NewPluginWithService(testAMIService(func(action map[string]string) string {
		if action["Action"] != "Hangup" || action["Cause"] != "16" {
			t.Fatalf("unexpected action %#v", action)
		}
		return wire("Response: Error", "ActionID: {{ACTIONID}}", "Message: No such channel", "")
	}))

	err := plugintest.RunError(t, plugin, OperationChannelHangup, map[string]any{"url": "ami://pbx:5038", "channel": "PJSIP/gone-000001", "cause": 16})
	if err.Code != "asterisk" || !strings.Contains(err.Message, "No such channel") {
		t.Fatalf("hangup error = %#v", err)
	}
}
