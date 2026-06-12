package sql

import (
	stdsql "database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	fpdatasource "github.com/fluxplane/fluxplane-datasource"
	core "github.com/fluxplane/fluxplane-plugin/manifest"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
	"github.com/fluxplane/fluxplane-plugin/pluginbinding/plugintest"
	_ "modernc.org/sqlite"
)

func TestQueryRejectsWrites(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationQuery, QueryInput{EndpointRef: "db", Query: "delete from users"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestQueryRequiresEndpointRef(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.RunError(t, plugin, OperationQuery, QueryInput{Query: "select 1"})
	if err.Code != "bad_input" {
		t.Fatalf("err = %#v", err)
	}
}

func TestReadOnlyQueryRejectsMultiStatementAndWriteCTE(t *testing.T) {
	blocked := []string{
		"select 1; delete from users",
		"with deleted as (delete from users returning id) select * from deleted",
		"select * from users into outfile '/tmp/users'",
	}
	for _, query := range blocked {
		if readOnlyQuery(query) {
			t.Fatalf("readOnlyQuery(%q) = true, want false", query)
		}
	}
}

func TestReadOnlyQueryIgnoresStringLiteralAndCommentTokens(t *testing.T) {
	allowed := []string{
		"select 'delete from users' as text",
		"select \"drop table users\" as text",
		"select `delete` from audit_log",
		"select 'semi;colon' as text",
		"select 1 -- delete from users\n",
		"select /* drop table users */ 1",
	}
	for _, query := range allowed {
		if !readOnlyQuery(query) {
			t.Fatalf("readOnlyQuery(%q) = false, want true", query)
		}
	}
}

func TestDatasourceSearchRejectsFreeTextWithSearchSpecificMessage(t *testing.T) {
	plugin := NewPluginWithService(Service{})
	err := plugintest.DatasourceSearchError(t, plugin, map[string]any{
		"datasource": DatasourceQueryRows,
		"query":      "api",
	})
	if err.Code != "bad_input" || err.Message == readOnlySQLQueryMessage {
		t.Fatalf("err = %#v", err)
	}
}

func TestManifestQuality(t *testing.T) {
	plugintest.AssertManifestQuality(t, Manifest())
}

func TestQueryRunsAgainstResolvedEndpoint(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteUsersDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, QueryInput{
		EndpointRef: "warehouse",
		Query:       "select id, name from users order by id",
		MaxRows:     10,
	}, plugintest.WithHost(host))
	if out.Driver != "sqlite" || out.RowCount != 2 {
		t.Fatalf("out = %#v", out)
	}
	if out.Rows[0]["name"] != "Ada" || out.Rows[1]["name"] != "Linus" {
		t.Fatalf("rows = %#v", out.Rows)
	}
}

func TestQueryRowsBuildsDatasourceRecords(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteUsersDB(t)}
	plugin := NewPluginWithService(Service{})
	records := plugintest.DatasourceSearchOK[QueryRowsResult](t, plugin, QueryInput{
		EndpointRef: "warehouse",
		Query:       "select id, name from users order by id",
		MaxRows:     10,
	}, plugintest.WithHost(host))
	if records.Count != 2 || records.Records[0].Row["name"] != "Ada" || records.Records[0].Driver != "sqlite" {
		t.Fatalf("records = %#v", records)
	}
}

// newSQLiteUsersDB creates a temporary sqlite database with a users table and
// returns its endpoint URL. SQLite is file-backed, so it exercises runSQLQuery's
// target resolution, read-only transaction, and row scanning without a network
// dial (the dialer path is covered by docker/kubernetes dogfooding).
func newSQLiteUsersDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "users.db")
	db, err := stdsql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	stmts := []string{
		"create table users (id integer primary key, name text)",
		"insert into users (id, name) values (1, 'Ada'), (2, 'Linus')",
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("seed sqlite (%s): %v", stmt, err)
		}
	}
	return "sqlite://" + path
}

type sqlTestHost struct {
	pluginbinding.HostClient

	t   *testing.T
	url string
}

