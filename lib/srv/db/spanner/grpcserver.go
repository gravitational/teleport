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
	"errors"
	"fmt"
	"io"
	"strings"

	spannerapi "cloud.google.com/go/spanner/apiv1"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/option"
	gtransport "google.golang.org/api/transport/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/stats"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/common/role"
)

type upstream[T any] interface {
	Recv() (T, error)
	grpc.ClientStream
}

type downstream[T any] interface {
	Send(T) error
	grpc.ServerStream
}

func proxyServerStream[T any](ctx context.Context, up upstream[T], down downstream[T]) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		res, err := up.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// EOF is expected and signals end of the server stream.
				return nil
			}
			return trace.Wrap(err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := down.Send(res); err != nil {
			return trace.Wrap(err)
		}
	}
}

func (e *Engine) CreateSession(ctx context.Context, req *spannerpb.CreateSessionRequest) (*spannerpb.Session, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetDatabase(), "CreateSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.CreateSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BatchCreateSessions(ctx context.Context, req *spannerpb.BatchCreateSessionsRequest) (*spannerpb.BatchCreateSessionsResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetDatabase(), "BatchCreateSessions", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.BatchCreateSessions(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) GetSession(ctx context.Context, req *spannerpb.GetSessionRequest) (*spannerpb.Session, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetName(), "GetSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.GetSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ListSessions(ctx context.Context, req *spannerpb.ListSessionsRequest) (*spannerpb.ListSessionsResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetDatabase(), "ListSessions", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.ListSessions(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) DeleteSession(ctx context.Context, req *spannerpb.DeleteSessionRequest) (*emptypb.Empty, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetName(), "DeleteSession", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.DeleteSession(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ExecuteBatchDml(ctx context.Context, req *spannerpb.ExecuteBatchDmlRequest) (*spannerpb.ExecuteBatchDmlResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "ExecuteBatchDml", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.ExecuteBatchDml(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Read(ctx context.Context, req *spannerpb.ReadRequest) (*spannerpb.ResultSet, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "Read", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.Read(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BeginTransaction(ctx context.Context, req *spannerpb.BeginTransactionRequest) (*spannerpb.Transaction, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "BeginTransaction", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.BeginTransaction(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Commit(ctx context.Context, req *spannerpb.CommitRequest) (*spannerpb.CommitResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "Commit", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.Commit(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) Rollback(ctx context.Context, req *spannerpb.RollbackRequest) (*emptypb.Empty, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "Rollback", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.Rollback(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) PartitionRead(ctx context.Context, req *spannerpb.PartitionReadRequest) (*spannerpb.PartitionResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "PartitionRead", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.PartitionRead(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) BatchWrite(req *spannerpb.BatchWriteRequest, stream spannerpb.Spanner_BatchWriteServer) error {
	ctx := stream.Context()
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "BatchWrite", req); err != nil {
		return trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	cc, err := clt.BatchWrite(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream(e.clientConnContext, cc, stream))
}

func (e *Engine) ExecuteSql(ctx context.Context, req *spannerpb.ExecuteSqlRequest) (*spannerpb.ResultSet, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "ExecuteSql", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.ExecuteSql(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) PartitionQuery(ctx context.Context, req *spannerpb.PartitionQueryRequest) (*spannerpb.PartitionResponse, error) {
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "PartitionQuery", req); err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.PartitionQuery(ctx, req)
	return res, trace.Wrap(err)
}

func (e *Engine) ExecuteStreamingSql(req *spannerpb.ExecuteSqlRequest, stream spannerpb.Spanner_ExecuteStreamingSqlServer) error {
	ctx := stream.Context()
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "ExecuteStreamingSql", req); err != nil {
		return trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	cc, err := clt.ExecuteStreamingSql(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream(e.clientConnContext, cc, stream))
}

func (e *Engine) StreamingRead(req *spannerpb.ReadRequest, stream spannerpb.Spanner_StreamingReadServer) error {
	ctx := stream.Context()
	if err := e.authorizeRPCRequest(ctx, req.GetSession(), "StreamingRead", req); err != nil {
		return trace.Wrap(err)
	}
	clt, err := e.getClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	cc, err := clt.StreamingRead(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyServerStream(e.clientConnContext, cc, stream))
}

func (e *Engine) getClient(ctx context.Context) (spannerpb.SpannerClient, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	clt, err := e.getClientLocked(ctx)
	if err != nil {
		e.cancelContext(err)
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

func (e *Engine) getClientLocked(ctx context.Context) (spannerpb.SpannerClient, error) {
	// re-use the connection if we already connected.
	if e.gcloudClient != nil {
		return spannerpb.NewSpannerClient(e.gcloudClient), nil
	}

	tlsCfg, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get a token source for RPC auth. Assume (essentially require) that the
	// db username is the <name> portion of <name>@<project>.iam.gserviceaccount.com.
	// TODO(gavin): should we relax the naming requirements? Is there ever a
	// reason someone would want to use the full service account name, like for
	// a service account in a different project?
	dbUser := databaseUserToGCPServiceAccount(e.sessionCtx)
	ts, err := e.Auth.GetSpannerTokenSource(e.clientConnContext, e.sessionCtx.WithUser(dbUser))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The context passed to DialPool controls dialing, but more importantly it
	// controls GCP oauth2 credential refreshing.
	// We do not want credential refreshing to stop working after the first
	// oauth token expires (by default after one hour).
	// Therefore use the client connection context rather than the RPC context.
	cc, err := gtransport.DialPool(e.clientConnContext,
		append(spannerapi.DefaultClientOptions(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg))),
			option.WithGRPCDialOption(grpc.WithStatsHandler(&messageStatsHandler{
				messagesReceived: common.GetMessagesFromServerMetric(e.sessionCtx.Database),
			})),
			option.WithTokenSource(ts),
			option.WithEndpoint(e.sessionCtx.Database.GetURI()),
			// pool size seems to be adjusted to number of gRPC channels, which is
			// 4 by default in Google's code and seemingly other clients as well,
			// but with Teleport in the middle each downstream client connection
			// will be handled by a separate engine.
			// Therefore, avoid amplifying the number of connections dialed to
			// GCP and just use a pool size of 1.
			option.WithGRPCConnectionPool(1))...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	e.connSetupObserverFn()
	e.gcloudClient = cc
	return spannerpb.NewSpannerClient(e.gcloudClient), nil
}

// authorizeRPCRequest checks authorization for an RPC and handles audit
// logging events based on the result of that check.
//
// Spanner IDs look like this:
// project ID: projects/<project>
// instance ID: <project ID>/instances/<instance>
// database ID: <instance ID>/databases/<database>
// session ID: <database ID>/sessions/<session>
//
// All Spanner RPCs either include a database ID or a session ID.
// As outlined above, a session ID is prefixed by a database ID, and a database
// ID provides the target GCP project, instance, and database name.
// This function therefore expects a target ID that looks like
// `projects/<project>/instances/<instance>/databases/<database>[/sessions/<session]`
// and will extract the project, instance, and database name to check access.
func (e *Engine) authorizeRPCRequest(ctx context.Context, targetID, procedure string, req any) error {
	var (
		dbName        string
		authzErr      error
		checkedAccess bool
	)
	e.startSessionOnce.Do(func() {
		dbName, authzErr = e.checkAccess(ctx, targetID)
		checkedAccess = true
		sessionCtx := e.sessionCtx
		if dbName != "" {
			sessionCtx = sessionCtx.WithDatabase(dbName)
		}
		e.Audit.OnSessionStart(e.Context, sessionCtx, authzErr)
		e.sessionStartErr = authzErr
		e.sessionStarted = true
	})

	if !checkedAccess {
		dbName, authzErr = e.checkAccess(ctx, targetID)
	}
	if authzErr != nil {
		// start session shutdown if any RPC is denied by RBAC.
		e.cancelContext(authzErr)
	}

	if e.sessionStartErr != nil {
		// don't emit any other audit events after a failed session start,
		// and don't bother checking access again.
		e.engineErrors.Inc()
		return trace.Wrap(e.sessionStartErr)
	}

	args, err := rpcRequestToStruct(req)
	if err != nil {
		e.Log.WithError(err).Debug("failed to convert Spanner RPC args to audit event info")
		if authzErr == nil {
			// if there is no access error, but we can't fully audit what they
			// are doing, then report an access denied error in the audit log
			// and to the client caused by a bad request message.
			authzErr = trace.AccessDenied("access to db denied: %v", err)
		}
	}
	r := rpcInfo{
		database:  dbName,
		procedure: procedure,
		args:      args,
		err:       authzErr,
	}
	auditRPC(e.Context, e.Audit, e.sessionCtx, r)
	if authzErr != nil {
		e.engineErrors.Inc()
	}
	return trace.Wrap(authzErr)
}

func (e *Engine) checkAccess(ctx context.Context, targetID string) (string, error) {
	dbName, err := e.extractDatabaseNameFromID(targetID)
	if err != nil {
		return "", trace.AccessDenied("access to db denied: %v", err)
	}

	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return dbName, trace.Wrap(err)
	}
	state := e.sessionCtx.GetAccessState(authPref)
	dbRoleMatchers := role.GetDatabaseRoleMatchers(role.RoleMatchersConfig{
		Database:       e.sessionCtx.Database,
		DatabaseUser:   e.sessionCtx.DatabaseUser,
		DatabaseName:   dbName,
		AutoCreateUser: e.sessionCtx.AutoCreateUserMode.IsEnabled(),
	})
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		dbRoleMatchers...,
	)
	return dbName, trace.Wrap(err)
}

func (e *Engine) extractDatabaseNameFromID(id string) (string, error) {
	suffix, ok := strings.CutPrefix(id, e.instanceID)
	if !ok {
		return "", trace.BadParameter("database ID must start with %s", e.instanceID)
	}

	dbName, err := parseDatabaseName(suffix)
	return dbName, trace.Wrap(err)
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
		return "", trace.BadParameter("invalid database name")
	}
	return parts[database], nil
}

func databaseUserToGCPServiceAccount(sessionCtx *common.Session) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com",
		sessionCtx.DatabaseUser,
		sessionCtx.Database.GetGCP().ProjectID,
	)
}

// messageStatsHandler tracks messages received as a counter metric.
// Use it as a server option [grpc.Handler] to track messages received
// from a client.
// Use it as a dial option [grpc.WithHandler] to track messages received from a
// server.
type messageStatsHandler struct {
	// messagesReceived tracks RPC payloads received.
	messagesReceived prometheus.Counter
}

func (s *messageStatsHandler) HandleRPC(ctx context.Context, rpcStats stats.RPCStats) {
	if _, ok := rpcStats.(*stats.InPayload); ok {
		s.messagesReceived.Inc()
	}
}

// TagRPC is a no-op for the message stats handler.
func (s *messageStatsHandler) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	// no-op
	return ctx
}

// TagConn is a no-op for the message stats handler.
func (s *messageStatsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn is a no-op for the message stats handler.
func (s *messageStatsHandler) HandleConn(_ context.Context, _ stats.ConnStats) {}
