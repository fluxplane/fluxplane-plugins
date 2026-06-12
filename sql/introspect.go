package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// ConnInput carries the shared connection fields of the schema-introspection
// operations. It mirrors sql.query's connection fields key-for-key.
type ConnInput struct {
	EndpointRef string `json:"endpoint_ref,omitempty" jsonschema:"required,description=Registered SQL endpoint ref resolved by the host."`
	Driver      string `json:"driver,omitempty" jsonschema:"description=SQL driver or dialect.,enum=mysql,enum=postgres,enum=sqlite"`
	Database    string `json:"database,omitempty" jsonschema:"description=Database override. For postgres this reconnects to the named database."`
	Timeout     string `json:"timeout,omitempty" jsonschema:"description=Timeout as a Go duration such as 5s or 1m. Defaults to 10s."`
}

func (c ConnInput) validate() error {
	if strings.TrimSpace(c.EndpointRef) == "" {
		return pluginbinding.Fail("bad_input", "endpoint_ref is required")
	}
	return nil
}

type DatabaseListInput struct {
	ConnInput
}

type DatabaseInfo struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // database | schema
	Owner     string `json:"owner,omitempty"`
	Charset   string `json:"charset,omitempty"`
	Collation string `json:"collation,omitempty"`
	File      string `json:"file,omitempty"` // sqlite attached database file
	Current   bool   `json:"current,omitempty"`
}

type DatabaseListOutput struct {
	EndpointRef string         `json:"endpoint_ref,omitempty"`
	EndpointURL string         `json:"endpoint_url,omitempty"`
	Driver      string         `json:"driver,omitempty"`
	Databases   []DatabaseInfo `json:"databases"`
	Count       int            `json:"count"`
	DurationMS  int64          `json:"duration_ms,omitempty"`
}

type TableListInput struct {
	ConnInput
	Schema       string `json:"schema,omitempty" jsonschema:"description=Schema filter. mysql treats this as the database; postgres defaults to all non-system schemas."`
	IncludeViews bool   `json:"include_views,omitempty" jsonschema:"description=Include views (and postgres materialized views)."`
	MaxResults   int    `json:"max_results,omitempty" jsonschema:"description=Maximum tables returned. Defaults to 200 and is capped at 1000.,minimum=0,maximum=1000"`
}

type TableInfo struct {
	Name        string `json:"name"`
	Schema      string `json:"schema,omitempty"`
	Type        string `json:"type"`                   // table | view | materialized_view
	RowEstimate *int64 `json:"row_estimate,omitempty"` // cheap statistics estimate; absent when the engine keeps none (sqlite, views)
	Comment     string `json:"comment,omitempty"`
}

type TableListOutput struct {
	EndpointRef string      `json:"endpoint_ref,omitempty"`
	EndpointURL string      `json:"endpoint_url,omitempty"`
	Driver      string      `json:"driver,omitempty"`
	Database    string      `json:"database,omitempty"`
	Tables      []TableInfo `json:"tables"`
	Count       int         `json:"count"`
	Truncated   bool        `json:"truncated,omitempty"`
	DurationMS  int64       `json:"duration_ms,omitempty"`
}

type TableShowInput struct {
	ConnInput
	Schema string `json:"schema,omitempty" jsonschema:"description=Schema holding the table. postgres defaults to public; mysql defaults to the connected database."`
	Table  string `json:"table,omitempty" jsonschema:"required,description=Table or view name."`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Position   int    `json:"position"`
	DataType   string `json:"data_type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	MaxLength  *int64 `json:"max_length,omitempty"`
	PrimaryKey bool   `json:"primary_key,omitempty"`
}

type ForeignKeyInfo struct {
	Name       string   `json:"name,omitempty"`
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns,omitempty"`
}

type TableShowOutput struct {
	EndpointRef string           `json:"endpoint_ref,omitempty"`
	EndpointURL string           `json:"endpoint_url,omitempty"`
	Driver      string           `json:"driver,omitempty"`
	Database    string           `json:"database,omitempty"`
	Schema      string           `json:"schema,omitempty"`
	Table       string           `json:"table"`
	Columns     []ColumnInfo     `json:"columns"`
	PrimaryKey  []string         `json:"primary_key"`
	ForeignKeys []ForeignKeyInfo `json:"foreign_keys"`
	DurationMS  int64            `json:"duration_ms,omitempty"`
}

