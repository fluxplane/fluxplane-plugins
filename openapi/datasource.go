package openapi

import (
	"fmt"
	"sort"
	"strings"

	coredatasource "github.com/fluxplane/fluxplane-datasource"
	sdkmanifest "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/getkin/kin-openapi/openapi3"
)

type DocRecord struct {
	ID      string                    `json:"id" datasource:"id" jsonschema:"description=Stable documentation record id."`
	Entity  coredatasource.EntityType `json:"entity" datasource:"filterable" jsonschema:"description=OpenAPI documentation entity."`
	Title   string                    `json:"title" datasource:"searchable" jsonschema:"description=Record title."`
	Content string                    `json:"content" datasource:"searchable,corpus" jsonschema:"description=Record content."`
	URL     string                    `json:"url,omitempty" datasource:"url" jsonschema:"description=Documentation URL."`
	Method  string                    `json:"method,omitempty" datasource:"filterable" jsonschema:"description=HTTP method."`
	Path    string                    `json:"path,omitempty" datasource:"searchable,filterable" jsonschema:"description=OpenAPI path."`
	Name    string                    `json:"name,omitempty" datasource:"searchable,filterable" jsonschema:"description=OpenAPI component or field name."`
	Status  string                    `json:"status,omitempty" datasource:"filterable" jsonschema:"description=Response status."`
	Scheme  string                    `json:"scheme,omitempty" datasource:"filterable" jsonschema:"description=Security scheme."`
	Raw     any                       `json:"raw,omitempty" jsonschema:"description=Original record metadata."`
}

func datasourceEntities(spec sdkmanifest.DatasourceSpec) []coredatasource.EntityType {
	return []coredatasource.EntityType{OperationEntity, SchemaEntity, ParameterEntity, ResponseEntity, SecuritySchemeEntity}
}

