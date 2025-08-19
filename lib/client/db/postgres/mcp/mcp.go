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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/utils/retryutils"
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

// queryToolForCockroachDB is same as queryTool but promotes CockroachDB instead
// of PostgreSQL.
var queryToolForCockroachDB = mcp.NewTool(dbmcp.ToolName(defaults.ProtocolCockroachDB, "query"),
	mcp.WithDescription("Execute SQL query against CockroachDB database connected using Teleport"),
	mcp.WithString(queryToolDatabaseParam,
		mcp.Required(),
		mcp.Description("Teleport database resource URI where the query will be executed"),
	),
	mcp.WithString(queryToolQueryParam,
		mcp.Required(),
		mcp.Description("CockroachDB SQL query to execute"),
	),
)

type database struct {
	name string
	conn *dbmcp.ManagedConn[*pgconn.PgConn]
}

// Server handles PostgreSQL-specific MCP tools requests.
type Server struct {
	logger    *slog.Logger
	databases map[string]*database
	retry     *retryutils.RetryV2
}

// NewServer initializes a PostgreSQL MCP server, creating the database
// configurations and registering Server tools into the root server.
func NewServer(ctx context.Context, cfg *dbmcp.NewServerConfig) (dbmcp.Server, error) {
	return newServer(ctx, queryTool, cfg)
}

// NewServerForCockroachDB initializes a CockroachDB MCP server, creating the
// database configurations and registering Server tools into the root server.
//
// Teleport differentiates CockroachDB and PostgreSQL as different protocols but
// uses the same db engine under the hood. Here we use the same PostgreSQL
// MCP server implementation as well, except for a slightly different tool
// name and description.
func NewServerForCockroachDB(ctx context.Context, cfg *dbmcp.NewServerConfig) (dbmcp.Server, error) {
	return newServer(ctx, queryToolForCockroachDB, cfg)
}

