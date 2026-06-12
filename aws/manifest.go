package aws

import (
	"encoding/json"

	fpcontext "github.com/fluxplane/fluxplane-context"
	evidence "github.com/fluxplane/fluxplane-evidence"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "aws"
	PluginVersion     = "0.3.1"
	PluginDescription = "Read-only AWS operations (STS, EC2, EKS, RDS, S3, CloudWatch logs and metrics) plus environment inspection."

	AuthMethodAccessKeys = "access_keys"
	EnvAccessKeyID       = "AWS_ACCESS_KEY_ID"
	EnvSecretAccessKey   = "AWS_SECRET_ACCESS_KEY"
	EnvSessionToken      = "AWS_SESSION_TOKEN"

	OperationTest              = "aws.test"
	OperationInspect           = "aws.inspect"
	OperationEC2Instances      = "aws.ec2.instances"
	OperationEKSClusters       = "aws.eks.clusters"
	OperationRDSInstances      = "aws.rds.instances"
	OperationS3Buckets         = "aws.s3.buckets"
	OperationS3Objects         = "aws.s3.objects"
	OperationLogsGroups        = "aws.logs.groups"
	OperationLogsTail          = "aws.logs.tail"
	OperationLogsQuery         = "aws.logs.query"
	OperationCloudWatchMetrics = "aws.cloudwatch.metrics"

	DatasourceEC2     = "aws.ec2"
	EntityEC2Instance = "aws.ec2.instance"

	ContextName                      = "aws.environment"
	ObserverEnvironment              = "aws.environment"
	ObservationEnvironmentConfigured = "aws.environment.configured"
	ObservationEnvironmentAvailable  = "aws.environment.available"
	AssertionConfigured              = "integration.configured"
	AssertionAvailable               = "integration.available"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe`. Kept local to the aws plugin rather than
// promoted to the SDK.
func withInputExamples(spec core.OperationSpec, examples ...map[string]any) core.OperationSpec {
	if len(examples) == 0 || len(spec.Input) == 0 {
		return spec
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Input, &schema); err != nil {
		return spec
	}
	arr := make([]any, 0, len(examples))
	for _, example := range examples {
		arr = append(arr, example)
	}
	schema["examples"] = arr
	if raw, err := json.Marshal(schema); err == nil {
		spec.Input = raw
	}
	return spec
}

func Manifest() core.PluginManifest {
	return pluginbinding.Manifest(manifestSpec())
}

func manifestSpec() pluginbinding.ManifestSpec {
	return pluginbinding.ManifestSpec{
		Name:        PluginName,
		Version:     PluginVersion,
		Description: PluginDescription,
		Aliases:     []string{PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{{
			Name: AuthMethodAccessKeys,
			Kind: "credentials",
			Description: "AWS access keys (static or temporary SSO/STS credentials). Setup: " +
				"aws sso login --profile <p>; eval \"$(aws configure export-credentials --profile <p> --format env)\"; " +
				"fluxplane-plugin auth auto aws [--instance <account>]. Use one plugin instance per AWS account. " +
				"Operations resolve credentials only from the persisted secret store, never from the environment or ~/.aws at invoke time.",
			Env: []string{EnvAccessKeyID, EnvSecretAccessKey, EnvSessionToken},
			Fields: []core.AuthField{
				pluginbinding.AuthField(SecretPurposeAccessKeyID, "AWS access key ID", true, false, EnvAccessKeyID),
				pluginbinding.AuthField(SecretPurposeSecretAccessKey, "AWS secret access key", true, true, EnvSecretAccessKey),
				pluginbinding.AuthField(SecretPurposeSessionToken, "AWS session token (temporary credentials)", false, true, EnvSessionToken),
			},
		}},
		Operations: []core.OperationSpec{
			testSpec(),
			inspectSpec(),
			ec2InstancesSpec(),
			eksClustersSpec(),
			rdsInstancesSpec(),
			s3BucketsSpec(),
			s3ObjectsSpec(),
			logsGroupsSpec(),
			logsTailSpec(),
			logsQuerySpec(),
			cloudWatchMetricsSpec(),
		},
		Datasources: []core.DatasourceSpec{ec2DatasourceSpec()},
		Context: []core.ContextSpec{{
			Name:             ContextName,
			Description:      "Non-secret AWS profile, region, and credential presence.",
			Kinds:            []fpcontext.BlockKind{fpcontext.BlockText, fpcontext.BlockData},
			DefaultPlacement: fpcontext.PlacementSystem,
		}},
		Observers:         []core.ObserverSpec{environmentObserverSpec()},
		AssertionDerivers: []core.AssertionDeriverSpec{configuredAssertionDeriverSpec(), availableAssertionDeriverSpec()},
	}
}

// awsReadSpecOptions are shared by every networked read operation.
func awsReadSpecOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(SecretPurposeAccessKeyID, SecretPurposeSecretAccessKey, SecretPurposeSessionToken),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessNetwork),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
}

func testSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[TestInput, TestResult](
		OperationTest,
		"Verify AWS connectivity and credential validity via STS GetCallerIdentity.",
		awsReadSpecOptions()...,
	), map[string]any{})
}

func ec2InstancesSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[EC2InstancesInput, EC2InstancesResult](
		OperationEC2Instances,
		"List EC2 instances with Name-tag wildcard and state filters.",
		awsReadSpecOptions()...,
	), map[string]any{"name": "*kamailio*", "states": []any{"running"}})
}

func eksClustersSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[EKSClustersInput, EKSClustersResult](
		OperationEKSClusters,
		"List and describe EKS clusters (version, status, endpoint, VPC).",
		awsReadSpecOptions()...,
	), map[string]any{})
}

func rdsInstancesSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[RDSInstancesInput, RDSInstancesResult](
		OperationRDSInstances,
		"List RDS/Aurora clusters (writer/reader endpoints, members) and database instances.",
		awsReadSpecOptions()...,
	), map[string]any{"engine": "aurora-mysql"})
}

func s3BucketsSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[S3BucketsInput, S3BucketsResult](
		OperationS3Buckets,
		"List S3 buckets, optionally filtered by name prefix.",
		awsReadSpecOptions()...,
	), map[string]any{})
}

func s3ObjectsSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[S3ObjectsInput, S3ObjectsResult](
		OperationS3Objects,
		"List S3 objects under a prefix with continuation-token pagination.",
		awsReadSpecOptions()...,
	), map[string]any{"bucket": "my-bucket", "prefix": "logs/", "limit": 100})
}

func logsGroupsSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[LogsGroupsInput, LogsGroupsResult](
		OperationLogsGroups,
		"List CloudWatch log groups with retention and size.",
		awsReadSpecOptions()...,
	), map[string]any{"prefix": "/aws/eks/"})
}

func logsTailSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[LogsTailInput, LogsTailResult](
		OperationLogsTail,
		"Read recent events from a CloudWatch log group (FilterLogEvents over a time window).",
		awsReadSpecOptions()...,
	), map[string]any{"group": "/aws/eks/dev/cluster", "since": "15m", "pattern": "ERROR"})
}

func logsQuerySpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[LogsQueryInput, LogsQueryResult](
		OperationLogsQuery,
		"Run a bounded CloudWatch Logs Insights query and wait for its results.",
		awsReadSpecOptions()...,
	), map[string]any{"groups": []any{"/aws/eks/dev/cluster"}, "query": "fields @timestamp, @message | sort @timestamp desc | limit 20", "since": "1h"})
}

func cloudWatchMetricsSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[CloudWatchMetricsInput, CloudWatchMetricsResult](
		OperationCloudWatchMetrics,
		"Fetch one CloudWatch metric series (GetMetricData) over a time window.",
		awsReadSpecOptions()...,
	), map[string]any{"namespace": "AWS/RDS", "metric": "CPUUtilization", "dimensions": map[string]any{"DBClusterIdentifier": "dev-aurora2-mysql"}, "since": "3h", "stat": "Average"})
}

func ec2DatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[EC2SearchInput, EC2SearchResult](
		DatasourceEC2,
		EntityEC2Instance,
		"EC2 instances searchable by Name tag.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceSecretPurposes(SecretPurposeAccessKeyID, SecretPurposeSecretAccessKey, SecretPurposeSessionToken),
		pluginbinding.DatasourceAccess(core.OperationAccessNetwork),
		pluginbinding.EntitySchemaFor[EC2InstanceRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "instance_id", TitleField: "name"}),
		pluginbinding.Completion("EC2 instance fields.", "state", "private_ip", "name"),
	)
}

func inspectSpec() core.OperationSpec {
	return pluginbinding.TypedOperationSpec[InspectInput, Environment](
		OperationInspect,
		"Inspect non-secret AWS environment configuration and credential presence (setup-time tooling; reading the environment is this operation's declared purpose).",
		pluginbinding.ReadOnly(),
		pluginbinding.Compact(),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectLocalSystem),
		pluginbinding.Access(core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	)
}

func environmentObserverSpec() core.ObserverSpec {
	return core.ObserverSpec{
		Name:        ObserverEnvironment,
		Description: "Observes non-secret AWS environment configuration and credential presence.",
		Environment: evidence.Ref{
			Name: evidence.Name(PluginName),
		},
		Phase: core.ObservationPhaseTurn,
		ObservableKinds: []string{
			ObservationEnvironmentConfigured,
			ObservationEnvironmentAvailable,
		},
		Dynamic: true,
	}
}

func configuredAssertionDeriverSpec() core.AssertionDeriverSpec {
	return core.AssertionDeriverSpec{
		Name:             "aws.environment.configured",
		Description:      "Derives AWS integration configuration from non-secret environment evidence.",
		ObservationKinds: []string{ObservationEnvironmentConfigured},
		Assertions: []core.AssertionTemplate{{
			Kind:    AssertionConfigured,
			Target:  PluginName,
			Subject: evidence.Subject{Kind: evidence.SubjectIntegration, Name: PluginName},
		}},
	}
}

func availableAssertionDeriverSpec() core.AssertionDeriverSpec {
	return core.AssertionDeriverSpec{
		Name:             "aws.environment.available",
		Description:      "Derives AWS integration availability from non-secret environment evidence.",
		ObservationKinds: []string{ObservationEnvironmentAvailable},
		Assertions: []core.AssertionTemplate{{
			Kind:    AssertionAvailable,
			Target:  PluginName,
			Subject: evidence.Subject{Kind: evidence.SubjectIntegration, Name: PluginName},
		}},
	}
}
