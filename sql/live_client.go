package sql

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	pgx "github.com/jackc/pgx/v5"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/fluxplane/fluxplane-plugin/pluginbinding"
)

// hostMySQLNet is the go-sql-driver/mysql network name bound to the host conn
// dialer. Registration is process-global, which is safe because each plugin
// invocation runs in its own short-lived subprocess.
const hostMySQLNet = "fphostconn"

type sqlTarget struct {
	Driver   string // database/sql driver name: mysql | pgx | sqlite
	Dialect  string
	DSN      string
	SafeURL  string
	Database string
}

// runSQLQuery resolves the endpoint, opens a read-only connection dialed through
// the host conn capability (so the plugin performs no direct network IO; the
// driver still speaks the wire protocol over the host-provided stream), and runs
// the query. SQLite is file-backed and needs no dialer.
func runSQLQuery(ctx pluginbinding.Context, input QueryInput) (QueryOutput, error) {
	maxRows := input.MaxRows
	if maxRows <= 0 {
		maxRows = 100
	}
	if maxRows > 1000 {
		maxRows = 1000
	}
	out := QueryOutput{EndpointRef: input.EndpointRef}
	target, duration, err := withReadOnlySQL(ctx, input.EndpointRef, input.Driver, input.Database, input.Timeout, func(queryCtx context.Context, tx *stdsql.Tx, _ sqlTarget) error {
		rows, columns, truncated, err := queryAll(queryCtx, tx, maxRows, strings.TrimSpace(input.Query))
		if err != nil {
			return err
		}
		out.Columns = columns
		out.Rows = rows
		out.RowCount = len(rows)
		out.Truncated = truncated
		return nil
	})
	if err != nil {
		return QueryOutput{}, err
	}
	out.EndpointURL = target.SafeURL
	out.Driver = sqlFirstNonEmpty(target.Dialect, target.Driver)
	out.Database = target.Database
	out.DurationMS = duration
	return out, nil
}

// withReadOnlySQL resolves the endpoint, opens the host-dialed connection, and
// runs fn inside a read-only transaction bounded by the resolved timeout. It
// returns the resolved target and the elapsed milliseconds of fn.
func withReadOnlySQL(ctx pluginbinding.Context, endpointRef, driver, database, timeoutValue string, fn func(queryCtx context.Context, tx *stdsql.Tx, target sqlTarget) error) (sqlTarget, int64, error) {
	timeout, err := parseDurationDefault(timeoutValue, 10*time.Second)
	if err != nil {
		return sqlTarget{}, 0, pluginbinding.Errorf("bad_input", "%s", err)
	}
	target, err := resolveSQLTarget(ctx, endpointRef, driver, database)
	if err != nil {
		return sqlTarget{}, 0, err
	}
	db, err := openSQLDB(ctx, target)
	if err != nil {
		return sqlTarget{}, 0, pluginbinding.Errorf("sql", "%s", err)
	}
	defer func() { _ = db.Close() }()
	queryCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	tx, err := db.BeginTx(queryCtx, &stdsql.TxOptions{ReadOnly: true})
	if err != nil {
		return sqlTarget{}, 0, pluginbinding.Errorf("sql", "%s", err)
	}
	defer func() { _ = tx.Rollback() }()
	start := time.Now()
	if err := fn(queryCtx, tx, target); err != nil {
		return target, 0, err
	}
	return target, time.Since(start).Milliseconds(), nil
}

// queryAll runs one bounded query inside the transaction and scans every row
// into column-keyed maps ([]byte values become strings).
func queryAll(queryCtx context.Context, tx *stdsql.Tx, maxRows int, query string, args ...any) ([]map[string]any, []string, bool, error) {
	rows, err := tx.QueryContext(queryCtx, query, args...)
	if err != nil {
		return nil, nil, false, pluginbinding.Errorf("sql", "%s", err)
	}
	out, columns, truncated, err := scanSQLRows(rows, maxRows)
	if closeErr := rows.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return nil, nil, false, pluginbinding.Errorf("sql", "%s", err)
	}
	return out, columns, truncated, nil
}