func newServer(ctx context.Context, queryTool mcp.Tool, cfg *dbmcp.NewServerConfig) (*Server, error) {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		// Given that the connection might have been dropped (e.g., due to net
		// work interruption). We should retry immediately.
		First:  0,
		Max:    maxQueryRetry * time.Second,
		Driver: retryutils.NewLinearDriver(time.Second),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Server{
		logger:    cfg.Logger,
		databases: make(map[string]*database, len(cfg.Databases)),
		retry:     retry,
	}

	for _, db := range cfg.Databases {
		if db.DatabaseUser == "" || db.DatabaseName == "" {
			return nil, trace.BadParameter("database %q is missing the username and database name", db.DB.GetName())
		}

		connCfg, err := buildConnConfig(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := dbmcp.NewManagedConn(
			&dbmcp.ManagedConnConfig{MaxIdleTime: connectionMaxIdleTime, Logger: s.logger},
			func(ctx context.Context) (*pgconn.PgConn, error) {
				conn, err := pgconn.ConnectConfig(ctx, connCfg)
				return conn, trace.Wrap(err)
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		s.databases[db.ResourceURI().WithoutParams().String()] = &database{
			name: db.DB.GetName(),
			conn: conn,
		}
	}

	cfg.RootServer.AddTool(queryTool, s.runQuery)
	return s, nil
}

// Close implements dbmcp.Server.
func (s *Server) Close(ctx context.Context) error {
	var errs []error
	for _, db := range s.databases {
		errs = append(errs, db.conn.Close(ctx))
	}
	return trace.NewAggregate(errs...)
}

// RunQueryResult is the run query tool result.
type RunQueryResult struct {
	// Results is the executed queries results.
	Results []QueryResult `json:"results"`
	// ErrorMessage if the queries execution wasn't successful, this field
	// contains the error message. The most common error will be connectivity
	// issues.
	ErrorMessage string `json:"error,omitempty"`
}

// QueryResult is a single query result.
type QueryResult struct {
	// Data contains the data returned from the query. It can be empty in case
	// the query doesn't return any data.
	Data []map[string]string `json:"data"`
	// RowsAffected number of rows affected by the query or returned as data.
	RowsAffected int `json:"rows_affected"`
	// ErrorMessage if the query contains any error, this field will contain the
	// error message. Given an execution of multiple queries, not all of them
	// can fail.
	ErrorMessage string `json:"error,omitempty"`
}

// runQuery tool function used to execute queries on databases.
func (s *Server) runQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	uri, err := request.RequireString(queryToolDatabaseParam)
	if err != nil {
		return s.wrapErrorResult(ctx, trace.Wrap(err))
	}

	sql, err := request.RequireString(queryToolQueryParam)
	if err != nil {
		return s.wrapErrorResult(ctx, trace.Wrap(err))
	}

	db, err := s.getDatabase(uri)
	if err != nil {
		return s.wrapErrorResult(ctx, trace.Wrap(err))
	}

	// TODO(gabrielcorado): ensure the connection used is consistent for the
	// session, making most of its queries to be present in a single audit
	// session/recording.

	s.retry.Reset()
	var (
		queryRes []*pgconn.Result
		queryErr error
	)
	for range maxQueryRetry {
		queryRes, queryErr = dbmcp.Exec(ctx, db.conn, func(ctx context.Context, conn *pgconn.PgConn) ([]*pgconn.Result, error) {
			res, err := conn.Exec(ctx, sql).ReadAll()
			return res, trace.Wrap(err)
		})
		if queryErr == nil {
			break
		}

		select {
		case <-s.retry.After():
		case <-ctx.Done():
		}
	}
	if queryErr != nil {
		return s.wrapErrorResult(ctx, trace.Wrap(queryErr))
	}

	result, err := buildQueryResult(queryRes)
	if err != nil {
		return s.wrapErrorResult(ctx, err)
	}

	return mcp.NewToolResultText(result), nil
}

func (s *Server) wrapErrorResult(ctx context.Context, toolErr error) (*mcp.CallToolResult, error) {
	s.logger.ErrorContext(ctx, "error while querying database", "error", toolErr)
	out, err := json.Marshal(RunQueryResult{ErrorMessage: dbmcp.FormatErrorMessage(toolErr).Error()})
	return mcp.NewToolResultError(string(out)), trace.Wrap(err)
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

// buildQueryResult takes a the response from pgconn and converts into a JSON
// format (which will be returned to MCP clients).
func buildQueryResult(results []*pgconn.Result) (string, error) {
	data := make([]QueryResult, len(results))

	for i, result := range results {
		commandTag := result.CommandTag
		queryRes := QueryResult{RowsAffected: int(commandTag.RowsAffected())}

		if result.Err != nil {
			queryRes.ErrorMessage = result.Err.Error()
		}

		// Initialize the slice so the resulting JSON will have an empty
		// array instead of null. This helps LLMs to not think there was an
		// error on the query, but instead it returned no records.
		if result.Err == nil && len(result.Rows) == 0 && commandTag.Select() {
			queryRes.Data = []map[string]string{}
		}

		columns := make([]string, len(result.FieldDescriptions))
		for i, fd := range result.FieldDescriptions {
			columns[i] = fd.Name
		}

		for _, row := range result.Rows {
			rowData := make(map[string]string)
			for columnIdx, contents := range row {
				// Given we're using the PostgreSQL text format we can safely
				// cast the returned values and they'll be in a readable format.
				//
				// References:
				// - https://www.postgresql.org/docs/current/protocol-overview.html#PROTOCOL-FORMAT-CODES
				rowData[columns[columnIdx]] = string(contents)
			}

			queryRes.Data = append(queryRes.Data, rowData)
		}

		data[i] = queryRes
	}

	out, err := json.Marshal(RunQueryResult{
		Results: data,
	})
	return string(out), trace.Wrap(err)
}

func buildConnConfig(db *dbmcp.Database) (*pgconn.Config, error) {
	// No need to provide a valid address here as the Lookup and DialContext
	// will handle the connection.
	config, err := pgconn.ParseConfig("postgres://")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.LookupFunc = func(ctx context.Context, host string) ([]string, error) {
		return db.LookupFunc(ctx, host)
	}
	config.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return db.DialContextFunc(ctx, network, addr)
	}

	config.User = db.DatabaseUser
	config.Database = db.DatabaseName
	config.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.RuntimeParams = map[string]string{
		applicationNameParamName: applicationNameParamValue,
	}
	config.TLSConfig = nil
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
	// connectionMaxIdleTime is the max connection idle time before it gets closed
	// automatically.
	connectionMaxIdleTime = 30 * time.Minute
	// maxQueryRetry is the maximum query execution retries.
	maxQueryRetry = 3
)
