/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package spanner

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

type upstreamServer[T any] interface {
	Recv() (T, error)
	grpc.ClientStream
}

type downstreamClient[T any] interface {
	Send(T) error
	grpc.ServerStream
}

func proxyServerStream[T any](up upstreamServer[T], down downstreamClient[T]) error {
	for {
		res, err := up.Recv()
		if err != nil {
			if err == io.EOF {
				// EOF is expected and signals end of the server stream.
				return nil
			}
			return trace.Wrap(err)
		}
		if err := down.Send(res); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) CreateSession(ctx context.Context, req *spannerpb.CreateSessionRequest) (*spannerpb.Session, error) {
	if err := e.authorizeRPCRequest(req.Database, "CreateSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.CreateSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BatchCreateSessions(ctx context.Context, req *spannerpb.BatchCreateSessionsRequest) (*spannerpb.BatchCreateSessionsResponse, error) {
	if err := e.authorizeRPCRequest(req.Database, "BatchCreateSessions", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.BatchCreateSessions(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) GetSession(ctx context.Context, req *spannerpb.GetSessionRequest) (*spannerpb.Session, error) {
	if err := e.authorizeRPCRequest(req.Name, "GetSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.GetSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ListSessions(ctx context.Context, req *spannerpb.ListSessionsRequest) (*spannerpb.ListSessionsResponse, error) {
	if err := e.authorizeRPCRequest(req.Database, "ListSessions", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.ListSessions(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) DeleteSession(ctx context.Context, req *spannerpb.DeleteSessionRequest) (*emptypb.Empty, error) {
	if err := e.authorizeRPCRequest(req.Name, "DeleteSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.DeleteSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ExecuteBatchDml(ctx context.Context, req *spannerpb.ExecuteBatchDmlRequest) (*spannerpb.ExecuteBatchDmlResponse, error) {
	if err := e.authorizeRPCRequest(req.Session, "ExecuteBatchDml", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.ExecuteBatchDml(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Read(ctx context.Context, req *spannerpb.ReadRequest) (*spannerpb.ResultSet, error) {
	if err := e.authorizeRPCRequest(req.Session, "Read", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.Read(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BeginTransaction(ctx context.Context, req *spannerpb.BeginTransactionRequest) (*spannerpb.Transaction, error) {
	if err := e.authorizeRPCRequest(req.Session, "BeginTransaction", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.BeginTransaction(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Commit(ctx context.Context, req *spannerpb.CommitRequest) (*spannerpb.CommitResponse, error) {
	if err := e.authorizeRPCRequest(req.Session, "Commit", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.Commit(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Rollback(ctx context.Context, req *spannerpb.RollbackRequest) (*emptypb.Empty, error) {
	if err := e.authorizeRPCRequest(req.Session, "Rollback", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.Rollback(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) PartitionRead(ctx context.Context, req *spannerpb.PartitionReadRequest) (*spannerpb.PartitionResponse, error) {
	if err := e.authorizeRPCRequest(req.Session, "PartitionRead", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.PartitionRead(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BatchWrite(req *spannerpb.BatchWriteRequest, stream spannerpb.Spanner_BatchWriteServer) error {
	if err := e.authorizeRPCRequest(req.Session, "BatchWrite", req); err != nil {
		return trace.Wrap(err)
	}
	cc, err := e.cloudClient.BatchWrite(stream.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream[*spannerpb.BatchWriteResponse](cc, stream))
}

func (e *Engine) ExecuteSql(ctx context.Context, req *spannerpb.ExecuteSqlRequest) (*spannerpb.ResultSet, error) {
	if skipQueryAudit(req.Sql) {
		if _, err := e.authorizeDatabaseName(req.Session); err != nil {
			return nil, trace.AccessDenied("access denied")
		}
	} else if err := e.authorizeRPCRequest(req.Session, "ExecuteSql", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.ExecuteSql(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) PartitionQuery(ctx context.Context, req *spannerpb.PartitionQueryRequest) (*spannerpb.PartitionResponse, error) {
	if skipQueryAudit(req.Sql) {
		if _, err := e.authorizeDatabaseName(req.Session); err != nil {
			return nil, trace.AccessDenied("access denied")
		}
	} else if err := e.authorizeRPCRequest(req.Session, "PartitionQuery", req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := e.cloudClient.PartitionQuery(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ExecuteStreamingSql(req *spannerpb.ExecuteSqlRequest, stream spannerpb.Spanner_ExecuteStreamingSqlServer) error {
	if skipQueryAudit(req.Sql) {
		if _, err := e.authorizeDatabaseName(req.Session); err != nil {
			return trace.AccessDenied("access denied")
		}
	} else if err := e.authorizeRPCRequest(req.Session, "ExecuteStreamingSql", req); err != nil {
		return trace.Wrap(err)
	}
	cc, err := e.cloudClient.ExecuteStreamingSql(stream.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream[*spannerpb.PartialResultSet](cc, stream))
}

func (e *Engine) StreamingRead(req *spannerpb.ReadRequest, stream spannerpb.Spanner_StreamingReadServer) error {
	if err := e.authorizeRPCRequest(req.Session, "StreamingRead", req); err != nil {
		return trace.Wrap(err)
	}
	cc, err := e.cloudClient.StreamingRead(stream.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream[*spannerpb.PartialResultSet](cc, stream))
}

// skipQueryAudit returns whether to skip emitting an audit log event for an
// sql query. This is to avoid spamming the audit log with queries that
// some GUIs (DataGrip, to name one) spam automatically for their own purposes.
func skipQueryAudit(sql string) bool {
	sql = strings.ToLower(sql)
	switch sql {
	case "select 1", "select 'keep alive'":
		return true
	}
	return false
}

// parseDatabaseName parses a string of the form
// databases/<database>[/sessions/<session>] and returns the database name.
func parseDatabaseName(name string) (string, error) {
	const (
		databasesPrefix = iota
		database
		numParts
	)
	parts := strings.Split(name, "/")
	if len(parts) < numParts || parts[databasesPrefix] != "databases" {
		return "", trace.AccessDenied("invalid database name")
	}
	return parts[database], nil
}

func (e *Engine) authorizeDatabaseName(targetID string) (string, error) {
	suffix, ok := strings.CutPrefix(targetID, e.instanceName)
	if !ok {
		return "", trace.AccessDenied(`access denied to %q

Database name must start with %q
`, targetID, e.instanceName)
	}
	dbName, err := parseDatabaseName(suffix)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// authorize the RPC against allowed databases.
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		services.AccessState{MFAVerified: true},
		&services.DatabaseNameMatcher{Name: dbName},
	)
	return dbName, trace.Wrap(err)
}

type rpcInfo struct {
	// database is the name of the Spanner database within the instance for
	// which the RPC was called. This can be different for each RPC, overriding
	// the session database name as long as Teleport RBAC allows it.
	database string
	// procedure is the name of the remote procedure.
	procedure string
	// args is the RPC including all arguments.
	args *events.Struct
	// err contains an error if the RPC was rejected by Teleport.
	err error
}

func (e *Engine) auditRPC(ctx context.Context, r rpcInfo) {
	sessionCtx := e.sessionCtx.WithDatabase(r.database)
	event := &events.SpannerRPC{
		Metadata: common.MakeEventMetadata(sessionCtx,
			libevents.DatabaseSessionSpannerRPCEvent,
			libevents.SpannerRPCCode),
		UserMetadata:     common.MakeUserMetadata(sessionCtx),
		SessionMetadata:  common.MakeSessionMetadata(sessionCtx),
		DatabaseMetadata: common.MakeDatabaseMetadata(sessionCtx),
		Status: events.Status{
			Success: true,
		},
		Procedure: r.procedure,
		Args:      r.args,
	}
	if r.err != nil {
		event.Metadata.Code = libevents.SpannerRPCDeniedCode
		event.Status = events.Status{
			Success:     false,
			Error:       trace.Unwrap(r.err).Error(),
			UserMessage: r.err.Error(),
		}
	}
	e.Audit.EmitEvent(ctx, event)
}

func rpcRequestToStruct(req any) (*events.Struct, error) {
	blob, err := json.Marshal(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	s := &events.Struct{}
	if err := s.UnmarshalJSON(blob); err != nil {
		return nil, trace.Wrap(err)
	}
	return s, nil
}

func (e *Engine) authorizeRPCRequest(targetID, procedure string, req any) error {
	dbName, authzErr := e.authorizeDatabaseName(targetID)
	args, conversionErr := rpcRequestToStruct(req)
	r := rpcInfo{
		database:  dbName,
		procedure: procedure,
		args:      args,
		err:       trace.NewAggregate(authzErr, conversionErr),
	}
	e.auditRPC(e.Context, r)
	if r.err != nil {
		// if authz rejects access to the db or the audit log can't capture
		// the complete RPC info due to some conversion error, denied access.
		return trace.AccessDenied("access denied")
	}
	return nil
}
