package opsgenie

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

type TargetInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"description=Registered Opsgenie endpoint ref. Empty uses the EU API host."`
}

func (s Service) client(ctx pluginbinding.Context, input TargetInput) Client {
	return Client{EndpointRef: strings.TrimSpace(input.EndpointRef), Host: ctx.Host}
}

type TestInput struct {
	TargetInput
}

type TestResult struct {
	OK        bool   `json:"ok"`
	Name      string `json:"account_name,omitempty"`
	UserCount int    `json:"user_count,omitempty"`
	Plan      string `json:"plan,omitempty"`
}

func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	var wire struct {
		Data struct {
			Name      string `json:"name"`
			UserCount int    `json:"userCount"`
			Plan      struct {
				Name string `json:"name"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := s.client(ctx, input.TargetInput).get(context.Background(), "/v2/account", nil, &wire); err != nil {
		return TestResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	return TestResult{OK: true, Name: wire.Data.Name, UserCount: wire.Data.UserCount, Plan: wire.Data.Plan.Name}, nil
}

type AlertListInput struct {
	TargetInput
	Query string `json:"query,omitempty" jsonschema:"description=Opsgenie alert query language\\, e.g. status: open AND priority: P1\\, or tag: prod createdAt >= -2h."`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum alerts. Defaults to 20\\, capped at 100.,minimum=0,maximum=100"`
}

