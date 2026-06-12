package asterisk

import (
	"strconv"
	"strings"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// AMIActionInput is the shared target + timeout shape for AMI operations.
type AMIActionInput struct {
	AMITargetInput
	Timeout string `json:"timeout,omitempty" jsonschema:"description=AMI connection timeout duration. Defaults to 10s."`
}

type ChannelListInput struct {
	AMIActionInput
	Limit int `json:"limit,omitempty" jsonschema:"description=Maximum channels to return."`
}

type ChannelRecord struct {
	Channel         string `json:"channel"`
	UniqueID        string `json:"unique_id,omitempty"`
	State           string `json:"state,omitempty"`
	CallerIDNum     string `json:"caller_id_num,omitempty"`
	CallerIDName    string `json:"caller_id_name,omitempty"`
	ConnectedNum    string `json:"connected_num,omitempty"`
	Context         string `json:"context,omitempty"`
	Exten           string `json:"exten,omitempty"`
	Application     string `json:"application,omitempty"`
	ApplicationData string `json:"application_data,omitempty"`
	Duration        string `json:"duration,omitempty"`
	BridgeID        string `json:"bridge_id,omitempty"`
	AccountCode     string `json:"account_code,omitempty"`
}

type ChannelListResult struct {
	Count    int             `json:"count"`
	Channels []ChannelRecord `json:"channels"`
}

type PeerListInput struct {
	AMIActionInput
	Technology string `json:"technology,omitempty" jsonschema:"description=Channel technology to list peers for,enum=pjsip,enum=sip,enum=iax"`
	Limit      int    `json:"limit,omitempty" jsonschema:"description=Maximum peers to return."`
}

type PeerRecord struct {
	Technology string `json:"technology"`
	Name       string `json:"name"`
	Address    string `json:"address,omitempty"`
	Status     string `json:"status,omitempty"`
	Dynamic    bool   `json:"dynamic,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

type PeerListResult struct {
	Count      int          `json:"count"`
	Technology string       `json:"technology"`
	Peers      []PeerRecord `json:"peers"`
}

type QueueStatusInput struct {
	AMIActionInput
	Queue string `json:"queue,omitempty" jsonschema:"description=Limit status to this queue."`
}

type QueueMember struct {
	Interface  string `json:"interface"`
	Name       string `json:"name,omitempty"`
	Membership string `json:"membership,omitempty"`
	Penalty    int    `json:"penalty,omitempty"`
	CallsTaken int    `json:"calls_taken,omitempty"`
	LastCall   string `json:"last_call,omitempty"`
	Status     string `json:"status,omitempty"`
	Paused     bool   `json:"paused,omitempty"`
	InCall     bool   `json:"in_call,omitempty"`
}

type QueueCaller struct {
	Position     int    `json:"position"`
	Channel      string `json:"channel,omitempty"`
	CallerIDNum  string `json:"caller_id_num,omitempty"`
	CallerIDName string `json:"caller_id_name,omitempty"`
	WaitSeconds  int    `json:"wait_seconds,omitempty"`
}

type QueueRecord struct {
	Name         string        `json:"name"`
	Strategy     string        `json:"strategy,omitempty"`
	Calls        int           `json:"calls"`
	HoldTime     int           `json:"hold_time,omitempty"`
	TalkTime     int           `json:"talk_time,omitempty"`
	Completed    int           `json:"completed,omitempty"`
	Abandoned    int           `json:"abandoned,omitempty"`
	ServiceLevel int           `json:"service_level,omitempty"`
	Members      []QueueMember `json:"members,omitempty"`
	Callers      []QueueCaller `json:"callers,omitempty"`
}

type QueueStatusResult struct {
	Count  int           `json:"count"`
	Queues []QueueRecord `json:"queues"`
}

type DeviceStateListInput struct {
	AMIActionInput
	Device string `json:"device,omitempty" jsonschema:"description=Substring filter on the device name (e.g. PJSIP/agent-7)."`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum device states to return."`
}

type DeviceStateRecord struct {
	Device string `json:"device"`
	State  string `json:"state"`
}

type DeviceStateListResult struct {
	Count  int                 `json:"count"`
	States []DeviceStateRecord `json:"states"`
}

type CommandInput struct {
	AMIActionInput
	Command string `json:"command,omitempty" jsonschema:"description=Asterisk CLI command to run (e.g. core show uptime)."`
}

type CommandResult struct {
	Command string   `json:"command"`
	Output  string   `json:"output,omitempty"`
	Lines   []string `json:"lines"`
}

type OriginateInput struct {
	AMIActionInput
	Channel       string            `json:"channel,omitempty" jsonschema:"description=Channel to call first (e.g. PJSIP/agent-7 or Local/100@from-internal)."`
	Exten         string            `json:"exten,omitempty" jsonschema:"description=Extension to connect the answered channel to. Requires context. Mutually exclusive with application."`
	DialContext   string            `json:"context,omitempty" jsonschema:"description=Dialplan context for exten."`
	Priority      int               `json:"priority,omitempty" jsonschema:"description=Dialplan priority (default 1)."`
	Application   string            `json:"application,omitempty" jsonschema:"description=Application to run on answer (e.g. Playback). Mutually exclusive with exten."`
	Data          string            `json:"data,omitempty" jsonschema:"description=Application argument data."`
	CallerID      string            `json:"caller_id,omitempty" jsonschema:"description=Caller ID for the originated call."`
	TimeoutMS     int               `json:"timeout_ms,omitempty" jsonschema:"description=Answer timeout in milliseconds (default 30000)."`
	Variables     map[string]string `json:"variables,omitempty" jsonschema:"description=Channel variables to set on the originated channel."`
	Async         *bool             `json:"async,omitempty" jsonschema:"description=Originate asynchronously (default true): the action returns immediately and the call proceeds in the background."`
	AccountCode   string            `json:"account_code,omitempty" jsonschema:"description=Account code for the originated call."`
	EarlyMedia    bool              `json:"early_media,omitempty" jsonschema:"description=Connect on early media instead of answer."`
	ChannelID     string            `json:"channel_id,omitempty" jsonschema:"description=Explicit unique id for the first channel."`
	OtherChanneld string            `json:"other_channel_id,omitempty" jsonschema:"description=Explicit unique id for the second channel."`
}

type OriginateResult struct {
	OK       bool   `json:"ok"`
	Channel  string `json:"channel"`
	Response string `json:"response,omitempty"`
	Message  string `json:"message,omitempty"`
	UniqueID string `json:"unique_id,omitempty"`
}

type HangupInput struct {
	AMIActionInput
	Channel string `json:"channel,omitempty" jsonschema:"description=Exact channel name to hang up (from asterisk.channel.list)."`
	Cause   int    `json:"cause,omitempty" jsonschema:"description=ISDN hangup cause code (e.g. 16 normal clearing)."`
}

type HangupResult struct {
	OK       bool   `json:"ok"`
	Channel  string `json:"channel"`
	Response string `json:"response,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (s Service) ChannelList(ctx pluginbinding.Context, input ChannelListInput) (ChannelListResult, error) {
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return ChannelListResult{}, err
	}
	defer session.Close()
	_, events, err := session.collect(map[string]string{"Action": "CoreShowChannels"}, "CoreShowChannelsComplete")
	if err != nil {
		return ChannelListResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	channels := make([]ChannelRecord, 0, len(events))
	for _, event := range events {
		if !strings.EqualFold(event["Event"], "CoreShowChannel") {
			continue
		}
		channels = append(channels, ChannelRecord{
			Channel:         event["Channel"],
			UniqueID:        event["Uniqueid"],
			State:           firstNonEmpty(event["ChannelStateDesc"], event["ChannelState"]),
			CallerIDNum:     event["CallerIDNum"],
			CallerIDName:    event["CallerIDName"],
			ConnectedNum:    event["ConnectedLineNum"],
			Context:         event["Context"],
			Exten:           event["Exten"],
			Application:     event["Application"],
			ApplicationData: event["ApplicationData"],
			Duration:        event["Duration"],
			BridgeID:        event["BridgeId"],
			AccountCode:     event["AccountCode"],
		})
	}
	channels = limitRecords(channels, input.Limit)
	return ChannelListResult{Count: len(channels), Channels: channels}, nil
}

func (s Service) PeerList(ctx pluginbinding.Context, input PeerListInput) (PeerListResult, error) {
	technology := strings.ToLower(strings.TrimSpace(input.Technology))
	if technology == "" {
		technology = "pjsip"
	}
	var action string
	var completes []string
	switch technology {
	case "pjsip":
		action, completes = "PJSIPShowEndpoints", []string{"EndpointListComplete"}
	case "sip":
		action, completes = "SIPpeers", []string{"PeerlistComplete"}
	case "iax":
		action, completes = "IAXpeerlist", []string{"PeerlistComplete"}
	default:
		return PeerListResult{}, pluginbinding.Fail("bad_input", "technology must be pjsip, sip, or iax")
	}
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return PeerListResult{}, err
	}
	defer session.Close()
	_, events, err := session.collect(map[string]string{"Action": action}, completes...)
	if err != nil {
		// PJSIPShowEndpoints answers an Error response when zero endpoints are
		// configured — that's an empty list, not a failure.
		if strings.Contains(strings.ToLower(err.Error()), "no endpoints found") {
			return PeerListResult{Count: 0, Technology: technology, Peers: []PeerRecord{}}, nil
		}
		return PeerListResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	peers := make([]PeerRecord, 0, len(events))
	for _, event := range events {
		switch strings.ToLower(event["Event"]) {
		case "endpointlist": // PJSIP
			peers = append(peers, PeerRecord{
				Technology: "pjsip",
				Name:       event["ObjectName"],
				Address:    event["Contacts"],
				Status:     firstNonEmpty(event["DeviceState"], event["State"]),
				Comment:    activeChannelsComment(event["ActiveChannels"]),
			})
		case "peerentry": // SIP / IAX
			address := event["IPaddress"]
			if port := event["IPport"]; port != "" && port != "0" && address != "" && address != "-none-" {
				address += ":" + port
			}
			peers = append(peers, PeerRecord{
				Technology: strings.ToLower(firstNonEmpty(event["Channeltype"], technology)),
				Name:       event["ObjectName"],
				Address:    address,
				Status:     event["Status"],
				Dynamic:    strings.EqualFold(event["Dynamic"], "yes"),
				Comment:    event["Description"],
			})
		}
	}
	peers = limitRecords(peers, input.Limit)
	return PeerListResult{Count: len(peers), Technology: technology, Peers: peers}, nil
}

func (s Service) QueueStatus(ctx pluginbinding.Context, input QueueStatusInput) (QueueStatusResult, error) {
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return QueueStatusResult{}, err
	}
	defer session.Close()
	action := map[string]string{"Action": "QueueStatus"}
	if queue := strings.TrimSpace(input.Queue); queue != "" {
		action["Queue"] = queue
	}
	_, events, err := session.collect(action, "QueueStatusComplete")
	if err != nil {
		return QueueStatusResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	byName := map[string]*QueueRecord{}
	var order []string
	queueFor := func(name string) *QueueRecord {
		if name == "" {
			return nil
		}
		if record, ok := byName[name]; ok {
			return record
		}
		record := &QueueRecord{Name: name}
		byName[name] = record
		order = append(order, name)
		return record
	}
	for _, event := range events {
		switch strings.ToLower(event["Event"]) {
		case "queueparams":
			record := queueFor(event["Queue"])
			if record == nil {
				continue
			}
			record.Strategy = event["Strategy"]
			record.Calls = atoiSafe(event["Calls"])
			record.HoldTime = atoiSafe(event["Holdtime"])
			record.TalkTime = atoiSafe(event["TalkTime"])
			record.Completed = atoiSafe(event["Completed"])
			record.Abandoned = atoiSafe(event["Abandoned"])
			record.ServiceLevel = atoiSafe(event["ServiceLevel"])
		case "queuemember":
			record := queueFor(event["Queue"])
			if record == nil {
				continue
			}
			record.Members = append(record.Members, QueueMember{
				Interface:  firstNonEmpty(event["StateInterface"], event["Location"], event["Interface"]),
				Name:       firstNonEmpty(event["MemberName"], event["Name"]),
				Membership: event["Membership"],
				Penalty:    atoiSafe(event["Penalty"]),
				CallsTaken: atoiSafe(event["CallsTaken"]),
				LastCall:   formatUnixTime(event["LastCall"]),
				Status:     queueMemberStatus(event["Status"]),
				Paused:     event["Paused"] == "1",
				InCall:     event["InCall"] == "1",
			})
		case "queueentry":
			record := queueFor(event["Queue"])
			if record == nil {
				continue
			}
			record.Callers = append(record.Callers, QueueCaller{
				Position:     atoiSafe(event["Position"]),
				Channel:      event["Channel"],
				CallerIDNum:  event["CallerIDNum"],
				CallerIDName: event["CallerIDName"],
				WaitSeconds:  atoiSafe(event["Wait"]),
			})
		}
	}
	queues := make([]QueueRecord, 0, len(order))
	for _, name := range order {
		queues = append(queues, *byName[name])
	}
	return QueueStatusResult{Count: len(queues), Queues: queues}, nil
}

func (s Service) DeviceStateList(ctx pluginbinding.Context, input DeviceStateListInput) (DeviceStateListResult, error) {
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return DeviceStateListResult{}, err
	}
	defer session.Close()
	_, events, err := session.collect(map[string]string{"Action": "DeviceStateList"}, "DeviceStateListComplete")
	if err != nil {
		return DeviceStateListResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	filter := strings.ToLower(strings.TrimSpace(input.Device))
	states := make([]DeviceStateRecord, 0, len(events))
	for _, event := range events {
		if !strings.EqualFold(event["Event"], "DeviceStateChange") {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(event["Device"]), filter) {
			continue
		}
		states = append(states, DeviceStateRecord{Device: event["Device"], State: event["State"]})
	}
	states = limitRecords(states, input.Limit)
	return DeviceStateListResult{Count: len(states), States: states}, nil
}

func (s Service) Command(ctx pluginbinding.Context, input CommandInput) (CommandResult, error) {
	commandLine := strings.TrimSpace(input.Command)
	if commandLine == "" {
		return CommandResult{}, pluginbinding.Fail("bad_input", "command is required")
	}
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return CommandResult{}, err
	}
	defer session.Close()
	output, err := session.command(commandLine)
	if err != nil {
		return CommandResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	output = strings.TrimRight(output, "\n")
	var lines []string
	if output != "" {
		lines = strings.Split(output, "\n")
	}
	if lines == nil {
		lines = []string{}
	}
	return CommandResult{Command: commandLine, Output: output, Lines: lines}, nil
}

func (s Service) Originate(ctx pluginbinding.Context, input OriginateInput) (OriginateResult, error) {
	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		return OriginateResult{}, pluginbinding.Fail("bad_input", "channel is required")
	}
	exten := strings.TrimSpace(input.Exten)
	application := strings.TrimSpace(input.Application)
	switch {
	case exten == "" && application == "":
		return OriginateResult{}, pluginbinding.Fail("bad_input", "provide exten+context or application")
	case exten != "" && application != "":
		return OriginateResult{}, pluginbinding.Fail("bad_input", "exten and application are mutually exclusive")
	case exten != "" && strings.TrimSpace(input.DialContext) == "":
		return OriginateResult{}, pluginbinding.Fail("bad_input", "context is required with exten")
	}
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return OriginateResult{}, err
	}
	defer session.Close()
	action := map[string]string{
		"Action":  "Originate",
		"Channel": channel,
	}
	if exten != "" {
		action["Exten"] = exten
		action["Context"] = strings.TrimSpace(input.DialContext)
		priority := input.Priority
		if priority <= 0 {
			priority = 1
		}
		action["Priority"] = strconv.Itoa(priority)
	} else {
		action["Application"] = application
		action["Data"] = strings.TrimSpace(input.Data)
	}
	if callerID := strings.TrimSpace(input.CallerID); callerID != "" {
		action["CallerID"] = callerID
	}
	timeoutMS := input.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = 30000
	}
	action["Timeout"] = strconv.Itoa(timeoutMS)
	async := true
	if input.Async != nil {
		async = *input.Async
	}
	if async {
		action["Async"] = "true"
	}
	if accountCode := strings.TrimSpace(input.AccountCode); accountCode != "" {
		action["Account"] = accountCode
	}
	if input.EarlyMedia {
		action["EarlyMedia"] = "true"
	}
	if channelID := strings.TrimSpace(input.ChannelID); channelID != "" {
		action["ChannelId"] = channelID
	}
	if otherChannelID := strings.TrimSpace(input.OtherChanneld); otherChannelID != "" {
		action["OtherChannelId"] = otherChannelID
	}
	if len(input.Variables) > 0 {
		var pairs []string
		for key, value := range input.Variables {
			pairs = append(pairs, key+"="+value)
		}
		// AMI accepts multiple comma-separated assignments in one Variable header.
		action["Variable"] = strings.Join(pairs, ",")
	}
	// Originate can block until answer when synchronous: extend the session
	// read deadline to cover the answer timeout.
	if !async {
		session.timeout += time.Duration(timeoutMS) * time.Millisecond
	}
	response, err := session.do(action)
	if err != nil {
		return OriginateResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	result := OriginateResult{
		Channel:  channel,
		Response: response["Response"],
		Message:  response["Message"],
		UniqueID: response["Uniqueid"],
		OK:       strings.EqualFold(response["Response"], "Success"),
	}
	if !result.OK {
		return result, pluginbinding.Errorf("asterisk", "originate failed: %s", firstNonEmpty(result.Message, result.Response))
	}
	return result, nil
}

func (s Service) Hangup(ctx pluginbinding.Context, input HangupInput) (HangupResult, error) {
	channel := strings.TrimSpace(input.Channel)
	if channel == "" {
		return HangupResult{}, pluginbinding.Fail("bad_input", "channel is required")
	}
	session, err := s.session(ctx, input.AMITargetInput, input.Timeout)
	if err != nil {
		return HangupResult{}, err
	}
	defer session.Close()
	action := map[string]string{"Action": "Hangup", "Channel": channel}
	if input.Cause > 0 {
		action["Cause"] = strconv.Itoa(input.Cause)
	}
	response, err := session.do(action)
	if err != nil {
		return HangupResult{}, pluginbinding.Errorf("asterisk", "%s", err)
	}
	result := HangupResult{
		Channel:  channel,
		Response: response["Response"],
		Message:  response["Message"],
		OK:       strings.EqualFold(response["Response"], "Success"),
	}
	if !result.OK {
		return result, pluginbinding.Errorf("asterisk", "hangup failed: %s", firstNonEmpty(result.Message, result.Response))
	}
	return result, nil
}

// session opens an authenticated AMI session, honoring the test hook.
func (s Service) session(ctx pluginbinding.Context, target AMITargetInput, timeout string) (*amiSession, error) {
	duration, err := amiDuration(timeout, 10*time.Second)
	if err != nil {
		return nil, pluginbinding.Errorf("bad_input", "%s", err)
	}
	if strings.TrimSpace(target.EndpointRef) == "" && strings.TrimSpace(target.URL) == "" {
		return nil, pluginbinding.Fail("bad_input", "endpoint_ref or url is required")
	}
	if s.DialAMI != nil {
		return s.DialAMI(ctx, target, duration)
	}
	return dialAMISession(ctx, target, duration)
}

func queueMemberStatus(value string) string {
	switch strings.TrimSpace(value) {
	case "0":
		return "unknown"
	case "1":
		return "not_in_use"
	case "2":
		return "in_use"
	case "3":
		return "busy"
	case "4":
		return "invalid"
	case "5":
		return "unavailable"
	case "6":
		return "ringing"
	case "7":
		return "ring_in_use"
	case "8":
		return "on_hold"
	default:
		return value
	}
}

func activeChannelsComment(value string) string {
	if value = strings.TrimSpace(value); value != "" && value != "0" {
		return value + " active channel(s)"
	}
	return ""
}

func formatUnixTime(value string) string {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}

func atoiSafe(value string) int {
	out, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return out
}

func limitRecords[T any](records []T, limit int) []T {
	if limit <= 0 || len(records) <= limit {
		return records
	}
	return records[:limit]
}
