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
	"errors"
	"io"
	"net"
	"sync"

	"cloud.google.com/go/spanner/apiv1/spannerpb"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	gtransport "google.golang.org/api/transport/grpc"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/lib/defaults"
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
	// clientConnContext is the connection context passed into HandleConnection.
	// It's used to control the upstream connection (and credentials refresher)
	// lifetime, and canceled when the engine exits.
	clientConnContext context.Context
	// cancelContext is used to cancel clientConnContext.
	cancelContext context.CancelCauseFunc
	// sessionCtx is current session context.
	sessionCtx *common.Session
	// instanceID is used for authz and is of the form
	// projects/<project-id>/instances/<instance-id>/,
	// where the project ID and instance ID come from the Teleport database's
	// GCP metadata.
	// Every RPC that specifies a database name must be prefixed by this or the
	// RPC will be denied.
	instanceID string

	// mu guards access to gcloudClient
	mu sync.Mutex
	// gcloudConn is an upstream client to Google cloud API used to forward RPCs
	// to the real Spanner API.
	gcloudClient gtransport.ConnPool

	// sessionStarted is true after the session start event has been emitted.
	sessionStarted bool
	// sessionStartErr is the error (or nil) from the first RPC access check
	// that starts the session.
	sessionStartErr error
	// startSessionOnce is used to start the session only once.
	startSessionOnce sync.Once

	// connSetupObserverFn observes connection setup after the first authorized
	// RPC triggers a dial to GCP.
	connSetupObserverFn func()
	// engineErrors tracks synthetic errors sent by the engine to a client.
	// Since all access/auditing errors within the gRPC server must be handled
	// within the gRPC server, the usual engine.SendError mechanism is not
	// enough to track all errors sent to the client.
	engineErrors prometheus.Counter

	// UnimplementedSpannerServer is embedded for forward interface compat.
	spannerpb.UnimplementedSpannerServer
}

// InitializeConnection initializes the database connection.
func (e *Engine) InitializeConnection(clientConn net.Conn, sessionCtx *common.Session) error {
	e.clientConn = clientConn
	gcp := sessionCtx.Database.GetGCP()
	e.instanceID = "projects/" + gcp.ProjectID + "/instances/" + gcp.InstanceID + "/"
	e.sessionCtx = sessionCtx
	return nil
}

func (e *Engine) SendError(err error) {
	if err == nil || utils.IsOKNetworkError(err) {
		return
	}
	// the grpc server handles sending all errors, if an error is sent outside
	// of that, just log it here.
	e.Log.WithError(err).Debug("GCP Spanner connection error")
}

// HandleConnection processes the connection from the proxy coming over reverse
// tunnel.
func (e *Engine) HandleConnection(ctx context.Context, _ *common.Session) error {
	e.connSetupObserverFn = common.GetConnectionSetupTimeObserver(e.sessionCtx.Database)
	e.engineErrors = common.GetEngineErrorsMetric(e.sessionCtx.Database)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	e.clientConnContext = ctx
	e.cancelContext = cancel

	// stand up a gRPC grpcServer to handle the client connection.
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(e.unaryServerInterceptors()...),
		grpc.ChainStreamInterceptor(e.streamServerInterceptors()...),
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
		grpc.StatsHandler(&messageStatsHandler{
			messagesReceived: common.GetMessagesFromClientMetric(e.sessionCtx.Database),
		}),
	)
	// register server services to proxy RPCs.
	spannerpb.RegisterSpannerServer(grpcServer, e)

	// this doesn't block, because the listener returns when Accept is called
	// for a second time.
	err := grpcServer.Serve(newSingleUseListener(e.clientConn))
	if err != nil && !errors.Is(err, io.EOF) {
		return trace.Wrap(err)
	}
	select {
	case <-ctx.Done():
	case <-e.Context.Done():
	}

	// this will cause the server to reject all new RPCs and block until all
	// outstanding handlers have returned.
	grpcServer.GracefulStop()
	if e.gcloudClient != nil {
		e.gcloudClient.Close()
	}

	// emit a session end event if the session was started successfully.
	if e.sessionStarted && e.sessionStartErr == nil {
		e.Audit.OnSessionEnd(e.Context, e.sessionCtx)
	}
	return trace.Wrap(context.Cause(ctx))
}

func (e *Engine) unaryServerInterceptors() []grpc.UnaryServerInterceptor {
	// intercept and log some info, then convert errors to gRPC codes.
	return []grpc.UnaryServerInterceptor{
		interceptors.GRPCServerUnaryErrorInterceptor,
		unaryServerLoggingInterceptor(e.Log),
	}
}

func (e *Engine) streamServerInterceptors() []grpc.StreamServerInterceptor {
	// intercept and log some info, then convert errors to gRPC codes.
	return []grpc.StreamServerInterceptor{
		interceptors.GRPCServerStreamErrorInterceptor,
		streamServerLoggingInterceptor(e.Log),
	}
}
