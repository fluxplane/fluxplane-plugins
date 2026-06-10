package aws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	smithycbor "github.com/aws/smithy-go/encoding/cbor"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
)

// awsAPITestHost simulates the host secret store and http capability. It
// dispatches on the request URL host plus the query-protocol Action or the
// JSON-1.1 X-Amz-Target header, capturing every request so tests can assert
// the SigV4 signature produced by the SDK survived the host transport.
type awsAPITestHost struct {
	pluginbinding.HostClient
	secrets   map[string]string
	requests  []pluginbinding.HTTPRequest
	responses map[string][]hostResponse // key: dispatch key; popped in order
}

type hostResponse struct {
	status      int
	contentType string
	body        string
}

func newAWSTestHost() *awsAPITestHost {
	return &awsAPITestHost{
		secrets: map[string]string{
			SecretPurposeAccessKeyID:     "AKIAEXAMPLE",
			SecretPurposeSecretAccessKey: "secretsecret",
			SecretPurposeSessionToken:    "session-token-xyz",
		},
		responses: map[string][]hostResponse{},
	}
}

func (h *awsAPITestHost) stub(key string, responses ...hostResponse) {
	h.responses[key] = append(h.responses[key], responses...)
}

func (h *awsAPITestHost) Secret(purpose string) (pluginbinding.SecretMaterial, error) {
	value, ok := h.secrets[purpose]
	if !ok {
		return pluginbinding.SecretMaterial{}, fmt.Errorf("secret %q is not connected", purpose)
	}
	return pluginbinding.SecretMaterial{Value: value}, nil
}

// dispatchKey derives the lookup key for a captured request.
func dispatchKey(input pluginbinding.HTTPRequest) string {
	parsed, err := url.Parse(input.URL)
	if err != nil {
		return "parse-error"
	}
	host := parsed.Hostname()
	if target := input.Headers["X-Amz-Target"]; target != "" {
		return target
	}
	service := strings.Split(host, ".")[0]
	if strings.Contains(host, ".s3.") || strings.HasPrefix(host, "s3.") {
		service = "s3"
	}
	values, _ := url.ParseQuery(string(input.Body))
	if action := values.Get("Action"); action != "" {
		return service + "." + action
	}
	return service + " " + input.Method + " " + parsed.EscapedPath()
}

func (h *awsAPITestHost) HTTP(input pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	h.requests = append(h.requests, input)
	key := dispatchKey(input)
	queue := h.responses[key]
	if len(queue) == 0 {
		return pluginbinding.HTTPResponse{}, fmt.Errorf("no stub for %q (url %s)", key, input.URL)
	}
	response := queue[0]
	h.responses[key] = queue[1:]
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	headers := map[string][]string{}
	if response.contentType != "" {
		headers["Content-Type"] = []string{response.contentType}
	}
	if response.contentType == "application/cbor" {
		headers["Smithy-Protocol"] = []string{"rpc-v2-cbor"}
	}
	return pluginbinding.HTTPResponse{URL: input.URL, StatusCode: status, Headers: headers, Body: []byte(response.body)}, nil
}

const stsIdentityXML = `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:sts::123456789012:assumed-role/DeveloperAccess/devuser</Arn>
    <UserId>AROAEXAMPLE:devuser</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>req-1</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`

const stsExpiredXML = `<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <Error><Type>Sender</Type><Code>ExpiredToken</Code><Message>The security token included in the request is expired</Message></Error>
  <RequestId>req-2</RequestId>
</ErrorResponse>`

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestOperationExamplesDecode(t *testing.T) {
	manifest := Manifest()
	withExamples := 0
	for _, op := range manifest.Operations {
		var schema struct {
			Examples []map[string]any `json:"examples"`
		}
		if err := json.Unmarshal(op.Input, &schema); err != nil {
			t.Fatalf("%s input schema: %v", op.Name, err)
		}
		if len(schema.Examples) > 0 {
			withExamples++
		}
	}
	if withExamples < 10 {
		t.Fatalf("examples on %d ops, want at least the 10 networked ones", withExamples)
	}
}