type IndexListInput struct {
	ConnInput
	Schema string `json:"schema,omitempty" jsonschema:"description=Schema filter. mysql treats this as the database."`
	Table  string `json:"table,omitempty" jsonschema:"description=Limit to one table. Default lists indexes across the schema."`
}

type IndexInfo struct {
	Name       string   `json:"name"`
	Table      string   `json:"table"`
	Schema     string   `json:"schema,omitempty"`
	Columns    []string `json:"columns,omitempty"`
	Unique     bool     `json:"unique,omitempty"`
	Primary    bool     `json:"primary,omitempty"`
	Method     string   `json:"method,omitempty"`     // postgres access method (btree, gin, ...)
	Definition string   `json:"definition,omitempty"` // postgres pg_get_indexdef
}

type IndexListOutput struct {
	EndpointRef string      `json:"endpoint_ref,omitempty"`
	EndpointURL string      `json:"endpoint_url,omitempty"`
	Driver      string      `json:"driver,omitempty"`
	Database    string      `json:"database,omitempty"`
	Indexes     []IndexInfo `json:"indexes"`
	Count       int         `json:"count"`
	DurationMS  int64       `json:"duration_ms,omitempty"`
}

const introspectMaxRows = 5000

// lowerRowKeys lower-cases every column key. MySQL returns information_schema
// column names upper-case (TABLE_NAME) while postgres and sqlite return them
// lower-case; the mapping code matches on lower-case keys.
func lowerRowKeys(rows []map[string]any) []map[string]any {
	for i, row := range rows {
		normalized := make(map[string]any, len(row))
		for key, value := range row {
			normalized[strings.ToLower(key)] = value
		}
		rows[i] = normalized
	}
	return rows
}

// introspectAll runs an introspection query and returns rows with
// lower-cased column keys plus the truncation flag.
func introspectAll(queryCtx context.Context, tx *stdsql.Tx, maxRows int, query introspectQuery) ([]map[string]any, bool, error) {
	rows, _, truncated, err := queryAll(queryCtx, tx, maxRows, query.SQL, query.Args...)
	if err != nil {
		return nil, false, err
	}
	return lowerRowKeys(rows), truncated, nil
}

// DatabaseList reports databases (and for postgres also non-system schemas of
// the connected database).
func (s Service) DatabaseList(ctx pluginbinding.Context, input DatabaseListInput) (DatabaseListOutput, error) {
	if err := input.validate(); err != nil {
		return DatabaseListOutput{}, err
	}
	out := DatabaseListOutput{EndpointRef: input.EndpointRef}
	target, duration, err := withReadOnlySQL(ctx, input.EndpointRef, input.Driver, input.Database, input.Timeout, func(queryCtx context.Context, tx *stdsql.Tx, target sqlTarget) error {
		for _, query := range databaseListQueries(target.Driver) {
			rows, _, err := introspectAll(queryCtx, tx, introspectMaxRows, query)
			if err != nil {
				return err
			}
			for _, row := range rows {
				info := DatabaseInfo{
					Kind:      query.Kind,
					Name:      asString(row["name"]),
					Owner:     asString(row["owner"]),
					Charset:   asString(row["charset"]),
					Collation: asString(row["collation"]),
					File:      asString(row["file"]),
					Current:   asBool(row["current_db"]),
				}
				if info.Name == "" {
					continue
				}
				out.Databases = append(out.Databases, info)
			}
		}
		return nil
	})
	if err != nil {
		return DatabaseListOutput{}, err
	}
	out.EndpointURL = target.SafeURL
	out.Driver = sqlFirstNonEmpty(target.Dialect, target.Driver)
	out.Count = len(out.Databases)
	out.DurationMS = duration
	out.Databases = nonNil(out.Databases)
	return out, nil
}

