package aws

import (
	"errors"

	"github.com/aws/smithy-go"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// reconnectHint is the full recovery flow for missing or expired credentials.
// Operations never read the environment; `auth auto` ingests the exported
// temporary credentials once and persists them in the secret store.
const reconnectHint = "refresh and reconnect: aws sso login --profile <profile>; " +
	"eval \"$(aws configure export-credentials --profile <profile> --format env)\"; " +
	"fluxplane-plugin auth auto aws [--instance <account>]"

var expiredCredentialCodes = map[string]bool{
	"ExpiredToken":                true,
	"ExpiredTokenException":       true,
	"RequestExpired":              true,
	"InvalidClientTokenId":        true,
	"UnrecognizedClientException": true,
	"InvalidAccessKeyId":          true,
	"AuthFailure":                 true,
}

// notConnectedError wraps a missing-secret failure with the setup flow.
func notConnectedError(err error) error {
	return pluginbinding.Errorf("not_connected", "AWS credentials are not connected (%s) — %s", err, reconnectHint)
}

// mapAWSError converts AWS SDK errors into structured, actionable plugin
// failures. Expired or invalid temporary credentials point at the reconnect
// flow instead of surfacing a bare 403.
func mapAWSError(op string, err error) error {
	if err == nil {
		return nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		switch {
		case expiredCredentialCodes[code]:
			return pluginbinding.Errorf("expired_credentials", "AWS credentials are expired or invalid (%s) — %s", code, reconnectHint)
		case code == "AccessDenied" || code == "AccessDeniedException" || code == "UnauthorizedOperation" || code == "UnauthorizedAccess":
			return pluginbinding.Errorf("access_denied", "%s: %s", op, apiErr.ErrorMessage())
		case code == "Throttling" || code == "ThrottlingException" || code == "RequestLimitExceeded" || code == "TooManyRequestsException":
			return pluginbinding.Errorf("throttled", "%s: %s — retry shortly", op, apiErr.ErrorMessage())
		}
	}
	return pluginbinding.Errorf("aws", "%s: %v", op, err)
}
