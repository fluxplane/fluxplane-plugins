package aws

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

const maxEKSDescribes = 20

type EKSClustersInput struct {
	RegionInput
	Name string `json:"name,omitempty" jsonschema:"description=Exact cluster name to describe. Default lists and describes all (bounded)."`
}

type EKSCluster struct {
	Name            string `json:"name"`
	ARN             string `json:"arn,omitempty"`
	Version         string `json:"version,omitempty"`
	Status          string `json:"status,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"`
	PlatformVersion string `json:"platform_version,omitempty"`
	VPC             string `json:"vpc,omitempty"`
	Created         string `json:"created,omitempty"`
}

type EKSClustersResult struct {
	Region    string       `json:"region"`
	Clusters  []EKSCluster `json:"clusters,omitempty"`
	Count     int          `json:"count"`
	Truncated bool         `json:"truncated,omitempty"`
}

// EKSClusters lists EKS clusters and describes each (bounded).
func (s Service) EKSClusters(ctx pluginbinding.Context, input EKSClustersInput) (EKSClustersResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return EKSClustersResult{}, err
	}
	callCtx, cancel := opContext()
	defer cancel()
	client := eks.NewFromConfig(cfg)
	names := []string{}
	if name := strings.TrimSpace(input.Name); name != "" {
		names = append(names, name)
	} else {
		paginator := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(callCtx)
			if err != nil {
				return EKSClustersResult{}, mapAWSError("eks list-clusters", err)
			}
			names = append(names, page.Clusters...)
		}
	}
	out := EKSClustersResult{Region: cfg.Region}
	if len(names) > maxEKSDescribes {
		names = names[:maxEKSDescribes]
		out.Truncated = true
	}
	for _, name := range names {
		described, err := client.DescribeCluster(callCtx, &eks.DescribeClusterInput{Name: strPtr(name)})
		if err != nil {
			return EKSClustersResult{}, mapAWSError("eks describe-cluster "+name, err)
		}
		cluster := EKSCluster{Name: name}
		if c := described.Cluster; c != nil {
			cluster.ARN = str(c.Arn)
			cluster.Version = str(c.Version)
			cluster.Status = string(c.Status)
			cluster.Endpoint = str(c.Endpoint)
			cluster.PlatformVersion = str(c.PlatformVersion)
			if c.ResourcesVpcConfig != nil {
				cluster.VPC = str(c.ResourcesVpcConfig.VpcId)
			}
			if c.CreatedAt != nil {
				cluster.Created = c.CreatedAt.UTC().Format(time.RFC3339)
			}
		}
		out.Clusters = append(out.Clusters, cluster)
	}
	out.Count = len(out.Clusters)
	return out, nil
}