// TableList lists tables (and optionally views) with a cheap row estimate
// where the engine keeps one.
func (s Service) TableList(ctx pluginbinding.Context, input TableListInput) (TableListOutput, error) {
	if err := input.validate(); err != nil {
		return TableListOutput{}, err
	}
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 200
	}
	if maxResults > 1000 {
		maxResults = 1000
	}
	out := TableListOutput{EndpointRef: input.EndpointRef}
	target, duration, err := withReadOnlySQL(ctx, input.EndpointRef, input.Driver, input.Database, input.Timeout, func(queryCtx context.Context, tx *stdsql.Tx, target sqlTarget) error {
		query := tableListQuery(target.Driver, input.Schema, input.IncludeViews)
		rows, truncated, err := introspectAll(queryCtx, tx, maxResults, query)
		if err != nil {
			return err
		}
		out.Truncated = truncated
		for _, row := range rows {
			info := TableInfo{
				Name:    asString(firstNonNil(row["table_name"], row["name"])),
				Schema:  asString(row["table_schema"]),
				Type:    normalizeTableType(asString(firstNonNil(row["table_type"], row["type"]))),
				Comment: asString(row["table_comment"]),
			}
			if estimate, ok := asInt64(firstNonNil(row["table_rows"], row["row_estimate"])); ok && estimate >= 0 {
				value := estimate
				info.RowEstimate = &value
			}
			if info.Name == "" {
				continue
			}
			out.Tables = append(out.Tables, info)
		}
		return nil
	})
	if err != nil {
		return TableListOutput{}, err
	}
	out.EndpointURL = target.SafeURL
	out.Driver = sqlFirstNonEmpty(target.Dialect, target.Driver)
	out.Database = target.Database
	out.Count = len(out.Tables)
	out.DurationMS = duration
	out.Tables = nonNil(out.Tables)
	return out, nil
}

// TableShow reports a table's columns, primary key, and foreign keys.
func (s Service) TableShow(ctx pluginbinding.Context, input TableShowInput) (TableShowOutput, error) {
	if err := input.validate(); err != nil {
		return TableShowOutput{}, err
	}
	table := strings.TrimSpace(input.Table)
	if table == "" {
		return TableShowOutput{}, pluginbinding.Fail("bad_input", "table is required")
	}
	out := TableShowOutput{EndpointRef: input.EndpointRef, Table: table, Schema: strings.TrimSpace(input.Schema)}
	target, duration, err := withReadOnlySQL(ctx, input.EndpointRef, input.Driver, input.Database, input.Timeout, func(queryCtx context.Context, tx *stdsql.Tx, target sqlTarget) error {
		if target.Driver == "sqlite" {
			return introspectSQLiteTable(queryCtx, tx, table, &out)
		}
		columnsQuery := tableColumnsQuery(target.Driver, input.Schema, table)
		rows, _, err := introspectAll(queryCtx, tx, introspectMaxRows, columnsQuery)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return pluginbinding.Errorf("not_found", "table %q not found", table)
		}
		for _, row := range rows {
			column := ColumnInfo{
				Name:     asString(row["column_name"]),
				DataType: asString(firstNonNil(row["column_type"], row["data_type"])),
				Nullable: strings.EqualFold(asString(row["is_nullable"]), "YES"),
				Default:  asString(row["column_default"]),
			}
			if position, ok := asInt64(row["ordinal_position"]); ok {
				column.Position = int(position)
			}
			if maxLength, ok := asInt64(row["character_maximum_length"]); ok {
				value := maxLength
				column.MaxLength = &value
			}
			if strings.EqualFold(asString(row["column_key"]), "PRI") {
				column.PrimaryKey = true
				out.PrimaryKey = append(out.PrimaryKey, column.Name)
			}
			out.Columns = append(out.Columns, column)
		}
		if target.Driver == "pgx" {
			pkQuery := tablePrimaryKeyQuery(input.Schema, table)
			pkRows, _, err := introspectAll(queryCtx, tx, introspectMaxRows, pkQuery)
			if err != nil {
				return err
			}
			primary := map[string]bool{}
			for _, row := range pkRows {
				name := asString(row["column_name"])
				primary[name] = true
				out.PrimaryKey = append(out.PrimaryKey, name)
			}
			for i := range out.Columns {
				if primary[out.Columns[i].Name] {
					out.Columns[i].PrimaryKey = true
				}
			}
		}
		fkQuery := tableForeignKeysQuery(target.Driver, input.Schema, table)
		fkRows, _, err := introspectAll(queryCtx, tx, introspectMaxRows, fkQuery)
		if err != nil {
			return err
		}
		out.ForeignKeys = groupForeignKeys(fkRows)
		return nil
	})
	if err != nil {
		return TableShowOutput{}, err
	}
	out.EndpointURL = target.SafeURL
	out.Driver = sqlFirstNonEmpty(target.Dialect, target.Driver)
	out.Database = target.Database
	out.DurationMS = duration
	out.Columns, out.PrimaryKey, out.ForeignKeys = nonNil(out.Columns), nonNil(out.PrimaryKey), nonNil(out.ForeignKeys)
	return out, nil
}

