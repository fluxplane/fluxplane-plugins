package aws

import (
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const (
	// DefaultRegion is used when an operation omits region.
	DefaultRegion = "eu-central-1"

	SecretPurposeAccessKeyID     = "access_key_id"
	SecretPurposeSecretAccessKey = "secret_access_key"
	SecretPurposeSessionToken    = "session_token"
)

// awsConfig builds an aws.Config whose credentials come exclusively from the
// host secret store (persisted by `auth auto`/`auth connect`) and whose HTTP
// transport is the host http capability. Never use config.LoadDefaultConfig
// here: it reads the environment and ~/.aws at invoke time, which breaks
// reproducibility.
func awsConfig(ctx pluginbinding.Context, region string) (awssdk.Config, error) {
	if ctx.Host == nil {
		return awssdk.Config{}, pluginbinding.Fail("host_unavailable", "host client is unavailable")
	}
	access, err := ctx.Host.Secret(SecretPurposeAccessKeyID)
	if err != nil {
		return awssdk.Config{}, notConnectedError(err)
	}
	secret, err := ctx.Host.Secret(SecretPurposeSecretAccessKey)
	if err != nil {
		return awssdk.Config{}, notConnectedError(err)
	}
	token := ""
	if material, err := ctx.Host.Secret(SecretPurposeSessionToken); err == nil {
		token = strings.TrimSpace(material.Value)
	}
	if region = strings.TrimSpace(region); region == "" {
		region = DefaultRegion
	}
	return awssdk.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(access.Value), strings.TrimSpace(secret.Value), token),
		HTTPClient: pluginbinding.HostHTTPClient(ctx.Host,
			pluginbinding.HostHTTPClientTimeout(60_000),
			pluginbinding.HostHTTPClientMaxBytes(32<<20)),
		// Keep synchronous invocations snappy; throttling errors surface as
		// structured "throttled" failures instead of long retry sleeps.
		RetryMaxAttempts: 2,
	}, nil
}