// resolveSQLTarget turns the endpoint ref (resolved by the host to a URL) plus
// any input overrides into a concrete driver + DSN. Address resolution comes
// from the registered endpoint, never the environment.
func resolveSQLTarget(ctx pluginbinding.Context, endpointRef, driver, database string) (sqlTarget, error) {
	if ctx.Host == nil {
		return sqlTarget{}, pluginbinding.Fail("host_unavailable", "host client is unavailable")
	}
	endpoint, err := ctx.Host.ResolveEndpoint(strings.TrimSpace(endpointRef))
	if err != nil {
		return sqlTarget{}, pluginbinding.Errorf("sql", "resolve endpoint: %s", err)
	}
	rawURL := strings.TrimSpace(endpoint.URL)
	if rawURL == "" {
		return sqlTarget{}, pluginbinding.Fail("bad_input", "endpoint has no url")
	}
	return sqlTargetFromURL(driver, rawURL, database)
}

// openSQLDB opens a database/sql handle whose underlying connection is dialed via
// the host conn capability for networked drivers (mysql, pgx). SQLite is local.
func openSQLDB(ctx pluginbinding.Context, target sqlTarget) (*stdsql.DB, error) {
	switch target.Driver {
	case "sqlite":
		return stdsql.Open("sqlite", readOnlySQLiteDSN(target.DSN))
	case "mysql":
		dialer, err := hostConnDialer(ctx)
		if err != nil {
			return nil, err
		}
		mysql.RegisterDialContext(hostMySQLNet, func(dialCtx context.Context, addr string) (net.Conn, error) {
			return dialer(dialCtx, "tcp", addr)
		})
		dsn := strings.Replace(target.DSN, "@tcp(", "@"+hostMySQLNet+"(", 1)
		return stdsql.Open("mysql", dsn)
	case "pgx":
		dialer, err := hostConnDialer(ctx)
		if err != nil {
			return nil, err
		}
		cfg, err := pgx.ParseConfig(target.DSN)
		if err != nil {
			return nil, err
		}
		cfg.DialFunc = func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			return dialer(dialCtx, network, addr)
		}
		return stdsql.Open("pgx", pgxstdlib.RegisterConnConfig(cfg))
	default:
		return nil, fmt.Errorf("unsupported SQL driver %q", target.Driver)
	}
}

func hostConnDialer(ctx pluginbinding.Context) (pluginbinding.HostDialFunc, error) {
	if _, ok := ctx.Host.(pluginbinding.ConnDialer); !ok {
		return nil, fmt.Errorf("host does not support the conn dial capability required by sql")
	}
	return pluginbinding.HostDialer(ctx.Host), nil
}

func sqlTargetFromURL(driverOverride, rawURL, databaseOverride string) (sqlTarget, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return sqlTarget{}, err
	}
	driver := normalizeSQLDriver(sqlFirstNonEmpty(driverOverride, parsed.Scheme))
	if driver == "" {
		return sqlTarget{}, fmt.Errorf("unsupported SQL URL scheme %q", parsed.Scheme)
	}
	if !supportedSQLDriver(driver) {
		return sqlTarget{}, fmt.Errorf("unsupported SQL driver %q", driver)
	}
	if driver == "sqlite" {
		return sqliteTargetFromURL(parsed, rawURL, databaseOverride), nil
	}
	host := parsed.Hostname()
	if host == "" {
		return sqlTarget{}, fmt.Errorf("endpoint URL has no host")
	}
	port := parsed.Port()
	user := parsed.User.Username()
	pass, hasPass := parsed.User.Password()
	if user == "" {
		user = defaultSQLUser(driver)
	}
	database := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	if driver == "pgx" {
		return postgresTargetFromParts(user, pass, hasPass, host, port, database, parsed.RawQuery), nil
	}
	if port == "" {
		port = "3306"
	}
	addr := net.JoinHostPort(host, port)
	auth := user
	if hasPass {
		auth += ":" + pass
	}
	dsn := auth + "@tcp(" + addr + ")/" + database + "?parseTime=true"
	safeAuth := user
	if hasPass {
		safeAuth += ":xxxxx"
	}
	safeURL := "mysql://" + safeAuth + "@" + addr
	if database != "" {
		safeURL += "/" + database
	}
	return sqlTarget{Driver: "mysql", Dialect: "mysql", DSN: dsn, SafeURL: safeURL, Database: database}, nil
}