// IndexList lists indexes across a schema or for a single table.
func (s Service) IndexList(ctx pluginbinding.Context, input IndexListInput) (IndexListOutput, error) {
	if err := input.validate(); err != nil {
		return IndexListOutput{}, err
	}
	out := IndexListOutput{EndpointRef: input.EndpointRef}
	target, duration, err := withReadOnlySQL(ctx, input.EndpointRef, input.Driver, input.Database, input.Timeout, func(queryCtx context.Context, tx *stdsql.Tx, target sqlTarget) error {
		if target.Driver == "sqlite" {
			indexes, err := introspectSQLiteIndexes(queryCtx, tx, input.Table)
			out.Indexes = indexes
			return err
		}
		query := indexListQuery(target.Driver, input.Schema, input.Table)
		rows, _, err := introspectAll(queryCtx, tx, introspectMaxRows, query)
		if err != nil {
			return err
		}
		if target.Driver == "mysql" {
			out.Indexes = groupMySQLIndexes(rows)
			return nil
		}
		for _, row := range rows {
			definition := asString(row["definition"])
			out.Indexes = append(out.Indexes, IndexInfo{
				Name:       asString(row["index_name"]),
				Table:      asString(row["table_name"]),
				Schema:     asString(row["table_schema"]),
				Columns:    parseIndexDefColumns(definition),
				Unique:     asBool(row["indisunique"]),
				Primary:    asBool(row["indisprimary"]),
				Method:     asString(row["amname"]),
				Definition: definition,
			})
		}
		return nil
	})
	if err != nil {
		return IndexListOutput{}, err
	}
	out.EndpointURL = target.SafeURL
	out.Driver = sqlFirstNonEmpty(target.Dialect, target.Driver)
	out.Database = target.Database
	out.Count = len(out.Indexes)
	out.DurationMS = duration
	out.Indexes = nonNil(out.Indexes)
	return out, nil
}

// resolveSQLiteName resolves a user-supplied table/view name against
// sqlite_master with a parameterized lookup, returning the stored identifier.
// PRAGMAs cannot take parameters, so only this resolved name is ever
// interpolated (double-quote escaped).
func resolveSQLiteName(queryCtx context.Context, tx *stdsql.Tx, name string) (string, error) {
	rows, _, _, err := queryAll(queryCtx, tx, 2, "SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name = ?", name)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", pluginbinding.Errorf("not_found", "table %q not found", name)
	}
	return asString(rows[0]["name"]), nil
}

func introspectSQLiteTable(queryCtx context.Context, tx *stdsql.Tx, table string, out *TableShowOutput) error {
	resolved, err := resolveSQLiteName(queryCtx, tx, table)
	if err != nil {
		return err
	}
	quoted := quoteSQLiteIdentifier(resolved)
	rows, _, _, err := queryAll(queryCtx, tx, introspectMaxRows, "PRAGMA table_info("+quoted+")")
	if err != nil {
		return err
	}
	for _, row := range rows {
		column := ColumnInfo{
			Name:     asString(row["name"]),
			DataType: asString(row["type"]),
			Nullable: !asBool(row["notnull"]),
			Default:  asString(row["dflt_value"]),
		}
		if cid, ok := asInt64(row["cid"]); ok {
			column.Position = int(cid) + 1
		}
		if pk, ok := asInt64(row["pk"]); ok && pk > 0 {
			column.PrimaryKey = true
			out.PrimaryKey = append(out.PrimaryKey, column.Name)
		}
		out.Columns = append(out.Columns, column)
	}
	fkRows, _, _, err := queryAll(queryCtx, tx, introspectMaxRows, "PRAGMA foreign_key_list("+quoted+")")
	if err != nil {
		return err
	}
	byID := map[string]*ForeignKeyInfo{}
	order := []string{}
	for _, row := range fkRows {
		id := asString(row["id"])
		fk, ok := byID[id]
		if !ok {
			fk = &ForeignKeyInfo{Name: "fk_" + id, RefTable: asString(row["table"])}
			byID[id] = fk
			order = append(order, id)
		}
		fk.Columns = append(fk.Columns, asString(row["from"]))
		fk.RefColumns = append(fk.RefColumns, asString(row["to"]))
	}
	for _, id := range order {
		out.ForeignKeys = append(out.ForeignKeys, *byID[id])
	}
	return nil
}

