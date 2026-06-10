package aws

import (
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

type EC2InstanceRecord struct {
	pluginbinding.DatasourceRecord
	InstanceID string `json:"instance_id" datasource:"id,completion,view=compact|lookup|table"`
	Name       string `json:"name,omitempty" datasource:"title,completion,view=compact|lookup|table"`
	State      string `json:"state,omitempty" datasource:"completion,view=compact|lookup|table"`
	Type       string `json:"type,omitempty" datasource:"view=lookup|table"`
	AZ         string `json:"az,omitempty" datasource:"view=lookup|table"`
	PrivateIP  string `json:"private_ip,omitempty" datasource:"completion,view=compact|lookup|table"`
	PublicIP   string `json:"public_ip,omitempty" datasource:"view=lookup|table"`
}

type EC2SearchResult = pluginbinding.DatasourceSearchResult[EC2InstanceRecord]

type EC2SearchInput struct {
	RegionInput
	Query string `json:"query,omitempty" jsonschema:"description=Name fragment to match (wrapped in * wildcards)."`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Maximum instances returned. Defaults to 50.,minimum=0,maximum=500"`
}

// EC2Search exposes EC2 instances as a search datasource.
func (s Service) EC2Search(ctx pluginbinding.Context, input EC2SearchInput) (EC2SearchResult, error) {
	listInput := EC2InstancesInput{RegionInput: input.RegionInput, Limit: input.Limit}
	if query := strings.TrimSpace(input.Query); query != "" {
		listInput.Name = "*" + strings.Trim(query, "*") + "*"
	}
	listed, err := s.EC2Instances(ctx, listInput)
	if err != nil {
		return EC2SearchResult{}, err
	}
	records := make([]EC2InstanceRecord, 0, len(listed.Instances))
	for _, instance := range listed.Instances {
		record := EC2InstanceRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(
				ctx.DatasourceSource(),
				EntityEC2Instance,
				instance.ID,
				pluginbinding.RecordTitle(firstNonEmpty(instance.Name, instance.ID)),
				pluginbinding.RecordMetadata(map[string]any{
					"state":      instance.State,
					"type":       instance.Type,
					"az":         instance.AZ,
					"private_ip": instance.PrivateIP,
					"region":     listed.Region,
				}),
			),
			InstanceID: instance.ID,
			Name:       instance.Name,
			State:      instance.State,
			Type:       instance.Type,
			AZ:         instance.AZ,
			PrivateIP:  instance.PrivateIP,
			PublicIP:   instance.PublicIP,
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
}
