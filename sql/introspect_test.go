package sql

import (
	stdsql "database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	_ "modernc.org/sqlite"
)

// newSQLiteSchemaDB builds a richer fixture than newSQLiteUsersDB: an index, a
// foreign-keyed table, a view, and a hostile table name that would break
// naive PRAGMA interpolation.
func newSQLiteSchemaDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.db")
	db, err := stdsql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	stmts := []string{
		"create table users (id integer primary key, name text not null, email text)",
		"create index users_name_idx on users(name)",
		"create table orders (id integer primary key, user_id integer not null references users(id), total real)",
		"create view active_users as select id, name from users",
		`create table "weird""name; drop" (id integer primary key)`,
		"insert into users (id, name, email) values (1, 'Ada', 'ada@example.com')",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return "sqlite://" + path
}

func TestDatabaseListAgainstSQLite(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteSchemaDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[DatabaseListOutput](t, plugin, OperationDatabaseList, DatabaseListInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
	}, plugintest.WithHost(host))
	if out.Count != 1 || out.Databases[0].Name != "main" || out.Databases[0].Kind != "database" {
		t.Fatalf("out = %#v", out)
	}
	if out.Databases[0].File == "" {
		t.Fatalf("sqlite database should report its file: %#v", out.Databases[0])
	}
}

func TestTableListAgainstSQLite(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteSchemaDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[TableListOutput](t, plugin, OperationTableList, TableListInput{
		ConnInput:    ConnInput{EndpointRef: "warehouse"},
		IncludeViews: true,
	}, plugintest.WithHost(host))
	byName := map[string]TableInfo{}
	for _, table := range out.Tables {
		byName[table.Name] = table
	}
	if byName["users"].Type != "table" || byName["active_users"].Type != "view" {
		t.Fatalf("tables = %#v", out.Tables)
	}
	if _, ok := byName[`weird"name; drop`]; !ok {
		t.Fatalf("hostile-named table missing: %#v", out.Tables)
	}
	if byName["users"].RowEstimate != nil {
		t.Fatalf("sqlite keeps no row estimate, got %#v", byName["users"])
	}
	// Without views the view disappears.
	out = plugintest.RunOK[TableListOutput](t, plugin, OperationTableList, TableListInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
	}, plugintest.WithHost(host))
	for _, table := range out.Tables {
		if table.Type == "view" {
			t.Fatalf("views should be excluded by default: %#v", out.Tables)
		}
	}
}

func TestTableShowAgainstSQLite(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteSchemaDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[TableShowOutput](t, plugin, OperationTableShow, TableShowInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
		Table:     "orders",
	}, plugintest.WithHost(host))
	if len(out.Columns) != 3 || out.Columns[0].Name != "id" || !out.Columns[0].PrimaryKey {
		t.Fatalf("columns = %#v", out.Columns)
	}
	if out.Columns[1].Name != "user_id" || out.Columns[1].Nullable {
		t.Fatalf("user_id should be not-null: %#v", out.Columns[1])
	}
	if len(out.PrimaryKey) != 1 || out.PrimaryKey[0] != "id" {
		t.Fatalf("primary key = %#v", out.PrimaryKey)
	}
	if len(out.ForeignKeys) != 1 || out.ForeignKeys[0].RefTable != "users" || out.ForeignKeys[0].Columns[0] != "user_id" {
		t.Fatalf("foreign keys = %#v", out.ForeignKeys)
	}
}

func TestTableShowHostileNameAndNotFound(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteSchemaDB(t)}
	plugin := NewPluginWithService(Service{})
	// The hostile name resolves through the parameterized sqlite_master lookup
	// and is safely quoted into the PRAGMA.
	out := plugintest.RunOK[TableShowOutput](t, plugin, OperationTableShow, TableShowInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
		Table:     `weird"name; drop`,
	}, plugintest.WithHost(host))
	if len(out.Columns) != 1 || out.Columns[0].Name != "id" {
		t.Fatalf("hostile table columns = %#v", out.Columns)
	}
	err := plugintest.RunError(t, plugin, OperationTableShow, TableShowInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
		Table:     "nope",
	}, plugintest.WithHost(host))
	if err.Code != "not_found" {
		t.Fatalf("err = %#v", err)
	}
}

func TestIndexListAgainstSQLite(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteSchemaDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[IndexListOutput](t, plugin, OperationIndexList, IndexListInput{
		ConnInput: ConnInput{EndpointRef: "warehouse"},
		Table:     "users",
	}, plugintest.WithHost(host))
	var named *IndexInfo
	for i := range out.Indexes {
		if out.Indexes[i].Name == "users_name_idx" {
			named = &out.Indexes[i]
		}
	}
	if named == nil || len(named.Columns) != 1 || named.Columns[0] != "name" || named.Unique {
		t.Fatalf("indexes = %#v", out.Indexes)
	}
}

