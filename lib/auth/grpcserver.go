/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/proto"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
)

func init() {
	grpclog.SetLoggerV2(&GLogger{
		Entry: logrus.WithFields(
			logrus.Fields{
				trace.Component: teleport.Component(teleport.ComponentAuth, teleport.ComponentGRPC),
			}),
	})
}

// GRPCServer is GPRC Auth Server API
type GRPCServer struct {
	*logrus.Entry
	APIConfig
	// httpHandler is a server serving HTTP API
	httpHandler http.Handler
	// grpcHandler is golang GRPC handler
	grpcHandler *grpc.Server
}

// SendKeepAlives allows node to send a stream of keep alive requests
func (g *GRPCServer) SendKeepAlives(stream proto.AuthService_SendKeepAlivesServer) error {
	defer stream.SendAndClose(&empty.Empty{})
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}
	g.Debugf("Got heartbeat connection from %v.", auth.User.GetName())
	for {
		keepAlive, err := stream.Recv()
		if err == io.EOF {
			g.Debugf("Connection closed.")
			return nil
		}
		if err != nil {
			g.Debugf("Failed to receive heartbeat: %v", err)
			return trail.ToGRPC(err)
		}
		err = auth.KeepAliveNode(stream.Context(), *keepAlive)
		if err != nil {
			return trail.ToGRPC(err)
		}
	}
}

// WatchEvents returns a new stream of cluster events
func (g *GRPCServer) WatchEvents(watch *proto.Watch, stream proto.AuthService_WatchEventsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}
	watcher, err := auth.NewWatcher(stream.Context(), services.Watch{Kinds: watch.Kinds})
	if err != nil {
		return trail.ToGRPC(err)
	}
	defer watcher.Close()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event := <-watcher.Events():
			out, err := eventToGRPC(event)
			if err != nil {
				return trail.ToGRPC(err)
			}
			if err := stream.Send(out); err != nil {
				return trail.ToGRPC(err)
			}
		}
	}
}

// UpsertNode upserts node
func (g *GRPCServer) UpsertNode(ctx context.Context, server *services.ServerV2) (*services.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	keepAlive, err := auth.UpsertNode(server)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return keepAlive, nil
}

type grpcContext struct {
	*AuthContext
	*AuthWithRoles
}

// authenticate extracts authentication context and returns initialized auth server
func (g *GRPCServer) authenticate(ctx context.Context) (*grpcContext, error) {
	// HTTPS server expects auth  context to be set by the auth middleware
	authContext, err := g.Authorizer.Authorize(ctx)
	if err != nil {
		// propagate connection problem error so we can differentiate
		// between connection failed and access denied
		if trace.IsConnectionProblem(err) {
			return nil, trace.ConnectionProblem(err, "[10] failed to connect to the database")
		} else if trace.IsAccessDenied(err) {
			// don't print stack trace, just log the warning
			log.Warn(err)
		} else {
			log.Warn(trace.DebugReport(err))
		}
		return nil, trace.AccessDenied("[10] access denied")
	}
	return &grpcContext{
		AuthContext: authContext,
		AuthWithRoles: &AuthWithRoles{
			authServer: g.AuthServer,
			user:       authContext.User,
			checker:    authContext.Checker,
			sessions:   g.SessionService,
			alog:       g.AuthServer.IAuditLog,
		},
	}, nil
}

// NewGRPCServer returns a new instance of GRPC server
func NewGRPCServer(cfg APIConfig) http.Handler {
	authServer := &GRPCServer{
		APIConfig: cfg,
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentAuth, teleport.ComponentGRPC),
		}),
		httpHandler: NewAPIServer(&cfg),
		grpcHandler: grpc.NewServer(),
	}
	proto.RegisterAuthServiceServer(authServer.grpcHandler, authServer)
	return authServer
}

// ServeHTTP dispatches requests based on the request type
func (g *GRPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// magic combo match signifying GRPC request
	// https://grpc.io/blog/coreos
	if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
		g.grpcHandler.ServeHTTP(w, r)
	} else {
		g.httpHandler.ServeHTTP(w, r)
	}
}

func eventToGRPC(in services.Event) (*proto.Event, error) {
	out := proto.Event{
		Type: eventTypeToGRPC(in.Type),
	}
	switch r := in.Resource.(type) {
	case *services.ResourceHeader:
		out.Resource = &proto.Event_ResourceHeader{
			ResourceHeader: r,
		}
	case *services.CertAuthorityV2:
		out.Resource = &proto.Event_CertAuthority{
			CertAuthority: r,
		}
	default:
		return nil, trace.BadParameter("resource type %T is not supported", in.Resource)
	}
	return &out, nil
}

func eventTypeToGRPC(in backend.OpType) proto.Operation {
	if in == backend.OpPut {
		return proto.Operation_PUT
	}
	return proto.Operation_DELETE
}

func eventFromGRPC(in proto.Event) (*services.Event, error) {
	out := services.Event{
		Type: eventTypeFromGRPC(in.Type),
	}
	if r := in.GetResourceHeader(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetCertAuthority(); r != nil {
		out.Resource = r
		return &out, nil
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

func eventTypeFromGRPC(in proto.Operation) backend.OpType {
	if in == proto.Operation_PUT {
		return backend.OpPut
	}
	return backend.OpDelete
}