func (h sqlTestHost) Secret(string) (pluginbinding.SecretMaterial, error) {
	return pluginbinding.SecretMaterial{}, nil
}

func (h sqlTestHost) Lookup(pluginbinding.DatasourceLookupInput) (pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]], error) {
	return pluginbinding.DatasourceLookupResult[pluginbinding.LookupMatch[any]]{}, nil
}

func (h sqlTestHost) Search(pluginbinding.DatasourceSearchInput) (pluginbinding.DatasourceSearchResult[any], error) {
	return pluginbinding.DatasourceSearchResult[any]{}, nil
}

func (h sqlTestHost) Get(pluginbinding.DatasourceGetInput) (pluginbinding.DatasourceGetResult[any], error) {
	return pluginbinding.DatasourceGetResult[any]{}, nil
}

func (h sqlTestHost) ResolveEndpoint(string) (core.EndpointRef, error) {
	return core.EndpointRef{URL: h.url}, nil
}

func (h sqlTestHost) HTTP(pluginbinding.HTTPRequest) (pluginbinding.HTTPResponse, error) {
	return pluginbinding.HTTPResponse{}, nil
}

func (h sqlTestHost) BlobRead(pluginbinding.BlobReadRequest) (pluginbinding.BlobReadResponse, error) {
	return pluginbinding.BlobReadResponse{}, nil
}

func (h sqlTestHost) BlobWrite(pluginbinding.BlobWriteRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h sqlTestHost) BlobInfo(pluginbinding.BlobInfoRequest) (pluginbinding.BlobRef, error) {
	return pluginbinding.BlobRef{}, nil
}

func (h sqlTestHost) EnvLookup(string) (pluginbinding.EnvLookupResponse, error) {
	return pluginbinding.EnvLookupResponse{}, nil
}

var _ pluginbinding.HostClient = sqlTestHost{}

func TestDatasourceDeclaresProviderAccess(t *testing.T) {
	spec := queryRowsDatasourceSpec()
	for _, access := range spec.Access {
		if access == fpdatasource.AccessProvider {
			return
		}
	}
	t.Fatalf("sql datasource access = %v, want provider", spec.Access)
}

func TestReadOnlyQueryAllowsWriteKeywordFunctionForms(t *testing.T) {
	allowed := []string{
		"SELECT REPLACE(name, 'a', 'b') FROM users",
		"select insert('abcdef', 2, 3, 'xy')",
		"SELECT id, REPLACE(`path`, '/old/', '/new/') AS p FROM files WHERE p LIKE '%x%'",
	}
	for _, query := range allowed {
		if !readOnlyQuery(query) {
			t.Fatalf("readOnlyQuery(%q) = false, want true (function form)", query)
		}
	}
	rejected := []string{
		"REPLACE INTO users VALUES (1, 'x')",
		"REPLACE(users) INTO something", // statement-leading stays rejected
		"INSERT INTO users VALUES (1)",
		"SELECT 1; REPLACE INTO users VALUES (1, 'x')",
		"WITH x AS (SELECT 1) DELETE FROM users",
		"SELECT * FROM t INTO OUTFILE '/tmp/x'",
	}
	for _, query := range rejected {
		if readOnlyQuery(query) {
			t.Fatalf("readOnlyQuery(%q) = true, want false", query)
		}
	}
}

func TestQueryEmptyResultKeepsCollectionsPresent(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteUsersDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[QueryOutput](t, plugin, OperationQuery, QueryInput{
		EndpointRef: "warehouse",
		Query:       "select id, name from users where id < 0",
	}, plugintest.WithHost(host))
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"rows":[]`, `"columns":[`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("empty result must keep %s present: %s", key, raw)
		}
	}
}

func TestSQLTestProbe(t *testing.T) {
	host := sqlTestHost{t: t, url: newSQLiteUsersDB(t)}
	plugin := NewPluginWithService(Service{})
	out := plugintest.RunOK[TestResult](t, plugin, OperationTest, TestInput{EndpointRef: "warehouse"}, plugintest.WithHost(host))
	if out.Status != "ok" || out.Driver != "sqlite" {
		t.Fatalf("out = %#v", out)
	}
}
