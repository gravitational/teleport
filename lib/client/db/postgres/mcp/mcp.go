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
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jonboulle/clockwork"
	"github.com/mark3labs/mcp-go/mcp"

	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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
	mu sync.Mutex

	clock            clockwork.Clock
	connConfig       *pgconn.Config
	activeConnection *pgconn.PgConn
	lastActivity     time.Time
}

func (d *database) exec(ctx context.Context, query string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastActivity = d.clock.Now()

	// Establish connection if none is active.
	if d.activeConnection == nil {
		var err error
		d.activeConnection, err = pgconn.ConnectConfig(ctx, d.connConfig)
		if err != nil {
			return "", trace.ConnectionProblem(err, "Unable to connect to database: %v", err)
		}
	}

	queryRes, err := d.activeConnection.Exec(ctx, query).ReadAll()
	if err != nil {
		return "", trace.Wrap(err)
	}

	res, err := buildQueryResult(queryRes)
	return res, trace.Wrap(err)
}

// closeIdle closes the database connection if idle time passes the threshold.
func (d *database) closeIdle(ctx context.Context, now time.Time) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastActivity.IsZero() || now.Sub(d.lastActivity) < connectionIdleTime {
		return false, nil
	}

	return true, trace.Wrap(d.closeLocked(ctx))
}

// close closes the active connection.
func (d *database) close(ctx context.Context) error {
	d.mu.Lock()
	err := d.closeLocked(ctx)
	d.mu.Unlock()
	return trace.Wrap(err)
}

func (d *database) closeLocked(ctx context.Context) error {
	d.lastActivity = time.Time{}
	if d.activeConnection != nil {
		err := d.activeConnection.Close(ctx)
		d.activeConnection = nil
		return trace.Wrap(err)
	}

	return nil
}

// Server handles PostgreSQL-specific MCP tools requests.
type Server struct {
	logger    *slog.Logger
	clock     clockwork.Clock
	databases utils.SyncMap[string, *database]
	cancel    context.CancelFunc
}

// NewServer initializes a PostgreSQL MCP server, creating the database
// configurations and registering Server tools into the root server.
func NewServer(ctx context.Context, cfg *dbmcp.NewServerConfig) (dbmcp.Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	s := &Server{logger: cfg.Logger, clock: cfg.Clock, cancel: cancel}

	for _, db := range cfg.Databases {
		if db.DatabaseUser == "" || db.DatabaseName == "" {
			return nil, trace.BadParameter("database %q is missing the username and database name", db.DB.GetName())
		}

		connCfg, err := buildConnConfig(db)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		s.databases.Store(db.ResourceURI().WithoutParams().String(), &database{
			Database:   db,
			clock:      cfg.Clock,
			connConfig: connCfg,
		})
	}

	cfg.RootServer.AddTool(queryTool, s.RunQuery)
	go s.idleChecker(ctx)
	return s, nil
}

// Close implements dbmcp.Server.
func (s *Server) Close(ctx context.Context) error {
	s.cancel()

	var errs []error
	s.databases.Range(func(key string, db *database) bool {
		errs = append(errs, db.close(ctx))
		return true
	})

	return trace.NewAggregate(errs...)
}

// QueryResult is the run query tool result.
type RunQueryResult struct {
	// Results is the executed queries results.
	Results []QueryResult `json:"results"`
	// ErrorMessage if the queries execution wasn't successful, this field
	// contains the error message.
	ErrorMessage string `json:"error,omitempty"`
}

// QueryResult is a single query result.
type QueryResult struct {
	// Data contains the data returned from the query. It can be empty in case
	// the query doesn't return any data.
	Data []map[string]string `json:"data"`
	// RowsCount number of rows affected by the query or returned as data.
	RowsCount int `json:"rows_count"`
	// ErrorMessage if the query contains any error, this field will contain the
	// error message.
	ErrorMessage string `json:"error,omitempty"`
}

// RunQuery tool function used to execute queries on databases.
func (s *Server) RunQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		return s.wrapErrorResult(ctx, err)
	}

	// TODO(gabrielcorado): ensure the connection used is consistent for the
	// session, making most of its queries to be present in a single audit
	// session/recording.
	result, err := db.exec(ctx, sql)
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

// idleChecker runs continuously checking if database connections are
// idle and closes them.
func (s *Server) idleChecker(ctx context.Context) {
	timer := s.clock.NewTicker(idleCheckerInterval)
	defer timer.Stop()

	for {
		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-timer.Chan():
			s.logger.DebugContext(ctx, "checking idle database connections")
			s.databases.Range(func(_ string, db *database) bool {
				if closed, err := db.closeIdle(ctx, s.clock.Now()); closed {
					s.logger.DebugContext(ctx, "closed idle database connection", "database", db.DB.GetName(), "error", err)
				}
				return true
			})
		}
	}
}

// buildQueryResult takes a the response from pgconn and converts into a JSON
// format (which will be returned to MCP clients).
func buildQueryResult(results []*pgconn.Result) (string, error) {
	data := make([]QueryResult, len(results))

	for i, result := range results {
		commandTag := result.CommandTag
		queryRes := QueryResult{RowsCount: int(commandTag.RowsAffected())}

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
			columns[i] = string(fd.Name)
		}

		for _, row := range result.Rows {
			rowData := make(map[string]string)
			for columnIdx, contents := range row {
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

func (s *Server) getDatabase(uri string) (*database, error) {
	if !clientmcp.IsDatabaseResourceURI(uri) {
		return nil, dbmcp.WrongDatabaseURIFormatError
	}

	db, ok := s.databases.Load(uri)
	if !ok {
		return nil, dbmcp.DatabaseNotFoundError
	}

	return db, nil
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
	// connectionIdleTime is the max connection idle time before it gets closed
	// automatically.
	connectionIdleTime = 1 * time.Minute
	// idleCheckerInterval is the interval which we'll check if the database
	// connections are idle.
	idleCheckerInterval = connectionIdleTime / 2
)
