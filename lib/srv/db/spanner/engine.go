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
	"fmt"
	"net"

	vkit "cloud.google.com/go/spanner/apiv1"
	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/gravitational/trace"
	"google.golang.org/api/option"
	gtransport "google.golang.org/api/transport/grpc"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

// NewEngine creates a new Spanner engine.
func NewEngine(ec common.EngineConfig) common.Engine {
	return &Engine{
		EngineConfig: ec,
	}
}

// Engine implements common.Engine.
type Engine struct {
	// EngineConfig is the common database engine configuration.
	common.EngineConfig
	// clientConn is a client connection.
	clientConn net.Conn
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// cloudClient is used to forward RPCs upstream to Cloud Spanner.
	cloudClient spannerpb.SpannerClient
	// instanceName is used for authz and is of the form
	// projects/<project-id>/instances/<instance-id>/,
	// where the project ID and instance ID come from the Teleport database's
	// GCP metadata.
	// Every RPC that specifies a database name must be prefixed by this or the
	// RPC will be denied.
	instanceName string
	// UnimplementedSpannerServer is embedded for forward interface compat.
	spannerpb.UnimplementedSpannerServer
}

func (e *Engine) SendError(sErr error) {
	if sErr == nil || utils.IsOKNetworkError(sErr) {
		return
	}
	e.Log.WithError(sErr).Error("GCP Spanner connection error", sErr)
	// error will be sent by the gRPC server to clients.
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	// database name will be checked for each RPC - db_name in the cert db
	// routing info should just be ignored.
	e.sessionCtx = sessionCtx.WithDatabase("")
	gcp := sessionCtx.Database.GetGCP()
	e.instanceName = "projects/" + gcp.ProjectID + "/instances/" + gcp.InstanceID + "/"
	return nil
}

// TODO(gavin): I ripped this out of MySQL, should probably put it in a
// common lib somewhere.
func databaseUserToGCPServiceAccount(sessionCtx *common.Session) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", sessionCtx.DatabaseUser, sessionCtx.Database.GetGCP().ProjectID)
}

func (e *Engine) connect(ctx context.Context) error {
	// get a token source for RPC auth.
	dbUser := databaseUserToGCPServiceAccount(e.sessionCtx)
	ts, err := e.Auth.GetSpannerTokenSource(ctx, e.sessionCtx.WithUser(dbUser))
	if err != nil {
		return trace.Wrap(err)
	}
	opts := append(vkit.DefaultClientOptions(), option.WithTokenSource(ts))
	conn, err := gtransport.Dial(ctx, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	e.cloudClient = spannerpb.NewSpannerClient(conn)
	return nil
}

// checkAccess does authorization check for Spanner connection about
// to be established.
func (e *Engine) checkAccess(ctx context.Context) error {
	authPref, err := e.Auth.GetAuthPreference(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	state := e.sessionCtx.GetAccessState(authPref)
	err = e.sessionCtx.Checker.CheckAccess(
		e.sessionCtx.Database,
		state,
		// Only the username is checked upon initial connection. Spanner sends
		// database name with each RPC, so we check before authorizing each
		// RPC from the client instead.
		services.NewDatabaseUserMatcher(e.sessionCtx.Database, e.sessionCtx.DatabaseUser),
	)
	if err != nil {
		e.Audit.OnSessionStart(e.Context, e.sessionCtx, err)
		return trace.Wrap(err)
	}
	return nil
}

// HandleConnection processes the connection from the proxy coming over reverse
// tunnel.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	// Check that the user has access to the database.
	err := e.checkAccess(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Establish connection to Cloud Spanner.
	if err := e.connect(ctx); err != nil {
		return trace.Wrap(err)
	}

	// stand up a gRPC grpcServer to handle the client connection.
	grpcServer, err := e.newGRPCServer()
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO(gavin): consider using .Stop() here instead.
	// We don't want pending RPCs to prevent a db agent shutdown.
	defer grpcServer.GracefulStop()

	// register server services to proxy RPCs.
	spannerpb.RegisterSpannerServer(grpcServer, e)

	e.Audit.OnSessionStart(e.Context, e.sessionCtx, nil)
	defer e.Audit.OnSessionEnd(e.Context, e.sessionCtx)

	// when ctx is Done, the fake listener will close and the gRPC server will
	// exit.
	// observe connection setup time when the fake listener returns the accepted
	// connection to the gRPC server connection loop (only happens once).
	l := newFakeListener(e.Context, ctx, e.clientConn)
	defer l.Close()

	err = grpcServer.Serve(l)
	return trace.Wrap(err)
}
