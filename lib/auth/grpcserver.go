/*
Copyright 2018-2021 Gravitational, Inc.

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
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/u2f"
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

// GetServer returns an instance of grpc server
func (g *GRPCServer) GetServer() (*grpc.Server, error) {
	if g.server == nil {
		return nil, trace.BadParameter("grpc server has not been initialized")
	}

	return g.server, nil
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
	var sessionID session.ID
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
			sessionID = session.ID(create.SessionID)
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
			if err != nil {
				return trail.ToGRPC(err)
			}
			clusterName, err := auth.GetClusterName()
			if err != nil {
				return trail.ToGRPC(err)
			}
			if g.APIConfig.MetadataGetter != nil {
				sessionData := g.APIConfig.MetadataGetter.GetUploadMetadata(sessionID)
				event := &apievents.SessionUpload{
					Metadata: events.Metadata{
						Type:        events.SessionUploadEvent,
						Code:        events.SessionUploadCode,
						Index:       events.SessionUploadIndex,
						ClusterName: clusterName.GetClusterName(),
					},
					SessionMetadata: events.SessionMetadata{
						SessionID: string(sessionData.SessionID),
					},
					SessionURL: sessionData.URL,
				}
				if err := g.Emitter.EmitAuditEvent(auth.CloseContext(), event); err != nil {
					return trail.ToGRPC(err)
				}
			}
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
		servicesWatch.Kinds = append(servicesWatch.Kinds, proto.ToWatchKind(kind))
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
	if err := services.ValidateAccessRequest(req); err != nil {
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

func (g *GRPCServer) SubmitAccessReview(ctx context.Context, review *types.AccessReviewSubmission) (*types.AccessRequestV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	req, err := auth.ServerWithRoles.SubmitAccessReview(ctx, *review)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	r, ok := req.(*services.AccessRequestV3)
	if !ok {
		err = trace.BadParameter("unexpected access request type %T", req)
		return nil, trail.ToGRPC(err)
	}

	return r, nil
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

	if err := services.ValidateUser(req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.ServerWithRoles.CreateUser(ctx, req); err != nil {
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

	if err := services.ValidateUser(req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.ServerWithRoles.UpdateUser(ctx, req); err != nil {
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

	if err := auth.ServerWithRoles.DeleteUser(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}

	log.Infof("%q user deleted", req.Name)

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
	var opts []services.MarshalOption
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
		Username:    req.GetUsername(),
		PublicAddr:  req.GetPublicAddr(),
		ClusterName: req.GetClusterName(),
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
func (g *GRPCServer) GetWebSession(ctx context.Context, req *types.GetWebSessionRequest) (*proto.GetWebSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	session, err := auth.WebSessions().Get(ctx, *req)
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

// DeleteWebSession removes the web session given with req.
func (g *GRPCServer) DeleteWebSession(ctx context.Context, req *types.DeleteWebSessionRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.WebSessions().Delete(ctx, *req); err != nil {
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

// GetWebToken gets a web token.
func (g *GRPCServer) GetWebToken(ctx context.Context, req *types.GetWebTokenRequest) (*proto.GetWebTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	resp, err := auth.WebTokens().Get(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	token, ok := resp.(*types.WebTokenV3)
	if !ok {
		return nil, trail.ToGRPC(trace.BadParameter("unexpected web token type %T", resp))
	}

	return &proto.GetWebTokenResponse{
		Token: token,
	}, nil
}

// GetWebTokens gets all web tokens.
func (g *GRPCServer) GetWebTokens(ctx context.Context, _ *empty.Empty) (*proto.GetWebTokensResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	tokens, err := auth.WebTokens().List(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	var out []*types.WebTokenV3
	for _, t := range tokens {
		token, ok := t.(*types.WebTokenV3)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", t))
		}
		out = append(out, token)
	}

	return &proto.GetWebTokensResponse{
		Tokens: out,
	}, nil
}

// DeleteWebToken removes the web token given with req.
func (g *GRPCServer) DeleteWebToken(ctx context.Context, req *types.DeleteWebTokenRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.WebTokens().Delete(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllWebTokens removes all web tokens.
func (g *GRPCServer) DeleteAllWebTokens(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.WebTokens().DeleteAll(ctx); err != nil {
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

// GetRole retrieves a role by name.
func (g *GRPCServer) GetRole(ctx context.Context, req *proto.GetRoleRequest) (*types.RoleV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	role, err := auth.ServerWithRoles.GetRole(ctx, req.Name)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	roleV3, ok := role.(*types.RoleV3)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected role type"))
	}
	return roleV3, nil
}

// GetRoles retrieves all roles.
func (g *GRPCServer) GetRoles(ctx context.Context, _ *empty.Empty) (*proto.GetRolesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	roles, err := auth.ServerWithRoles.GetRoles(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	var rolesV3 []*types.RoleV3
	for _, r := range roles {
		role, ok := r.(*types.RoleV3)
		if !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", r))
		}
		rolesV3 = append(rolesV3, role)
	}
	return &proto.GetRolesResponse{
		Roles: rolesV3,
	}, nil
}

// UpsertRole upserts a role.
func (g *GRPCServer) UpsertRole(ctx context.Context, role *types.RoleV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = services.ValidateRole(role); err != nil {
		return nil, trail.ToGRPC(err)
	}
	err = auth.ServerWithRoles.UpsertRole(ctx, role)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	g.Debugf("%q role upserted", role.GetName())

	return &empty.Empty{}, nil
}

// DeleteRole deletes a role by name.
func (g *GRPCServer) DeleteRole(ctx context.Context, req *proto.DeleteRoleRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteRole(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}

	g.Debugf("%q role deleted", req.GetName())

	return &empty.Empty{}, nil
}

func (g *GRPCServer) AddMFADevice(stream proto.AuthService_AddMFADeviceServer) error {
	actx, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}

	// The RPC is streaming both ways and the message sequence is:
	// (-> means client-to-server, <- means server-to-client)
	//
	// 1. -> Init
	// 2. <- ExistingMFAChallenge
	// 3. -> ExistingMFAResponse
	// 4. <- NewMFARegisterChallenge
	// 5. -> NewMFARegisterResponse
	// 6. <- Ack

	// 1. receive client Init
	initReq, err := addMFADeviceInit(actx, stream)
	if err != nil {
		return trail.ToGRPC(err)
	}

	// 2. send ExistingMFAChallenge
	// 3. receive and validate ExistingMFAResponse
	if err := addMFADeviceAuthChallenge(actx, stream); err != nil {
		return trail.ToGRPC(err)
	}

	// 4. send MFARegisterChallenge
	// 5. receive and validate MFARegisterResponse
	dev, err := addMFADeviceRegisterChallenge(actx, stream, initReq)
	if err != nil {
		return trail.ToGRPC(err)
	}

	clusterName, err := actx.GetClusterName()
	if err != nil {
		return trail.ToGRPC(err)
	}
	if err := g.Emitter.EmitAuditEvent(g.Context, &apievents.MFADeviceAdd{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceAddEvent,
			Code:        events.MFADeviceAddEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: apievents.UserMetadata{
			User: actx.Identity.GetIdentity().Username,
		},
		MFADeviceMetadata: mfaDeviceEventMetadata(dev),
	}); err != nil {
		return trail.ToGRPC(err)
	}

	// 6. send Ack
	if err := stream.Send(&proto.AddMFADeviceResponse{
		Response: &proto.AddMFADeviceResponse_Ack{Ack: &proto.AddMFADeviceResponseAck{Device: dev}},
	}); err != nil {
		return trail.ToGRPC(err)
	}
	return nil
}

func addMFADeviceInit(gctx *grpcContext, stream proto.AuthService_AddMFADeviceServer) (*proto.AddMFADeviceRequestInit, error) {
	req, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	initReq := req.GetInit()
	if initReq == nil {
		return nil, trace.BadParameter("expected AddMFADeviceRequestInit, got %T", req)
	}
	devs, err := gctx.authServer.GetMFADevices(stream.Context(), gctx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, d := range devs {
		if d.Metadata.Name == initReq.DeviceName {
			return nil, trace.AlreadyExists("MFA device named %q already exists", d.Metadata.Name)
		}
	}
	return initReq, nil
}

func addMFADeviceAuthChallenge(gctx *grpcContext, stream proto.AuthService_AddMFADeviceServer) error {
	auth := gctx.authServer
	user := gctx.User.GetName()
	ctx := stream.Context()
	u2fStorage, err := u2f.InMemoryAuthenticationStorage(auth.Identity)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: authChallenge may be empty if this user has no existing MFA devices.
	authChallenge, err := auth.mfaAuthChallenge(ctx, user, u2fStorage)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&proto.AddMFADeviceResponse{
		Response: &proto.AddMFADeviceResponse_ExistingMFAChallenge{ExistingMFAChallenge: authChallenge},
	}); err != nil {
		return trace.Wrap(err)
	}

	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	authResp := req.GetExistingMFAResponse()
	if authResp == nil {
		return trace.BadParameter("expected MFAAuthenticateResponse, got %T", req)
	}
	// Only validate if there was a challenge.
	if authChallenge.TOTP != nil || len(authChallenge.U2F) > 0 {
		if _, err := auth.validateMFAAuthResponse(ctx, user, authResp, u2fStorage); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func addMFADeviceRegisterChallenge(gctx *grpcContext, stream proto.AuthService_AddMFADeviceServer, initReq *proto.AddMFADeviceRequestInit) (*types.MFADevice, error) {
	auth := gctx.authServer
	user := gctx.User.GetName()
	ctx := stream.Context()
	u2fStorage, err := u2f.InMemoryRegistrationStorage(auth.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Send registration challenge for the requested device type.
	regChallenge := new(proto.MFARegisterChallenge)
	switch initReq.Type {
	case proto.AddMFADeviceRequestInit_TOTP:
		otpKey, otpOpts, err := auth.newTOTPKey(user)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		regChallenge.Request = &proto.MFARegisterChallenge_TOTP{TOTP: &proto.TOTPRegisterChallenge{
			Secret:        otpKey.Secret(),
			Issuer:        otpKey.Issuer(),
			PeriodSeconds: uint32(otpOpts.Period),
			Algorithm:     otpOpts.Algorithm.String(),
			Digits:        uint32(otpOpts.Digits.Length()),
			Account:       otpKey.AccountName(),
		}}
	case proto.AddMFADeviceRequestInit_U2F:
		cap, err := auth.GetAuthPreference()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		u2fConfig, err := cap.GetU2F()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		challenge, err := u2f.RegisterInit(u2f.RegisterInitParams{
			StorageKey: user,
			AppConfig:  *u2fConfig,
			Storage:    u2fStorage,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		regChallenge.Request = &proto.MFARegisterChallenge_U2F{U2F: &proto.U2FRegisterChallenge{
			Version:   challenge.Version,
			Challenge: challenge.Challenge,
			AppID:     challenge.AppID,
		}}
	default:
		return nil, trace.BadParameter("AddMFADeviceRequestInit sent an unknown DeviceType %v", initReq.Type)
	}
	if err := stream.Send(&proto.AddMFADeviceResponse{
		Response: &proto.AddMFADeviceResponse_NewMFARegisterChallenge{NewMFARegisterChallenge: regChallenge},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// 5. receive client MFARegisterResponse
	req, err := stream.Recv()
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	regResp := req.GetNewMFARegisterResponse()
	if regResp == nil {
		return nil, trace.BadParameter("expected MFARegistrationResponse, got %T", req)
	}

	// Validate MFARegisterResponse and upsert the new device on success.
	var dev *types.MFADevice
	switch resp := regResp.Response.(type) {
	case *proto.MFARegisterResponse_TOTP:
		challenge := regChallenge.GetTOTP()
		if challenge == nil {
			return nil, trace.BadParameter("got unexpected %T in response to %T", regResp.Response, regChallenge.Request)
		}
		dev, err = services.NewTOTPDevice(initReq.DeviceName, challenge.Secret, auth.clock.Now())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := auth.checkTOTP(ctx, user, resp.TOTP.Code, dev); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := auth.UpsertMFADevice(ctx, user, dev); err != nil {
			return nil, trace.Wrap(err)
		}
	case *proto.MFARegisterResponse_U2F:
		cap, err := auth.GetAuthPreference()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		u2fConfig, err := cap.GetU2F()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// u2f.RegisterVerify will upsert the new device internally.
		dev, err = u2f.RegisterVerify(ctx, u2f.RegisterVerifyParams{
			DevName: initReq.DeviceName,
			Resp: u2f.RegisterChallengeResponse{
				RegistrationData: resp.U2F.RegistrationData,
				ClientData:       resp.U2F.ClientData,
			},
			ChallengeStorageKey:    user,
			RegistrationStorageKey: user,
			Storage:                u2fStorage,
			Clock:                  auth.clock,
			AttestationCAs:         u2fConfig.DeviceAttestationCAs,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("MFARegisterResponse sent an unknown response type %v", regResp.Response)
	}
	return dev, nil
}

func (g *GRPCServer) DeleteMFADevice(stream proto.AuthService_DeleteMFADeviceServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trail.ToGRPC(err)
	}
	auth := actx.authServer
	user := actx.User.GetName()

	// The RPC is streaming both ways and the message sequence is:
	// (-> means client-to-server, <- means server-to-client)
	//
	// 1. -> Init
	// 2. <- MFAChallenge
	// 3. -> MFAResponse
	// 4. <- Ack

	// 1. receive client Init
	req, err := stream.Recv()
	if err != nil {
		return trail.ToGRPC(err)
	}
	initReq := req.GetInit()
	if initReq == nil {
		return trail.ToGRPC(trace.BadParameter("expected DeleteMFADeviceRequestInit, got %T", req))
	}

	// 2. send MFAAuthenticateChallenge
	// 3. receive and validate MFAAuthenticateResponse
	if err := deleteMFADeviceAuthChallenge(actx, stream); err != nil {
		return trail.ToGRPC(err)
	}

	// Find the device and delete it from backend.
	devs, err := auth.GetMFADevices(ctx, user)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, d := range devs {
		// Match device by name or ID.
		if d.Metadata.Name != initReq.DeviceName && d.Id != initReq.DeviceName {
			continue
		}
		if err := auth.DeleteMFADevice(ctx, user, d.Id); err != nil {
			return trail.ToGRPC(err)
		}

		clusterName, err := actx.GetClusterName()
		if err != nil {
			return trail.ToGRPC(err)
		}
		if err := g.Emitter.EmitAuditEvent(g.Context, &apievents.MFADeviceDelete{
			Metadata: apievents.Metadata{
				Type:        events.MFADeviceDeleteEvent,
				Code:        events.MFADeviceDeleteEventCode,
				ClusterName: clusterName.GetClusterName(),
			},
			UserMetadata: apievents.UserMetadata{
				User: actx.Identity.GetIdentity().Username,
			},
			MFADeviceMetadata: mfaDeviceEventMetadata(d),
		}); err != nil {
			return trail.ToGRPC(err)
		}

		// 4. send Ack
		if err := stream.Send(&proto.DeleteMFADeviceResponse{
			Response: &proto.DeleteMFADeviceResponse_Ack{Ack: &proto.DeleteMFADeviceResponseAck{}},
		}); err != nil {
			return trail.ToGRPC(err)
		}
		return nil
	}
	return trail.ToGRPC(trace.NotFound("MFA device %q does not exist", initReq.DeviceName))
}

func deleteMFADeviceAuthChallenge(gctx *grpcContext, stream proto.AuthService_DeleteMFADeviceServer) error {
	ctx := stream.Context()
	auth := gctx.authServer
	user := gctx.User.GetName()
	u2fStorage, err := u2f.InMemoryAuthenticationStorage(auth.Identity)
	if err != nil {
		return trace.Wrap(err)
	}

	authChallenge, err := auth.mfaAuthChallenge(ctx, user, u2fStorage)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := stream.Send(&proto.DeleteMFADeviceResponse{
		Response: &proto.DeleteMFADeviceResponse_MFAChallenge{MFAChallenge: authChallenge},
	}); err != nil {
		return trace.Wrap(err)
	}

	// 3. receive client MFAAuthenticateResponse
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}
	authResp := req.GetMFAResponse()
	if authResp == nil {
		return trace.BadParameter("expected MFAAuthenticateResponse, got %T", req)
	}
	if _, err := auth.validateMFAAuthResponse(ctx, user, authResp, u2fStorage); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func mfaDeviceEventMetadata(d *types.MFADevice) apievents.MFADeviceMetadata {
	m := apievents.MFADeviceMetadata{
		DeviceName: d.Metadata.Name,
		DeviceID:   d.Id,
	}
	switch d.Device.(type) {
	case *types.MFADevice_Totp:
		m.DeviceType = string(constants.SecondFactorOTP)
	case *types.MFADevice_U2F:
		m.DeviceType = string(constants.SecondFactorU2F)
	default:
		m.DeviceType = "unknown"
		log.Warningf("Unknown MFA device type %T when generating audit event metadata", d.Device)
	}
	return m
}

func (g *GRPCServer) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devs, err := actx.authServer.GetMFADevices(ctx, actx.User.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO(awly): mfa: remove secrets from MFA devices.
	return &proto.GetMFADevicesResponse{
		Devices: devs,
	}, nil
}

func (g *GRPCServer) GenerateUserSingleUseCerts(stream proto.AuthService_GenerateUserSingleUseCertsServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trail.ToGRPC(err)
	}

	// The RPC is streaming both ways and the message sequence is:
	// (-> means client-to-server, <- means server-to-client)
	//
	// 1. -> Init
	// 2. <- MFAChallenge
	// 3. -> MFAResponse
	// 4. <- Certs

	// 1. receive client Init
	req, err := stream.Recv()
	if err != nil {
		return trail.ToGRPC(err)
	}
	initReq := req.GetInit()
	if initReq == nil {
		return trail.ToGRPC(trace.BadParameter("expected UserCertsRequest, got %T", req.Request))
	}
	if err := validateUserSingleUseCertRequest(ctx, actx, initReq); err != nil {
		g.Entry.Debugf("Validation of single-use cert request failed: %v", err)
		return trail.ToGRPC(err)
	}

	// 2. send MFAChallenge
	// 3. receive and validate MFAResponse
	mfaDev, err := userSingleUseCertsAuthChallenge(actx, stream)
	if err != nil {
		g.Entry.Debugf("Failed to perform single-use cert challenge: %v", err)
		return trail.ToGRPC(err)
	}

	// Generate the cert.
	respCert, err := userSingleUseCertsGenerate(ctx, actx, *initReq, mfaDev)
	if err != nil {
		g.Entry.Warningf("Failed to generate single-use cert: %v", err)
		return trail.ToGRPC(err)
	}

	// 4. send Certs
	if err := stream.Send(&proto.UserSingleUseCertsResponse{
		Response: &proto.UserSingleUseCertsResponse_Cert{Cert: respCert},
	}); err != nil {
		return trail.ToGRPC(err)
	}
	return nil
}

// validateUserSingleUseCertRequest validates the request for a single-use user
// cert.
func validateUserSingleUseCertRequest(ctx context.Context, actx *grpcContext, req *proto.UserCertsRequest) error {
	if err := actx.currentUserAction(req.Username); err != nil {
		return trace.Wrap(err)
	}

	switch req.Usage {
	case proto.UserCertsRequest_SSH:
		if req.NodeName == "" {
			return trace.BadParameter("missing NodeName field in a ssh-only UserCertsRequest")
		}
	case proto.UserCertsRequest_Kubernetes:
		if req.KubernetesCluster == "" {
			return trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest")
		}
	case proto.UserCertsRequest_Database:
		if req.RouteToDatabase.ServiceName == "" {
			return trace.BadParameter("missing ServiceName field in a database-only UserCertsRequest")
		}
	case proto.UserCertsRequest_All:
		return trace.BadParameter("must specify a concrete Usage in UserCertsRequest, one of SSH, Kubernetes or Database")
	default:
		return trace.BadParameter("unknown certificate Usage %q", req.Usage)
	}

	maxExpiry := actx.authServer.GetClock().Now().Add(teleport.UserSingleUseCertTTL)
	if req.Expires.After(maxExpiry) {
		req.Expires = maxExpiry
	}
	return nil
}

func userSingleUseCertsAuthChallenge(gctx *grpcContext, stream proto.AuthService_GenerateUserSingleUseCertsServer) (*types.MFADevice, error) {
	ctx := stream.Context()
	auth := gctx.authServer
	user := gctx.User.GetName()
	u2fStorage, err := u2f.InMemoryAuthenticationStorage(auth.Identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authChallenge, err := auth.mfaAuthChallenge(ctx, user, u2fStorage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if authChallenge.TOTP == nil && len(authChallenge.U2F) == 0 {
		return nil, trace.AccessDenied("MFA is required to access this resource but user has no MFA devices; use 'tsh mfa add' to register MFA devices")
	}
	if err := stream.Send(&proto.UserSingleUseCertsResponse{
		Response: &proto.UserSingleUseCertsResponse_MFAChallenge{MFAChallenge: authChallenge},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authResp := req.GetMFAResponse()
	if authResp == nil {
		return nil, trace.BadParameter("expected MFAAuthenticateResponse, got %T", req.Request)
	}
	mfaDev, err := auth.validateMFAAuthResponse(ctx, user, authResp, u2fStorage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return mfaDev, nil
}

func userSingleUseCertsGenerate(ctx context.Context, actx *grpcContext, req proto.UserCertsRequest, mfaDev *types.MFADevice) (*proto.SingleUseUserCert, error) {
	// Get the client IP.
	clientPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.BadParameter("no peer info in gRPC stream, can't get client IP")
	}
	clientIP, _, err := net.SplitHostPort(clientPeer.Addr.String())
	if err != nil {
		return nil, trace.BadParameter("can't parse client IP from peer info: %v", err)
	}

	// Generate the cert.
	certs, err := actx.generateUserCerts(ctx, req, certRequestMFAVerified(mfaDev.Id), certRequestClientIP(clientIP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp := new(proto.SingleUseUserCert)
	switch req.Usage {
	case proto.UserCertsRequest_SSH:
		resp.Cert = &proto.SingleUseUserCert_SSH{SSH: certs.SSH}
	case proto.UserCertsRequest_Kubernetes, proto.UserCertsRequest_Database:
		resp.Cert = &proto.SingleUseUserCert_TLS{TLS: certs.TLS}
	default:
		return nil, trace.BadParameter("unknown certificate usage %q", req.Usage)
	}
	return resp, nil
}

func (g *GRPCServer) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	resp, err := actx.IsMFARequired(ctx, req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	return resp, nil
}

// GetOIDCConnector retrieves an OIDC connector by name.
func (g *GRPCServer) GetOIDCConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.OIDCConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	oidcConnector, err := auth.ServerWithRoles.GetOIDCConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	oidcConnectorV2, ok := oidcConnector.(*types.OIDCConnectorV2)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected OIDC connector type %T", oidcConnector))
	}
	return oidcConnectorV2, nil
}

// GetOIDCConnectors retrieves all OIDC connectors.
func (g *GRPCServer) GetOIDCConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.OIDCConnectorV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	oidcConnectors, err := auth.ServerWithRoles.GetOIDCConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	oidcConnectorsV2 := make([]*types.OIDCConnectorV2, len(oidcConnectors))
	for i, oc := range oidcConnectors {
		var ok bool
		if oidcConnectorsV2[i], ok = oc.(*types.OIDCConnectorV2); !ok {
			return nil, trail.ToGRPC(trace.Errorf("encountered unexpected OIDC connector type %T", oc))
		}
	}
	return &types.OIDCConnectorV2List{
		OIDCConnectors: oidcConnectorsV2,
	}, nil
}

// UpsertOIDCConnector upserts an OIDC connector.
func (g *GRPCServer) UpsertOIDCConnector(ctx context.Context, oidcConnector *types.OIDCConnectorV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = services.ValidateOIDCConnector(oidcConnector); err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = auth.ServerWithRoles.UpsertOIDCConnector(ctx, oidcConnector); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (g *GRPCServer) DeleteOIDCConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteOIDCConnector(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetSAMLConnector retrieves a SAML connector by name.
func (g *GRPCServer) GetSAMLConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.SAMLConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	samlConnector, err := auth.ServerWithRoles.GetSAMLConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	samlConnectorV2, ok := samlConnector.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected SAML connector type: %T", samlConnector))
	}
	return samlConnectorV2, nil
}

// GetSAMLConnectors retrieves all SAML connectors.
func (g *GRPCServer) GetSAMLConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.SAMLConnectorV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	samlConnectors, err := auth.ServerWithRoles.GetSAMLConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	samlConnectorsV2 := make([]*types.SAMLConnectorV2, len(samlConnectors))
	for i, sc := range samlConnectors {
		var ok bool
		if samlConnectorsV2[i], ok = sc.(*types.SAMLConnectorV2); !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", sc))
		}
	}
	return &types.SAMLConnectorV2List{
		SAMLConnectors: samlConnectorsV2,
	}, nil
}

// UpsertSAMLConnector upserts a SAML connector.
func (g *GRPCServer) UpsertSAMLConnector(ctx context.Context, samlConnector *types.SAMLConnectorV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = services.ValidateSAMLConnector(samlConnector); err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = auth.ServerWithRoles.UpsertSAMLConnector(ctx, samlConnector); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (g *GRPCServer) DeleteSAMLConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteSAMLConnector(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetGithubConnector retrieves a Github connector by name.
func (g *GRPCServer) GetGithubConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.GithubConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	githubConnector, err := auth.ServerWithRoles.GetGithubConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	githubConnectorV3, ok := githubConnector.(*types.GithubConnectorV3)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected Github connector type: %T", githubConnector))
	}
	return githubConnectorV3, nil
}

// GetGithubConnectors retrieves all Github connectors.
func (g *GRPCServer) GetGithubConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.GithubConnectorV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	githubConnectors, err := auth.ServerWithRoles.GetGithubConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	githubConnectorsV3 := make([]*types.GithubConnectorV3, len(githubConnectors))
	for i, gc := range githubConnectors {
		var ok bool
		if githubConnectorsV3[i], ok = gc.(*types.GithubConnectorV3); !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", gc))
		}
	}
	return &types.GithubConnectorV3List{
		GithubConnectors: githubConnectorsV3,
	}, nil
}

// UpsertGithubConnector upserts a Github connector.
func (g *GRPCServer) UpsertGithubConnector(ctx context.Context, GithubConnector *types.GithubConnectorV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = auth.ServerWithRoles.UpsertGithubConnector(ctx, GithubConnector); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteGithubConnector deletes a Github connector by name.
func (g *GRPCServer) DeleteGithubConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteGithubConnector(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetTrustedCluster retrieves a Trusted Cluster by name.
func (g *GRPCServer) GetTrustedCluster(ctx context.Context, req *types.ResourceRequest) (*types.TrustedClusterV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	tc, err := auth.ServerWithRoles.GetTrustedCluster(ctx, req.Name)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected Trusted Cluster type %T", tc))
	}
	return trustedClusterV2, nil
}

// GetTrustedClusters retrieves all Trusted Clusters.
func (g *GRPCServer) GetTrustedClusters(ctx context.Context, _ *empty.Empty) (*types.TrustedClusterV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	trustedClusters, err := auth.ServerWithRoles.GetTrustedClusters(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	trustedClustersV2 := make([]*types.TrustedClusterV2, len(trustedClusters))
	for i, tc := range trustedClusters {
		var ok bool
		if trustedClustersV2[i], ok = tc.(*types.TrustedClusterV2); !ok {
			return nil, trail.ToGRPC(trace.BadParameter("unexpected type %T", tc))
		}
	}
	return &types.TrustedClusterV2List{
		TrustedClusters: trustedClustersV2,
	}, nil
}

// UpsertTrustedCluster upserts a Trusted Cluster.
func (g *GRPCServer) UpsertTrustedCluster(ctx context.Context, cluster *types.TrustedClusterV2) (*types.TrustedClusterV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = services.ValidateTrustedCluster(cluster); err != nil {
		return nil, trail.ToGRPC(err)
	}
	trustedCluster, err := auth.ServerWithRoles.UpsertTrustedCluster(ctx, cluster)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	trustedClusterV2, ok := trustedCluster.(*types.TrustedClusterV2)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected Trusted Cluster type"))
	}
	return trustedClusterV2, nil
}

// DeleteTrustedCluster deletes a Trusted Cluster by name.
func (g *GRPCServer) DeleteTrustedCluster(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteTrustedCluster(ctx, req.Name); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetToken retrieves a token by name.
func (g *GRPCServer) GetToken(ctx context.Context, req *types.ResourceRequest) (*types.ProvisionTokenV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	token, err := auth.ServerWithRoles.GetToken(ctx, req.Name)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	provisionTokenV2, ok := token.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trail.ToGRPC(trace.Errorf("encountered unexpected token type: %T", token))
	}
	return provisionTokenV2, nil
}

// GetTokens retrieves all tokens.
func (g *GRPCServer) GetTokens(ctx context.Context, _ *empty.Empty) (*types.ProvisionTokenV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	tokens, err := auth.ServerWithRoles.GetTokens(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	provisionTokensV2 := make([]*types.ProvisionTokenV2, len(tokens))
	for i, t := range tokens {
		var ok bool
		if provisionTokensV2[i], ok = t.(*types.ProvisionTokenV2); !ok {
			return nil, trail.ToGRPC(trace.Errorf("encountered unexpected token type: %T", t))
		}
	}
	return &types.ProvisionTokenV2List{
		ProvisionTokens: provisionTokensV2,
	}, nil
}

// UpsertToken upserts a token.
func (g *GRPCServer) UpsertToken(ctx context.Context, token *types.ProvisionTokenV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err = auth.ServerWithRoles.UpsertToken(ctx, token); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteToken deletes a token by name.
func (g *GRPCServer) DeleteToken(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	if err := auth.ServerWithRoles.DeleteToken(ctx, req.Name); err != nil {
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
	// HTTPS server expects auth context to be set by the auth middleware
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
