package aws

import (
	"encoding/json"
	"errors"
	"testing"

	fpendpoint "github.com/fluxplane/fluxplane-endpoint"
	evidence "github.com/fluxplane/fluxplane-evidence"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

func TestManifestDeclaresAWSOperationAndContext(t *testing.T) {
	manifest := Manifest()
	plugintest.AssertManifestQuality(t, manifest)
	if manifest.Name != PluginName {
		t.Fatalf("name = %q, want %q", manifest.Name, PluginName)
	}
	if len(manifest.Operations) != 1 || manifest.Operations[0].Name != OperationInspect {
		t.Fatalf("operations = %#v, want inspect", manifest.Operations)
	}
	if !manifest.Operations[0].ReadOnly || len(manifest.Operations[0].SecretPurposes) != 0 {
		t.Fatalf("operation = %#v, want read-only without secret purposes", manifest.Operations[0])
	}
	if len(manifest.Context) != 1 || manifest.Context[0].Name != ContextName {
		t.Fatalf("context = %#v, want AWS context", manifest.Context)
	}
	if len(manifest.Observers) != 1 || manifest.Observers[0].Name != ObserverEnvironment {
		t.Fatalf("observers = %#v, want AWS environment observer", manifest.Observers)
	}
	if len(manifest.AssertionDerivers) != 2 {
		t.Fatalf("assertion derivers = %#v, want configured and available derivers", manifest.AssertionDerivers)
	}
}

func TestInspectReportsNonSecretCredentialPresence(t *testing.T) {
	host := awsTestHost{values: map[string]string{
		"AWS_PROFILE":           "dev-profile",
		"AWS_REGION":            "us-east-1",
		"AWS_ACCESS_KEY_ID":     "AKIAEXAMPLE",
		"AWS_SECRET_ACCESS_KEY": "secret",
		"AWS_SESSION_TOKEN":     "token",
	}}
	env := plugintest.RunOK[Environment](t, NewPlugin(), OperationInspect, map[string]any{}, plugintest.WithHost(host))
	if !env.Configured || !env.Available {
		t.Fatalf("env = %#v, want configured and available", env)
	}
	if env.Profile != "dev-profile" || env.Region != "us-east-1" {
		t.Fatalf("env = %#v, want profile and region", env)
	}
	if !env.AccessKeyConfigured || !env.SecretKeyConfigured || !env.SessionTokenConfigured {
		t.Fatalf("env = %#v, want credential presence booleans", env)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"AKIAEXAMPLE", "secret", "token"} {
		if string(raw) == secret || containsJSONValue(raw, secret) {
			t.Fatalf("environment leaked secret value %q: %s", secret, string(raw))
		}
	}
}

func TestInspectTreatsRegionOnlyAsConfiguredNotAvailable(t *testing.T) {
	host := awsTestHost{values: map[string]string{"AWS_REGION": "eu-central-1"}}
	env := plugintest.RunOK[Environment](t, NewPlugin(), OperationInspect, nil, plugintest.WithHost(host))
	if !env.Configured {
		t.Fatalf("env = %#v, want configured", env)
	}
	if env.Available {
		t.Fatalf("env = %#v, want unavailable with region only", env)
	}
}

func TestObserveReportsAWSConfiguredAndAvailableEvidence(t *testing.T) {
	host := awsTestHost{values: map[string]string{
		"AWS_PROFILE":           "dev-profile",
		"AWS_REGION":            "us-east-1",
		"AWS_ACCESS_KEY_ID":     "AKIAEXAMPLE",
		"AWS_SECRET_ACCESS_KEY": "secret",
		"AWS_SESSION_TOKEN":     "token",
	}}
	resp := NewPlugin().HandleWithHost(protocol.Request{
		Command: protocol.CommandEvidenceObserve,
		Plugin:  PluginName,
		Payload: mustJSON(t, protocol.EvidenceObserveRequest{Phase: evidence.PhaseTurn}),
	}, host)
	if !resp.OK {
		t.Fatalf("observe failed: %#v", resp.Error)
	}
	var result protocol.EvidenceObserveResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 2 {
		t.Fatalf("observations = %#v, want configured and available", result.Observations)
	}
	if !hasObservationKind(result.Observations, ObservationEnvironmentConfigured) || !hasObservationKind(result.Observations, ObservationEnvironmentAvailable) {
		t.Fatalf("observations = %#v", result.Observations)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"AKIAEXAMPLE", "secret", "token"} {
		if containsJSONValue(raw, secret) {
			t.Fatalf("observation leaked secret value %q: %s", secret, string(raw))
		}
	}
}

func TestObserveTreatsRegionOnlyAsConfiguredNotAvailable(t *testing.T) {
	resp := NewPlugin().HandleWithHost(protocol.Request{
		Command: protocol.CommandEvidenceObserve,
		Plugin:  PluginName,
		Payload: mustJSON(t, protocol.EvidenceObserveRequest{Phase: evidence.PhaseTurn}),
	}, awsTestHost{values: map[string]string{"AWS_REGION": "eu-central-1"}})
	if !resp.OK {
		t.Fatalf("observe failed: %#v", resp.Error)
	}
	var result protocol.EvidenceObserveResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 1 || result.Observations[0].Kind != ObservationEnvironmentConfigured {
		t.Fatalf("observations = %#v, want configured only", result.Observations)
	}
}

func TestBuildContextSkipsUnconfiguredAWS(t *testing.T) {
	resp := NewPlugin().HandleWithHost(protocol.Request{
		Command: protocol.CommandContextBuild,
		Plugin:  PluginName,
		Payload: []byte(`{}`),
	}, awsTestHost{})
	if !resp.OK {
		t.Fatalf("context failed: %#v", resp.Error)
	}
	var result pluginbinding.ContextBuildResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 0 {
		t.Fatalf("blocks = %#v, want none", result.Blocks)
	}
}

func TestBuildContextRendersConfiguredAWS(t *testing.T) {
	resp := NewPlugin().HandleWithHost(protocol.Request{
		Command: protocol.CommandContextBuild,
		Plugin:  PluginName,
		Payload: []byte(`{}`),
	}, awsTestHost{values: map[string]string{"AWS_PROFILE": "dev"}})
	if !resp.OK {
		t.Fatalf("context failed: %#v", resp.Error)
	}
	var result struct {
		Blocks []core.ContextBlock `json:"blocks"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Blocks) != 1 || result.Blocks[0].Source == nil || result.Blocks[0].Source.Plugin != PluginName {
		t.Fatalf("blocks = %#v, want AWS context block", result.Blocks)
	}
}

type awsTestHost struct {
	pluginbinding.HostClient
	values map[string]string
}

func (h awsTestHost) EnvLookup(key string) (pluginbinding.EnvLookupResponse, error) {
	if h.values == nil {
		return pluginbinding.EnvLookupResponse{Key: key}, nil
	}
	value, ok := h.values[key]
	return pluginbinding.EnvLookupResponse{Key: key, Value: value, Found: ok}, nil
}

func (h awsTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h awsTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h awsTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h awsTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h awsTestHost) ResolveEndpoint(string) (fpendpoint.EndpointRef, error) {
	return fpendpoint.EndpointRef{}, nil
}

func (h awsTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, errors.New("unexpected HTTP")
}

func (h awsTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h awsTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h awsTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h awsTestHost) ProcessRun(pluginbinding.ProcessRunRequest) (pluginbinding.ProcessRunResponse, error) {
	return pluginbinding.ProcessRunResponse{}, nil
}

func (h awsTestHost) CapabilityCall(pluginbinding.ProviderCallRequest) (pluginbinding.ProviderCallResponse, error) {
	return pluginbinding.ProviderCallResponse{}, nil
}

func containsJSONValue(raw []byte, value string) bool {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return false
	}
	return containsValue(decoded, value)
}

func hasObservationKind(observations []evidence.Observation, kind string) bool {
	for _, observation := range observations {
		if observation.Kind == kind {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func containsValue(candidate any, value string) bool {
	switch typed := candidate.(type) {
	case string:
		return typed == value
	case []any:
		for _, item := range typed {
			if containsValue(item, value) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsValue(item, value) {
				return true
			}
		}
	}
	return false
}