func searchDocs(docs []DocRecord, req pluginbinding.DatasourceSearchInput) ([]DocRecord, error) {
	entity := coredatasource.EntityType(req.Entity)
	if entity == "" {
		entity = OperationEntity
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	query := strings.ToLower(strings.TrimSpace(req.Query))
	var records []DocRecord
	for _, doc := range docs {
		if doc.Entity != entity {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(doc.Title+" "+doc.Content+" "+doc.Name+" "+doc.Path), query) {
			continue
		}
		records = append(records, doc)
		if len(records) >= limit {
			break
		}
	}
	return records, nil
}

func listDocs(docs []DocRecord, req pluginbinding.DatasourceListInput) ([]DocRecord, error) {
	entity := req.Entity
	if entity == "" {
		entity = OperationEntity
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	var records []DocRecord
	for _, doc := range docs {
		if doc.Entity != entity {
			continue
		}
		records = append(records, doc)
		if len(records) >= limit {
			break
		}
	}
	return records, nil
}

func getDoc(docs []DocRecord, req pluginbinding.DatasourceGetInput) (DocRecord, error) {
	entity := coredatasource.EntityType(req.Entity)
	for _, doc := range docs {
		if doc.ID == req.ID && (entity == "" || doc.Entity == entity) {
			return doc, nil
		}
	}
	return DocRecord{}, pluginbinding.Fail("not_found", fmt.Sprintf("openapi doc %q not found", req.ID))
}

func docRecordsToAny(records []DocRecord) []coredatasource.Record {
	out := make([]coredatasource.Record, 0, len(records))
	for _, record := range records {
		out = append(out, docRecord(record))
	}
	return out
}

func docRecord(doc DocRecord) coredatasource.Record {
	metadata := map[string]string{}
	if doc.Method != "" {
		metadata["method"] = doc.Method
	}
	if doc.Path != "" {
		metadata["path"] = doc.Path
	}
	if doc.Name != "" {
		metadata["name"] = doc.Name
	}
	if doc.Status != "" {
		metadata["status"] = doc.Status
	}
	if doc.Scheme != "" {
		metadata["scheme"] = doc.Scheme
	}
	return coredatasource.Record{
		Entity:   doc.Entity,
		ID:       doc.ID,
		Title:    doc.Title,
		Content:  doc.Content,
		URL:      doc.URL,
		Metadata: metadata,
		Raw:      doc,
	}
}

func docsForSpec(datasource string, doc *openapi3.T) []DocRecord {
	var docs []DocRecord
	for _, path := range doc.Paths.InMatchingOrder() {
		item := doc.Paths.Value(path)
		if item == nil {
			continue
		}
		for _, method := range sortedOperationMethods(item.Operations()) {
			op := item.GetOperation(method)
			if op == nil {
				continue
			}
			name := operationBaseName(method, path, op)
			title := strings.ToUpper(method) + " " + path
			content := strings.Join(nonEmpty([]string{op.Summary, op.Description, "Tags: " + strings.Join(op.Tags, ", ")}), "\n")
			docs = append(docs, DocRecord{ID: "operation:" + name, Entity: OperationEntity, Title: title, Content: content, Method: strings.ToUpper(method), Path: path, Name: name})
			for _, param := range append(parametersFromRefs(item.Parameters), parametersFromRefs(op.Parameters)...) {
				if param == nil {
					continue
				}
				id := "parameter:" + name + ":" + param.In + ":" + param.Name
				docs = append(docs, DocRecord{ID: id, Entity: ParameterEntity, Title: param.In + " parameter " + param.Name, Content: param.Description, Method: strings.ToUpper(method), Path: path, Name: param.Name})
			}
			if op.Responses != nil {
				for _, status := range op.Responses.Keys() {
					respRef := op.Responses.Value(status)
					if respRef == nil || respRef.Value == nil {
						continue
					}
					desc := ""
					if respRef.Value.Description != nil {
						desc = *respRef.Value.Description
					}
					docs = append(docs, DocRecord{ID: "response:" + name + ":" + status, Entity: ResponseEntity, Title: "Response " + status + " for " + title, Content: desc, Method: strings.ToUpper(method), Path: path, Status: status})
				}
			}
		}
	}
	if doc.Components != nil {
		schemaNames := make([]string, 0, len(doc.Components.Schemas))
		for name := range doc.Components.Schemas {
			schemaNames = append(schemaNames, name)
		}
		sort.Strings(schemaNames)
		for _, name := range schemaNames {
			ref := doc.Components.Schemas[name]
			docs = append(docs, DocRecord{ID: "schema:" + name, Entity: SchemaEntity, Title: "Schema " + name, Content: schemaSummary(ref), Name: name})
		}
		securityNames := make([]string, 0, len(doc.Components.SecuritySchemes))
		for name := range doc.Components.SecuritySchemes {
			securityNames = append(securityNames, name)
		}
		sort.Strings(securityNames)
		for _, name := range securityNames {
			ref := doc.Components.SecuritySchemes[name]
			if ref == nil || ref.Value == nil {
				continue
			}
			scheme := ref.Value
			content := strings.Join(nonEmpty([]string{scheme.Description, "type: " + scheme.Type, "scheme: " + scheme.Scheme, "in: " + scheme.In, "name: " + scheme.Name}), "\n")
			docs = append(docs, DocRecord{ID: "security:" + name, Entity: SecuritySchemeEntity, Title: "Security scheme " + name, Content: content, Name: name, Scheme: firstNonEmpty(scheme.Scheme, scheme.Type)})
		}
	}
	return docs
}

func schemaSummary(ref *openapi3.SchemaRef) string {
	if ref == nil || ref.Value == nil {
		return ""
	}
	schema := ref.Value
	parts := nonEmpty([]string{schema.Title, schema.Description})
	if len(schema.Properties) > 0 {
		names := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		parts = append(parts, "properties: "+strings.Join(names, ", "))
	}
	return strings.Join(parts, "\n")
}

func docDescription(doc *openapi3.T) string {
	if doc == nil || doc.Info == nil {
		return ""
	}
	return strings.Join(nonEmpty([]string{doc.Info.Title, doc.Info.Description}), "\n")
}

func nonEmpty(values []string) []string {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
