package sql

import "strings"

// Pure per-driver introspection query builders. Schema/table filters are
// parameterized; the only interpolations are fixed internal literals and
// (for sqlite PRAGMAs, which cannot take parameters) identifiers that were
// first resolved against sqlite_master with a parameterized lookup and then
// double-quote escaped.

func databaseListQueries(driver string) []introspectQuery {
	switch driver {
	case "mysql":
		return []introspectQuery{{
			Kind: "database",
			SQL: "SELECT schema_name AS name, default_character_set_name AS charset, default_collation_name AS collation, " +
				"schema_name = DATABASE() AS current_db FROM information_schema.schemata ORDER BY schema_name",
		}}
	case "pgx":
		return []introspectQuery{
			{
				Kind: "database",
				SQL: "SELECT datname AS name, pg_get_userbyid(datdba) AS owner, datname = current_database() AS current_db " +
					"FROM pg_database WHERE NOT datistemplate ORDER BY datname",
			},
			{
				Kind: "schema",
				SQL: "SELECT schema_name AS name FROM information_schema.schemata " +
					"WHERE schema_name NOT IN ('pg_catalog','information_schema') AND schema_name NOT LIKE 'pg_%' ORDER BY schema_name",
			},
		}
	default: // sqlite
		return []introspectQuery{{Kind: "database", SQL: "PRAGMA database_list"}}
	}
}

type introspectQuery struct {
	Kind string
	SQL  string
	Args []any
}

func tableListQuery(driver, schema string, includeViews bool) introspectQuery {
	switch driver {
	case "mysql":
		sql := "SELECT table_schema, table_name, table_type, table_rows, table_comment FROM information_schema.tables WHERE "
		args := []any{}
		if strings.TrimSpace(schema) != "" {
			sql += "table_schema = ?"
			args = append(args, strings.TrimSpace(schema))
		} else {
			sql += "table_schema = DATABASE()"
		}
		if !includeViews {
			sql += " AND table_type = 'BASE TABLE'"
		}
		sql += " ORDER BY table_schema, table_name"
		return introspectQuery{SQL: sql, Args: args}
	case "pgx":
		relkinds := "('r','p')"
		if includeViews {
			relkinds = "('r','p','v','m')"
		}
		sql := "SELECT n.nspname AS table_schema, c.relname AS table_name, c.relkind::text AS table_type, c.reltuples::bigint AS row_estimate " +
			"FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace " +
			"WHERE c.relkind IN " + relkinds + " AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_%' " +
			"AND ($1 = '' OR n.nspname = $1) ORDER BY n.nspname, c.relname"
		return introspectQuery{SQL: sql, Args: []any{strings.TrimSpace(schema)}}
	default: // sqlite
		types := "('table')"
		if includeViews {
			types = "('table','view')"
		}
		return introspectQuery{SQL: "SELECT name, type FROM sqlite_master WHERE type IN " + types + " AND name NOT LIKE 'sqlite_%' ORDER BY name"}
	}
}

func tableColumnsQuery(driver, schema, table string) introspectQuery {
	switch driver {
	case "mysql":
		sql := "SELECT column_name, ordinal_position, column_type, is_nullable, column_default, character_maximum_length, column_key " +
			"FROM information_schema.columns WHERE table_name = ? AND "
		args := []any{strings.TrimSpace(table)}
		if strings.TrimSpace(schema) != "" {
			sql += "table_schema = ?"
			args = append(args, strings.TrimSpace(schema))
		} else {
			sql += "table_schema = DATABASE()"
		}
		sql += " ORDER BY ordinal_position"
		return introspectQuery{SQL: sql, Args: args}
	default: // pgx
		return introspectQuery{
			SQL: "SELECT column_name, ordinal_position, data_type, udt_name, is_nullable, column_default, character_maximum_length " +
				"FROM information_schema.columns WHERE table_schema = COALESCE(NULLIF($1,''),'public') AND table_name = $2 ORDER BY ordinal_position",
			Args: []any{strings.TrimSpace(schema), strings.TrimSpace(table)},
		}
	}
}