func TestTestReturnsCallerIdentityAndSignsRequests(t *testing.T) {
	host := newAWSTestHost()
	host.stub("sts.GetCallerIdentity", hostResponse{contentType: "text/xml", body: stsIdentityXML})
	out := plugintest.RunOK[TestResult](t, NewPlugin(), OperationTest, TestInput{}, plugintest.WithHost(host))
	if out.Account != "123456789012" || !strings.Contains(out.ARN, "DeveloperAccess") || out.Region != DefaultRegion {
		t.Fatalf("out = %#v", out)
	}
	if len(host.requests) != 1 {
		t.Fatalf("requests = %d", len(host.requests))
	}
	request := host.requests[0]
	if !strings.Contains(request.URL, "sts.eu-central-1.amazonaws.com") {
		t.Fatalf("url = %s", request.URL)
	}
	authorization := request.Headers["Authorization"]
	if !strings.HasPrefix(authorization, "AWS4-HMAC-SHA256 Credential=AKIAEXAMPLE/") || !strings.Contains(authorization, "/eu-central-1/sts/aws4_request") {
		t.Fatalf("authorization = %q, want a SigV4 signature through the host transport", authorization)
	}
	if request.Headers["X-Amz-Security-Token"] != "session-token-xyz" {
		t.Fatalf("session token header missing: %#v", request.Headers)
	}
}

func TestRegionOverrideChangesHost(t *testing.T) {
	host := newAWSTestHost()
	host.stub("sts.GetCallerIdentity", hostResponse{contentType: "text/xml", body: stsIdentityXML})
	out := plugintest.RunOK[TestResult](t, NewPlugin(), OperationTest, TestInput{RegionInput: RegionInput{Region: "us-east-1"}}, plugintest.WithHost(host))
	if out.Region != "us-east-1" {
		t.Fatalf("region = %s", out.Region)
	}
	if !strings.Contains(host.requests[0].URL, "sts.us-east-1.amazonaws.com") {
		t.Fatalf("url = %s", host.requests[0].URL)
	}
}

func TestExpiredTokenMapsToActionableError(t *testing.T) {
	host := newAWSTestHost()
	// Two stubs: the SDK retries the 403 once (RetryMaxAttempts: 2).
	host.stub("sts.GetCallerIdentity",
		hostResponse{status: 403, contentType: "text/xml", body: stsExpiredXML},
		hostResponse{status: 403, contentType: "text/xml", body: stsExpiredXML})
	err := plugintest.RunError(t, NewPlugin(), OperationTest, TestInput{}, plugintest.WithHost(host))
	if err.Code != "expired_credentials" || !strings.Contains(err.Message, "auth auto aws") {
		t.Fatalf("err = %#v", err)
	}
}

func TestMissingSecretsFailActionably(t *testing.T) {
	host := newAWSTestHost()
	delete(host.secrets, SecretPurposeAccessKeyID)
	err := plugintest.RunError(t, NewPlugin(), OperationTest, TestInput{}, plugintest.WithHost(host))
	if err.Code != "not_connected" || !strings.Contains(err.Message, "auth auto aws") {
		t.Fatalf("err = %#v", err)
	}
}

const ec2InstancesXML = `<?xml version="1.0" encoding="UTF-8"?>
<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>req-3</requestId>
  <reservationSet>
    <item>
      <instancesSet>
        <item>
          <instanceId>i-0abc</instanceId>
          <instanceType>m5.large</instanceType>
          <imageId>ami-123</imageId>
          <privateIpAddress>10.0.1.5</privateIpAddress>
          <instanceState><code>16</code><name>running</name></instanceState>
          <placement><availabilityZone>eu-central-1a</availabilityZone></placement>
          <launchTime>2026-01-01T00:00:00.000Z</launchTime>
          <tagSet><item><key>Name</key><value>kamailio-1</value></item></tagSet>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`

