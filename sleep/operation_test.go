package sleep

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/pluginruntime"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestManifestDeclaresSleepOperation(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != OperationSleep {
		t.Fatalf("operations = %#v", manifest.Operations)
	}
	if !manifest.Operations[0].ReadOnly {
		t.Fatalf("operation should be read-only: %#v", manifest.Operations[0])
	}
}

func TestSleepRejectsInvalidDuration(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationSleep, Input{Duration: -1})
	if err.Code != "invalid_sleep_duration" {
		t.Fatalf("negative duration error = %#v", err)
	}
	for _, duration := range []float64{math.NaN(), math.Inf(1)} {
		_, err := Run(pluginbinding.Context{Context: context.Background()}, Input{Duration: duration})
		if err == nil || err.Error() == "" {
			t.Fatalf("duration %v error = %v", duration, err)
		}
	}
}

func TestSleepReturnsRenderedOutput(t *testing.T) {
	out := plugintest.RunOK[Output](t, NewPlugin(), OperationSleep, Input{Duration: 0})
	if out.Text != "Slept 0.000s" || out.Data["duration"] == nil {
		t.Fatalf("output = %#v", out)
	}
}

func TestDirectRuntimeSleepCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	host, err := pluginruntime.NewHost(pluginruntime.Direct(NewPlugin()))
	if err != nil {
		t.Fatalf("NewHost: %v", err)
	}
	resp, err := host.CallOperation(ctx, PluginName, protocol.OperationCall{Name: OperationSleep, Input: mustJSON(t, Input{Duration: time.Second.Seconds()})})
	if err != nil {
		t.Fatalf("CallOperation: %v", err)
	}
	if resp.OK || resp.Error == nil || resp.Error.Code != "canceled" {
		t.Fatalf("response = %#v", resp)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
