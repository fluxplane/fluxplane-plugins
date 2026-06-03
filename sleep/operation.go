package sleep

import (
	"fmt"
	"math"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type Input struct {
	Duration float64 `json:"duration" jsonschema:"description=Sleep duration in seconds.,required"`
}

type Output struct {
	Text    string         `json:"text,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func Run(ctx pluginbinding.Context, input Input) (Output, error) {
	if math.IsNaN(input.Duration) || math.IsInf(input.Duration, 0) {
		return Output{}, pluginbinding.Fail("invalid_sleep_duration", "duration must be a finite number of seconds")
	}
	if input.Duration < 0 {
		return Output{}, pluginbinding.Fail("invalid_sleep_duration", "duration must be greater than or equal to zero")
	}
	if input.Duration > float64(math.MaxInt64)/float64(time.Second) {
		return Output{}, pluginbinding.Fail("invalid_sleep_duration", "duration is too large")
	}

	duration := time.Duration(input.Duration * float64(time.Second))
	started := time.Now()
	if duration > 0 {
		timer := time.NewTimer(duration)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return Output{}, pluginbinding.Fail("canceled", "sleep interrupted")
		case <-timer.C:
		}
	}

	elapsed := time.Since(started)
	text := fmt.Sprintf("Slept %.3fs", input.Duration)
	return Output{
		Text:    text,
		Summary: text,
		Data: map[string]any{
			"duration": input.Duration,
			"elapsed":  elapsed.Seconds(),
		},
	}, nil
}