func TestEC2InstancesFilterAndShape(t *testing.T) {
	host := newAWSTestHost()
	host.stub("ec2.DescribeInstances", hostResponse{contentType: "text/xml", body: ec2InstancesXML})
	out := plugintest.RunOK[EC2InstancesResult](t, NewPlugin(), OperationEC2Instances, EC2InstancesInput{
		Name:   "*kamailio*",
		States: []string{"running"},
	}, plugintest.WithHost(host))
	if out.Count != 1 || out.Instances[0].Name != "kamailio-1" || out.Instances[0].State != "running" || out.Instances[0].PrivateIP != "10.0.1.5" {
		t.Fatalf("out = %#v", out)
	}
	form, _ := url.ParseQuery(string(host.requests[0].Body))
	if form.Get("Filter.1.Name") != "tag:Name" || form.Get("Filter.1.Value.1") != "*kamailio*" {
		t.Fatalf("filters = %#v", form)
	}
	if form.Get("Filter.2.Name") != "instance-state-name" {
		t.Fatalf("state filter missing: %#v", form)
	}
}

func TestEC2DatasourceSearchWrapsInstances(t *testing.T) {
	host := newAWSTestHost()
	host.stub("ec2.DescribeInstances", hostResponse{contentType: "text/xml", body: ec2InstancesXML})
	records := plugintest.DatasourceSearchOK[EC2SearchResult](t, NewPlugin(), EC2SearchInput{Query: "kamailio"}, plugintest.WithHost(host))
	if records.Count != 1 || records.Records[0].InstanceID != "i-0abc" || records.Records[0].State != "running" {
		t.Fatalf("records = %#v", records)
	}
	form, _ := url.ParseQuery(string(host.requests[0].Body))
	if form.Get("Filter.1.Value.1") != "*kamailio*" {
		t.Fatalf("query should become a wildcard Name filter: %#v", form)
	}
}

func TestEKSClustersListsAndDescribes(t *testing.T) {
	host := newAWSTestHost()
	host.stub("eks GET /clusters", hostResponse{contentType: "application/json", body: `{"clusters":["dev-eu-central-1"]}`})
	host.stub("eks GET /clusters/dev-eu-central-1", hostResponse{contentType: "application/json", body: `{"cluster":{"name":"dev-eu-central-1","arn":"arn:aws:eks:eu-central-1:123456789012:cluster/dev-eu-central-1","version":"1.31","status":"ACTIVE","endpoint":"https://example.eks.amazonaws.com","platformVersion":"eks.10","resourcesVpcConfig":{"vpcId":"vpc-1"},"createdAt":1700000000.0}}`})
	out := plugintest.RunOK[EKSClustersResult](t, NewPlugin(), OperationEKSClusters, EKSClustersInput{}, plugintest.WithHost(host))
	if out.Count != 1 || out.Clusters[0].Name != "dev-eu-central-1" || out.Clusters[0].Version != "1.31" || out.Clusters[0].VPC != "vpc-1" {
		t.Fatalf("out = %#v", out)
	}
}

const rdsClustersXML = `<DescribeDBClustersResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBClustersResult>
    <DBClusters>
      <DBCluster>
        <DBClusterIdentifier>dev-aurora2-mysql</DBClusterIdentifier>
        <Engine>aurora-mysql</Engine>
        <EngineVersion>8.0.mysql_aurora.3</EngineVersion>
        <Status>available</Status>
        <Endpoint>writer.cluster.example</Endpoint>
        <ReaderEndpoint>reader.cluster.example</ReaderEndpoint>
        <Port>3306</Port>
        <DBClusterMembers><DBClusterMember><DBInstanceIdentifier>dev-aurora2-mysql-1</DBInstanceIdentifier></DBClusterMember></DBClusterMembers>
      </DBCluster>
    </DBClusters>
  </DescribeDBClustersResult>
  <ResponseMetadata><RequestId>r</RequestId></ResponseMetadata>
</DescribeDBClustersResponse>`

