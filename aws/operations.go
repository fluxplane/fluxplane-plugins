package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// Service implements the AWS operations. All AWS traffic flows through the
// host http capability; credentials resolve only from the host secret store.
type Service struct{}

func NewService() Service {
	return Service{}
}

// RegionInput is embedded by every networked operation input.
type RegionInput struct {
	Region string `json:"region,omitempty" jsonschema:"description=AWS region. Defaults to eu-central-1."`
}

// TimeRangeInput selects a time window for log and metric operations.
type TimeRangeInput struct {
	Since string `json:"since,omitempty" jsonschema:"description=Start time as RFC3339\\, unix seconds\\, or duration ago (e.g. 1h)."`
	Until string `json:"until,omitempty" jsonschema:"description=End time as RFC3339\\, unix seconds\\, or duration ago. Defaults to now."`
}

// window resolves the time window with the given default lookback.
func (t TimeRangeInput) window(defaultSince string) (time.Time, time.Time, error) {
	now := time.Now()
	from, err := parseTimeValue(firstNonEmpty(t.Since, defaultSince), now)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid since: %s", err)
	}
	to := now
	if strings.TrimSpace(t.Until) != "" {
		to, err = parseTimeValue(t.Until, now)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid until: %s", err)
		}
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("since must be before until")
	}
	return from, to, nil
}

// parseTimeValue accepts a duration-ago ("30m"), unix seconds, or RFC3339.
func parseTimeValue(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("time value is empty")
	}
	if d, err := time.ParseDuration(value); err == nil {
		return now.Add(-d), nil
	}
	if ts, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}
	return time.Parse(time.RFC3339, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func str(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// opContext bounds one AWS API conversation.
func opContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 120*time.Second)
}

// ---- aws.test ----

type TestInput struct {
	RegionInput
}

type TestResult struct {
	Account   string `json:"account"`
	ARN       string `json:"arn"`
	UserID    string `json:"user_id,omitempty"`
	Region    string `json:"region"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
}

// Test proves connectivity and credential validity via STS GetCallerIdentity.
func (s Service) Test(ctx pluginbinding.Context, input TestInput) (TestResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return TestResult{}, err
	}
	callCtx, cancel := opContext()
	defer cancel()
	start := time.Now()
	identity, err := sts.NewFromConfig(cfg).GetCallerIdentity(callCtx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return TestResult{}, mapAWSError("sts get-caller-identity", err)
	}
	return TestResult{
		Account:   str(identity.Account),
		ARN:       str(identity.Arn),
		UserID:    str(identity.UserId),
		Region:    cfg.Region,
		LatencyMS: time.Since(start).Milliseconds(),
	}, nil
}