type AlertSummary struct {
	ID           string   `json:"id"`
	TinyID       string   `json:"tiny_id,omitempty"`
	Alias        string   `json:"alias,omitempty"`
	Message      string   `json:"message,omitempty"`
	Status       string   `json:"status,omitempty"`
	Acknowledged bool     `json:"acknowledged"`
	Priority     string   `json:"priority,omitempty"`
	Owner        string   `json:"owner,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Source       string   `json:"source,omitempty"`
	Count        int      `json:"count,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

type AlertListResult struct {
	Alerts []AlertSummary `json:"alerts"`
	Count  int            `json:"count"`
}

type apiAlertSummary struct {
	ID           string   `json:"id"`
	TinyID       string   `json:"tinyId"`
	Alias        string   `json:"alias"`
	Message      string   `json:"message"`
	Status       string   `json:"status"`
	Acknowledged bool     `json:"acknowledged"`
	Priority     string   `json:"priority"`
	Owner        string   `json:"owner"`
	Tags         []string `json:"tags"`
	Source       string   `json:"source"`
	Count        int      `json:"count"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

func alertFromAPI(wire apiAlertSummary) AlertSummary {
	return AlertSummary{
		ID: wire.ID, TinyID: wire.TinyID, Alias: wire.Alias, Message: wire.Message,
		Status: wire.Status, Acknowledged: wire.Acknowledged, Priority: wire.Priority,
		Owner: wire.Owner, Tags: wire.Tags, Source: wire.Source, Count: wire.Count,
		CreatedAt: wire.CreatedAt, UpdatedAt: wire.UpdatedAt,
	}
}

func (s Service) AlertList(ctx pluginbinding.Context, input AlertListInput) (AlertListResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	values := url.Values{}
	values.Set("limit", strconv.Itoa(limit))
	values.Set("sort", "createdAt")
	values.Set("order", "desc")
	if query := strings.TrimSpace(input.Query); query != "" {
		values.Set("query", query)
	}
	var wire struct {
		Data []apiAlertSummary `json:"data"`
	}
	if err := s.client(ctx, input.TargetInput).get(context.Background(), "/v2/alerts", values, &wire); err != nil {
		return AlertListResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	alerts := []AlertSummary{}
	for _, alert := range wire.Data {
		alerts = append(alerts, alertFromAPI(alert))
	}
	return AlertListResult{Alerts: alerts, Count: len(alerts)}, nil
}

type AlertGetInput struct {
	TargetInput
	ID             string `json:"id,omitempty" jsonschema:"required,description=Alert id\\, alias\\, or tiny id (see identifier_type)."`
	IdentifierType string `json:"identifier_type,omitempty" jsonschema:"description=How to interpret id. Defaults to id.,enum=id,enum=alias,enum=tiny"`
}

type AlertGetResult struct {
	Alert       AlertSummary      `json:"alert"`
	Description string            `json:"description,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
}

func (s Service) AlertGet(ctx pluginbinding.Context, input AlertGetInput) (AlertGetResult, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return AlertGetResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	values := identifierValues(input.IdentifierType)
	var wire struct {
		Data struct {
			apiAlertSummary
			Description string            `json:"description"`
			Details     map[string]string `json:"details"`
		} `json:"data"`
	}
	if err := s.client(ctx, input.TargetInput).get(context.Background(), "/v2/alerts/"+url.PathEscape(id), values, &wire); err != nil {
		return AlertGetResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	return AlertGetResult{
		Alert:       alertFromAPI(wire.Data.apiAlertSummary),
		Description: wire.Data.Description,
		Details:     wire.Data.Details,
	}, nil
}

type AlertActionInput struct {
	TargetInput
	ID             string `json:"id,omitempty" jsonschema:"required,description=Alert id\\, alias\\, or tiny id (see identifier_type)."`
	IdentifierType string `json:"identifier_type,omitempty" jsonschema:"description=How to interpret id. Defaults to id.,enum=id,enum=alias,enum=tiny"`
	Note           string `json:"note,omitempty" jsonschema:"description=Note attached to the action."`
	User           string `json:"user,omitempty" jsonschema:"description=Display name of the actor. Defaults to fluxplane-plugin."`
}

type AlertNoteInput struct {
	TargetInput
	ID             string `json:"id,omitempty" jsonschema:"required,description=Alert id\\, alias\\, or tiny id (see identifier_type)."`
	IdentifierType string `json:"identifier_type,omitempty" jsonschema:"description=How to interpret id. Defaults to id.,enum=id,enum=alias,enum=tiny"`
	Note           string `json:"note,omitempty" jsonschema:"required,description=The note text."`
	User           string `json:"user,omitempty" jsonschema:"description=Display name of the actor. Defaults to fluxplane-plugin."`
}

type AlertActionResult struct {
	// Accepted reports Opsgenie queued the action (its write API is async).
	Accepted  bool   `json:"accepted"`
	RequestID string `json:"request_id,omitempty"`
	Result    string `json:"result,omitempty"`
}

func (s Service) alertAction(ctx pluginbinding.Context, target TargetInput, id, identifierType, action string, body map[string]any) (AlertActionResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return AlertActionResult{}, pluginbinding.Fail("bad_input", "id is required")
	}
	values := identifierValues(identifierType)
	var wire struct {
		Result    string `json:"result"`
		RequestID string `json:"requestId"`
	}
	path := "/v2/alerts/" + url.PathEscape(id) + "/" + action
	if err := s.client(ctx, target).request(context.Background(), "POST", path, values, body, &wire); err != nil {
		return AlertActionResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	return AlertActionResult{Accepted: true, RequestID: wire.RequestID, Result: wire.Result}, nil
}

func actionBody(note, user string) map[string]any {
	body := map[string]any{
		"user":   firstNonEmpty(strings.TrimSpace(user), "fluxplane-plugin"),
		"source": "fluxplane-plugin",
	}
	if note = strings.TrimSpace(note); note != "" {
		body["note"] = note
	}
	return body
}

func (s Service) AlertAck(ctx pluginbinding.Context, input AlertActionInput) (AlertActionResult, error) {
	return s.alertAction(ctx, input.TargetInput, input.ID, input.IdentifierType, "acknowledge", actionBody(input.Note, input.User))
}

func (s Service) AlertClose(ctx pluginbinding.Context, input AlertActionInput) (AlertActionResult, error) {
	return s.alertAction(ctx, input.TargetInput, input.ID, input.IdentifierType, "close", actionBody(input.Note, input.User))
}

func (s Service) AlertNote(ctx pluginbinding.Context, input AlertNoteInput) (AlertActionResult, error) {
	if strings.TrimSpace(input.Note) == "" {
		return AlertActionResult{}, pluginbinding.Fail("bad_input", "note is required")
	}
	return s.alertAction(ctx, input.TargetInput, input.ID, input.IdentifierType, "notes", actionBody(input.Note, input.User))
}

type ScheduleListInput struct {
	TargetInput
}

type Schedule struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Enabled  bool   `json:"enabled"`
	Team     string `json:"team,omitempty"`
}

type ScheduleListResult struct {
	Schedules []Schedule `json:"schedules"`
	Count     int        `json:"count"`
}

func (s Service) scheduleList(ctx pluginbinding.Context, target TargetInput) ([]Schedule, error) {
	var wire struct {
		Data []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Timezone  string `json:"timezone"`
			Enabled   bool   `json:"enabled"`
			OwnerTeam struct {
				Name string `json:"name"`
			} `json:"ownerTeam"`
		} `json:"data"`
	}
	if err := s.client(ctx, target).get(context.Background(), "/v2/schedules", nil, &wire); err != nil {
		return nil, err
	}
	schedules := []Schedule{}
	for _, item := range wire.Data {
		schedules = append(schedules, Schedule{ID: item.ID, Name: item.Name, Timezone: item.Timezone, Enabled: item.Enabled, Team: item.OwnerTeam.Name})
	}
	return schedules, nil
}