const rdsInstancesXML = `<DescribeDBInstancesResponse xmlns="http://rds.amazonaws.com/doc/2014-10-31/">
  <DescribeDBInstancesResult>
    <DBInstances>
      <DBInstance>
        <DBInstanceIdentifier>dev-aurora2-mysql-1</DBInstanceIdentifier>
        <Engine>aurora-mysql</Engine>
        <EngineVersion>8.0.mysql_aurora.3</EngineVersion>
        <DBInstanceStatus>available</DBInstanceStatus>
        <DBInstanceClass>db.r6g.large</DBInstanceClass>
        <AvailabilityZone>eu-central-1a</AvailabilityZone>
        <DBClusterIdentifier>dev-aurora2-mysql</DBClusterIdentifier>
        <Endpoint><Address>instance.example</Address><Port>3306</Port></Endpoint>
      </DBInstance>
    </DBInstances>
  </DescribeDBInstancesResult>
  <ResponseMetadata><RequestId>r</RequestId></ResponseMetadata>
</DescribeDBInstancesResponse>`

func TestRDSInstancesMapsClustersAndInstances(t *testing.T) {
	host := newAWSTestHost()
	host.stub("rds.DescribeDBClusters", hostResponse{contentType: "text/xml", body: rdsClustersXML})
	host.stub("rds.DescribeDBInstances", hostResponse{contentType: "text/xml", body: rdsInstancesXML})
	out := plugintest.RunOK[RDSInstancesResult](t, NewPlugin(), OperationRDSInstances, RDSInstancesInput{Engine: "aurora-mysql"}, plugintest.WithHost(host))
	if len(out.Clusters) != 1 || out.Clusters[0].WriterEndpoint != "writer.cluster.example" || out.Clusters[0].ReaderEndpoint != "reader.cluster.example" {
		t.Fatalf("clusters = %#v", out.Clusters)
	}
	if len(out.Instances) != 1 || out.Instances[0].Endpoint != "instance.example" || out.Instances[0].Cluster != "dev-aurora2-mysql" {
		t.Fatalf("instances = %#v", out.Instances)
	}
}

const s3ObjectsXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>my-bucket</Name>
  <KeyCount>1</KeyCount>
  <IsTruncated>true</IsTruncated>
  <NextContinuationToken>tok-2</NextContinuationToken>
  <Contents><Key>logs/a.txt</Key><Size>42</Size><LastModified>2026-01-01T00:00:00.000Z</LastModified><StorageClass>STANDARD</StorageClass></Contents>