func TestIntrospectionRequiresEndpointRef(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	for _, op := range []string{OperationDatabaseList, OperationTableList, OperationIndexList} {
		err := plugintest.RunError(t, plugin, op, ConnInput{})
		if err.Code != "bad_input" {
			t.Fatalf("%s err = %#v", op, err)
		}
	}
	err := plugintest.RunError(t, plugin, OperationTableShow, TableShowInput{ConnInput: ConnInput{EndpointRef: "db"}})
	if err.Code != "bad_input" {
		t.Fatalf("table.show without table err = %#v", err)
	}
}

func TestMySQLIntrospectionQueryBuilders(t *testing.T) {
	tables := tableListQuery("mysql", "", false)
	if !strings.Contains(tables.SQL, "table_schema = DATABASE()") || !strings.Contains(tables.SQL, "table_type = 'BASE TABLE'") || len(tables.Args) != 0 {
		t.Fatalf("tables = %#v", tables)
	}
	tables = tableListQuery("mysql", "latest-backend", true)
	if !strings.Contains(tables.SQL, "table_schema = ?") || strings.Contains(tables.SQL, "BASE TABLE") || len(tables.Args) != 1 || tables.Args[0] != "latest-backend" {
		t.Fatalf("tables with schema = %#v", tables)
	}
	columns := tableColumnsQuery("mysql", "", "users")
	if !strings.Contains(columns.SQL, "information_schema.columns") || columns.Args[0] != "users" {
		t.Fatalf("columns = %#v", columns)
	}
	indexes := indexListQuery("mysql", "", "users")
	if !strings.Contains(indexes.SQL, "information_schema.statistics") || len(indexes.Args) != 1 {
		t.Fatalf("indexes = %#v", indexes)
	}
}

func TestPostgresIntrospectionQueryBuilders(t *testing.T) {
	tables := tableListQuery("pgx", "", true)
	if !strings.Contains(tables.SQL, "('r','p','v','m')") || !strings.Contains(tables.SQL, "$1") {
		t.Fatalf("tables = %#v", tables)
	}
	pk := tablePrimaryKeyQuery("public", "users")
	if !strings.Contains(pk.SQL, "PRIMARY KEY") || len(pk.Args) != 2 {
		t.Fatalf("pk = %#v", pk)
	}
	fk := tableForeignKeysQuery("pgx", "", "orders")
	if !strings.Contains(fk.SQL, "FOREIGN KEY") || fk.Args[1] != "orders" {
		t.Fatalf("fk = %#v", fk)
	}
	indexes := indexListQuery("pgx", "", "")
	if !strings.Contains(indexes.SQL, "pg_get_indexdef") {
		t.Fatalf("indexes = %#v", indexes)
	}
}

func TestParseIndexDefColumns(t *testing.T) {
	got := parseIndexDefColumns("CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email, lower(name))")
	if len(got) != 2 || got[0] != "email" {
		t.Fatalf("columns = %#v", got)
	}
}

func TestGroupMySQLIndexesAggregatesColumns(t *testing.T) {
	rows := []map[string]any{
		{"table_name": "users", "index_name": "PRIMARY", "non_unique": "0", "seq_in_index": "1", "column_name": "id", "index_type": "BTREE"},
		{"table_name": "users", "index_name": "users_name_idx", "non_unique": "1", "seq_in_index": "1", "column_name": "name", "index_type": "BTREE"},
		{"table_name": "users", "index_name": "users_name_idx", "non_unique": "1", "seq_in_index": "2", "column_name": "email", "index_type": "BTREE"},
	}
	indexes := groupMySQLIndexes(rows)
	if len(indexes) != 2 || !indexes[0].Primary || !indexes[0].Unique {
		t.Fatalf("indexes = %#v", indexes)
	}
	if len(indexes[1].Columns) != 2 || indexes[1].Columns[1] != "email" || indexes[1].Unique {
		t.Fatalf("composite index = %#v", indexes[1])
	}
}

func TestLowerRowKeysHandlesMySQLUppercaseColumns(t *testing.T) {
	rows := lowerRowKeys([]map[string]any{{
		"TABLE_SCHEMA": "latest-backend", "TABLE_NAME": []byte("users"), "TABLE_TYPE": "BASE TABLE", "TABLE_ROWS": int64(42),
	}})
	if asString(rows[0]["table_name"]) != "users" {
		t.Fatalf("rows = %#v", rows)
	}
	if estimate, ok := asInt64(rows[0]["table_rows"]); !ok || estimate != 42 {
		t.Fatalf("estimate = %v %v", estimate, ok)
	}
}