func (s Service) ScheduleList(ctx pluginbinding.Context, input ScheduleListInput) (ScheduleListResult, error) {
	schedules, err := s.scheduleList(ctx, input.TargetInput)
	if err != nil {
		return ScheduleListResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	return ScheduleListResult{Schedules: schedules, Count: len(schedules)}, nil
}

type OnCallInput struct {
	TargetInput
	Schedule string `json:"schedule,omitempty" jsonschema:"description=Only schedules whose name contains this (case-insensitive)."`
}

type OnCallEntry struct {
	Schedule   string   `json:"schedule"`
	ScheduleID string   `json:"schedule_id,omitempty"`
	OnCall     []string `json:"on_call"`
}

type OnCallResult struct {
	Entries []OnCallEntry `json:"entries"`
	Count   int           `json:"count"`
}

func (s Service) OnCall(ctx pluginbinding.Context, input OnCallInput) (OnCallResult, error) {
	schedules, err := s.scheduleList(ctx, input.TargetInput)
	if err != nil {
		return OnCallResult{}, pluginbinding.Errorf("opsgenie", "%s", err)
	}
	filter := strings.ToLower(strings.TrimSpace(input.Schedule))
	entries := []OnCallEntry{}
	client := s.client(ctx, input.TargetInput)
	for _, schedule := range schedules {
		if !schedule.Enabled {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(schedule.Name), filter) {
			continue
		}
		values := url.Values{}
		values.Set("flat", "true")
		var wire struct {
			Data struct {
				OnCallRecipients []string `json:"onCallRecipients"`
			} `json:"data"`
		}
		if err := client.get(context.Background(), "/v2/schedules/"+url.PathEscape(schedule.ID)+"/on-calls", values, &wire); err != nil {
			return OnCallResult{}, pluginbinding.Errorf("opsgenie", "%s: %s", schedule.Name, err)
		}
		recipients := wire.Data.OnCallRecipients
		if recipients == nil {
			recipients = []string{}
		}
		entries = append(entries, OnCallEntry{Schedule: schedule.Name, ScheduleID: schedule.ID, OnCall: recipients})
	}
	return OnCallResult{Entries: entries, Count: len(entries)}, nil
}

func identifierValues(identifierType string) url.Values {
	values := url.Values{}
	switch strings.ToLower(strings.TrimSpace(identifierType)) {
	case "", "id":
	case "alias":
		values.Set("identifierType", "alias")
	case "tiny":
		values.Set("identifierType", "tiny")
	}
	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
