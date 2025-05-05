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
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"

	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	"github.com/gravitational/teleport/lib/defaults"
)

// runQueryTool is the run query MCP tool definition.
var runQueryTool = mcp.NewTool(dbmcp.ToolName(defaults.ProtocolPostgres, "run_query"),
	mcp.WithDescription("Execute SQL query against PostgreSQL database connected using Teleport"),
	mcp.WithString(runQueryDatabaseParam,
		mcp.Required(),
		mcp.Description("Teleport database resource URI where the query will be executed"),
	),
	mcp.WithString(runQueryQueryParam,
		mcp.Required(),
		mcp.Description("PostgresSQL SQL query to execute"),
	),
)

type database struct {
	*dbmcp.Database
	pool *pgxpool.Pool
}

// Server PostgreSQL MCP server.
type Server struct {
	databases map[string]*database
}

// NewServer initializes a PostgreSQL MCP server.
func NewServer(ctx context.Context, cfg *dbmcp.NewServerConfig) (dbmcp.Server, error) {
	s := &Server{databases: make(map[string]*database)}

	for _, db := range cfg.Databases {
		if db.DatabaseUser == "" || db.DatabaseName == "" {
			return nil, trace.BadParameter("must specify the username or database name used to connect to the database")
		}

		connCfg, err := buildConnConfig(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		pool, err := pgxpool.NewWithConfig(ctx, connCfg)
		if err != nil {
			return nil, trace.BadParameter("failed to parse database %q connection config: %s", db.DB.GetName(), err)
		}

		uri := dbmcp.DatabaseResourceURI(db.DB.GetName())
		s.databases[uri] = &database{
			Database: db,
			pool:     pool,
		}
	}

	cfg.RootServer.AddTool(runQueryTool, s.RunQuery)
	return s, nil
}

// Stop implements dbmcp.Server.
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
	uri := request.Params.Arguments[runQueryDatabaseParam].(string)
	sql := request.Params.Arguments[runQueryQueryParam].(string)

	db, err := s.getDatabase(uri)
	if err != nil {
		return wrapErrorResult(err)
	}

	// TODO(gabrielcorado): ensure the connection used is consistent for the
	// session, making most of its queries to be present in a single audit
	// session/recording.
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return wrapErrorResult(err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, sql)
	if err != nil {
		return wrapErrorResult(err)
	}

	result, err := buildQueryResult(rows)
	if err != nil {
		return wrapErrorResult(err)
	}

	if err = tx.Commit(ctx); err != nil {
		return wrapErrorResult(err)
	}

	return mcp.NewToolResultText(result), nil
}

func wrapErrorResult(err error) (*mcp.CallToolResult, error) {
	out, err := json.Marshal(RunQueryResult{ErrorMessage: dbmcp.FormatErrorMessage(err).Error()})
	return mcp.NewToolResultError(string(out)), nil
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
	if !dbmcp.IsDatabaseResourceURI(uri) {
		uri = dbmcp.DatabaseResourceURI(uri)
	}

	db, ok := s.databases[uri]
	if !ok {
		return nil, trace.NotFound("Database %q not found. Only Teleport databases resources can be used for queries.", uri)
	}

	return db, nil
}

func buildConnConfig(db *dbmcp.Database) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s", db.Addr))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config.MaxConnIdleTime = connectionIdleTime
	config.MaxConns = int32(maxConnections)
	config.ConnConfig.User = db.DatabaseUser
	config.ConnConfig.Database = db.DatabaseName
	config.ConnConfig.ConnectTimeout = defaults.DatabaseConnectTimeout
	config.ConnConfig.RuntimeParams = map[string]string{
		applicationNameParamName: applicationNameParamValue,
	}
	config.ConnConfig.TLSConfig = nil
	return config, nil
}

const (
	// runQueryDatabaseParam is the name of the database URI param name from
	// runQuery tool.
	runQueryDatabaseParam = "database"
	// runQueryQueryParam is the name of the query param name from runQuery
	// tool.
	runQueryQueryParam = "query"

	// applicationNameParamName defines the application name parameter name.
	//
	// https://www.postgresql.org/docs/17/libpq-connect.html#LIBPQ-CONNECT-APPLICATION-NAME
	applicationNameParamName = "application_name"
	// applicationNameParamValue defines the application name parameter value.
	applicationNameParamValue = "teleport-mcp"
	// connectionIdleTime is the max connection idle time before it gets closed
	// automatically.
	connectionIdleTime = 5 * time.Minute
	// maxConnections defines the max number of concurrent connections the pool
	// can have.
	//
	// Given the current MCP usage, the clients will most likely do one query at
	// time, even on multiple sessions.
	maxConnections = 1
)
