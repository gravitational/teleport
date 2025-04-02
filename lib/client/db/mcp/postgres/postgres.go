package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/defaults"
)

type Session struct {
	dbs map[string]*servedDB
}

type servedDB struct {
	conn *pgx.Conn
	info DBInfo
}

type NewSessionConfig struct {
	MCPServer   *mcpserver.MCPServer
	Datatabases []DBInfo
}

type DBInfo struct {
	RawConn     net.Conn
	Route       clientproto.RouteToDatabase
	Description string
}

func NewSession(ctx context.Context, cfg NewSessionConfig) (*Session, error) {
	slog.DebugContext(ctx, "New database MCP session")

	sess := &Session{}
	dbs := make(map[string]*servedDB, len(cfg.Datatabases))
	for _, dbInfo := range cfg.Datatabases {
		pgConfig, err := buildConfig(dbInfo)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pgConn, err := pgx.ConnectConfig(ctx, pgConfig)
		if err != nil {
			return nil, trace.ConnectionProblem(err, "Unable to connect to database %q: %v", dbInfo.Route.ServiceName, err)
		}

		dbURI := buildDBURI(dbInfo.Route.ServiceName)
		dbs[dbURI] = &servedDB{conn: pgConn, info: dbInfo}
		cfg.MCPServer.AddResource(mcp.NewResource(dbURI, fmt.Sprintf("%s PostgreSQL Datatabase", dbInfo.Route.ServiceName)), sess.DatabaseResourceHandler)
	}

	sess.dbs = dbs
	cfg.MCPServer.AddTool(queryTool, sess.QueryToolHandler)

	// TODO add resources support. Note: the current MCP SDK doesn't support
	// dynamic resources: https://github.com/mark3labs/mcp-go/issues/51

	return sess, nil
}

func (sess *Session) Close(ctx context.Context) error {
	var errors []error
	for _, db := range sess.dbs {
		errors = append(errors, db.conn.Close(ctx))
	}
	return trace.NewAggregate(errors...)
}

func (sess *Session) DatabaseResourceHandler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	if db, ok := sess.dbs[request.Params.URI]; ok {
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "text/plain",
				Text:     db.info.Description,
			},
		}, nil
	}

	return nil, trace.NotFound("database %q not found", request.Params.URI)
}

func (sess *Session) QueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	slog.DebugContext(ctx, "Handle query", "request", request)
	sql := request.Params.Arguments[queryToolSQLParamName].(string)
	dbURI := request.Params.Arguments[queryToolDBNameParamName].(string)

	// TODO have some better handling for db name, db URI, and or description (or query).
	db, ok := sess.dbs[buildDBURI(dbURI)]
	if !ok {
		return nil, trace.NotFound("Database %q not found. Only databases resources can be used for queries.")
	}

	tx, err := db.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rows.Close()

	result, err := buildQueryResult(rows)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Commit after rows are drained.
	if err = tx.Commit(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return mcp.NewToolResultText(result), nil
}

func buildQueryResult(rows pgx.Rows) (string, error) {
	var res []map[string]any
	columns := rows.FieldDescriptions()

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return "", trace.Wrap(err)
		}

		item := make(map[string]any, len(values))
		for i, v := range values {
			item[columns[i].Name] = v
		}

		res = append(res, item)
	}

	out, err := json.Marshal(res)
	return string(out), trace.Wrap(err)
}

func buildDBURI(name string) string {
	return fmt.Sprintf("teleport://db/postgres/%s", name)
}

var (
	queryTool = mcp.NewTool(
		queryToolName,
		mcp.WithDescription("Run a read-only SQL query"),
		mcp.WithString(queryToolDBNameParamName, mcp.Description("Database resource URI where the query will be executed"), mcp.Required()),
		mcp.WithString(queryToolSQLParamName, mcp.Required()),
	)
)

const (
	hostnamePlaceholder = "repl"

	queryToolName            = "query"
	queryToolSQLParamName    = "sql"
	queryToolDBNameParamName = "db_name"

	// TODO use to list tables and their schema.
	// listTablesQuery       = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
	// listTableColumnsQuery = "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1"

	databaseResourceURITemplate      = "postgres://%s"
	databaseTableResourceURITemplate = "postgres://%s/%s"
)

const (
	// applicationNameParamName defines the application name parameter name.
	//
	// https://www.postgresql.org/docs/17/libpq-connect.html#LIBPQ-CONNECT-APPLICATION-NAME
	applicationNameParamName = "application_name"
	// applicationNameParamValue defines the application name parameter value.
	applicationNameParamValue = "teleport-repl"
)

func buildConfig(info DBInfo) (*pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s", hostnamePlaceholder))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.User = info.Route.Username
	config.Database = info.Route.Database
	config.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.RuntimeParams = map[string]string{
		applicationNameParamName: applicationNameParamValue,
	}
	config.TLSConfig = nil

	// Provide a lookup function to avoid having the hostname placeholder to
	// resolve into something else. Note that the returned value won't be used.
	config.LookupFunc = func(_ context.Context, _ string) ([]string, error) {
		return []string{hostnamePlaceholder}, nil
	}
	config.DialFunc = func(_ context.Context, _, _ string) (net.Conn, error) {
		return info.RawConn, nil
	}

	return config, nil
}