func postgresTargetFromParts(user, password string, hasPassword bool, host, port, database, query string) sqlTarget {
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	parsed := url.URL{Scheme: "postgres", Host: host, RawQuery: query}
	switch {
	case user != "" && hasPassword:
		parsed.User = url.UserPassword(user, password)
	case user != "":
		parsed.User = url.User(user)
	}
	if database != "" {
		parsed.Path = "/" + database
	}
	dsn := parsed.String()
	safe := parsed
	if safe.User != nil && safe.User.Username() != "" && hasPassword {
		safe.User = url.UserPassword(safe.User.Username(), "xxxxx")
	}
	return sqlTarget{Driver: "pgx", Dialect: "postgres", DSN: dsn, SafeURL: safe.String(), Database: database}
}

func sqliteTargetFromURL(parsed *url.URL, rawURL, databaseOverride string) sqlTarget {
	dsn := strings.TrimSpace(rawURL)
	switch parsed.Scheme {
	case "sqlite", "sqlite3":
		switch {
		case parsed.Opaque != "":
			dsn = parsed.Opaque
		case parsed.Host == "" && parsed.Path != "":
			dsn = parsed.Path
		case parsed.Host != "" && parsed.Path != "":
			dsn = strings.TrimLeft(parsed.Host+parsed.Path, "/")
		case parsed.Host != "":
			dsn = parsed.Host
		}
	case "file":
		dsn = rawURL
	}
	database := dsn
	if strings.TrimSpace(databaseOverride) != "" {
		database = strings.TrimSpace(databaseOverride)
	}
	return sqlTarget{Driver: "sqlite", Dialect: "sqlite", DSN: dsn, SafeURL: "sqlite://" + dsn, Database: database}
}

func readOnlySQLiteDSN(dsn string) string {
	if strings.TrimSpace(dsn) == ":memory:" {
		return dsn
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	if strings.HasPrefix(dsn, "file:") {
		if strings.Contains(dsn, "mode=") {
			return dsn
		}
		return dsn + separator + "mode=ro"
	}
	return "file:" + dsn + separator + "mode=ro"
}

func normalizeSQLDriver(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mysql", "mariadb":
		return "mysql"
	case "postgres", "postgresql", "pg", "pgx":
		return "pgx"
	case "sqlite", "sqlite3", "file":
		return "sqlite"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func supportedSQLDriver(driver string) bool {
	switch driver {
	case "mysql", "pgx", "sqlite":
		return true
	default:
		return false
	}
}

func defaultSQLUser(driver string) string {
	switch driver {
	case "mysql":
		return "root"
	case "pgx":
		return "postgres"
	default:
		return ""
	}
}

func scanSQLRows(rows *stdsql.Rows, maxRows int) ([]map[string]any, []string, bool, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	var out []map[string]any
	truncated := false
	for rows.Next() {
		if len(out) >= maxRows {
			truncated = true
			break
		}
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, nil, false, err
		}
		row := map[string]any{}
		for i, column := range columns {
			value := values[i]
			if b, ok := value.([]byte); ok {
				value = string(b)
			}
			row[column] = value
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return out, columns, truncated, nil
}

func sqlFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