func introspectSQLiteIndexes(queryCtx context.Context, tx *stdsql.Tx, table string) ([]IndexInfo, error) {
	tables := []string{}
	if strings.TrimSpace(table) != "" {
		resolved, err := resolveSQLiteName(queryCtx, tx, strings.TrimSpace(table))
		if err != nil {
			return nil, err
		}
		tables = append(tables, resolved)
	} else {
		rows, _, _, err := queryAll(queryCtx, tx, introspectMaxRows, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			tables = append(tables, asString(row["name"]))
		}
	}
	var out []IndexInfo
	for _, name := range tables {
		rows, _, _, err := queryAll(queryCtx, tx, introspectMaxRows, "PRAGMA index_list("+quoteSQLiteIdentifier(name)+")")
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			index := IndexInfo{
				Name:    asString(row["name"]),
				Table:   name,
				Unique:  asBool(row["unique"]),
				Primary: strings.EqualFold(asString(row["origin"]), "pk"),
			}
			columnRows, _, _, err := queryAll(queryCtx, tx, introspectMaxRows, "PRAGMA index_info("+quoteSQLiteIdentifier(index.Name)+")")
			if err != nil {
				return nil, err
			}
			for _, columnRow := range columnRows {
				if column := asString(columnRow["name"]); column != "" {
					index.Columns = append(index.Columns, column)
				}
			}
			out = append(out, index)
		}
	}
	return out, nil
}

func groupForeignKeys(rows []map[string]any) []ForeignKeyInfo {
	byName := map[string]*ForeignKeyInfo{}
	order := []string{}
	for _, row := range rows {
		name := asString(row["constraint_name"])
		fk, ok := byName[name]
		if !ok {
			fk = &ForeignKeyInfo{Name: name, RefTable: asString(row["referenced_table_name"])}
			byName[name] = fk
			order = append(order, name)
		}
		fk.Columns = append(fk.Columns, asString(row["column_name"]))
		if ref := asString(row["referenced_column_name"]); ref != "" {
			fk.RefColumns = append(fk.RefColumns, ref)
		}
	}
	out := make([]ForeignKeyInfo, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out
}

func groupMySQLIndexes(rows []map[string]any) []IndexInfo {
	type key struct{ table, index string }
	byKey := map[key]*IndexInfo{}
	order := []key{}
	for _, row := range rows {
		k := key{table: asString(row["table_name"]), index: asString(row["index_name"])}
		index, ok := byKey[k]
		if !ok {
			nonUnique, _ := asInt64(row["non_unique"])
			index = &IndexInfo{
				Name:    k.index,
				Table:   k.table,
				Unique:  nonUnique == 0,
				Primary: strings.EqualFold(k.index, "PRIMARY"),
				Method:  strings.ToLower(asString(row["index_type"])),
			}
			byKey[k] = index
			order = append(order, k)
		}
		if column := asString(row["column_name"]); column != "" {
			index.Columns = append(index.Columns, column)
		}
	}
	out := make([]IndexInfo, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

func normalizeTableType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BASE TABLE", "TABLE", "R", "P":
		return "table"
	case "VIEW", "V":
		return "view"
	case "M":
		return "materialized_view"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func asString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func asInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case []byte:
		parsed, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func asBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int64:
		return v != 0
	case float64:
		return v != 0
	case []byte:
		return sqlTruthy(string(v))
	case string:
		return sqlTruthy(v)
	default:
		return false
	}
}

func sqlTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "yes", "y":
		return true
	default:
		return false
	}
}