</ListBucketResult>`

func TestS3ObjectsTruncationAndToken(t *testing.T) {
	host := newAWSTestHost()
	host.stub("s3 GET /", hostResponse{contentType: "application/xml", body: s3ObjectsXML})
	out := plugintest.RunOK[S3ObjectsResult](t, NewPlugin(), OperationS3Objects, S3ObjectsInput{Bucket: "my-bucket", Prefix: "logs/"}, plugintest.WithHost(host))
	if out.Count != 1 || out.Objects[0].Key != "logs/a.txt" || out.Objects[0].Size != 42 {
		t.Fatalf("out = %#v", out)
	}
	if !out.Truncated || out.NextToken != "tok-2" {
		t.Fatalf("truncation = %#v", out)
	}
	if !strings.Contains(host.requests[0].URL, "my-bucket.s3.eu-central-1.amazonaws.com") {
		t.Fatalf("url = %s", host.requests[0].URL)
	}
}

func TestLogsTailMapsEvents(t *testing.T) {
	host := newAWSTestHost()
	host.stub("Logs_20140328.FilterLogEvents", hostResponse{contentType: "application/x-amz-json-1.1", body: `{"events":[{"timestamp":1700000000000,"message":"ERROR boom\n","logStreamName":"s1"}]}`})
	out := plugintest.RunOK[LogsTailResult](t, NewPlugin(), OperationLogsTail, LogsTailInput{Group: "/aws/eks/dev"}, plugintest.WithHost(host))
	if out.Count != 1 || out.Events[0].Message != "ERROR boom" || out.Events[0].Stream != "s1" {
		t.Fatalf("out = %#v", out)
	}
}

func TestLogsQueryPollsUntilComplete(t *testing.T) {
	host := newAWSTestHost()
	host.stub("Logs_20140328.StartQuery", hostResponse{contentType: "application/x-amz-json-1.1", body: `{"queryId":"qid-1"}`})
	host.stub("Logs_20140328.GetQueryResults",
		hostResponse{contentType: "application/x-amz-json-1.1", body: `{"status":"Running"}`},
		hostResponse{contentType: "application/x-amz-json-1.1", body: `{"status":"Complete","results":[[{"field":"@timestamp","value":"2026-06-10 12:00:00.000"},{"field":"@message","value":"hello"}]],"statistics":{"recordsMatched":1,"recordsScanned":10}}`})
	out := plugintest.RunOK[LogsQueryResult](t, NewPlugin(), OperationLogsQuery, LogsQueryInput{
		Groups: []string{"/aws/eks/dev"},
		Query:  "fields @timestamp, @message | limit 1",
	}, plugintest.WithHost(host))
	if out.Status != "Complete" || len(out.Rows) != 1 || out.Rows[0]["@message"] != "hello" || out.RecordsScanned != 10 {
		t.Fatalf("out = %#v", out)
	}
	// StartQuery + 2 polls.
	if len(host.requests) != 3 {
		t.Fatalf("requests = %d, want 3 (start + 2 polls)", len(host.requests))
	}
}

// cloudwatchMetricsCBOR builds a GetMetricData response in the RPCv2 CBOR
// protocol the CloudWatch SDK speaks (timestamps are tag-1 epoch seconds).
func cloudwatchMetricsCBOR() string {
	return string(smithycbor.Encode(smithycbor.Map{
		"MetricDataResults": smithycbor.List{
			smithycbor.Map{
				"Id":    smithycbor.String("series0"),
				"Label": smithycbor.String("CPUUtilization"),
				"Timestamps": smithycbor.List{
					smithycbor.Tag{ID: 1, Value: smithycbor.Float64(1765364700)}, // later
					smithycbor.Tag{ID: 1, Value: smithycbor.Float64(1765364400)}, // earlier
				},
				"Values":     smithycbor.List{smithycbor.Float64(14.0), smithycbor.Float64(12.5)},
				"StatusCode": smithycbor.String("Complete"),
			},
		},
	}))
}

func TestCloudWatchMetricsSortsSeries(t *testing.T) {
	host := newAWSTestHost()
	host.stub("monitoring POST /service/GraniteServiceVersion20100801/operation/GetMetricData",
		hostResponse{contentType: "application/cbor", body: cloudwatchMetricsCBOR()})
	out := plugintest.RunOK[CloudWatchMetricsResult](t, NewPlugin(), OperationCloudWatchMetrics, CloudWatchMetricsInput{
		Namespace:  "AWS/RDS",
		Metric:     "CPUUtilization",
		Dimensions: map[string]string{"DBClusterIdentifier": "dev-aurora2-mysql"},
	}, plugintest.WithHost(host))
	if out.Count != 2 || out.Datapoints[0].Value != 12.5 || out.Datapoints[1].Value != 14.0 {
		t.Fatalf("out = %#v", out)
	}
	if out.Label != "CPUUtilization" || out.Stat != "Average" {
		t.Fatalf("series meta = %#v", out)
	}
}

func TestLogsQueryValidatesInput(t *testing.T) {
	err := plugintest.RunError(t, NewPlugin(), OperationLogsQuery, LogsQueryInput{Query: "fields @message"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
	err = plugintest.RunError(t, NewPlugin(), OperationS3Objects, S3ObjectsInput{})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}
