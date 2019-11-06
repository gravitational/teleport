/*
Copyright 2018-2019 Gravitational, Inc.

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
	"github.com/gravitational/teleport/lib/utils"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

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
	servicesWatch := services.Watch{
		Name: auth.User.GetName(),
	}
	for _, kind := range watch.Kinds {
		servicesWatch.Kinds = append(servicesWatch.Kinds, services.WatchKind{
			Name:        kind.Name,
			Kind:        kind.Kind,
			LoadSecrets: kind.LoadSecrets,
		})
	}
	watcher, err := auth.NewWatcher(stream.Context(), servicesWatch)
	if err != nil {
		return trail.ToGRPC(err)
	}
	defer watcher.Close()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-watcher.Done():
			return trail.ToGRPC(watcher.Error())
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

	// Extract peer (remote host) from context and if the server sent 0.0.0.0 as
	// its address (meaning it did not set an advertise address) update it with
	// the address of the peer.
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trail.ToGRPC(trace.BadParameter("unable to find peer"))
	}
	server.SetAddr(utils.ReplaceLocalhost(server.GetAddr(), p.Addr.String()))

	keepAlive, err := auth.UpsertNode(server)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return keepAlive, nil
}

func (g *GRPCServer) GenerateUserCerts(ctx context.Context, req *proto.UserCertsRequest) (*proto.Certs, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	certs, err := auth.AuthWithRoles.GenerateUserCerts(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return certs, err
}

func (g *GRPCServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*services.UserV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	user, err := auth.AuthWithRoles.GetUser(req.Name, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	v2, ok := user.(*services.UserV2)
	if !ok {
		log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected user type"))
	}
	return v2, nil
}

func (g *GRPCServer) GetUsers(req *proto.GetUsersRequest, stream proto.AuthService_GetUsersServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}
	users, err := auth.AuthWithRoles.GetUsers(req.WithSecrets)
	if err != nil {
		return trail.ToGRPC(err)
	}
	for _, user := range users {
		v2, ok := user.(*services.UserV2)
		if !ok {
			log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
			return trail.ToGRPC(trace.Errorf("encountered unexpected user type"))
		}
		if err := stream.Send(v2); err != nil {
			return trail.ToGRPC(err)
		}
	}
	return nil
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
			identity:   authContext.Identity,
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
	eventType, err := eventTypeToGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := proto.Event{
		Type: eventType,
	}
	if in.Type == backend.OpInit {
		return &out, nil
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
	case *services.StaticTokensV2:
		out.Resource = &proto.Event_StaticTokens{
			StaticTokens: r,
		}
	case *services.ProvisionTokenV2:
		out.Resource = &proto.Event_ProvisionToken{
			ProvisionToken: r,
		}
	case *services.ClusterNameV2:
		out.Resource = &proto.Event_ClusterName{
			ClusterName: r,
		}
	case *services.ClusterConfigV3:
		out.Resource = &proto.Event_ClusterConfig{
			ClusterConfig: r,
		}
	case *services.UserV2:
		out.Resource = &proto.Event_User{
			User: r,
		}
	case *services.RoleV3:
		out.Resource = &proto.Event_Role{
			Role: r,
		}
	case *services.Namespace:
		out.Resource = &proto.Event_Namespace{
			Namespace: r,
		}
	case *services.ServerV2:
		out.Resource = &proto.Event_Server{
			Server: r,
		}
	case *services.ReverseTunnelV2:
		out.Resource = &proto.Event_ReverseTunnel{
			ReverseTunnel: r,
		}
	case *services.TunnelConnectionV2:
		out.Resource = &proto.Event_TunnelConnection{
			TunnelConnection: r,
		}
	default:
		return nil, trace.BadParameter("resource type %T is not supported", in.Resource)
	}
	return &out, nil
}

func eventTypeToGRPC(in backend.OpType) (proto.Operation, error) {
	switch in {
	case backend.OpInit:
		return proto.Operation_INIT, nil
	case backend.OpPut:
		return proto.Operation_PUT, nil
	case backend.OpDelete:
		return proto.Operation_DELETE, nil
	default:
		return -1, trace.BadParameter("event type %v is not supported", in)
	}
}

func eventFromGRPC(in proto.Event) (*services.Event, error) {
	eventType, err := eventTypeFromGRPC(in.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := services.Event{
		Type: eventType,
	}
	if eventType == backend.OpInit {
		return &out, nil
	}
	if r := in.GetResourceHeader(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetCertAuthority(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetStaticTokens(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetProvisionToken(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterName(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetClusterConfig(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetUser(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetRole(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetNamespace(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetServer(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetReverseTunnel(); r != nil {
		out.Resource = r
		return &out, nil
	} else if r := in.GetTunnelConnection(); r != nil {
		out.Resource = r
		return &out, nil
	} else {
		return nil, trace.BadParameter("received unsupported resource %T", in.Resource)
	}
}

func eventTypeFromGRPC(in proto.Operation) (backend.OpType, error) {
	switch in {
	case proto.Operation_INIT:
		return backend.OpInit, nil
	case proto.Operation_PUT:
		return backend.OpPut, nil
	case proto.Operation_DELETE:
		return backend.OpDelete, nil
	default:
		return -1, trace.BadParameter("unsupported operation type: %v", in)
	}
}
