package homer

import (
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// CallDatasourceRecord is one grouped call as a datasource record.
type CallDatasourceRecord struct {
	pluginbinding.DatasourceRecord
	Title     string `json:"title,omitempty" datasource:"title,view=compact|lookup|table"`
	Caller    string `json:"caller,omitempty" datasource:"completion,view=compact|lookup|table"`
	Callee    string `json:"callee,omitempty" datasource:"completion,view=compact|lookup|table"`
	Status    string `json:"status,omitempty" datasource:"completion,view=compact|lookup|table"`
	StartTime string `json:"start_time,omitempty" datasource:"view=compact|lookup|table"`
	Duration  string `json:"duration,omitempty" datasource:"view=lookup|table"`
	Route     string `json:"route,omitempty" datasource:"completion,view=lookup|table"`
}

type CallsDatasourceResult = pluginbinding.DatasourceSearchResult[CallDatasourceRecord]

// CallsDatasource serves `datasource search homer` over grouped calls.
func (s Service) CallsDatasource(ctx pluginbinding.Context, input CallListInput) (CallsDatasourceResult, error) {
	out, err := s.CallList(ctx, input)
	if err != nil {
		return CallsDatasourceResult{}, err
	}
	records := make([]CallDatasourceRecord, 0, len(out.Calls))
	for _, call := range out.Calls {
		title := call.Caller + " → " + call.Callee
		record := CallDatasourceRecord{
			DatasourceRecord: pluginbinding.NewDatasourceRecord(ctx.DatasourceSource(), EntityCall, call.CallID,
				pluginbinding.RecordTitle(title),
				pluginbinding.RecordMetadata(map[string]any{
					"caller": call.Caller, "callee": call.Callee, "status": call.Status,
					"start_time": call.StartTime, "duration": call.Duration, "route": call.Route,
					"msg_count": call.MsgCount, "endpoint_url": out.URL,
				})),
			Title:     title,
			Caller:    call.Caller,
			Callee:    call.Callee,
			Status:    call.Status,
			StartTime: call.StartTime,
			Duration:  call.Duration,
			Route:     call.Route,
		}
		records = append(records, record)
	}
	return pluginbinding.NewDatasourceSearchResult("live", input.Query, records), nil
}
