/*
Copyright 2018-2020 Gravitational, Inc.

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
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	// Register gzip compressor for gRPC.
	_ "google.golang.org/grpc/encoding/gzip"
)

// GRPCServer is GPRC Auth Server API
type GRPCServer struct {
	*logrus.Entry
	APIConfig
	server *grpc.Server
}

// EmitAuditEvent emits audit event
func (g *GRPCServer) EmitAuditEvent(ctx context.Context, req *events.OneOf) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	event, err := events.FromOneOf(*req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	err = auth.EmitAuditEvent(ctx, event)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
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
		err = auth.KeepAliveServer(stream.Context(), *keepAlive)
		if err != nil {
			return trail.ToGRPC(err)
		}
	}
}

// CreateAuditStream creates or resumes audit event stream
func (g *GRPCServer) CreateAuditStream(stream proto.AuthService_CreateAuditStreamServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}

	var eventStream events.Stream
	g.Debugf("CreateAuditStream connection from %v.", auth.User.GetName())
	streamStart := time.Now()
	processed := int64(0)
	counter := 0
	forwardEvents := func(eventStream events.Stream) {
		for {
			select {
			case <-stream.Context().Done():
				return
			case statusUpdate := <-eventStream.Status():
				if err := stream.Send(&statusUpdate); err != nil {
					g.WithError(err).Debugf("Failed to send status update.")
				}
			}
		}
	}

	closeStream := func(eventStream events.Stream) {
		if err := eventStream.Close(auth.CloseContext()); err != nil {
			g.WithError(err).Warningf("Failed to flush close the stream.")
		} else {
			g.Debugf("Flushed and closed the stream.")
		}
	}

	for {
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			g.WithError(err).Debugf("Failed to receive stream request.")
			return trail.ToGRPC(err)
		}
		if create := request.GetCreateStream(); create != nil {
			if eventStream != nil {
				return trail.ToGRPC(trace.BadParameter("stream is already created or resumed"))
			}
			eventStream, err = auth.CreateAuditStream(stream.Context(), session.ID(create.SessionID))
			if err != nil {
				return trace.Wrap(err)
			}
			g.Debugf("Created stream: %v.", err)
			go forwardEvents(eventStream)
			defer closeStream(eventStream)
		} else if resume := request.GetResumeStream(); resume != nil {
			if eventStream != nil {
				return trail.ToGRPC(trace.BadParameter("stream is already created or resumed"))
			}
			eventStream, err = auth.ResumeAuditStream(stream.Context(), session.ID(resume.SessionID), resume.UploadID)
			if err != nil {
				return trace.Wrap(err)
			}
			g.Debugf("Resumed stream: %v.", err)
			go forwardEvents(eventStream)
			defer closeStream(eventStream)
		} else if complete := request.GetCompleteStream(); complete != nil {
			if eventStream == nil {
				return trail.ToGRPC(trace.BadParameter("stream is not initialized yet, cannot complete"))
			}
			// do not use stream context to give the auth server finish the upload
			// even if the stream's context is cancelled
			err := eventStream.Complete(auth.CloseContext())
			g.Debugf("Completed stream: %v.", err)
			if err != nil {
				return trail.ToGRPC(err)
			}
			return nil
		} else if flushAndClose := request.GetFlushAndCloseStream(); flushAndClose != nil {
			if eventStream == nil {
				return trail.ToGRPC(trace.BadParameter("stream is not initialized yet, cannot flush and close"))
			}
			// flush and close is always done
			return nil
		} else if oneof := request.GetEvent(); oneof != nil {
			if eventStream == nil {
				return trail.ToGRPC(
					trace.BadParameter("stream cannot receive an event without first being created or resumed"))
			}
			event, err := events.FromOneOf(*oneof)
			if err != nil {
				g.WithError(err).Debugf("Failed to decode event.")
				return trail.ToGRPC(err)
			}
			start := time.Now()
			err = eventStream.EmitAuditEvent(stream.Context(), event)
			if err != nil {
				return trail.ToGRPC(err)
			}
			event.Size()
			processed += int64(event.Size())
			seconds := time.Since(streamStart) / time.Second
			counter++
			if counter%logInterval == 0 {
				if seconds > 0 {
					kbytes := float64(processed) / 1000
					g.Debugf("Processed %v events, tx rate kbytes %v/second.", counter, kbytes/float64(seconds))
				}
			}
			diff := time.Since(start)
			if diff > 100*time.Millisecond {
				log.Warningf("EmitAuditEvent(%v) took longer than 100ms: %v", event.GetType(), time.Since(event.GetTime()))
			}
		} else {
			g.Errorf("Rejecting unsupported stream request: %v.", request)
			return trail.ToGRPC(trace.BadParameter("unsupported stream request"))
		}
	}
}

// logInterval is used to log stats after this many events
const logInterval = 10000

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
			Filter:      kind.Filter,
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
			out, err := client.EventToGRPC(event)
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
	certs, err := auth.ServerWithRoles.GenerateUserCerts(ctx, *req)
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
	user, err := auth.ServerWithRoles.GetUser(req.Name, req.WithSecrets)
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
	users, err := auth.ServerWithRoles.GetUsers(req.WithSecrets)
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

func (g *GRPCServer) GetAccessRequests(ctx context.Context, f *services.AccessRequestFilter) (*proto.AccessRequests, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	var filter services.AccessRequestFilter
	if f != nil {
		filter = *f
	}
	reqs, err := auth.ServerWithRoles.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	collector := make([]*services.AccessRequestV3, 0, len(reqs))
	for _, req := range reqs {
		r, ok := req.(*services.AccessRequestV3)
		if !ok {
			err = trace.BadParameter("unexpected access request type %T", req)
			return nil, trail.ToGRPC(err)
		}
		collector = append(collector, r)
	}
	return &proto.AccessRequests{
		AccessRequests: collector,
	}, nil
}

func (g *GRPCServer) CreateAccessRequest(ctx context.Context, req *services.AccessRequestV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.CreateAccessRequest(ctx, req); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) DeleteAccessRequest(ctx context.Context, id *proto.RequestID) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteAccessRequest(ctx, id.ID); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) SetAccessRequestState(ctx context.Context, req *proto.RequestStateSetter) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if req.Delegator != "" {
		ctx = WithDelegator(ctx, req.Delegator)
	}
	if err := auth.ServerWithRoles.SetAccessRequestState(ctx, services.AccessRequestUpdate{
		RequestID:   req.ID,
		State:       req.State,
		Reason:      req.Reason,
		Annotations: req.Annotations,
		Roles:       req.Roles,
	}); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) GetAccessCapabilities(ctx context.Context, req *services.AccessCapabilitiesRequest) (*services.AccessCapabilities, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	caps, err := auth.ServerWithRoles.GetAccessCapabilities(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return caps, nil
}

func (g *GRPCServer) CreateResetPasswordToken(ctx context.Context, req *proto.CreateResetPasswordTokenRequest) (*services.ResetPasswordTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if req == nil {
		req = &proto.CreateResetPasswordTokenRequest{}
	}

	token, err := auth.CreateResetPasswordToken(ctx, CreateResetPasswordTokenRequest{
		Name: req.Name,
		TTL:  time.Duration(req.TTL),
		Type: req.Type,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := token.(*services.ResetPasswordTokenV3)
	if !ok {
		err = trace.BadParameter("unexpected ResetPasswordToken type %T", token)
		return nil, trail.ToGRPC(err)
	}

	return r, nil
}

func (g *GRPCServer) RotateResetPasswordTokenSecrets(ctx context.Context, req *proto.RotateResetPasswordTokenSecretsRequest) (*services.ResetPasswordTokenSecretsV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	tokenID := ""
	if req != nil {
		tokenID = req.TokenID
	}

	secrets, err := auth.RotateResetPasswordTokenSecrets(ctx, tokenID)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	r, ok := secrets.(*services.ResetPasswordTokenSecretsV3)
	if !ok {
		err = trace.BadParameter("unexpected ResetPasswordTokenSecrets type %T", secrets)
		return nil, trail.ToGRPC(err)
	}

	return r, nil
}

func (g *GRPCServer) GetResetPasswordToken(ctx context.Context, req *proto.GetResetPasswordTokenRequest) (*services.ResetPasswordTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	tokenID := ""
	if req != nil {
		tokenID = req.TokenID
	}

	token, err := auth.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	r, ok := token.(*services.ResetPasswordTokenV3)
	if !ok {
		err = trace.BadParameter("unexpected ResetPasswordToken type %T", token)
		return nil, trail.ToGRPC(err)
	}

	return r, nil
}

// GetPluginData loads all plugin data matching the supplied filter.
func (g *GRPCServer) GetPluginData(ctx context.Context, filter *services.PluginDataFilter) (*proto.PluginDataSeq, error) {
	// TODO(fspmarshall): Implement rate-limiting to prevent misbehaving plugins from
	// consuming too many server resources.
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	data, err := auth.ServerWithRoles.GetPluginData(ctx, *filter)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	var seq []*services.PluginDataV3
	for _, rsc := range data {
		d, ok := rsc.(*services.PluginDataV3)
		if !ok {
			err = trace.BadParameter("unexpected plugin data type %T", rsc)
			return nil, trail.ToGRPC(err)
		}
		seq = append(seq, d)
	}
	return &proto.PluginDataSeq{
		PluginData: seq,
	}, nil
}

// UpdatePluginData updates a per-resource PluginData entry.
func (g *GRPCServer) UpdatePluginData(ctx context.Context, params *services.PluginDataUpdateParams) (*empty.Empty, error) {
	// TODO(fspmarshall): Implement rate-limiting to prevent misbehaving plugins from
	// consuming too many server resources.
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.UpdatePluginData(ctx, *params); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	rsp, err := auth.Ping(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &rsp, nil
}

// CreateUser inserts a new user entry in a backend.
func (g *GRPCServer) CreateUser(ctx context.Context, req *services.UserV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.CreateUser(ctx, req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	log.Infof("%q user created", req.GetName())

	return &empty.Empty{}, nil
}

// UpdateUser updates an existing user in a backend.
func (g *GRPCServer) UpdateUser(ctx context.Context, req *services.UserV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.UpdateUser(ctx, req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	log.Infof("%q user updated", req.GetName())

	return &empty.Empty{}, nil
}

// DeleteUser deletes an existng user in a backend by username.
func (g *GRPCServer) DeleteUser(ctx context.Context, req *proto.DeleteUserRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.DeleteUser(ctx, req.GetName()); err != nil {
		return nil, trail.ToGRPC(err)
	}

	log.Infof("%q user deleted", req.GetName())

	return &empty.Empty{}, nil
}

// AcquireSemaphore acquires lease with requested resources from semaphore.
func (g *GRPCServer) AcquireSemaphore(ctx context.Context, params *services.AcquireSemaphoreRequest) (*services.SemaphoreLease, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	lease, err := auth.AcquireSemaphore(ctx, *params)
	return lease, trail.ToGRPC(err)
}

// KeepAliveSemaphoreLease updates semaphore lease.
func (g *GRPCServer) KeepAliveSemaphoreLease(ctx context.Context, req *services.SemaphoreLease) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.KeepAliveSemaphoreLease(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// CancelSemaphoreLease cancels semaphore lease early.
func (g *GRPCServer) CancelSemaphoreLease(ctx context.Context, req *services.SemaphoreLease) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.CancelSemaphoreLease(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (g *GRPCServer) GetSemaphores(ctx context.Context, req *services.SemaphoreFilter) (*proto.Semaphores, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	semaphores, err := auth.GetSemaphores(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	ss := make([]*services.SemaphoreV3, 0, len(semaphores))
	for _, sem := range semaphores {
		s, ok := sem.(*services.SemaphoreV3)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected semaphore type: %T", sem))
		}
		ss = append(ss, s)
	}
	return &proto.Semaphores{
		Semaphores: ss,
	}, nil
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (g *GRPCServer) DeleteSemaphore(ctx context.Context, req *services.SemaphoreFilter) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.DeleteSemaphore(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetDatabaseServers returns all registered database proxy servers.
func (g *GRPCServer) GetDatabaseServers(ctx context.Context, req *proto.GetDatabaseServersRequest) (*proto.GetDatabaseServersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	var opts []types.MarshalOption
	if req.GetSkipValidation() {
		opts = append(opts, services.SkipValidation())
	}
	databaseServers, err := auth.GetDatabaseServers(ctx, req.GetNamespace(), opts...)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	var servers []*types.DatabaseServerV3
	for _, s := range databaseServers {
		server, ok := s.(*types.DatabaseServerV3)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", s))
		}
		servers = append(servers, server)
	}
	return &proto.GetDatabaseServersResponse{
		Servers: servers,
	}, nil
}

// UpsertDatabaseServer registers a new database proxy server.
func (g *GRPCServer) UpsertDatabaseServer(ctx context.Context, req *proto.UpsertDatabaseServerRequest) (*services.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	keepAlive, err := auth.UpsertDatabaseServer(ctx, req.GetServer())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return keepAlive, nil
}

// DeleteDatabaseServer removes the specified database proxy server.
func (g *GRPCServer) DeleteDatabaseServer(ctx context.Context, req *proto.DeleteDatabaseServerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	err = auth.DeleteDatabaseServer(ctx, req.GetNamespace(), req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (g *GRPCServer) DeleteAllDatabaseServers(ctx context.Context, req *proto.DeleteAllDatabaseServersRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	err = auth.DeleteAllDatabaseServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (g *GRPCServer) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	response, err := auth.SignDatabaseCSR(ctx, req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return response, nil
}

// GenerateDatabaseCert generates client certificate used by a database
// service to authenticate with the database instance.
func (g *GRPCServer) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	response, err := auth.GenerateDatabaseCert(ctx, req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return response, nil
}

// GetAppServers gets all application servers.
func (g *GRPCServer) GetAppServers(ctx context.Context, req *proto.GetAppServersRequest) (*proto.GetAppServersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var opts []services.MarshalOption
	if req.GetSkipValidation() {
		opts = append(opts, services.SkipValidation())
	}

	appServers, err := auth.GetAppServers(ctx, req.GetNamespace(), opts...)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var servers []*services.ServerV2
	for _, s := range appServers {
		server, ok := s.(*services.ServerV2)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", s))
		}
		servers = append(servers, server)
	}

	return &proto.GetAppServersResponse{
		Servers: servers,
	}, nil
}

// UpsertAppServer adds an application server.
func (g *GRPCServer) UpsertAppServer(ctx context.Context, req *proto.UpsertAppServerRequest) (*services.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	keepAlive, err := auth.UpsertAppServer(ctx, req.GetServer())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return keepAlive, nil
}

// DeleteAppServer removes an application server.
func (g *GRPCServer) DeleteAppServer(ctx context.Context, req *proto.DeleteAppServerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	err = auth.DeleteAppServer(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllAppServers removes all application servers.
func (g *GRPCServer) DeleteAllAppServers(ctx context.Context, req *proto.DeleteAllAppServersRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	err = auth.DeleteAllAppServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// GetAppSession gets an application web session.
func (g *GRPCServer) GetAppSession(ctx context.Context, req *proto.GetAppSessionRequest) (*proto.GetAppSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	session, err := auth.GetAppSession(ctx, services.GetAppSessionRequest{
		SessionID: req.GetSessionID(),
	})
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	sess, ok := session.(*services.WebSessionV2)
	if !ok {
		return nil, trail.ToGRPC(trace.BadParameter("unexpected session type %T", session))
	}

	return &proto.GetAppSessionResponse{
		Session: sess,
	}, nil
}

// GetAppSessions gets all application web sessions.
func (g *GRPCServer) GetAppSessions(ctx context.Context, _ *empty.Empty) (*proto.GetAppSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	sessions, err := auth.GetAppSessions(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var out []*services.WebSessionV2
	for _, session := range sessions {
		sess, ok := session.(*services.WebSessionV2)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", session))
		}
		out = append(out, sess)
	}

	return &proto.GetAppSessionsResponse{
		Sessions: out,
	}, nil
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (g *GRPCServer) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest) (*proto.CreateAppSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	session, err := auth.CreateAppSession(ctx, services.CreateAppSessionRequest{
		Username:      req.GetUsername(),
		ParentSession: req.GetParentSession(),
		PublicAddr:    req.GetPublicAddr(),
		ClusterName:   req.GetClusterName(),
	})
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	sess, ok := session.(*services.WebSessionV2)
	if !ok {
		return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", session))
	}

	return &proto.CreateAppSessionResponse{
		Session: sess,
	}, nil
}

// DeleteAppSession removes an application web session.
func (g *GRPCServer) DeleteAppSession(ctx context.Context, req *proto.DeleteAppSessionRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.DeleteAppSession(ctx, services.DeleteAppSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllAppSessions removes all application web sessions.
func (g *GRPCServer) DeleteAllAppSessions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.DeleteAllAppSessions(ctx); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// GenerateAppToken creates a JWT token with application access.
func (g GRPCServer) GenerateAppToken(ctx context.Context, req *proto.GenerateAppTokenRequest) (*proto.GenerateAppTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	token, err := auth.GenerateAppToken(ctx, jwt.GenerateAppTokenRequest{
		Username: req.Username,
		Roles:    req.Roles,
		URI:      req.URI,
		Expires:  req.Expires,
	})
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &proto.GenerateAppTokenResponse{
		Token: token,
	}, nil
}

// GetWebSession gets a web session.
func (g *GRPCServer) GetWebSession(ctx context.Context, req *proto.GetWebSessionRequest) (*proto.GetWebSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	session, err := auth.WebSessions().Get(ctx, services.GetWebSessionRequest{
		SessionID: req.GetSessionID(),
	})
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	sess, ok := session.(*services.WebSessionV2)
	if !ok {
		return nil, trail.ToGRPC(trace.BadParameter("unexpected session type %T", session))
	}

	return &proto.GetWebSessionResponse{
		Session: sess,
	}, nil
}

// GetWebSessions gets all web sessions.
func (g *GRPCServer) GetWebSessions(ctx context.Context, _ *empty.Empty) (*proto.GetWebSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	sessions, err := auth.WebSessions().List(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var out []*services.WebSessionV2
	for _, session := range sessions {
		sess, ok := session.(*services.WebSessionV2)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", session))
		}
		out = append(out, sess)
	}

	return &proto.GetWebSessionsResponse{
		Sessions: out,
	}, nil
}

// DeleteWebSession removes an application web session.
func (g *GRPCServer) DeleteWebSession(ctx context.Context, req *proto.DeleteWebSessionRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.WebSessions().Delete(ctx, services.DeleteWebSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllWebSessions removes all web sessions.
func (g *GRPCServer) DeleteAllWebSessions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.WebSessions().DeleteAll(ctx); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// UpdateRemoteCluster updates remote cluster
func (g *GRPCServer) UpdateRemoteCluster(ctx context.Context, req *services.RemoteClusterV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.UpdateRemoteCluster(ctx, req); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetKubeServices gets all kubernetes services.
func (g *GRPCServer) GetKubeServices(ctx context.Context, req *proto.GetKubeServicesRequest) (*proto.GetKubeServicesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	kubeServices, err := auth.GetKubeServices(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var servers []*services.ServerV2
	for _, s := range kubeServices {
		server, ok := s.(*services.ServerV2)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", s))
		}
		servers = append(servers, server)
	}

	return &proto.GetKubeServicesResponse{
		Servers: servers,
	}, nil
}

// UpsertKubeService adds a kubernetes service.
func (g *GRPCServer) UpsertKubeService(ctx context.Context, req *proto.UpsertKubeServiceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	server := req.GetServer()
	// If Addr in the server is localhost, replace it with the address we see
	// from our end.
	//
	// Services that listen on "0.0.0.0:12345" will put that exact address in
	// the server.Addr field. It's not useful for other services that want to
	// connect to it (like a proxy). Remote address of the gRPC connection is
	// the closest thing we have to a public IP for the service.
	clientAddr, ok := ctx.Value(ContextClientAddr).(net.Addr)
	if !ok {
		return nil, status.Errorf(codes.FailedPrecondition, "bug: client address not found in request context")
	}
	server.SetAddr(utils.ReplaceLocalhost(server.GetAddr(), clientAddr.String()))

	if err := auth.UpsertKubeService(ctx, server); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return new(empty.Empty), nil
}

// DeleteKubeService removes a kubernetes service.
func (g *GRPCServer) DeleteKubeService(ctx context.Context, req *proto.DeleteKubeServiceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	err = auth.DeleteKubeService(ctx, req.GetName())
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllKubeServices removes all kubernetes services.
func (g *GRPCServer) DeleteAllKubeServices(ctx context.Context, req *proto.DeleteAllKubeServicesRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	err = auth.DeleteAllKubeServices(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

type grpcContext struct {
	*Context
	*ServerWithRoles
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
		Context: authContext,
		ServerWithRoles: &ServerWithRoles{
			authServer: g.AuthServer,
			context:    *authContext,
			sessions:   g.SessionService,
			alog:       g.AuthServer.IAuditLog,
		},
	}, nil
}

// GRPCServerConfig specifies GRPC server configuration
type GRPCServerConfig struct {
	// APIConfig is GRPC server API configuration
	APIConfig
	// TLS is GRPC server config
	TLS *tls.Config
	// UnaryInterceptor intercepts individual GRPC requests
	// for authentication and rate limiting
	UnaryInterceptor grpc.UnaryServerInterceptor
	// UnaryInterceptor intercepts GRPC streams
	// for authentication and rate limiting
	StreamInterceptor grpc.StreamServerInterceptor
}

// CheckAndSetDefaults checks and sets default values
func (cfg *GRPCServerConfig) CheckAndSetDefaults() error {
	if cfg.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if cfg.UnaryInterceptor == nil {
		return trace.BadParameter("missing parameter UnaryInterceptor")
	}
	if cfg.StreamInterceptor == nil {
		return trace.BadParameter("missing parameter StreamInterceptor")
	}
	return nil
}

// NewGRPCServer returns a new instance of GRPC server
func NewGRPCServer(cfg GRPCServerConfig) (*GRPCServer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("GRPC(SERVER): keep alive %v count: %v.", cfg.KeepAlivePeriod, cfg.KeepAliveCount)
	opts := []grpc.ServerOption{
		grpc.Creds(&httplib.TLSCreds{
			Config: cfg.TLS,
		}),
		grpc.UnaryInterceptor(cfg.UnaryInterceptor),
		grpc.StreamInterceptor(cfg.StreamInterceptor),
		grpc.KeepaliveParams(
			keepalive.ServerParameters{
				Time:    cfg.KeepAlivePeriod,
				Timeout: cfg.KeepAlivePeriod * time.Duration(cfg.KeepAliveCount),
			},
		),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime:             cfg.KeepAlivePeriod,
				PermitWithoutStream: true,
			},
		),
	}
	server := grpc.NewServer(opts...)
	authServer := &GRPCServer{
		APIConfig: cfg.APIConfig,
		Entry: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.Component(teleport.ComponentAuth, teleport.ComponentGRPC),
		}),
		server: server,
	}
	proto.RegisterAuthServiceServer(authServer.server, authServer)
	return authServer, nil
}
