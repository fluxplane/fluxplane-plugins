package aws

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	logstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type LogsGroupsInput struct {
	RegionInput
	Prefix string `json:"prefix,omitempty" jsonschema:"description=Log group name prefix filter (e.g. /aws/eks/)."`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum groups returned. Defaults to 100 and is capped at 500.,minimum=0,maximum=500"`
}

type LogGroup struct {
	Name          string `json:"name"`
	RetentionDays int32  `json:"retention_days,omitempty"`
	StoredBytes   int64  `json:"stored_bytes,omitempty"`
	Created       string `json:"created,omitempty"`
}

type LogsGroupsResult struct {
	Region    string     `json:"region"`
	Groups    []LogGroup `json:"groups,omitempty"`
	Count     int        `json:"count"`
	Truncated bool       `json:"truncated,omitempty"`
}

// LogsGroups lists CloudWatch log groups.
func (s Service) LogsGroups(ctx pluginbinding.Context, input LogsGroupsInput) (LogsGroupsResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return LogsGroupsResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	request := &cloudwatchlogs.DescribeLogGroupsInput{}
	if prefix := strings.TrimSpace(input.Prefix); prefix != "" {
		request.LogGroupNamePrefix = &prefix
	}
	callCtx, cancel := opContext()
	defer cancel()
	client := cloudwatchlogs.NewFromConfig(cfg)
	out := LogsGroupsResult{Region: cfg.Region}
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(client, request)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(callCtx)
		if err != nil {
			return LogsGroupsResult{}, mapAWSError("logs describe-log-groups", err)
		}
		for _, group := range page.LogGroups {
			if len(out.Groups) >= limit {
				out.Truncated = true
				break
			}
			mapped := LogGroup{Name: str(group.LogGroupName)}
			if group.RetentionInDays != nil {
				mapped.RetentionDays = *group.RetentionInDays
			}
			if group.StoredBytes != nil {
				mapped.StoredBytes = *group.StoredBytes
			}
			if group.CreationTime != nil {
				mapped.Created = time.UnixMilli(*group.CreationTime).UTC().Format(time.RFC3339)
			}
			out.Groups = append(out.Groups, mapped)
		}
		if out.Truncated {
			break
		}
	}
	out.Count = len(out.Groups)
	return out, nil
}

type LogsTailInput struct {
	RegionInput
	TimeRangeInput
	Group   string `json:"group,omitempty" jsonschema:"required,description=Log group name."`
	Pattern string `json:"pattern,omitempty" jsonschema:"description=CloudWatch filter pattern (e.g. ERROR or a JSON term)."`
	Limit   int    `json:"limit,omitempty" jsonschema:"description=Maximum events returned. Defaults to 200 and is capped at 1000.,minimum=0,maximum=1000"`
}

type LogEvent struct {
	Time    string `json:"time"`
	Stream  string `json:"stream,omitempty"`
	Message string `json:"message"`
}

type LogsTailResult struct {
	Region    string     `json:"region"`
	Group     string     `json:"group"`
	Events    []LogEvent `json:"events,omitempty"`
	Count     int        `json:"count"`
	Truncated bool       `json:"truncated,omitempty"`
}

// LogsTail reads recent events from a log group via FilterLogEvents.
func (s Service) LogsTail(ctx pluginbinding.Context, input LogsTailInput) (LogsTailResult, error) {
	group := strings.TrimSpace(input.Group)
	if group == "" {
		return LogsTailResult{}, pluginbinding.Fail("bad_input", "group is required")
	}
	from, to, err := input.window("15m")
	if err != nil {
		return LogsTailResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return LogsTailResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	request := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &group,
		StartTime:    int64Ptr(from.UnixMilli()),
		EndTime:      int64Ptr(to.UnixMilli()),
	}
	if pattern := strings.TrimSpace(input.Pattern); pattern != "" {
		request.FilterPattern = &pattern
	}
	callCtx, cancel := opContext()
	defer cancel()
	client := cloudwatchlogs.NewFromConfig(cfg)
	out := LogsTailResult{Region: cfg.Region, Group: group}
	paginator := cloudwatchlogs.NewFilterLogEventsPaginator(client, request)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(callCtx)
		if err != nil {
			return LogsTailResult{}, mapAWSError("logs filter-log-events", err)
		}
		for _, event := range page.Events {
			if len(out.Events) >= limit {
				out.Truncated = true
				break
			}
			mapped := LogEvent{Stream: str(event.LogStreamName), Message: strings.TrimRight(str(event.Message), "\n")}
			if event.Timestamp != nil {
				mapped.Time = time.UnixMilli(*event.Timestamp).UTC().Format(time.RFC3339Nano)
			}
			out.Events = append(out.Events, mapped)
		}
		if out.Truncated {
			break
		}
	}
	out.Count = len(out.Events)
	return out, nil
}

