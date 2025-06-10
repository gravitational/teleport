// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"

	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/defaults"
)

// queryTool is the run query MCP tool definition.
var queryTool = mcp.NewTool(dbmcp.ToolName(defaults.ProtocolPostgres, "query"),
	mcp.WithDescription("Execute SQL query against PostgreSQL database connected using Teleport"),
	mcp.WithString(queryToolDatabaseParam,
		mcp.Required(),
		mcp.Description("Teleport database resource URI where the query will be executed"),
	),
	mcp.WithString(queryToolQueryParam,
		mcp.Required(),
		mcp.Description("PostgresSQL SQL query to execute"),
	),
)

type database struct {
	*dbmcp.Database
	pool *pgxpool.Pool
}

// Server handles PostgreSQL-specific MCP tools requests.
type Server struct {
	logger    *slog.Logger
	databases map[string]*database
}

// NewServer initializes a PostgreSQL MCP server, creating the database
// configurations and registering Server tools into the root server.
func NewServer(ctx context.Context, cfg *dbmcp.NewServerConfig) (dbmcp.Server, error) {
	s := &Server{logger: cfg.Logger, databases: make(map[string]*database)}

	for _, db := range cfg.Databases {
		if db.DatabaseUser == "" || db.DatabaseName == "" {
			return nil, trace.BadParameter("database %q is missing the username and database name", db.DB.GetName())
		}

		connCfg, err := buildConnConfig(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pool, err := pgxpool.NewWithConfig(ctx, connCfg)
		if err != nil {
			return nil, trace.BadParameter("failed to parse database %q connection config: %s", db.DB.GetName(), err)
		}

		s.databases[db.ResourceURI().String()] = &database{
			Database: db,
			pool:     pool,
		}
	}

	cfg.RootServer.AddTool(queryTool, s.RunQuery)
	return s, nil
}

// Close implements dbmcp.Server.
func (s *Server) Close(context.Context) error {
	for _, db := range s.databases {
		db.pool.Close()
	}

	return nil
}

// RunQueryResult is the run query tool result.
type RunQueryResult struct {
	// Data contains the data returned from the query. It can be empty in case
	// the query doesn't return any data.
	Data []map[string]any `json:"data"`
	// RowsCount number of rows affected by the query or returned as data.
	RowsCount int `json:"rowsCount"`
	// ErrorMessage if the query wasn't successful, this field contains the
	// error message.
	ErrorMessage string `json:"error,omitempty"`
}

// RunQuery tool function used to execute queries on databases.
func (s *Server) RunQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri, err := request.RequireString(queryToolDatabaseParam)
	if err != nil {
		return s.wrapErrorResult(ctx, nil, trace.Wrap(err))
	}

	sql, err := request.RequireString(queryToolQueryParam)
	if err != nil {
		return s.wrapErrorResult(ctx, nil, trace.Wrap(err))
	}

	db, err := s.getDatabase(uri)
	if err != nil {
		return s.wrapErrorResult(ctx, nil, err)
	}

	// TODO(gabrielcorado): ensure the connection used is consistent for the
	// session, making most of its queries to be present in a single audit
	// session/recording.
	rows, err := db.pool.Query(ctx, sql)
	if err != nil {
		return s.wrapErrorResult(ctx, db.ExternalErrorRetriever, err)
	}

	// Returned rows are being closed by this function.
	result, err := buildQueryResult(rows)
	if err != nil {
		return s.wrapErrorResult(ctx, db.ExternalErrorRetriever, err)
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) wrapErrorResult(ctx context.Context, externalRetriever dbmcp.ExternalErrorRetriever, toolErr error) (*mcp.CallToolResult, error) {
	s.logger.ErrorContext(ctx, "error while querying database", "error", toolErr)
	out, err := json.Marshal(RunQueryResult{ErrorMessage: dbmcp.FormatErrorMessage(externalRetriever, toolErr).Error()})
	return mcp.NewToolResultError(string(out)), trace.Wrap(err)
}

// buildQueryResult takes a the response from pgx and converts into a JSON
// format (which will be returned to LLMs).
func buildQueryResult(rows pgx.Rows) (string, error) {
	// Just ensure the rows is always closed. It is safe if this is called
	// multiple times.
	defer rows.Close()

	var data []map[string]any
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

		data = append(data, item)
	}

	// Close the rows to finish consuming it. Depending on the its type
	// we can only collect the command tag after rows is closed.
	rows.Close()
	commandTag := rows.CommandTag()

	// Initialize the slice so the resulting JSON will have an empty array
	// instead of null.
	if len(data) == 0 && commandTag.Select() {
		data = []map[string]any{}
	}

	out, err := json.Marshal(RunQueryResult{
		Data:      data,
		RowsCount: int(commandTag.RowsAffected()),
	})
	return string(out), trace.Wrap(err)
}

func (s *Server) getDatabase(uri string) (*database, error) {
	if !clientmcp.IsDatabaseResourceURI(uri) {
		return nil, dbmcp.WrongDatabaseURIFormatError
	}

	db, ok := s.databases[uri]
	if !ok {
		return nil, dbmcp.DatabaseNotFoundError
	}

	return db, nil
}

func buildConnConfig(db *dbmcp.Database) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig("postgres://" + db.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.MaxConnIdleTime = connectionIdleTime
	config.MaxConns = int32(maxConnections)

	config.ConnConfig.LookupFunc = func(ctx context.Context, host string) ([]string, error) {
		return db.LookupFunc(ctx, host)
	}
	config.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return db.DialContextFunc(ctx, network, addr)
	}

	config.ConnConfig.User = db.DatabaseUser
	config.ConnConfig.Database = db.DatabaseName
	config.ConnConfig.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.ConnConfig.RuntimeParams = map[string]string{
		applicationNameParamName: applicationNameParamValue,
	}
	config.ConnConfig.TLSConfig = nil
	// Use simple protocol to have a closer behavior to DB REPL and psql.
	//
	// This also avoids each query being prepared, binded and executed, reducing
	// the amount of audit events per query executed.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	return config, nil
}

const (
	// queryToolDatabaseParam is the name of the database URI param name from
	// query tool.
	queryToolDatabaseParam = "database"
	// queryToolQueryParam is the name of the query param name from query tool.
	queryToolQueryParam = "query"

	// applicationNameParamName defines the application name parameter name.
	//
	// https://www.postgresql.org/docs/17/libpq-connect.html#LIBPQ-CONNECT-APPLICATION-NAME
	applicationNameParamName = "application_name"
	// applicationNameParamValue defines the application name parameter value.
	applicationNameParamValue = "teleport-mcp"
	// connectionIdleTime is the max connection idle time before it gets closed
	// automatically.
	connectionIdleTime = 1 * time.Minute
	// maxConnections defines the max number of concurrent connections the pool
	// can have.
	//
	// Given the current MCP usage, the clients will most likely do one query at
	// time, even on multiple sessions.
	maxConnections = 1
)
