package aws

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type CloudWatchMetricsInput struct {
	RegionInput
	TimeRangeInput
	Namespace  string            `json:"namespace,omitempty" jsonschema:"required,description=Metric namespace such as AWS/RDS or AWS/EC2."`
	Metric     string            `json:"metric,omitempty" jsonschema:"required,description=Metric name such as CPUUtilization."`
	Dimensions map[string]string `json:"dimensions,omitempty" jsonschema:"description=Dimension name/value pairs\\, e.g. DBClusterIdentifier=dev-aurora2-mysql."`
	Stat       string            `json:"stat,omitempty" jsonschema:"description=Statistic: Average\\, Sum\\, Minimum\\, Maximum\\, SampleCount\\, or a percentile like p99. Defaults to Average."`
	Period     int               `json:"period,omitempty" jsonschema:"description=Period in seconds. Defaults to 300.,minimum=0"`
}

type MetricDatapoint struct {
	Time  string  `json:"time"`
	Value float64 `json:"value"`
}

type CloudWatchMetricsResult struct {
	Region     string            `json:"region"`
	Namespace  string            `json:"namespace"`
	Metric     string            `json:"metric"`
	Stat       string            `json:"stat"`
	Label      string            `json:"label,omitempty"`
	Datapoints []MetricDatapoint `json:"datapoints"`
	Count      int               `json:"count"`}

// CloudWatchMetrics fetches one metric series via GetMetricData.
func (s Service) CloudWatchMetrics(ctx pluginbinding.Context, input CloudWatchMetricsInput) (CloudWatchMetricsResult, error) {
	namespace := strings.TrimSpace(input.Namespace)
	metric := strings.TrimSpace(input.Metric)
	if namespace == "" || metric == "" {
		return CloudWatchMetricsResult{}, pluginbinding.Fail("bad_input", "namespace and metric are required")
	}
	from, to, err := input.window("3h")
	if err != nil {
		return CloudWatchMetricsResult{}, pluginbinding.Errorf("bad_input", "%s", err)
	}
	stat := firstNonEmpty(input.Stat, "Average")
	period := input.Period
	if period <= 0 {
		period = 300
	}
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return CloudWatchMetricsResult{}, err
	}
	var dimensions []cwtypes.Dimension
	for name, value := range input.Dimensions {
		dimensions = append(dimensions, cwtypes.Dimension{Name: strPtr(name), Value: strPtr(value)})
	}
	request := &cloudwatch.GetMetricDataInput{
		StartTime: &from,
		EndTime:   &to,
		MetricDataQueries: []cwtypes.MetricDataQuery{{
			Id: strPtr("series0"),
			MetricStat: &cwtypes.MetricStat{
				Metric: &cwtypes.Metric{
					Namespace:  &namespace,
					MetricName: &metric,
					Dimensions: dimensions,
				},
				Period: int32Ptr(int32(period)),
				Stat:   &stat,
			},
		}},
	}
	callCtx, cancel := opContext()
	defer cancel()
	out := CloudWatchMetricsResult{Region: cfg.Region, Namespace: namespace, Metric: metric, Stat: stat, Datapoints: []MetricDatapoint{}}
	client := cloudwatch.NewFromConfig(cfg)
	paginator := cloudwatch.NewGetMetricDataPaginator(client, request)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(callCtx)
		if err != nil {
			return CloudWatchMetricsResult{}, mapAWSError("cloudwatch get-metric-data", err)
		}
		for _, series := range page.MetricDataResults {
			if out.Label == "" {
				out.Label = str(series.Label)
			}
			for i := range series.Timestamps {
				point := MetricDatapoint{Time: series.Timestamps[i].UTC().Format(time.RFC3339)}
				if i < len(series.Values) {
					point.Value = series.Values[i]
				}
				out.Datapoints = append(out.Datapoints, point)
			}
		}
	}
	sortDatapoints(out.Datapoints)
	out.Count = len(out.Datapoints)
	return out, nil
}

func sortDatapoints(points []MetricDatapoint) {
	for i := 1; i < len(points); i++ {
		for j := i; j > 0 && points[j].Time < points[j-1].Time; j-- {
			points[j], points[j-1] = points[j-1], points[j]
		}
	}
}
