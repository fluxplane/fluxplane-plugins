package aws

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type RDSInstancesInput struct {
	RegionInput
	Engine string `json:"engine,omitempty" jsonschema:"description=Filter by engine such as aurora-mysql or postgres."`
	Limit  int    `json:"limit,omitempty" jsonschema:"description=Maximum instances returned. Defaults to 100 and is capped at 500.,minimum=0,maximum=500"`
}

type RDSCluster struct {
	ID             string   `json:"id"`
	Engine         string   `json:"engine,omitempty"`
	EngineVersion  string   `json:"engine_version,omitempty"`
	Status         string   `json:"status,omitempty"`
	WriterEndpoint string   `json:"writer_endpoint,omitempty"`
	ReaderEndpoint string   `json:"reader_endpoint,omitempty"`
	Port           int32    `json:"port,omitempty"`
	Members        []string `json:"members,omitempty"`
}

type RDSInstance struct {
	ID       string `json:"id"`
	Engine   string `json:"engine,omitempty"`
	Version  string `json:"version,omitempty"`
	Status   string `json:"status,omitempty"`
	Class    string `json:"class,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Port     int32  `json:"port,omitempty"`
	AZ       string `json:"az,omitempty"`
	MultiAZ  bool   `json:"multi_az,omitempty"`
	Cluster  string `json:"cluster,omitempty"`
}

type RDSInstancesResult struct {
	Region    string        `json:"region"`
	Clusters  []RDSCluster  `json:"clusters"`
	Instances []RDSInstance `json:"instances"`
	Count     int           `json:"count"`
	Truncated bool          `json:"truncated,omitempty"`}

// RDSInstances lists RDS/Aurora clusters and database instances.
func (s Service) RDSInstances(ctx pluginbinding.Context, input RDSInstancesInput) (RDSInstancesResult, error) {
	cfg, err := awsConfig(ctx, input.Region)
	if err != nil {
		return RDSInstancesResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	engine := strings.ToLower(strings.TrimSpace(input.Engine))
	callCtx, cancel := opContext()
	defer cancel()
	client := rds.NewFromConfig(cfg)
	out := RDSInstancesResult{Region: cfg.Region, Instances: []RDSInstance{}, Clusters: []RDSCluster{}}

	clusterPaginator := rds.NewDescribeDBClustersPaginator(client, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(callCtx)
		if err != nil {
			return RDSInstancesResult{}, mapAWSError("rds describe-db-clusters", err)
		}
		for _, cluster := range page.DBClusters {
			if engine != "" && !strings.Contains(strings.ToLower(str(cluster.Engine)), engine) {
				continue
			}
			mapped := RDSCluster{
				ID:             str(cluster.DBClusterIdentifier),
				Engine:         str(cluster.Engine),
				EngineVersion:  str(cluster.EngineVersion),
				Status:         str(cluster.Status),
				WriterEndpoint: str(cluster.Endpoint),
				ReaderEndpoint: str(cluster.ReaderEndpoint),
			}
			if cluster.Port != nil {
				mapped.Port = *cluster.Port
			}
			for _, member := range cluster.DBClusterMembers {
				mapped.Members = append(mapped.Members, str(member.DBInstanceIdentifier))
			}
			out.Clusters = append(out.Clusters, mapped)
		}
	}

	instancePaginator := rds.NewDescribeDBInstancesPaginator(client, &rds.DescribeDBInstancesInput{})
	for instancePaginator.HasMorePages() {
		page, err := instancePaginator.NextPage(callCtx)
		if err != nil {
			return RDSInstancesResult{}, mapAWSError("rds describe-db-instances", err)
		}
		for _, instance := range page.DBInstances {
			if engine != "" && !strings.Contains(strings.ToLower(str(instance.Engine)), engine) {
				continue
			}
			if len(out.Instances) >= limit {
				out.Truncated = true
				break
			}
			mapped := RDSInstance{
				ID:      str(instance.DBInstanceIdentifier),
				Engine:  str(instance.Engine),
				Version: str(instance.EngineVersion),
				Status:  str(instance.DBInstanceStatus),
				Class:   str(instance.DBInstanceClass),
				AZ:      str(instance.AvailabilityZone),
				Cluster: str(instance.DBClusterIdentifier),
			}
			if instance.MultiAZ != nil {
				mapped.MultiAZ = *instance.MultiAZ
			}
			if instance.Endpoint != nil {
				mapped.Endpoint = str(instance.Endpoint.Address)
				if instance.Endpoint.Port != nil {
					mapped.Port = *instance.Endpoint.Port
				}
			}
			out.Instances = append(out.Instances, mapped)
		}
		if out.Truncated {
			break
		}
	}
	out.Count = len(out.Clusters) + len(out.Instances)
	return out, nil
}
