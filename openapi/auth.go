package openapi

import (
	"strings"

	auth "github.com/fluxplane/fluxplane-auth"
	sdkmanifest "github.com/fluxplane/fluxplane-plugin/manifest"
	sharedsecret "github.com/fluxplane/fluxplane-secret"
	"github.com/getkin/kin-openapi/openapi3"
)

type authBinding struct {
	Method sdkmanifest.AuthMethod
	Config AuthSchemeConfig
}

func authMethodsFor(instance string, cfg SpecConfig, doc *openapi3.T) ([]sdkmanifest.AuthMethod, map[string]authBinding, map[string]*openapi3.SecurityScheme) {
	byScheme := map[string]authBinding{}
	schemes := map[string]*openapi3.SecurityScheme{}
	if doc == nil || doc.Components == nil {
		return nil, byScheme, schemes
	}
	for name, schemeRef := range doc.Components.SecuritySchemes {
		if schemeRef == nil || schemeRef.Value == nil {
			continue
		}
		name = strings.TrimSpace(name)
		schemes[name] = schemeRef.Value
		configured, ok := cfg.Auth.Schemes[name]
		if !ok {
			continue
		}
		method := authMethodForScheme(instance, name, configured, schemeRef.Value)
		byScheme[name] = method
	}
	out := make([]sdkmanifest.AuthMethod, 0, len(byScheme))
	for _, method := range byScheme {
		out = append(out, method.Method)
	}
	return out, byScheme, schemes
}

func authMethodForScheme(instance, name string, cfg AuthSchemeConfig, scheme *openapi3.SecurityScheme) authBinding {
	kind := cfg.Kind
	if kind == "" {
		kind = defaultSecretKind(scheme)
	}
	header := cfg.Header
	if strings.TrimSpace(header.Name) == "" {
		header = defaultHeaderSpec(scheme)
	}
	displayName := firstNonEmpty(cfg.DisplayName, "OpenAPI "+name)
	description := firstNonEmpty(cfg.Description, "Credential for OpenAPI security scheme "+name+".")
	env := trimStrings(append([]string{cfg.Env.Name}, cfg.Env.Aliases...))
	if len(env) == 0 && cfg.Method == auth.MethodEnv {
		env = []string{strings.ToUpper(strings.TrimSpace(instance + "_" + name))}
	}
	field := sdkmanifest.AuthField{
		Name:        name,
		Description: description,
		Required:    true,
		Sensitive:   true,
		Secret:      true,
		Env:         env,
	}
	return authBinding{Config: cfg, Method: sdkmanifest.AuthMethod{
		Name:        name,
		Kind:        kind,
		Description: description,
		Env:         env,
		Fields:      []sdkmanifest.AuthField{field},
		Metadata: map[string]string{
			"display_name":  displayName,
			"header":        header.Name,
			"header_scheme": header.Scheme,
		},
	}}
}

func defaultSecretKind(scheme *openapi3.SecurityScheme) sharedsecret.Kind {
	if scheme == nil {
		return sharedsecret.KindAPIKey
	}
	if strings.EqualFold(scheme.Type, "http") {
		switch strings.ToLower(scheme.Scheme) {
		case "bearer":
			return sharedsecret.KindBearerToken
		case "basic":
			return sharedsecret.KindBasic
		}
	}
	if strings.EqualFold(scheme.Type, "oauth2") || strings.EqualFold(scheme.Type, "openIdConnect") {
		return sharedsecret.KindBearerToken
	}
	return sharedsecret.KindAPIKey
}

func defaultHeaderSpec(scheme *openapi3.SecurityScheme) auth.HeaderSpec {
	if scheme == nil {
		return auth.HeaderSpec{}
	}
	if strings.EqualFold(scheme.Type, "http") && strings.EqualFold(scheme.Scheme, "bearer") {
		return auth.HeaderSpec{Name: "Authorization", Scheme: "Bearer"}
	}
	if strings.EqualFold(scheme.Type, "apiKey") && strings.EqualFold(scheme.In, "header") {
		return auth.HeaderSpec{Name: scheme.Name}
	}
	if strings.EqualFold(scheme.Type, "oauth2") || strings.EqualFold(scheme.Type, "openIdConnect") {
		return auth.HeaderSpec{Name: "Authorization", Scheme: "Bearer"}
	}
	return auth.HeaderSpec{}
}

func trimStrings(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
