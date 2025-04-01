package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/defaults"
)

type Session struct {
	conn *pgx.Conn
}

type NewSessionConfig struct {
	RawDBConn net.Conn
	Route     clientproto.RouteToDatabase
}

func NewSession(ctx context.Context, cfg NewSessionConfig, mcpServer *mcpserver.MCPServer) (*Session, error) {
	pgConfig, err := buildConfig(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pgConn, err := pgx.ConnectConfig(ctx, pgConfig)
	if err != nil {
		return nil, trace.ConnectionProblem(err, "Unable to connect to database: %v", err)
	}

	sess := &Session{conn: pgConn}
	mcpServer.AddTool(queryTool, sess.QueryToolHandler)

	// TODO add resources support. Note: the current MCP SDK doesn't support
	// dynamic resources: https://github.com/mark3labs/mcp-go/issues/51

	return sess, nil
}

func (sess *Session) Close(ctx context.Context) error {
	return sess.conn.Close(ctx)
}

func (sess *Session) QueryToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sql := request.Params.Arguments[queryToolSQLParamName].(string)

	tx, err := sess.conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = tx.Commit(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := buildQueryResult(rows)
	if err != nil {
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

// func (sess *Session) ListResources() ([]ListResourceItem, error) { return nil, nil }
// func (sess *Session) ReadResource(uri string) (Resource, error)  { return Resource{}, nil }

var (
	queryTool = mcp.NewTool(
		queryToolName,
		mcp.WithDescription("Run a read-only SQL query"),
		mcp.WithString(queryToolSQLParamName, mcp.Required()),
	)
)

const (
	hostnamePlaceholder = "repl"

	queryToolName         = "query"
	queryToolSQLParamName = "sql"

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

func buildConfig(cfg NewSessionConfig) (*pgx.ConnConfig, error) {
	config, err := pgx.ParseConfig(fmt.Sprintf("postgres://%s", hostnamePlaceholder))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.User = cfg.Route.Username
	config.Database = cfg.Route.Database
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
		return cfg.RawDBConn, nil
	}

	return config, nil
}