type LogsQueryInput struct {
	RegionInput
	TimeRangeInput
	Groups         []string `json:"groups,omitempty" jsonschema:"required,description=Log group names to query."`
	Query          string   `json:"query,omitempty" jsonschema:"required,description=CloudWatch Logs Insights query (e.g. fields @timestamp\\, @message | limit 20)."`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum seconds to wait for completion. Defaults to 30 and is capped at 120.,minimum=0,maximum=120"`
}

type LogsQueryResult struct {
	Region         string              `json:"region"`
	Status         string              `json:"status"`
	QueryID        string              `json:"query_id,omitempty"`
	Columns        []string            `json:"columns,omitempty"`
	Rows           []map[string]string `json:"rows,omitempty"`
	RecordsMatched float64             `json:"records_matched,omitempty"`
	RecordsScanned float64             `json:"records_scanned,omitempty"`
}

// LogsQuery runs a bounded CloudWatch Logs Insights query: StartQuery, poll
// until complete or timeout (best-effort StopQuery on deadline).
func (s Service) LogsQuery(ctx pluginbinding.Context, input LogsQueryInput) (LogsQueryResult, error) {
	if len(input.Groups) == 0 {
		return LogsQueryResult{}, pluginbinding.Fail("bad_input", "groups is required")
	}
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return LogsQueryResult{}, pluginbinding.Fail("bad_input", "query is required")
	}
	from, to, err := input.window("1h")
	if err != nil {
		return LogsQueryResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	timeout := input.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 120 {
		timeout = 120
	}
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return LogsQueryResult{}, err
	}
	callCtx, cancel := opContext()
	defer cancel()
	client := cloudwatchlogs.NewFromConfig(cfg)
	started, err := client.StartQuery(callCtx, &cloudwatchlogs.StartQueryInput{
		LogGroupNames: input.Groups,
		QueryString:   &query,
		StartTime:     int64Ptr(from.Unix()),
		EndTime:       int64Ptr(to.Unix()),
	})
	if err != nil {
		return LogsQueryResult{}, mapAWSError("logs start-query", err)
	}
	out := LogsQueryResult{Region: cfg.Region, QueryID: str(started.QueryId)}
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		results, err := client.GetQueryResults(callCtx, &cloudwatchlogs.GetQueryResultsInput{QueryId: started.QueryId})
		if err != nil {
			return LogsQueryResult{}, mapAWSError("logs get-query-results", err)
		}
		out.Status = string(results.Status)
		switch results.Status {
		case logstypes.QueryStatusComplete:
			columnSeen := map[string]bool{}
			for _, row := range results.Results {
				mapped := map[string]string{}
				for _, field := range row {
					name := str(field.Field)
					mapped[name] = str(field.Value)
					if !columnSeen[name] {
						columnSeen[name] = true
						out.Columns = append(out.Columns, name)
					}
				}
				out.Rows = append(out.Rows, mapped)
			}
			if results.Statistics != nil {
				out.RecordsMatched = results.Statistics.RecordsMatched
				out.RecordsScanned = results.Statistics.RecordsScanned
			}
			return out, nil
		case logstypes.QueryStatusFailed, logstypes.QueryStatusCancelled, logstypes.QueryStatusTimeout:
			return LogsQueryResult{}, pluginbinding.Errorf("aws", "logs insights query %s", strings.ToLower(string(results.Status)))
		}
		if time.Now().After(deadline) {
			_, _ = client.StopQuery(callCtx, &cloudwatchlogs.StopQueryInput{QueryId: started.QueryId})
			return LogsQueryResult{}, pluginbinding.Errorf("timeout", "logs insights query still %s after %ds — narrow the window or raise timeout_seconds", strings.ToLower(string(results.Status)), timeout)
		}
		time.Sleep(time.Second)
	}
}

func int64Ptr(value int64) *int64 { return &value }
