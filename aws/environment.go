package aws

import (
	"fmt"
	"strings"
	"time"

	fpcontext "github.com/fluxplane/fluxplane-context"
	evidence "github.com/fluxplane/fluxplane-evidence"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type InspectInput struct {
	Profile    string `json:"profile,omitempty" jsonschema:"description=Explicit AWS profile value."`
	Region     string `json:"region,omitempty" jsonschema:"description=Explicit AWS region value."`
	ProfileEnv string `json:"profile_env,omitempty" jsonschema:"description=Environment variable name used to resolve the AWS profile."`
	RegionEnv  string `json:"region_env,omitempty" jsonschema:"description=Environment variable name used to resolve the AWS region."`
}

type Environment struct {
	Configured             bool   `json:"configured"`
	Available              bool   `json:"available"`
	Profile                string `json:"profile,omitempty"`
	Region                 string `json:"region,omitempty"`
	AccessKeyConfigured    bool   `json:"access_key_configured"`
	SecretKeyConfigured    bool   `json:"secret_key_configured"`
	SessionTokenConfigured bool   `json:"session_token_configured"`
	WebIdentityConfigured  bool   `json:"web_identity_configured"`
	RoleARNConfigured      bool   `json:"role_arn_configured"`
	Source                 string `json:"source"`
	Scope                  string `json:"scope"`
}

func Inspect(ctx pluginbinding.Context, input InspectInput) (Environment, error) {
	return inspectEnvironment(ctx.Host, input)
}

func Observe(ctx pluginbinding.Context, _ pluginbinding.EvidenceObserveInput) (pluginbinding.EvidenceObserveResult, error) {
	env, err := inspectEnvironment(ctx.Host, InspectInput{})
	if err != nil {
		return pluginbinding.EvidenceObserveResult{}, err
	}
	now := time.Now().UTC()
	content := environmentContent(env)
	var observations []evidence.Observation
	if env.Configured {
		observations = append(observations, evidence.Observation{
			ID:          "integration:aws:configured:" + env.Scope,
			Kind:        ObservationEnvironmentConfigured,
			Scope:       env.Scope,
			Content:     content,
			At:          now,
			Environment: evidence.Ref{Name: evidence.Name(PluginName)},
		})
	}
	if env.Available {
		observations = append(observations, evidence.Observation{
			ID:          "integration:aws:available:" + env.Scope,
			Kind:        ObservationEnvironmentAvailable,
			Scope:       env.Scope,
			Content:     content,
			At:          now,
			Environment: evidence.Ref{Name: evidence.Name(PluginName)},
		})
	}
	return pluginbinding.EvidenceObserveResult{Observations: observations}, nil
}

func BuildContext(ctx pluginbinding.Context, _ pluginbinding.ContextBuildInput) (pluginbinding.ContextBuildResult, error) {
	env, err := inspectEnvironment(ctx.Host, InspectInput{})
	if err != nil {
		return pluginbinding.ContextBuildResult{}, err
	}
	if !env.Configured {
		return pluginbinding.ContextBuildResult{Blocks: []core.ContextBlock{}}, nil
	}
	return pluginbinding.ContextBuildResult{Blocks: []core.ContextBlock{{
		ID:        ContextName,
		Provider:  ContextName,
		Kind:      fpcontext.BlockText,
		Placement: fpcontext.PlacementSystem,
		Title:     "AWS Environment",
		Content:   renderEnvironment(env),
		MediaType: "text/plain",
		Priority:  70,
		Freshness: fpcontext.FreshnessDynamic,
		Metadata: map[string]string{
			"profile": env.Profile,
			"region":  env.Region,
			"scope":   env.Scope,
		},
	}}}, nil
}

func inspectEnvironment(host pluginbinding.HostClient, input InspectInput) (Environment, error) {
	input = normalizeInput(input)
	env := Environment{
		Profile: input.Profile,
		Region:  input.Region,
		Source:  "env",
	}
	if input.Profile != "" || input.Region != "" {
		env.Source = "input"
	}
	if host != nil {
		if env.Profile == "" {
			profile, _, err := lookupFirst(host, profileKeys(input)...)
			if err != nil {
				return Environment{}, err
			}
			env.Profile = profile
		}
		if env.Region == "" {
			region, _, err := lookupFirst(host, regionKeys(input)...)
			if err != nil {
				return Environment{}, err
			}
			env.Region = region
		}
		var err error
		if env.AccessKeyConfigured, err = lookupPresent(host, "AWS_ACCESS_KEY_ID"); err != nil {
			return Environment{}, err
		}
		if env.SecretKeyConfigured, err = lookupPresent(host, "AWS_SECRET_ACCESS_KEY"); err != nil {
			return Environment{}, err
		}
		if env.SessionTokenConfigured, err = lookupPresent(host, "AWS_SESSION_TOKEN"); err != nil {
			return Environment{}, err
		}
		if env.WebIdentityConfigured, err = lookupPresent(host, "AWS_WEB_IDENTITY_TOKEN_FILE"); err != nil {
			return Environment{}, err
		}
		if env.RoleARNConfigured, err = lookupPresent(host, "AWS_ROLE_ARN"); err != nil {
			return Environment{}, err
		}
	}
	staticCredentials := env.AccessKeyConfigured && env.SecretKeyConfigured
	webIdentity := env.WebIdentityConfigured && env.RoleARNConfigured
	env.Configured = env.Profile != "" || env.Region != "" || staticCredentials || webIdentity || env.SessionTokenConfigured
	env.Available = env.Profile != "" || staticCredentials || webIdentity
	env.Scope = awsScope(env.Profile, env.Region)
	return env, nil
}

func normalizeInput(input InspectInput) InspectInput {
	input.Profile = strings.TrimSpace(input.Profile)
	input.Region = strings.TrimSpace(input.Region)
	input.ProfileEnv = strings.TrimSpace(input.ProfileEnv)
	input.RegionEnv = strings.TrimSpace(input.RegionEnv)
	return input
}

func profileKeys(input InspectInput) []string {
	if input.ProfileEnv != "" {
		return []string{input.ProfileEnv}
	}
	return []string{"AWS_PROFILE", "AWS_DEFAULT_PROFILE"}
}

func regionKeys(input InspectInput) []string {
	if input.RegionEnv != "" {
		return []string{input.RegionEnv}
	}
	return []string{"AWS_REGION", "AWS_DEFAULT_REGION"}
}

func lookupFirst(host pluginbinding.HostClient, keys ...string) (string, bool, error) {
	for _, key := range keys {
		resp, err := host.EnvLookup(key)
		if err != nil {
			return "", false, err
		}
		value := strings.TrimSpace(resp.Value)
		if resp.Found && value != "" {
			return value, true, nil
		}
	}
	return "", false, nil
}

func lookupPresent(host pluginbinding.HostClient, key string) (bool, error) {
	resp, err := host.EnvLookup(key)
	if err != nil {
		return false, err
	}
	return resp.Found && strings.TrimSpace(resp.Value) != "", nil
}

func renderEnvironment(env Environment) string {
	var b strings.Builder
	b.WriteString("AWS environment:")
	fmt.Fprintf(&b, "\n- configured: %t", env.Configured)
	fmt.Fprintf(&b, "\n- available: %t", env.Available)
	if env.Profile != "" {
		b.WriteString("\n- profile: ")
		b.WriteString(env.Profile)
	}
	if env.Region != "" {
		b.WriteString("\n- region: ")
		b.WriteString(env.Region)
	}
	fmt.Fprintf(&b, "\n- access key configured: %t", env.AccessKeyConfigured)
	fmt.Fprintf(&b, "\n- secret key configured: %t", env.SecretKeyConfigured)
	fmt.Fprintf(&b, "\n- session token configured: %t", env.SessionTokenConfigured)
	fmt.Fprintf(&b, "\n- web identity configured: %t", env.WebIdentityConfigured)
	fmt.Fprintf(&b, "\n- role ARN configured: %t", env.RoleARNConfigured)
	return b.String()
}

func environmentContent(env Environment) map[string]any {
	return map[string]any{
		"configured":               env.Configured,
		"available":                env.Available,
		"profile":                  env.Profile,
		"region":                   env.Region,
		"access_key_configured":    env.AccessKeyConfigured,
		"secret_key_configured":    env.SecretKeyConfigured,
		"session_token_configured": env.SessionTokenConfigured,
		"web_identity_configured":  env.WebIdentityConfigured,
		"role_arn_configured":      env.RoleARNConfigured,
		"source":                   env.Source,
		"scope":                    env.Scope,
	}
}

func awsScope(profile, region string) string {
	parts := []string{"integration", PluginName}
	if profile != "" {
		parts = append(parts, "profile", sanitizeScopePart(profile))
	}
	if region != "" {
		parts = append(parts, "region", sanitizeScopePart(region))
	}
	return strings.Join(parts, ":")
}

func sanitizeScopePart(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_")
	return replacer.Replace(value)
}