func tablePrimaryKeyQuery(schema, table string) introspectQuery { // pgx only
	return introspectQuery{
		SQL: "SELECT kcu.column_name FROM information_schema.table_constraints tc " +
			"JOIN information_schema.key_column_usage kcu ON kcu.constraint_name = tc.constraint_name AND kcu.constraint_schema = tc.constraint_schema " +
			"WHERE tc.constraint_type = 'PRIMARY KEY' AND tc.table_schema = COALESCE(NULLIF($1,''),'public') AND tc.table_name = $2 " +
			"ORDER BY kcu.ordinal_position",
		Args: []any{strings.TrimSpace(schema), strings.TrimSpace(table)},
	}
}

func tableForeignKeysQuery(driver, schema, table string) introspectQuery {
	switch driver {
	case "mysql":
		sql := "SELECT constraint_name, column_name, referenced_table_name, referenced_column_name " +
			"FROM information_schema.key_column_usage WHERE referenced_table_name IS NOT NULL AND table_name = ? AND "
		args := []any{strings.TrimSpace(table)}
		if strings.TrimSpace(schema) != "" {
			sql += "table_schema = ?"
			args = append(args, strings.TrimSpace(schema))
		} else {
			sql += "table_schema = DATABASE()"
		}
		sql += " ORDER BY constraint_name, ordinal_position"
		return introspectQuery{SQL: sql, Args: args}
	default: // pgx
		return introspectQuery{
			SQL: "SELECT tc.constraint_name, kcu.column_name, ccu.table_name AS referenced_table_name, ccu.column_name AS referenced_column_name " +
				"FROM information_schema.table_constraints tc " +
				"JOIN information_schema.key_column_usage kcu ON kcu.constraint_name = tc.constraint_name AND kcu.constraint_schema = tc.constraint_schema " +
				"JOIN information_schema.constraint_column_usage ccu ON ccu.constraint_name = tc.constraint_name AND ccu.constraint_schema = tc.constraint_schema " +
				"WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = COALESCE(NULLIF($1,''),'public') AND tc.table_name = $2 " +
				"ORDER BY tc.constraint_name, kcu.ordinal_position",
			Args: []any{strings.TrimSpace(schema), strings.TrimSpace(table)},
		}
	}
}

func indexListQuery(driver, schema, table string) introspectQuery {
	switch driver {
	case "mysql":
		sql := "SELECT table_name, index_name, non_unique, seq_in_index, column_name, index_type " +
			"FROM information_schema.statistics WHERE "
		args := []any{}
		if strings.TrimSpace(schema) != "" {
			sql += "table_schema = ?"
			args = append(args, strings.TrimSpace(schema))
		} else {
			sql += "table_schema = DATABASE()"
		}
		if strings.TrimSpace(table) != "" {
			sql += " AND table_name = ?"
			args = append(args, strings.TrimSpace(table))
		}
		sql += " ORDER BY table_name, index_name, seq_in_index"
		return introspectQuery{SQL: sql, Args: args}
	default: // pgx
		return introspectQuery{
			SQL: "SELECT n.nspname AS table_schema, t.relname AS table_name, i.relname AS index_name, ix.indisunique, ix.indisprimary, am.amname, pg_get_indexdef(ix.indexrelid) AS definition " +
				"FROM pg_index ix JOIN pg_class i ON i.oid = ix.indexrelid JOIN pg_class t ON t.oid = ix.indrelid " +
				"JOIN pg_namespace n ON n.oid = t.relnamespace JOIN pg_am am ON am.oid = i.relam " +
				"WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_%' " +
				"AND ($1 = '' OR n.nspname = $1) AND ($2 = '' OR t.relname = $2) " +
				"ORDER BY n.nspname, t.relname, i.relname",
			Args: []any{strings.TrimSpace(schema), strings.TrimSpace(table)},
		}
	}
}

// quoteSQLiteIdentifier double-quote escapes an identifier for interpolation
// into a PRAGMA (which cannot be parameterized). Callers must first resolve
// the name against sqlite_master with a parameterized lookup.
func quoteSQLiteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// parseIndexDefColumns best-effort extracts the column list from a postgres
// pg_get_indexdef definition like `CREATE INDEX x ON t USING btree (a, b)`.
func parseIndexDefColumns(definition string) []string {
	open := strings.Index(definition, "(")
	closing := strings.LastIndex(definition, ")")
	if open < 0 || closing <= open {
		return nil
	}
	parts := strings.Split(definition[open+1:closing], ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			columns = append(columns, trimmed)
		}
	}
	return columns
}
