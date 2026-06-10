package aws

import (
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type EC2InstancesInput struct {
	RegionInput
	Name   string   `json:"name,omitempty" jsonschema:"description=Filter by the Name tag; * wildcards supported (e.g. *kamailio*)."`
	States []string `json:"states,omitempty" jsonschema:"description=Instance state filters such as running or stopped."`
	IDs    []string `json:"ids,omitempty" jsonschema:"description=Exact instance IDs."`
	Limit  int      `json:"limit,omitempty" jsonschema:"description=Maximum instances returned. Defaults to 50 and is capped at 500.,minimum=0,maximum=500"`
}

type EC2Instance struct {
	ID         string            `json:"id"`
	Name       string            `json:"name,omitempty"`
	State      string            `json:"state,omitempty"`
	Type       string            `json:"type,omitempty"`
	AZ         string            `json:"az,omitempty"`
	PrivateIP  string            `json:"private_ip,omitempty"`
	PublicIP   string            `json:"public_ip,omitempty"`
	Image      string            `json:"image,omitempty"`
	LaunchTime string            `json:"launch_time,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type EC2InstancesResult struct {
	Region    string        `json:"region"`
	Instances []EC2Instance `json:"instances,omitempty"`
	Count     int           `json:"count"`
	Truncated bool          `json:"truncated,omitempty"`
}

// EC2Instances lists EC2 instances with Name-tag and state filters.
func (s Service) EC2Instances(ctx pluginbinding.Context, input EC2InstancesInput) (EC2InstancesResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return EC2InstancesResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	request := &ec2.DescribeInstancesInput{}
	if name := strings.TrimSpace(input.Name); name != "" {
		request.Filters = append(request.Filters, ec2types.Filter{Name: strPtr("tag:Name"), Values: []string{name}})
	}
	if len(input.States) > 0 {
		request.Filters = append(request.Filters, ec2types.Filter{Name: strPtr("instance-state-name"), Values: input.States})
	}
	if len(input.IDs) > 0 {
		request.InstanceIds = input.IDs
	}
	callCtx, cancel := opContext()
	defer cancel()
	client := ec2.NewFromConfig(cfg)
	out := EC2InstancesResult{Region: cfg.Region}
	paginator := ec2.NewDescribeInstancesPaginator(client, request)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(callCtx)
		if err != nil {
			return EC2InstancesResult{}, mapAWSError("ec2 describe-instances", err)
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if len(out.Instances) >= limit {
					out.Truncated = true
					break
				}
				out.Instances = append(out.Instances, mapEC2Instance(instance))
			}
		}
		if out.Truncated {
			break
		}
	}
	sort.Slice(out.Instances, func(i, j int) bool { return out.Instances[i].Name < out.Instances[j].Name })
	out.Count = len(out.Instances)
	return out, nil
}

func mapEC2Instance(instance ec2types.Instance) EC2Instance {
	mapped := EC2Instance{
		ID:        str(instance.InstanceId),
		Type:      string(instance.InstanceType),
		PrivateIP: str(instance.PrivateIpAddress),
		PublicIP:  str(instance.PublicIpAddress),
		Image:     str(instance.ImageId),
	}
	if instance.State != nil {
		mapped.State = string(instance.State.Name)
	}
	if instance.Placement != nil {
		mapped.AZ = str(instance.Placement.AvailabilityZone)
	}
	if instance.LaunchTime != nil {
		mapped.LaunchTime = instance.LaunchTime.UTC().Format(time.RFC3339)
	}
	tags := map[string]string{}
	for _, tag := range instance.Tags {
		key := str(tag.Key)
		tags[key] = str(tag.Value)
		if key == "Name" {
			mapped.Name = str(tag.Value)
		}
	}
	if len(tags) > 0 {
		mapped.Tags = tags
	}
	return mapped
}

func strPtr(value string) *string { return &value }
