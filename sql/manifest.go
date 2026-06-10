package sql

import (
	"encoding/json"

	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/protocol"
)

const (
	PluginName        = "sql"
	PluginVersion     = "0.19.0"
	PluginDescription = "Read-only SQL query and schema introspection operations for MySQL, PostgreSQL, SQLite, and compatible endpoints."

	AuthMethodSQL         = "sql"
	AuthPurposeUsername   = "username"
	AuthPurposePassword   = "password"
	EnvSQLUsername        = "SQL_USERNAME"
	EnvSQLPassword        = "SQL_PASSWORD"
	EnvMySQLUsername      = "MYSQL_USERNAME"
	EnvMySQLPassword      = "MYSQL_PASSWORD"
	OperationQuery        = "sql.query"
	OperationDatabaseList = "sql.database.list"
	OperationTableList    = "sql.table.list"
	OperationTableShow    = "sql.table.show"
	OperationIndexList    = "sql.index.list"
	DatasourceQueryRows   = "sql.query_rows"
	EntitySQLQueryResult  = "sql.query_result"
)

// withInputExamples injects JSON Schema `examples` into an operation's input
// schema. The fluxplane-plugin CLI surfaces the first example as the runnable
// invocation in `operation describe`. Kept local to the sql plugin rather than
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
		Aliases:     []string{"mysql", PluginName},
		Metadata:    map[string]string{pluginbinding.ManifestProtocolKey: protocol.Version},
		Auth: []core.AuthMethod{{
			Name:        AuthMethodSQL,
			Kind:        "credentials",
			Description: "SQL credentials resolved by the plugin host secret broker.",
			Env:         []string{EnvSQLUsername, EnvSQLPassword, EnvMySQLUsername, EnvMySQLPassword},
			Fields: []core.AuthField{
				pluginbinding.AuthField(AuthPurposeUsername, "SQL username", false, true, EnvSQLUsername, EnvMySQLUsername),
				pluginbinding.AuthField(AuthPurposePassword, "SQL password, URL, or DSN", false, true, EnvSQLPassword, EnvMySQLPassword),
			},
		}},
		Operations: []core.OperationSpec{
			querySpec(),
			databaseListSpec(),
			tableListSpec(),
			tableShowSpec(),
			indexListSpec(),
		},
		Datasources: []core.DatasourceSpec{
			queryRowsDatasourceSpec(),
		},
	}
}

func sqlReadSpecOptions() []pluginbinding.OperationSpecOption {
	return []pluginbinding.OperationSpecOption{
		pluginbinding.ReadOnly(),
		pluginbinding.SecretPurposes(AuthPurposeUsername, AuthPurposePassword),
		pluginbinding.Effects(core.OperationEffectRead, core.OperationEffectNetwork),
		pluginbinding.Access(core.OperationAccessAuth, core.OperationAccessSecret, core.OperationAccessProvider),
		pluginbinding.Risk(core.OperationRiskLow),
		pluginbinding.Idempotency(core.OperationIdempotent),
	}
}

func querySpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[QueryInput, QueryOutput](
		OperationQuery,
		"Run a bounded read-only SQL query against a SQL endpoint.",
		sqlReadSpecOptions()...,
	), map[string]any{"endpoint_ref": "warehouse", "query": "select id, email from users order by id limit 10", "max_rows": 10})
}

func databaseListSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[DatabaseListInput, DatabaseListOutput](
		OperationDatabaseList,
		"List databases (and for postgres the non-system schemas of the connected database).",
		sqlReadSpecOptions()...,
	), map[string]any{"endpoint_ref": "warehouse"})
}

func tableListSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[TableListInput, TableListOutput](
		OperationTableList,
		"List tables (optionally views) with cheap row estimates where the engine keeps statistics.",
		sqlReadSpecOptions()...,
	), map[string]any{"endpoint_ref": "warehouse", "include_views": true})
}

func tableShowSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[TableShowInput, TableShowOutput](
		OperationTableShow,
		"Describe a table: columns with types and nullability, primary key, and foreign keys.",
		sqlReadSpecOptions()...,
	), map[string]any{"endpoint_ref": "warehouse", "table": "users"})
}

func indexListSpec() core.OperationSpec {
	return withInputExamples(pluginbinding.TypedOperationSpec[IndexListInput, IndexListOutput](
		OperationIndexList,
		"List indexes across a schema or for one table, with columns and uniqueness.",
		sqlReadSpecOptions()...,
	), map[string]any{"endpoint_ref": "warehouse", "table": "users"})
}

func queryRowsDatasourceSpec() core.DatasourceSpec {
	return pluginbinding.TypedDatasourceSpec[QueryInput, QueryRowsResult](
		DatasourceQueryRows,
		EntitySQLQueryResult,
		"SQL query result rows.",
		[]string{pluginbinding.CapabilitySearch},
		pluginbinding.DatasourceSecretPurposes(AuthPurposeUsername, AuthPurposePassword),
		pluginbinding.DatasourceAccess(core.OperationAccessProvider),
		pluginbinding.EntitySchemaFor[QueryRowRecord](),
		pluginbinding.EntitySchema(core.DatasourceEntitySchema{IDField: "id", TitleField: "title"}),
		pluginbinding.Completion("SQL result row fields.", "driver", "database", "endpoint_url"),
	)
}
