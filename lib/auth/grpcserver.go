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
	"fmt"
	"io"
	"net"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // gzip compressor for gRPC.
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/joinserver"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	heartbeatConnectionsReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: teleport.MetricHeartbeatConnectionsReceived,
			Help: "Number of times auth received a heartbeat connection",
		},
	)
	watcherEventsEmitted = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    teleport.MetricWatcherEventsEmitted,
			Help:    "Per resources size of events emitted",
			Buckets: prometheus.LinearBuckets(0, 200, 5),
		},
		[]string{teleport.TagResource},
	)
	watcherEventSizes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    teleport.MetricWatcherEventSizes,
			Help:    "Overall size of events emitted",
			Buckets: prometheus.LinearBuckets(0, 100, 20),
		},
	)
	connectedResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Name:      teleport.MetricConnectedResources,
			Help:      "Tracks the number and type of resources connected via keepalives",
		},
		[]string{teleport.TagType},
	)
)

// GRPCServer is GPRC Auth Server API
type GRPCServer struct {
	*logrus.Entry
	APIConfig
	server *grpc.Server

	// traceClient is used to forward spans to the upstream collector for components
	// within the cluster that don't have a direct connection to said collector
	traceClient otlptrace.Client
	// TraceServiceServer exposes the exporter server so that the auth server may
	// collect and forward spans
	collectortracepb.TraceServiceServer
}

func (g *GRPCServer) serverContext() context.Context {
	return g.AuthServer.closeCtx
}

// Export forwards OTLP traces to the upstream collector configured in the tracing service. This allows for
// tsh, tctl, etc to be able to export traces without having to know how to connect to the upstream collector
// for the cluster.
func (g *GRPCServer) Export(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	if len(req.ResourceSpans) == 0 {
		return &collectortracepb.ExportTraceServiceResponse{}, nil
	}

	if err := g.traceClient.UploadTraces(ctx, req.ResourceSpans); err != nil {
		return &collectortracepb.ExportTraceServiceResponse{}, trace.Wrap(err)
	}

	return &collectortracepb.ExportTraceServiceResponse{}, nil
}

// GetServer returns an instance of grpc server
func (g *GRPCServer) GetServer() (*grpc.Server, error) {
	if g.server == nil {
		return nil, trace.BadParameter("grpc server has not been initialized")
	}

	return g.server, nil
}

// EmitAuditEvent emits audit event
func (g *GRPCServer) EmitAuditEvent(ctx context.Context, req *apievents.OneOf) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	event, err := apievents.FromOneOf(*req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.EmitAuditEvent(ctx, event)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// SendKeepAlives allows node to send a stream of keep alive requests
func (g *GRPCServer) SendKeepAlives(stream proto.AuthService_SendKeepAlivesServer) error {
	defer stream.SendAndClose(&empty.Empty{})
	firstIteration := true
	for {
		// Authenticate within the loop to block locked-out nodes from heartbeating.
		auth, err := g.authenticate(stream.Context())
		if err != nil {
			return trace.Wrap(err)
		}
		keepAlive, err := stream.Recv()
		if err == io.EOF {
			g.Debugf("Connection closed.")
			return nil
		}
		if err != nil {
			g.Debugf("Failed to receive heartbeat: %v", err)
			return trace.Wrap(err)
		}
		err = auth.KeepAliveServer(stream.Context(), *keepAlive)
		if err != nil {
			return trace.Wrap(err)
		}
		if firstIteration {
			g.Debugf("Got heartbeat connection from %v.", auth.User.GetName())
			heartbeatConnectionsReceived.Inc()
			connectedResources.WithLabelValues(keepAlive.GetType()).Inc()
			defer connectedResources.WithLabelValues(keepAlive.GetType()).Dec()
			firstIteration = false
		}
	}
}

// CreateAuditStream creates or resumes audit event stream
func (g *GRPCServer) CreateAuditStream(stream proto.AuthService_CreateAuditStreamServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	var eventStream apievents.Stream
	var sessionID session.ID
	g.Debugf("CreateAuditStream connection from %v.", auth.User.GetName())
	streamStart := time.Now()
	processed := int64(0)
	counter := 0
	forwardEvents := func(eventStream apievents.Stream) {
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

	closeStream := func(eventStream apievents.Stream) {
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
			return trace.Wrap(err)
		}
		if create := request.GetCreateStream(); create != nil {
			if eventStream != nil {
				return trace.BadParameter("stream is already created or resumed")
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
				return trace.BadParameter("stream is already created or resumed")
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
				return trace.BadParameter("stream is not initialized yet, cannot complete")
			}
			// do not use stream context to give the auth server finish the upload
			// even if the stream's context is canceled
			err := eventStream.Complete(auth.CloseContext())
			if err != nil {
				return trace.Wrap(err)
			}
			clusterName, err := auth.GetClusterName()
			if err != nil {
				return trace.Wrap(err)
			}
			if g.APIConfig.MetadataGetter != nil {
				sessionData := g.APIConfig.MetadataGetter.GetUploadMetadata(sessionID)
				event := &apievents.SessionUpload{
					Metadata: apievents.Metadata{
						Type:        events.SessionUploadEvent,
						Code:        events.SessionUploadCode,
						ID:          uuid.New().String(),
						Index:       events.SessionUploadIndex,
						ClusterName: clusterName.GetClusterName(),
					},
					SessionMetadata: apievents.SessionMetadata{
						SessionID: string(sessionData.SessionID),
					},
					SessionURL: sessionData.URL,
				}
				if err := g.Emitter.EmitAuditEvent(auth.CloseContext(), event); err != nil {
					return trace.Wrap(err)
				}
			}
			g.Debugf("Completed stream: %v.", err)
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		} else if flushAndClose := request.GetFlushAndCloseStream(); flushAndClose != nil {
			if eventStream == nil {
				return trace.BadParameter("stream is not initialized yet, cannot flush and close")
			}
			// flush and close is always done
			return nil
		} else if oneof := request.GetEvent(); oneof != nil {
			if eventStream == nil {
				return trace.BadParameter("stream cannot receive an event without first being created or resumed")
			}
			event, err := apievents.FromOneOf(*oneof)
			if err != nil {
				g.WithError(err).Debugf("Failed to decode event.")
				return trace.Wrap(err)
			}
			start := time.Now()
			err = eventStream.EmitAuditEvent(stream.Context(), event)
			if err != nil {
				switch {
				case events.IsPermanentEmitError(err):
					g.WithError(err).WithField("event", event).
						Error("Failed to EmitAuditEvent due to a permanent error. Event wil be omitted.")
					continue
				default:
					return trace.Wrap(err)
				}
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
			return trace.BadParameter("unsupported stream request")
		}
	}
}

// logInterval is used to log stats after this many events
const logInterval = 10000

// WatchEvents returns a new stream of cluster events
func (g *GRPCServer) WatchEvents(watch *proto.Watch, stream proto.AuthService_WatchEventsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	servicesWatch := types.Watch{
		Name: auth.User.GetName(),
	}
	for _, kind := range watch.Kinds {
		servicesWatch.Kinds = append(servicesWatch.Kinds, proto.ToWatchKind(kind))
	}

	if clusterName, err := auth.GetClusterName(); err == nil {
		// we might want to enforce a filter for older clients in certain conditions
		maybeFilterCertAuthorityWatches(stream.Context(), clusterName.GetClusterName(), auth.Checker.RoleNames(), &servicesWatch)
	}

	watcher, err := auth.NewWatcher(stream.Context(), servicesWatch)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-watcher.Done():
			return watcher.Error()
		case event := <-watcher.Events():
			switch r := event.Resource.(type) {
			case *types.RoleV5:
				downgraded, err := downgradeRole(stream.Context(), r)
				if err != nil {
					return trace.Wrap(err)
				}
				event.Resource = downgraded
			}

			out, err := client.EventToGRPC(event)
			if err != nil {
				return trace.Wrap(err)
			}

			watcherEventsEmitted.WithLabelValues(resourceLabel(event)).Observe(float64(out.Size()))
			watcherEventSizes.Observe(float64(out.Size()))

			if err := stream.Send(out); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// maybeFilterCertAuthorityWatches will add filters to the CertAuthority
// WatchKinds in the watch if the client is authenticated as just a `Node` with
// no other roles and if the client is older than the cutoff version, and if the
// WatchKind for KindCertAuthority is trivial, i.e. it's a WatchKind{Kind:
// KindCertAuthority} with no other fields set. In any other case we will assume
// that the client knows what it's doing and the cache watcher will still send
// everything.
//
// DELETE IN 10.0, no supported clients should require this at that point
func maybeFilterCertAuthorityWatches(ctx context.Context, clusterName string, roleNames []string, watch *types.Watch) {
	if len(roleNames) != 1 || roleNames[0] != string(types.RoleNode) {
		return
	}

	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if !ok {
		log.Debug("no client version found in grpc context")
		return
	}

	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		log.WithError(err).Debugf("couldn't parse client version %q", clientVersionString)
		return
	}

	// we treat the entire previous major version as "old" for this version
	// check, even if there might have been backports; compliant clients will
	// supply their own filter anyway
	if !clientVersion.LessThan(certAuthorityFilterVersionCutoff) {
		return
	}

	for i, k := range watch.Kinds {
		if k.Kind != types.KindCertAuthority || !k.IsTrivial() {
			continue
		}

		log.Debugf("Injecting filter for CertAuthority watch for Node-only watcher with version %v", clientVersion)
		watch.Kinds[i].Filter = types.CertAuthorityFilter{
			types.HostCA: clusterName,
			types.UserCA: types.Wildcard,
		}.IntoMap()
	}
}

// certAuthorityFilterVersionCutoff is the version starting from which we stop
// injecting filters for CertAuthority watches in maybeFilterCertAuthorityWatches.
var certAuthorityFilterVersionCutoff = *semver.New("9.0.0")

// resourceLabel returns the label for the provided types.Event
func resourceLabel(event types.Event) string {
	if event.Resource == nil {
		return event.Type.String()
	}

	sub := event.Resource.GetSubKind()
	if sub == "" {
		return fmt.Sprintf("/%s", event.Resource.GetKind())
	}

	return fmt.Sprintf("/%s/%s", event.Resource.GetKind(), sub)
}

func (g *GRPCServer) GenerateUserCerts(ctx context.Context, req *proto.UserCertsRequest) (*proto.Certs, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certs, err := auth.ServerWithRoles.GenerateUserCerts(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

func (g *GRPCServer) GenerateHostCerts(ctx context.Context, req *proto.HostCertsRequest) (*proto.Certs, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pass along the remote address the request came from to the registration function.
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.BadParameter("unable to find peer")
	}
	req.RemoteAddr = p.Addr.String()

	certs, err := auth.ServerWithRoles.GenerateHostCerts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return certs, nil
}

// DELETE IN: 12.0 (deprecated in v11, but required for back-compat with v10 clients)
func (g *GRPCServer) UnstableAssertSystemRole(ctx context.Context, req *proto.UnstableSystemRoleAssertion) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.UnstableAssertSystemRole(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &empty.Empty{}, nil
}

func (g *GRPCServer) InventoryControlStream(stream proto.AuthService_InventoryControlStreamServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}

	p, ok := peer.FromContext(stream.Context())
	if !ok {
		return trace.BadParameter("unable to find peer")
	}

	ics := client.NewUpstreamInventoryControlStream(stream, p.Addr.String())

	if err := auth.RegisterInventoryControlStream(ics); err != nil {
		return trail.ToGRPC(err)
	}

	// hold open the stream until it completes
	<-ics.Done()

	if trace.IsEOF(ics.Error()) {
		return nil
	}

	return trail.ToGRPC(ics.Error())
}

func (g *GRPCServer) GetInventoryStatus(ctx context.Context, req *proto.InventoryStatusRequest) (*proto.InventoryStatusSummary, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	rsp, err := auth.GetInventoryStatus(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &rsp, nil
}

func (g *GRPCServer) PingInventory(ctx context.Context, req *proto.InventoryPingRequest) (*proto.InventoryPingResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	rsp, err := auth.PingInventory(ctx, *req)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &rsp, nil
}

func (g *GRPCServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*types.UserV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := auth.ServerWithRoles.GetUser(req.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v2, ok := user.(*types.UserV2)
	if !ok {
		log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
		return nil, trace.Errorf("encountered unexpected user type")
	}
	return v2, nil
}

func (g *GRPCServer) GetCurrentUser(ctx context.Context, req *empty.Empty) (*types.UserV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := auth.ServerWithRoles.GetCurrentUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v2, ok := user.(*types.UserV2)
	if !ok {
		log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
		return nil, trace.Errorf("encountered unexpected user type")
	}
	return v2, nil
}

func (g *GRPCServer) GetCurrentUserRoles(_ *empty.Empty, stream proto.AuthService_GetCurrentUserRolesServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	roles, err := auth.ServerWithRoles.GetCurrentUserRoles(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	for _, role := range roles {
		v5, ok := role.(*types.RoleV5)
		if !ok {
			log.Warnf("expected type RoleV5, got %T for role %q", role, role.GetName())
			return trace.Errorf("encountered unexpected role type")
		}
		if err := stream.Send(v5); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (g *GRPCServer) GetUsers(req *proto.GetUsersRequest, stream proto.AuthService_GetUsersServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	users, err := auth.ServerWithRoles.GetUsers(req.WithSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, user := range users {
		v2, ok := user.(*types.UserV2)
		if !ok {
			log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
			return trace.Errorf("encountered unexpected user type")
		}
		if err := stream.Send(v2); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DEPRECATED, DELETE IN 11.0.0: Use GetAccessRequestsV2 instead.
func (g *GRPCServer) GetAccessRequests(ctx context.Context, f *types.AccessRequestFilter) (*proto.AccessRequests, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var filter types.AccessRequestFilter
	if f != nil {
		filter = *f
	}
	reqs, err := auth.ServerWithRoles.GetAccessRequests(ctx, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	collector := make([]*types.AccessRequestV3, 0, len(reqs))
	for _, req := range reqs {
		r, ok := req.(*types.AccessRequestV3)
		if !ok {
			err = trace.BadParameter("unexpected access request type %T", req)
			return nil, trace.Wrap(err)
		}
		collector = append(collector, r)
	}
	return &proto.AccessRequests{
		AccessRequests: collector,
	}, nil
}

func (g *GRPCServer) GetAccessRequestsV2(f *types.AccessRequestFilter, stream proto.AuthService_GetAccessRequestsV2Server) error {
	ctx := stream.Context()
	auth, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	var filter types.AccessRequestFilter
	if f != nil {
		filter = *f
	}
	reqs, err := auth.ServerWithRoles.GetAccessRequests(ctx, filter)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, req := range reqs {
		r, ok := req.(*types.AccessRequestV3)
		if !ok {
			err = trace.BadParameter("unexpected access request type %T", req)
			return trace.Wrap(err)
		}

		if err := stream.Send(r); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (g *GRPCServer) CreateAccessRequest(ctx context.Context, req *types.AccessRequestV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.CreateAccessRequest(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) DeleteAccessRequest(ctx context.Context, id *proto.RequestID) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteAccessRequest(ctx, id.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) SetAccessRequestState(ctx context.Context, req *proto.RequestStateSetter) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.Delegator != "" {
		ctx = WithDelegator(ctx, req.Delegator)
	}
	if err := auth.ServerWithRoles.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID:   req.ID,
		State:       req.State,
		Reason:      req.Reason,
		Annotations: req.Annotations,
		Roles:       req.Roles,
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) SubmitAccessReview(ctx context.Context, review *types.AccessReviewSubmission) (*types.AccessRequestV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req, err := auth.ServerWithRoles.SubmitAccessReview(ctx, *review)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := req.(*types.AccessRequestV3)
	if !ok {
		err = trace.BadParameter("unexpected access request type %T", req)
		return nil, trace.Wrap(err)
	}

	return r, nil
}

func (g *GRPCServer) GetAccessCapabilities(ctx context.Context, req *types.AccessCapabilitiesRequest) (*types.AccessCapabilities, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caps, err := auth.ServerWithRoles.GetAccessCapabilities(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return caps, nil
}

func (g *GRPCServer) CreateResetPasswordToken(ctx context.Context, req *proto.CreateResetPasswordTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req == nil {
		req = &proto.CreateResetPasswordTokenRequest{}
	}

	token, err := auth.CreateResetPasswordToken(ctx, CreateUserTokenRequest{
		Name: req.Name,
		TTL:  time.Duration(req.TTL),
		Type: req.Type,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := token.(*types.UserTokenV3)
	if !ok {
		err = trace.BadParameter("unexpected UserToken type %T", token)
		return nil, trace.Wrap(err)
	}

	return r, nil
}

func (g *GRPCServer) RotateResetPasswordTokenSecrets(ctx context.Context, req *proto.RotateUserTokenSecretsRequest) (*types.UserTokenSecretsV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenID := ""
	if req != nil {
		tokenID = req.TokenID
	}

	secrets, err := auth.RotateUserTokenSecrets(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := secrets.(*types.UserTokenSecretsV3)
	if !ok {
		err = trace.BadParameter("unexpected ResetPasswordTokenSecrets type %T", secrets)
		return nil, trace.Wrap(err)
	}

	return r, nil
}

func (g *GRPCServer) GetResetPasswordToken(ctx context.Context, req *proto.GetResetPasswordTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenID := ""
	if req != nil {
		tokenID = req.TokenID
	}

	token, err := auth.GetResetPasswordToken(ctx, tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := token.(*types.UserTokenV3)
	if !ok {
		err = trace.BadParameter("unexpected UserToken type %T", token)
		return nil, trace.Wrap(err)
	}

	return r, nil
}

// CreateBot creates a new bot and an optional join token.
func (g *GRPCServer) CreateBot(ctx context.Context, req *proto.CreateBotRequest) (*proto.CreateBotResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := auth.ServerWithRoles.CreateBot(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%q bot created", req.GetName())

	return response, nil
}

// DeleteBot removes a bot and its associated resources.
func (g *GRPCServer) DeleteBot(ctx context.Context, req *proto.DeleteBotRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.DeleteBot(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%q bot deleted", req.Name)

	return &empty.Empty{}, nil
}

// GetBotUsers lists all users with a bot label
func (g *GRPCServer) GetBotUsers(_ *proto.GetBotUsersRequest, stream proto.AuthService_GetBotUsersServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	users, err := auth.ServerWithRoles.GetBotUsers(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	for _, user := range users {
		v2, ok := user.(*types.UserV2)
		if !ok {
			log.Warnf("expected type services.UserV2, got %T for user %q", user, user.GetName())
			return trace.Errorf("encountered unexpected user type")
		}
		if err := stream.Send(v2); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetPluginData loads all plugin data matching the supplied filter.
func (g *GRPCServer) GetPluginData(ctx context.Context, filter *types.PluginDataFilter) (*proto.PluginDataSeq, error) {
	// TODO(fspmarshall): Implement rate-limiting to prevent misbehaving plugins from
	// consuming too many server resources.
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := auth.ServerWithRoles.GetPluginData(ctx, *filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var seq []*types.PluginDataV3
	for _, rsc := range data {
		d, ok := rsc.(*types.PluginDataV3)
		if !ok {
			err = trace.BadParameter("unexpected plugin data type %T", rsc)
			return nil, trace.Wrap(err)
		}
		seq = append(seq, d)
	}
	return &proto.PluginDataSeq{
		PluginData: seq,
	}, nil
}

// UpdatePluginData updates a per-resource PluginData entry.
func (g *GRPCServer) UpdatePluginData(ctx context.Context, params *types.PluginDataUpdateParams) (*empty.Empty, error) {
	// TODO(fspmarshall): Implement rate-limiting to prevent misbehaving plugins from
	// consuming too many server resources.
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.UpdatePluginData(ctx, *params); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

func (g *GRPCServer) Ping(ctx context.Context, req *proto.PingRequest) (*proto.PingResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := auth.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rsp, nil
}

// CreateUser inserts a new user entry in a backend.
func (g *GRPCServer) CreateUser(ctx context.Context, req *types.UserV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUser(req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.CreateUser(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%q user created", req.GetName())

	return &empty.Empty{}, nil
}

// UpdateUser updates an existing user in a backend.
func (g *GRPCServer) UpdateUser(ctx context.Context, req *types.UserV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateUser(req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.UpdateUser(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%q user updated", req.GetName())

	return &empty.Empty{}, nil
}

// DeleteUser deletes an existng user in a backend by username.
func (g *GRPCServer) DeleteUser(ctx context.Context, req *proto.DeleteUserRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.DeleteUser(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("%q user deleted", req.Name)

	return &empty.Empty{}, nil
}

// AcquireSemaphore acquires lease with requested resources from semaphore.
func (g *GRPCServer) AcquireSemaphore(ctx context.Context, params *types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lease, err := auth.AcquireSemaphore(ctx, *params)
	return lease, trace.Wrap(err)
}

// KeepAliveSemaphoreLease updates semaphore lease.
func (g *GRPCServer) KeepAliveSemaphoreLease(ctx context.Context, req *types.SemaphoreLease) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.KeepAliveSemaphoreLease(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CancelSemaphoreLease cancels semaphore lease early.
func (g *GRPCServer) CancelSemaphoreLease(ctx context.Context, req *types.SemaphoreLease) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CancelSemaphoreLease(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (g *GRPCServer) GetSemaphores(ctx context.Context, req *types.SemaphoreFilter) (*proto.Semaphores, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	semaphores, err := auth.GetSemaphores(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ss := make([]*types.SemaphoreV3, 0, len(semaphores))
	for _, sem := range semaphores {
		s, ok := sem.(*types.SemaphoreV3)
		if !ok {
			return nil, trace.BadParameter("unexpected semaphore type: %T", sem)
		}
		ss = append(ss, s)
	}
	return &proto.Semaphores{
		Semaphores: ss,
	}, nil
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (g *GRPCServer) DeleteSemaphore(ctx context.Context, req *types.SemaphoreFilter) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteSemaphore(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetDatabaseServers returns all registered database proxy servers.
func (g *GRPCServer) GetDatabaseServers(ctx context.Context, req *proto.GetDatabaseServersRequest) (*proto.GetDatabaseServersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databaseServers, err := auth.GetDatabaseServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var servers []*types.DatabaseServerV3
	for _, s := range databaseServers {
		server, ok := s.(*types.DatabaseServerV3)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", s)
		}
		servers = append(servers, server)
	}
	return &proto.GetDatabaseServersResponse{
		Servers: servers,
	}, nil
}

// UpsertDatabaseServer registers a new database proxy server.
func (g *GRPCServer) UpsertDatabaseServer(ctx context.Context, req *proto.UpsertDatabaseServerRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keepAlive, err := auth.UpsertDatabaseServer(ctx, req.GetServer())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteDatabaseServer removes the specified database proxy server.
func (g *GRPCServer) DeleteDatabaseServer(ctx context.Context, req *proto.DeleteDatabaseServerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteDatabaseServer(ctx, req.GetNamespace(), req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (g *GRPCServer) DeleteAllDatabaseServers(ctx context.Context, req *proto.DeleteAllDatabaseServersRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllDatabaseServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (g *GRPCServer) SignDatabaseCSR(ctx context.Context, req *proto.DatabaseCSRRequest) (*proto.DatabaseCSRResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.SignDatabaseCSR(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GenerateDatabaseCert generates client certificate used by a database
// service to authenticate with the database instance.
func (g *GRPCServer) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.GenerateDatabaseCert(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GenerateSnowflakeJWT generates JWT in the format required by Snowflake.
func (g *GRPCServer) GenerateSnowflakeJWT(ctx context.Context, req *proto.SnowflakeJWTRequest) (*proto.SnowflakeJWTResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.GenerateSnowflakeJWT(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GetApplicationServers returns all registered application servers.
func (g *GRPCServer) GetApplicationServers(ctx context.Context, req *proto.GetApplicationServersRequest) (*proto.GetApplicationServersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serversI, err := auth.GetApplicationServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var servers []*types.AppServerV3
	for _, serverI := range serversI {
		server, ok := serverI.(*types.AppServerV3)
		if !ok {
			return nil, trace.BadParameter("expected application server type *types.AppServerV3, got %T", serverI)
		}
		servers = append(servers, server)
	}
	return &proto.GetApplicationServersResponse{
		Servers: servers,
	}, nil
}

// UpsertApplicationServer registers an application server.
func (g *GRPCServer) UpsertApplicationServer(ctx context.Context, req *proto.UpsertApplicationServerRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keepAlive, err := auth.UpsertApplicationServer(ctx, req.GetServer())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteApplicationServer deletes an application server.
func (g *GRPCServer) DeleteApplicationServer(ctx context.Context, req *proto.DeleteApplicationServerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteApplicationServer(ctx, req.GetNamespace(), req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllApplicationServers deletes all registered application servers.
func (g *GRPCServer) DeleteAllApplicationServers(ctx context.Context, req *proto.DeleteAllApplicationServersRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllApplicationServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetAppServers gets all application servers.
//
// DELETE IN 9.0. Deprecated, use GetApplicationServers.
func (g *GRPCServer) GetAppServers(ctx context.Context, req *proto.GetAppServersRequest) (*proto.GetAppServersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appServers, err := auth.GetAppServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var servers []*types.ServerV2
	for _, s := range appServers {
		server, ok := s.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", s)
		}
		servers = append(servers, server)
	}

	return &proto.GetAppServersResponse{
		Servers: servers,
	}, nil
}

// UpsertAppServer adds an application server.
//
// DELETE IN 9.0. Deprecated, use UpsertApplicationServer.
func (g *GRPCServer) UpsertAppServer(ctx context.Context, req *proto.UpsertAppServerRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keepAlive, err := auth.UpsertAppServer(ctx, req.GetServer())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteAppServer removes an application server.
//
// DELETE IN 9.0. Deprecated, use DeleteApplicationServer.
func (g *GRPCServer) DeleteAppServer(ctx context.Context, req *proto.DeleteAppServerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteAppServer(ctx, req.GetNamespace(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllAppServers removes all application servers.
//
// DELETE IN 9.0. Deprecated, use DeleteAllApplicationServers.
func (g *GRPCServer) DeleteAllAppServers(ctx context.Context, req *proto.DeleteAllAppServersRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteAllAppServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// GetAppSession gets an application web session.
func (g *GRPCServer) GetAppSession(ctx context.Context, req *proto.GetAppSessionRequest) (*proto.GetAppSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := auth.GetAppSession(ctx, types.GetAppSessionRequest{
		SessionID: req.GetSessionID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := session.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", session)
	}

	return &proto.GetAppSessionResponse{
		Session: sess,
	}, nil
}

// GetAppSessions gets all application web sessions.
func (g *GRPCServer) GetAppSessions(ctx context.Context, _ *empty.Empty) (*proto.GetAppSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := auth.GetAppSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*types.WebSessionV2
	for _, session := range sessions {
		sess, ok := session.(*types.WebSessionV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", session)
		}
		out = append(out, sess)
	}

	return &proto.GetAppSessionsResponse{
		Sessions: out,
	}, nil
}

func (g *GRPCServer) GetSnowflakeSession(ctx context.Context, req *proto.GetSnowflakeSessionRequest) (*proto.GetSnowflakeSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	snowflakeSession, err := auth.GetSnowflakeSession(ctx, types.GetSnowflakeSessionRequest{SessionID: req.GetSessionID()})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := snowflakeSession.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", snowflakeSession)
	}

	return &proto.GetSnowflakeSessionResponse{
		Session: sess,
	}, nil
}

func (g *GRPCServer) GetSnowflakeSessions(ctx context.Context, e *empty.Empty) (*proto.GetSnowflakeSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := auth.GetSnowflakeSessions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*types.WebSessionV2
	for _, session := range sessions {
		sess, ok := session.(*types.WebSessionV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", session)
		}
		out = append(out, sess)
	}

	return &proto.GetSnowflakeSessionsResponse{
		Sessions: out,
	}, nil
}

func (g *GRPCServer) DeleteSnowflakeSession(ctx context.Context, req *proto.DeleteSnowflakeSessionRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteSnowflakeSession(ctx, types.DeleteSnowflakeSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

func (g *GRPCServer) DeleteAllSnowflakeSessions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAllSnowflakeSessions(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (g *GRPCServer) CreateAppSession(ctx context.Context, req *proto.CreateAppSessionRequest) (*proto.CreateAppSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := auth.CreateAppSession(ctx, types.CreateAppSessionRequest{
		Username:    req.GetUsername(),
		PublicAddr:  req.GetPublicAddr(),
		ClusterName: req.GetClusterName(),
		AWSRoleARN:  req.GetAWSRoleARN(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := session.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", session)
	}

	return &proto.CreateAppSessionResponse{
		Session: sess,
	}, nil
}

func (g *GRPCServer) CreateSnowflakeSession(ctx context.Context, req *proto.CreateSnowflakeSessionRequest) (*proto.CreateSnowflakeSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	snowflakeSession, err := auth.CreateSnowflakeSession(ctx, types.CreateSnowflakeSessionRequest{
		Username:     req.GetUsername(),
		SessionToken: req.GetSessionToken(),
		TokenTTL:     time.Duration(req.TokenTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := snowflakeSession.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", snowflakeSession)
	}

	return &proto.CreateSnowflakeSessionResponse{
		Session: sess,
	}, nil
}

// DeleteAppSession removes an application web session.
func (g *GRPCServer) DeleteAppSession(ctx context.Context, req *proto.DeleteAppSessionRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAppSession(ctx, types.DeleteAppSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllAppSessions removes all application web sessions.
func (g *GRPCServer) DeleteAllAppSessions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAllAppSessions(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteUserAppSessions removes user's all application web sessions.
func (g *GRPCServer) DeleteUserAppSessions(ctx context.Context, req *proto.DeleteUserAppSessionsRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteUserAppSessions(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// GenerateAppToken creates a JWT token with application access.
func (g GRPCServer) GenerateAppToken(ctx context.Context, req *proto.GenerateAppTokenRequest) (*proto.GenerateAppTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := auth.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username: req.Username,
		Roles:    req.Roles,
		URI:      req.URI,
		Expires:  req.Expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.GenerateAppTokenResponse{
		Token: token,
	}, nil
}

// GetWebSession gets a web session.
func (g *GRPCServer) GetWebSession(ctx context.Context, req *types.GetWebSessionRequest) (*proto.GetWebSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := auth.WebSessions().Get(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := session.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", session)
	}

	return &proto.GetWebSessionResponse{
		Session: sess,
	}, nil
}

// GetWebSessions gets all web sessions.
func (g *GRPCServer) GetWebSessions(ctx context.Context, _ *empty.Empty) (*proto.GetWebSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, err := auth.WebSessions().List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*types.WebSessionV2
	for _, session := range sessions {
		sess, ok := session.(*types.WebSessionV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", session)
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
		return nil, trace.Wrap(err)
	}

	if err := auth.WebSessions().Delete(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllWebSessions removes all web sessions.
func (g *GRPCServer) DeleteAllWebSessions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebSessions().DeleteAll(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// GetWebToken gets a web token.
func (g *GRPCServer) GetWebToken(ctx context.Context, req *types.GetWebTokenRequest) (*proto.GetWebTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := auth.WebTokens().Get(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, ok := resp.(*types.WebTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected web token type %T", resp)
	}

	return &proto.GetWebTokenResponse{
		Token: token,
	}, nil
}

// GetWebTokens gets all web tokens.
func (g *GRPCServer) GetWebTokens(ctx context.Context, _ *empty.Empty) (*proto.GetWebTokensResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokens, err := auth.WebTokens().List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []*types.WebTokenV3
	for _, t := range tokens {
		token, ok := t.(*types.WebTokenV3)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", t)
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
		return nil, trace.Wrap(err)
	}

	if err := auth.WebTokens().Delete(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllWebTokens removes all web tokens.
func (g *GRPCServer) DeleteAllWebTokens(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebTokens().DeleteAll(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// UpdateRemoteCluster updates remote cluster
func (g *GRPCServer) UpdateRemoteCluster(ctx context.Context, req *types.RemoteClusterV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpdateRemoteCluster(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetKubeServices gets all kubernetes services.
func (g *GRPCServer) GetKubeServices(ctx context.Context, req *proto.GetKubeServicesRequest) (*proto.GetKubeServicesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeServices, err := auth.GetKubeServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var servers []*types.ServerV2
	for _, s := range kubeServices {
		server, ok := s.(*types.ServerV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", s)
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
		return nil, trace.Wrap(err)
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
		return nil, trace.Wrap(err)
	}
	return new(empty.Empty), nil
}

// UpsertKubeServiceV2 adds a kubernetes service
func (g *GRPCServer) UpsertKubeServiceV2(ctx context.Context, req *proto.UpsertKubeServiceRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
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

	keepAlive, err := auth.UpsertKubeServiceV2(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteKubeService removes a kubernetes service.
func (g *GRPCServer) DeleteKubeService(ctx context.Context, req *proto.DeleteKubeServiceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteKubeService(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteAllKubeServices removes all kubernetes services.
func (g *GRPCServer) DeleteAllKubeServices(ctx context.Context, req *proto.DeleteAllKubeServicesRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteAllKubeServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// downgradeRole tests the client version passed through the GRPC metadata, and
// if the client version is unknown or less than the minimum supported version
// for V5 roles returns a shallow copy of the given role downgraded to V4, If
// the passed in role is already V4, it is returned unmodified.
func downgradeRole(ctx context.Context, role *types.RoleV5) (*types.RoleV5, error) {
	if role.Version == types.V4 {
		// role is already V4, no need to downgrade
		return role, nil
	}

	var clientVersion *semver.Version
	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if ok {
		var err error
		clientVersion, err = semver.NewVersion(clientVersionString)
		if err != nil {
			return nil, trace.BadParameter("unrecognized client version: %s is not a valid semver", clientVersionString)
		}
	}

	if clientVersion == nil || clientVersion.LessThan(*MinSupportedModeratedSessionsVersion) {
		log.Debugf(`Client version "%s" is unknown or less than 9.0.0, converting role to v4`, clientVersionString)
		downgraded, err := services.DowngradeRoleToV4(role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return downgraded, nil
	}
	return role, nil
}

// GetRole retrieves a role by name.
func (g *GRPCServer) GetRole(ctx context.Context, req *proto.GetRoleRequest) (*types.RoleV5, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := auth.ServerWithRoles.GetRole(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleV5, ok := role.(*types.RoleV5)
	if !ok {
		return nil, trace.Errorf("encountered unexpected role type")
	}
	downgraded, err := downgradeRole(ctx, roleV5)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return downgraded, nil
}

// GetRoles retrieves all roles.
func (g *GRPCServer) GetRoles(ctx context.Context, _ *empty.Empty) (*proto.GetRolesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles, err := auth.ServerWithRoles.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rolesV5 []*types.RoleV5
	for _, r := range roles {
		role, ok := r.(*types.RoleV5)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", r)
		}
		downgraded, err := downgradeRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rolesV5 = append(rolesV5, downgraded)
	}
	return &proto.GetRolesResponse{
		Roles: rolesV5,
	}, nil
}

// UpsertRole upserts a role.
func (g *GRPCServer) UpsertRole(ctx context.Context, role *types.RoleV5) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = services.ValidateRole(role); err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.ServerWithRoles.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	g.Debugf("%q role upserted", role.GetName())

	return &empty.Empty{}, nil
}

// DeleteRole deletes a role by name.
func (g *GRPCServer) DeleteRole(ctx context.Context, req *proto.DeleteRoleRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteRole(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	g.Debugf("%q role deleted", req.GetName())

	return &empty.Empty{}, nil
}

// doMFAPresenceChallenge conducts an MFA presence challenge over a stream
// and updates the users presence for a given session.
//
// This function bypasses the `ServerWithRoles` RBAC layer. This is not
// usually how the GRPC layer accesses the underlying auth server API's but it's done
// here to avoid bloating the ClientI interface with special logic that isn't designed to be touched
// by anyone external to this process. This is not the norm and caution should be taken
// when looking at or modifying this function. This is the same approach taken by other MFA
// related GRPC API endpoints.
func doMFAPresenceChallenge(ctx context.Context, actx *grpcContext, stream proto.AuthService_MaintainSessionPresenceServer, challengeReq *proto.PresenceMFAChallengeRequest) error {
	user := actx.User.GetName()

	const passwordless = false
	authChallenge, err := actx.authServer.mfaAuthChallenge(ctx, user, passwordless)
	if err != nil {
		return trace.Wrap(err)
	}
	if authChallenge.WebauthnChallenge == nil {
		return trace.BadParameter("no MFA devices registered for %q", user)
	}

	if err := stream.Send(authChallenge); err != nil {
		return trace.Wrap(err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	challengeResp := resp.GetChallengeResponse()
	if challengeResp == nil {
		return trace.BadParameter("expected MFAAuthenticateResponse, got %T", challengeResp)
	}

	if _, _, err := actx.authServer.validateMFAAuthResponse(ctx, challengeResp, user, passwordless); err != nil {
		return trace.Wrap(err)
	}

	err = actx.authServer.UpdatePresence(ctx, challengeReq.SessionID, user)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// MaintainSessionPresence establishes a channel used to continuously verify the presence for a session.
func (g *GRPCServer) MaintainSessionPresence(stream proto.AuthService_MaintainSessionPresenceServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return trace.Wrap(err)
		}

		challengeReq := req.GetChallengeRequest()
		if challengeReq == nil {
			return trace.BadParameter("expected PresenceMFAChallengeRequest, got %T", req)
		}

		err = doMFAPresenceChallenge(ctx, actx, stream, challengeReq)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func (g *GRPCServer) AddMFADevice(stream proto.AuthService_AddMFADeviceServer) error {
	actx, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
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
		return trace.Wrap(err)
	}

	// 2. send ExistingMFAChallenge
	// 3. receive and validate ExistingMFAResponse
	if err := addMFADeviceAuthChallenge(actx, stream); err != nil {
		return trace.Wrap(err)
	}

	// 4. send MFARegisterChallenge
	// 5. receive and validate MFARegisterResponse
	dev, err := addMFADeviceRegisterChallenge(actx, stream, initReq)
	if err != nil {
		return trace.Wrap(err)
	}

	clusterName, err := actx.GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := g.Emitter.EmitAuditEvent(g.serverContext(), &apievents.MFADeviceAdd{
		Metadata: apievents.Metadata{
			Type:        events.MFADeviceAddEvent,
			Code:        events.MFADeviceAddEventCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata:      actx.Identity.GetIdentity().GetUserMetadata(),
		MFADeviceMetadata: mfaDeviceEventMetadata(dev),
	}); err != nil {
		return trace.Wrap(err)
	}

	// 6. send Ack
	if err := stream.Send(&proto.AddMFADeviceResponse{
		Response: &proto.AddMFADeviceResponse_Ack{Ack: &proto.AddMFADeviceResponseAck{Device: dev}},
	}); err != nil {
		return trace.Wrap(err)
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
	devs, err := gctx.authServer.Identity.GetMFADevices(stream.Context(), gctx.User.GetName(), false)
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

	// Note: authChallenge may be empty if this user has no existing MFA devices.
	const passwordless = false
	authChallenge, err := auth.mfaAuthChallenge(ctx, user, passwordless)
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
	if authChallenge.TOTP != nil || authChallenge.WebauthnChallenge != nil {
		if _, _, err := auth.validateMFAAuthResponse(ctx, authResp, user, passwordless); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func addMFADeviceRegisterChallenge(gctx *grpcContext, stream proto.AuthService_AddMFADeviceServer, initReq *proto.AddMFADeviceRequestInit) (*types.MFADevice, error) {
	auth := gctx.authServer
	user := gctx.User.GetName()
	ctx := stream.Context()

	// Keep Webauthn session data in memory, we can afford that for the streaming
	// RPCs.
	webIdentity := wanlib.WithInMemorySessionData(auth.Identity)

	// Send registration challenge for the requested device type.
	regChallenge := new(proto.MFARegisterChallenge)

	res, err := auth.createRegisterChallenge(ctx, &newRegisterChallengeRequest{
		username:            user,
		deviceType:          initReq.DeviceType,
		deviceUsage:         initReq.DeviceUsage,
		webIdentityOverride: webIdentity,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	regChallenge.Request = res.GetRequest()

	if err := stream.Send(&proto.AddMFADeviceResponse{
		Response: &proto.AddMFADeviceResponse_NewMFARegisterChallenge{NewMFARegisterChallenge: regChallenge},
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	// 5. receive client MFARegisterResponse
	req, err := stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	regResp := req.GetNewMFARegisterResponse()
	if regResp == nil {
		return nil, trace.BadParameter("expected MFARegistrationResponse, got %T", req)
	}

	// Validate MFARegisterResponse and upsert the new device on success.
	dev, err := auth.verifyMFARespAndAddDevice(ctx, &newMFADeviceFields{
		username:            user,
		newDeviceName:       initReq.DeviceName,
		totpSecret:          regChallenge.GetTOTP().GetSecret(),
		webIdentityOverride: webIdentity,
		deviceResp:          regResp,
		deviceUsage:         initReq.DeviceUsage,
	})

	return dev, trace.Wrap(err)
}

func (g *GRPCServer) DeleteMFADevice(stream proto.AuthService_DeleteMFADeviceServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
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
		return trace.Wrap(err)
	}
	initReq := req.GetInit()
	if initReq == nil {
		return trace.BadParameter("expected DeleteMFADeviceRequestInit, got %T", req)
	}

	// 2. send MFAAuthenticateChallenge
	// 3. receive and validate MFAAuthenticateResponse
	if err := deleteMFADeviceAuthChallenge(actx, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := auth.deleteMFADeviceSafely(ctx, user, initReq.DeviceName); err != nil {
		return trace.Wrap(err)
	}

	// 4. send Ack
	return trace.Wrap(stream.Send(&proto.DeleteMFADeviceResponse{
		Response: &proto.DeleteMFADeviceResponse_Ack{Ack: &proto.DeleteMFADeviceResponseAck{}},
	}))
}

func deleteMFADeviceAuthChallenge(gctx *grpcContext, stream proto.AuthService_DeleteMFADeviceServer) error {
	ctx := stream.Context()
	auth := gctx.authServer
	user := gctx.User.GetName()

	const passwordless = false
	authChallenge, err := auth.mfaAuthChallenge(ctx, user, passwordless)
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
	if _, _, err := auth.validateMFAAuthResponse(ctx, authResp, user, passwordless); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func mfaDeviceEventMetadata(d *types.MFADevice) apievents.MFADeviceMetadata {
	return apievents.MFADeviceMetadata{
		DeviceName: d.Metadata.Name,
		DeviceID:   d.Id,
		DeviceType: d.MFAType(),
	}
}

// AddMFADeviceSync is implemented by AuthService.AddMFADeviceSync.
func (g *GRPCServer) AddMFADeviceSync(ctx context.Context, req *proto.AddMFADeviceSyncRequest) (*proto.AddMFADeviceSyncResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := actx.ServerWithRoles.AddMFADeviceSync(ctx, req)
	return res, trace.Wrap(err)
}

// DeleteMFADeviceSync is implemented by AuthService.DeleteMFADeviceSync.
func (g *GRPCServer) DeleteMFADeviceSync(ctx context.Context, req *proto.DeleteMFADeviceSyncRequest) (*empty.Empty, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := actx.ServerWithRoles.DeleteMFADeviceSync(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

func (g *GRPCServer) GetMFADevices(ctx context.Context, req *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devs, err := actx.ServerWithRoles.GetMFADevices(ctx, req)
	return devs, trace.Wrap(err)
}

func (g *GRPCServer) GenerateUserSingleUseCerts(stream proto.AuthService_GenerateUserSingleUseCertsServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
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
		return trace.Wrap(err)
	}
	initReq := req.GetInit()
	if initReq == nil {
		return trace.BadParameter("expected UserCertsRequest, got %T", req.Request)
	}
	if err := validateUserSingleUseCertRequest(ctx, actx, initReq); err != nil {
		g.Entry.Debugf("Validation of single-use cert request failed: %v", err)
		return trace.Wrap(err)
	}

	// 2. send MFAChallenge
	// 3. receive and validate MFAResponse
	mfaDev, err := userSingleUseCertsAuthChallenge(actx, stream)
	if err != nil {
		g.Entry.Debugf("Failed to perform single-use cert challenge: %v", err)
		return trace.Wrap(err)
	}

	// Generate the cert.
	respCert, err := userSingleUseCertsGenerate(ctx, actx, *initReq, mfaDev)
	if err != nil {
		g.Entry.Warningf("Failed to generate single-use cert: %v", err)
		return trace.Wrap(err)
	}

	// 4. send Certs
	if err := stream.Send(&proto.UserSingleUseCertsResponse{
		Response: &proto.UserSingleUseCertsResponse_Cert{Cert: respCert},
	}); err != nil {
		return trace.Wrap(err)
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
	case proto.UserCertsRequest_WindowsDesktop:
		if req.RouteToWindowsDesktop.WindowsDesktop == "" {
			return trace.BadParameter("missing WindowsDesktop field in a windows-desktop-only UserCertsRequest")
		}
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

	const passwordless = false
	challenge, err := auth.mfaAuthChallenge(ctx, user, passwordless)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if challenge.TOTP == nil && challenge.WebauthnChallenge == nil {
		return nil, trace.AccessDenied("MFA is required to access this resource but user has no MFA devices; use 'tsh mfa add' to register MFA devices")
	}
	if err := stream.Send(&proto.UserSingleUseCertsResponse{
		Response: &proto.UserSingleUseCertsResponse_MFAChallenge{MFAChallenge: challenge},
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
	mfaDev, _, err := auth.validateMFAAuthResponse(ctx, authResp, user, passwordless)
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
	case proto.UserCertsRequest_Kubernetes, proto.UserCertsRequest_Database, proto.UserCertsRequest_WindowsDesktop:
		resp.Cert = &proto.SingleUseUserCert_TLS{TLS: certs.TLS}
	default:
		return nil, trace.BadParameter("unknown certificate usage %q", req.Usage)
	}
	return resp, nil
}

func (g *GRPCServer) IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := actx.IsMFARequired(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// GetOIDCConnector retrieves an OIDC connector by name.
func (g *GRPCServer) GetOIDCConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.OIDCConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	oc, err := auth.ServerWithRoles.GetOIDCConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connector, ok := oc.(*types.OIDCConnectorV3)
	if !ok {
		return nil, trace.Errorf("encountered unexpected OIDC connector type %T", oc)
	}
	return connector, nil
}

// GetOIDCConnectors retrieves all OIDC connectors.
func (g *GRPCServer) GetOIDCConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.OIDCConnectorV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ocs, err := auth.ServerWithRoles.GetOIDCConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]*types.OIDCConnectorV3, len(ocs))
	for i, oc := range ocs {
		var ok bool
		if connectors[i], ok = oc.(*types.OIDCConnectorV3); !ok {
			return nil, trace.Errorf("encountered unexpected OIDC connector type %T", oc)
		}
	}
	return &types.OIDCConnectorV3List{
		OIDCConnectors: connectors,
	}, nil
}

// UpsertOIDCConnector upserts an OIDC connector.
func (g *GRPCServer) UpsertOIDCConnector(ctx context.Context, oidcConnector *types.OIDCConnectorV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.UpsertOIDCConnector(ctx, oidcConnector); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (g *GRPCServer) DeleteOIDCConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteOIDCConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CreateOIDCAuthRequest creates OIDCAuthRequest
func (g *GRPCServer) CreateOIDCAuthRequest(ctx context.Context, req *types.OIDCAuthRequest) (*types.OIDCAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateOIDCAuthRequest(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GetOIDCAuthRequest gets OIDC AuthnRequest
func (g *GRPCServer) GetOIDCAuthRequest(ctx context.Context, req *proto.GetOIDCAuthRequestRequest) (*types.OIDCAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request, err := auth.ServerWithRoles.GetOIDCAuthRequest(ctx, req.StateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return request, nil
}

// GetSAMLConnector retrieves a SAML connector by name.
func (g *GRPCServer) GetSAMLConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.SAMLConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sc, err := auth.ServerWithRoles.GetSAMLConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	samlConnectorV2, ok := sc.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected SAML connector type: %T", sc)
	}
	return samlConnectorV2, nil
}

// GetSAMLConnectors retrieves all SAML connectors.
func (g *GRPCServer) GetSAMLConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.SAMLConnectorV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	scs, err := auth.ServerWithRoles.GetSAMLConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	samlConnectorsV2 := make([]*types.SAMLConnectorV2, len(scs))
	for i, sc := range scs {
		var ok bool
		if samlConnectorsV2[i], ok = sc.(*types.SAMLConnectorV2); !ok {
			return nil, trace.Errorf("encountered unexpected SAML connector type: %T", sc)
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
		return nil, trace.Wrap(err)
	}
	if err = services.ValidateSAMLConnector(samlConnector); err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.UpsertSAMLConnector(ctx, samlConnector); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (g *GRPCServer) DeleteSAMLConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteSAMLConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CreateSAMLAuthRequest creates SAMLAuthRequest.
func (g *GRPCServer) CreateSAMLAuthRequest(ctx context.Context, req *types.SAMLAuthRequest) (*types.SAMLAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateSAMLAuthRequest(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GetSAMLAuthRequest gets a SAMLAuthRequest by id.
func (g *GRPCServer) GetSAMLAuthRequest(ctx context.Context, req *proto.GetSAMLAuthRequestRequest) (*types.SAMLAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request, err := auth.GetSAMLAuthRequest(ctx, req.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return request, nil
}

// GetGithubConnector retrieves a Github connector by name.
func (g *GRPCServer) GetGithubConnector(ctx context.Context, req *types.ResourceWithSecretsRequest) (*types.GithubConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gc, err := auth.ServerWithRoles.GetGithubConnector(ctx, req.Name, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubConnectorV3, ok := gc.(*types.GithubConnectorV3)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Github connector type: %T", gc)
	}
	return githubConnectorV3, nil
}

// GetGithubConnectors retrieves all Github connectors.
func (g *GRPCServer) GetGithubConnectors(ctx context.Context, req *types.ResourcesWithSecretsRequest) (*types.GithubConnectorV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gcs, err := auth.ServerWithRoles.GetGithubConnectors(ctx, req.WithSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubConnectorsV3 := make([]*types.GithubConnectorV3, len(gcs))
	for i, gc := range gcs {
		var ok bool
		if githubConnectorsV3[i], ok = gc.(*types.GithubConnectorV3); !ok {
			return nil, trace.Errorf("encountered unexpected Github connector type: %T", gc)
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
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.UpsertGithubConnector(ctx, GithubConnector); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteGithubConnector deletes a Github connector by name.
func (g *GRPCServer) DeleteGithubConnector(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteGithubConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CreateGithubAuthRequest creates GithubAuthRequest.
func (g *GRPCServer) CreateGithubAuthRequest(ctx context.Context, req *types.GithubAuthRequest) (*types.GithubAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.CreateGithubAuthRequest(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// GetGithubAuthRequest gets a GithubAuthRequest by id.
func (g *GRPCServer) GetGithubAuthRequest(ctx context.Context, req *proto.GetGithubAuthRequestRequest) (*types.GithubAuthRequest, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request, err := auth.GetGithubAuthRequest(ctx, req.StateToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return request, nil
}

// GetSSODiagnosticInfo gets a SSO diagnostic info for a specific SSO auth request.
func (g *GRPCServer) GetSSODiagnosticInfo(ctx context.Context, req *proto.GetSSODiagnosticInfoRequest) (*types.SSODiagnosticInfo, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	info, err := auth.GetSSODiagnosticInfo(ctx, req.AuthRequestKind, req.AuthRequestID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return info, nil
}

// GetTrustedCluster retrieves a Trusted Cluster by name.
func (g *GRPCServer) GetTrustedCluster(ctx context.Context, req *types.ResourceRequest) (*types.TrustedClusterV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := auth.ServerWithRoles.GetTrustedCluster(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Trusted Cluster type %T", tc)
	}
	return trustedClusterV2, nil
}

// GetTrustedClusters retrieves all Trusted Clusters.
func (g *GRPCServer) GetTrustedClusters(ctx context.Context, _ *empty.Empty) (*types.TrustedClusterV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tcs, err := auth.ServerWithRoles.GetTrustedClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClustersV2 := make([]*types.TrustedClusterV2, len(tcs))
	for i, tc := range tcs {
		var ok bool
		if trustedClustersV2[i], ok = tc.(*types.TrustedClusterV2); !ok {
			return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
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
		return nil, trace.Wrap(err)
	}
	if err = services.ValidateTrustedCluster(cluster); err != nil {
		return nil, trace.Wrap(err)
	}
	tc, err := auth.ServerWithRoles.UpsertTrustedCluster(ctx, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedClusterV2, ok := tc.(*types.TrustedClusterV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected Trusted Cluster type: %T", tc)
	}
	return trustedClusterV2, nil
}

// DeleteTrustedCluster deletes a Trusted Cluster by name.
func (g *GRPCServer) DeleteTrustedCluster(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteTrustedCluster(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetToken retrieves a token by name.
func (g *GRPCServer) GetToken(ctx context.Context, req *types.ResourceRequest) (*types.ProvisionTokenV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	t, err := auth.ServerWithRoles.GetToken(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provisionTokenV2, ok := t.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected token type: %T", t)
	}
	return provisionTokenV2, nil
}

// GetTokens retrieves all tokens.
func (g *GRPCServer) GetTokens(ctx context.Context, _ *empty.Empty) (*types.ProvisionTokenV2List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ts, err := auth.ServerWithRoles.GetTokens(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provisionTokensV2 := make([]*types.ProvisionTokenV2, len(ts))
	for i, t := range ts {
		var ok bool
		if provisionTokensV2[i], ok = t.(*types.ProvisionTokenV2); !ok {
			return nil, trace.Errorf("encountered unexpected token type: %T", t)
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
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.UpsertToken(ctx, token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GenerateToken generates a new auth token.
func (g *GRPCServer) GenerateToken(ctx context.Context, req *proto.GenerateTokenRequest) (*proto.GenerateTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := auth.ServerWithRoles.GenerateToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.GenerateTokenResponse{Token: token}, nil
}

// DeleteToken deletes a token by name.
func (g *GRPCServer) DeleteToken(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteToken(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetNode retrieves a node by name and namespace.
func (g *GRPCServer) GetNode(ctx context.Context, req *types.ResourceInNamespaceRequest) (*types.ServerV2, error) {
	if req.Namespace == "" {
		return nil, trace.BadParameter("missing parameter namespace")
	}
	if req.Name == "" {
		return nil, trace.BadParameter("missing parameter name")
	}
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	node, err := auth.ServerWithRoles.GetNode(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverV2, ok := node.(*types.ServerV2)
	if !ok {
		return nil, trace.Errorf("encountered unexpected node type: %T", node)
	}
	return serverV2, nil
}

// UpsertNode upserts a node.
func (g *GRPCServer) UpsertNode(ctx context.Context, node *types.ServerV2) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extract peer (remote host) from context and if the node sent 0.0.0.0 as
	// its address (meaning it did not set an advertise address) update it with
	// the address of the peer.
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.BadParameter("unable to find peer")
	}
	node.SetAddr(utils.ReplaceLocalhost(node.GetAddr(), p.Addr.String()))

	keepAlive, err := auth.ServerWithRoles.UpsertNode(ctx, node)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteNode deletes a node by name.
func (g *GRPCServer) DeleteNode(ctx context.Context, req *types.ResourceInNamespaceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.DeleteNode(ctx, req.Namespace, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllNodes deletes all nodes in a given namespace.
func (g *GRPCServer) DeleteAllNodes(ctx context.Context, req *types.ResourcesInNamespaceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.DeleteAllNodes(ctx, req.Namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetClusterAuditConfig gets cluster audit configuration.
func (g *GRPCServer) GetClusterAuditConfig(ctx context.Context, _ *empty.Empty) (*types.ClusterAuditConfigV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auditConfig, err := auth.ServerWithRoles.GetClusterAuditConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auditConfigV2, ok := auditConfig.(*types.ClusterAuditConfigV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", auditConfig)
	}
	return auditConfigV2, nil
}

// GetClusterNetworkingConfig gets cluster networking configuration.
func (g *GRPCServer) GetClusterNetworkingConfig(ctx context.Context, _ *empty.Empty) (*types.ClusterNetworkingConfigV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfig, err := auth.ServerWithRoles.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfigV2, ok := netConfig.(*types.ClusterNetworkingConfigV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", netConfig)
	}
	return netConfigV2, nil
}

// SetClusterNetworkingConfig sets cluster networking configuration.
func (g *GRPCServer) SetClusterNetworkingConfig(ctx context.Context, netConfig *types.ClusterNetworkingConfigV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfig.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetClusterNetworkingConfig(ctx, netConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// ResetClusterNetworkingConfig resets cluster networking configuration to defaults.
func (g *GRPCServer) ResetClusterNetworkingConfig(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetClusterNetworkingConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetSessionRecordingConfig gets session recording configuration.
func (g *GRPCServer) GetSessionRecordingConfig(ctx context.Context, _ *empty.Empty) (*types.SessionRecordingConfigV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfig, err := auth.ServerWithRoles.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfigV2, ok := recConfig.(*types.SessionRecordingConfigV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", recConfig)
	}
	return recConfigV2, nil
}

// SetSessionRecordingConfig sets session recording configuration.
func (g *GRPCServer) SetSessionRecordingConfig(ctx context.Context, recConfig *types.SessionRecordingConfigV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfig.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetSessionRecordingConfig(ctx, recConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// ResetSessionRecordingConfig resets session recording configuration to defaults.
func (g *GRPCServer) ResetSessionRecordingConfig(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetSessionRecordingConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetAuthPreference gets cluster auth preference.
func (g *GRPCServer) GetAuthPreference(ctx context.Context, _ *empty.Empty) (*types.AuthPreferenceV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPref, err := auth.ServerWithRoles.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPrefV2, ok := authPref.(*types.AuthPreferenceV2)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected type %T", authPref))
	}
	return authPrefV2, nil
}

// SetAuthPreference sets cluster auth preference.
func (g *GRPCServer) SetAuthPreference(ctx context.Context, authPref *types.AuthPreferenceV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPref.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetAuthPreference(ctx, authPref); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// ResetAuthPreference resets cluster auth preference to defaults.
func (g *GRPCServer) ResetAuthPreference(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetAuthPreference(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (g *GRPCServer) StreamSessionEvents(req *proto.StreamSessionEventsRequest, stream proto.AuthService_StreamSessionEventsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	c, e := auth.ServerWithRoles.StreamSessionEvents(stream.Context(), session.ID(req.SessionID), int64(req.StartIndex))

	for {
		select {
		case event, more := <-c:
			if !more {
				return nil
			}

			oneOf, err := apievents.ToOneOf(event)
			if err != nil {
				return trail.ToGRPC(trace.Wrap(err))
			}

			if err := stream.Send(oneOf); err != nil {
				return trail.ToGRPC(trace.Wrap(err))
			}
		case err := <-e:
			return trail.ToGRPC(trace.Wrap(err))
		}
	}
}

// GetNetworkRestrictions retrieves all the network restrictions (allow/deny lists).
func (g *GRPCServer) GetNetworkRestrictions(ctx context.Context, _ *empty.Empty) (*types.NetworkRestrictionsV4, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	nr, err := auth.ServerWithRoles.GetNetworkRestrictions(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}
	restrictionsV4, ok := nr.(*types.NetworkRestrictionsV4)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected type %T", nr))
	}
	return restrictionsV4, nil
}

// SetNetworkRestrictions updates the network restrictions.
func (g *GRPCServer) SetNetworkRestrictions(ctx context.Context, nr *types.NetworkRestrictionsV4) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err = auth.ServerWithRoles.SetNetworkRestrictions(ctx, nr); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// DeleteNetworkRestrictions deletes the network restrictions.
func (g *GRPCServer) DeleteNetworkRestrictions(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err = auth.ServerWithRoles.DeleteNetworkRestrictions(ctx); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &empty.Empty{}, nil
}

// GetEvents searches for events on the backend and sends them back in a response.
func (g *GRPCServer) GetEvents(ctx context.Context, req *proto.GetEventsRequest) (*proto.Events, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastkey, err := auth.ServerWithRoles.SearchEvents(req.StartDate, req.EndDate, req.Namespace, req.EventTypes, int(req.Limit), types.EventOrder(req.Order), req.StartKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res *proto.Events = &proto.Events{}

	encodedEvents := make([]*apievents.OneOf, 0, len(rawEvents))

	for _, rawEvent := range rawEvents {
		event, err := apievents.ToOneOf(rawEvent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		encodedEvents = append(encodedEvents, event)
	}

	res.Items = encodedEvents
	res.LastKey = lastkey
	return res, nil
}

// GetSessionEvents searches for session events on the backend and sends them back in a response.
func (g *GRPCServer) GetSessionEvents(ctx context.Context, req *proto.GetSessionEventsRequest) (*proto.Events, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastkey, err := auth.ServerWithRoles.SearchSessionEvents(req.StartDate, req.EndDate, int(req.Limit), types.EventOrder(req.Order), req.StartKey, nil, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res *proto.Events = &proto.Events{}

	encodedEvents := make([]*apievents.OneOf, 0, len(rawEvents))

	for _, rawEvent := range rawEvents {
		event, err := apievents.ToOneOf(rawEvent)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		encodedEvents = append(encodedEvents, event)
	}

	res.Items = encodedEvents
	res.LastKey = lastkey
	return res, nil
}

// GetLock retrieves a lock by name.
func (g *GRPCServer) GetLock(ctx context.Context, req *proto.GetLockRequest) (*types.LockV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lock, err := auth.GetLock(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lockV2, ok := lock.(*types.LockV2)
	if !ok {
		return nil, trace.Errorf("unexpected lock type %T", lock)
	}
	return lockV2, nil
}

// GetLocks gets all/in-force locks that match at least one of the targets when specified.
func (g *GRPCServer) GetLocks(ctx context.Context, req *proto.GetLocksRequest) (*proto.GetLocksResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	targets := make([]types.LockTarget, 0, len(req.Targets))
	for _, targetPtr := range req.Targets {
		if targetPtr != nil {
			targets = append(targets, *targetPtr)
		}
	}
	locks, err := auth.GetLocks(ctx, req.InForceOnly, targets...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	lockV2s := make([]*types.LockV2, 0, len(locks))
	for _, lock := range locks {
		lockV2, ok := lock.(*types.LockV2)
		if !ok {
			return nil, trace.BadParameter("unexpected lock type %T", lock)
		}
		lockV2s = append(lockV2s, lockV2)
	}
	return &proto.GetLocksResponse{
		Locks: lockV2s,
	}, nil
}

// UpsertLock upserts a lock.
func (g *GRPCServer) UpsertLock(ctx context.Context, lock *types.LockV2) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertLock(ctx, lock); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteLock deletes a lock.
func (g *GRPCServer) DeleteLock(ctx context.Context, req *proto.DeleteLockRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteLock(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (g *GRPCServer) ReplaceRemoteLocks(ctx context.Context, req *proto.ReplaceRemoteLocksRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	locks := make([]types.Lock, 0, len(req.Locks))
	for _, lock := range req.Locks {
		locks = append(locks, lock)
	}
	if err := auth.ReplaceRemoteLocks(ctx, req.ClusterName, locks); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CreateApp creates a new application resource.
func (g *GRPCServer) CreateApp(ctx context.Context, app *types.AppV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app.SetOrigin(types.OriginDynamic)
	if err := auth.CreateApp(ctx, app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// UpdateApp updates existing application resource.
func (g *GRPCServer) UpdateApp(ctx context.Context, app *types.AppV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app.SetOrigin(types.OriginDynamic)
	if err := auth.UpdateApp(ctx, app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetApp returns the specified application resource.
func (g *GRPCServer) GetApp(ctx context.Context, req *types.ResourceRequest) (*types.AppV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := auth.GetApp(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appV3, ok := app.(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("unsupported application type %T", app)
	}
	return appV3, nil
}

// GetApps returns all application resources.
func (g *GRPCServer) GetApps(ctx context.Context, _ *empty.Empty) (*types.AppV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := auth.GetApps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var appsV3 []*types.AppV3
	for _, app := range apps {
		appV3, ok := app.(*types.AppV3)
		if !ok {
			return nil, trace.BadParameter("unsupported application type %T", app)
		}
		appsV3 = append(appsV3, appV3)
	}
	return &types.AppV3List{
		Apps: appsV3,
	}, nil
}

// DeleteApp removes the specified application resource.
func (g *GRPCServer) DeleteApp(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteApp(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllApps removes all application resources.
func (g *GRPCServer) DeleteAllApps(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllApps(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// CreateDatabase creates a new database resource.
func (g *GRPCServer) CreateDatabase(ctx context.Context, database *types.DatabaseV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := auth.CreateDatabase(ctx, database); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// UpdateDatabase updates existing database resource.
func (g *GRPCServer) UpdateDatabase(ctx context.Context, database *types.DatabaseV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	database.SetOrigin(types.OriginDynamic)
	if err := auth.UpdateDatabase(ctx, database); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetDatabase returns the specified database resource.
func (g *GRPCServer) GetDatabase(ctx context.Context, req *types.ResourceRequest) (*types.DatabaseV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	database, err := auth.GetDatabase(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databaseV3, ok := database.(*types.DatabaseV3)
	if !ok {
		return nil, trace.BadParameter("unsupported database type %T", database)
	}
	return databaseV3, nil
}

// GetDatabases returns all database resources.
func (g *GRPCServer) GetDatabases(ctx context.Context, _ *empty.Empty) (*types.DatabaseV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	databases, err := auth.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var databasesV3 []*types.DatabaseV3
	for _, database := range databases {
		databaseV3, ok := database.(*types.DatabaseV3)
		if !ok {
			return nil, trace.BadParameter("unsupported database type %T", database)
		}
		databasesV3 = append(databasesV3, databaseV3)
	}
	return &types.DatabaseV3List{
		Databases: databasesV3,
	}, nil
}

// DeleteDatabase removes the specified database.
func (g *GRPCServer) DeleteDatabase(ctx context.Context, req *types.ResourceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteDatabase(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllDatabases removes all databases.
func (g *GRPCServer) DeleteAllDatabases(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllDatabases(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (g *GRPCServer) GetWindowsDesktopServices(ctx context.Context, req *empty.Empty) (*proto.GetWindowsDesktopServicesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	windowsDesktopServices, err := auth.GetWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var services []*types.WindowsDesktopServiceV3
	for _, s := range windowsDesktopServices {
		service, ok := s.(*types.WindowsDesktopServiceV3)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", s)
		}
		services = append(services, service)
	}
	return &proto.GetWindowsDesktopServicesResponse{
		Services: services,
	}, nil
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (g *GRPCServer) GetWindowsDesktopService(ctx context.Context, req *proto.GetWindowsDesktopServiceRequest) (*proto.GetWindowsDesktopServiceResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	windowsDesktopService, err := auth.GetWindowsDesktopService(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	service, ok := windowsDesktopService.(*types.WindowsDesktopServiceV3)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", service)
	}
	return &proto.GetWindowsDesktopServiceResponse{
		Service: service,
	}, nil
}

// UpsertWindowsDesktopService registers a new Windows desktop service.
func (g *GRPCServer) UpsertWindowsDesktopService(ctx context.Context, service *types.WindowsDesktopServiceV3) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If Addr in the server is localhost, replace it with the address we see
	// from our end.
	//
	// Services that listen on "0.0.0.0:12345" will put that exact address in
	// the server.Addr field. It's not useful for other services that want to
	// connect to it (like a proxy). Remote address of the gRPC connection is
	// the closest thing we have to a public IP for the service.
	clientAddr, ok := ctx.Value(ContextClientAddr).(net.Addr)
	if !ok {
		return nil, status.Errorf(codes.FailedPrecondition, "client address not found in request context")
	}
	service.Spec.Addr = utils.ReplaceLocalhost(service.GetAddr(), clientAddr.String())

	keepAlive, err := auth.UpsertWindowsDesktopService(ctx, service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteWindowsDesktopService removes the specified Windows desktop service.
func (g *GRPCServer) DeleteWindowsDesktopService(ctx context.Context, req *proto.DeleteWindowsDesktopServiceRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteWindowsDesktopService(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllWindowsDesktopServices removes all registered Windows desktop services.
func (g *GRPCServer) DeleteAllWindowsDesktopServices(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (g *GRPCServer) GetWindowsDesktops(ctx context.Context, filter *types.WindowsDesktopFilter) (*proto.GetWindowsDesktopsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	windowsDesktops, err := auth.GetWindowsDesktops(ctx, *filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var desktops []*types.WindowsDesktopV3
	for _, s := range windowsDesktops {
		desktop, ok := s.(*types.WindowsDesktopV3)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", s)
		}
		desktops = append(desktops, desktop)
	}
	return &proto.GetWindowsDesktopsResponse{
		Desktops: desktops,
	}, nil
}

// CreateWindowsDesktop registers a new Windows desktop host.
func (g *GRPCServer) CreateWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CreateWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// UpdateWindowsDesktop updates an existing Windows desktop host.
func (g *GRPCServer) UpdateWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpdateWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// UpsertWindowsDesktop updates a Windows desktop host, creating it if it doesn't exist.
func (g *GRPCServer) UpsertWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &empty.Empty{}, nil
}

// DeleteWindowsDesktop removes the specified windows desktop host.
// Note: unlike GetWindowsDesktops, this will delete at-most one desktop.
// Passing an empty host ID will not trigger "delete all" behavior. To delete
// all desktops, use DeleteAllWindowsDesktops.
func (g *GRPCServer) DeleteWindowsDesktop(ctx context.Context, req *proto.DeleteWindowsDesktopRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteWindowsDesktop(ctx, req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// DeleteAllWindowsDesktops removes all registered Windows desktop hosts.
func (g *GRPCServer) DeleteAllWindowsDesktops(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllWindowsDesktops(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GenerateWindowsDesktopCert generates client certificate for Windows RDP
// authentication.
func (g *GRPCServer) GenerateWindowsDesktopCert(ctx context.Context, req *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := auth.GenerateWindowsDesktopCert(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// ChangeUserAuthentication implements AuthService.ChangeUserAuthentication.
func (g *GRPCServer) ChangeUserAuthentication(ctx context.Context, req *proto.ChangeUserAuthenticationRequest) (*proto.ChangeUserAuthenticationResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := auth.ServerWithRoles.ChangeUserAuthentication(ctx, req)
	return res, trace.Wrap(err)
}

// StartAccountRecovery is implemented by AuthService.StartAccountRecovery.
func (g *GRPCServer) StartAccountRecovery(ctx context.Context, req *proto.StartAccountRecoveryRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resetToken, err := auth.ServerWithRoles.StartAccountRecovery(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := resetToken.(*types.UserTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected UserToken type %T", resetToken)
	}

	return r, nil
}

// VerifyAccountRecovery is implemented by AuthService.VerifyAccountRecovery.
func (g *GRPCServer) VerifyAccountRecovery(ctx context.Context, req *proto.VerifyAccountRecoveryRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	approvedToken, err := auth.ServerWithRoles.VerifyAccountRecovery(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := approvedToken.(*types.UserTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected UserToken type %T", approvedToken)
	}

	return r, nil
}

// CompleteAccountRecovery is implemented by AuthService.CompleteAccountRecovery.
func (g *GRPCServer) CompleteAccountRecovery(ctx context.Context, req *proto.CompleteAccountRecoveryRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.ServerWithRoles.CompleteAccountRecovery(ctx, req)
	return &empty.Empty{}, trace.Wrap(err)
}

// CreateAccountRecoveryCodes is implemented by AuthService.CreateAccountRecoveryCodes.
func (g *GRPCServer) CreateAccountRecoveryCodes(ctx context.Context, req *proto.CreateAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := auth.ServerWithRoles.CreateAccountRecoveryCodes(ctx, req)
	return res, trace.Wrap(err)
}

// GetAccountRecoveryToken is implemented by AuthService.GetAccountRecoveryToken.
func (g *GRPCServer) GetAccountRecoveryToken(ctx context.Context, req *proto.GetAccountRecoveryTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	approvedToken, err := auth.ServerWithRoles.GetAccountRecoveryToken(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := approvedToken.(*types.UserTokenV3)
	if !ok {
		return nil, trace.BadParameter("unexpected UserToken type %T", approvedToken)
	}

	return r, nil
}

// GetAccountRecoveryCodes is implemented by AuthService.GetAccountRecoveryCodes.
func (g *GRPCServer) GetAccountRecoveryCodes(ctx context.Context, req *proto.GetAccountRecoveryCodesRequest) (*proto.RecoveryCodes, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rc, err := auth.ServerWithRoles.GetAccountRecoveryCodes(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rc, nil
}

// CreateAuthenticateChallenge is implemented by AuthService.CreateAuthenticateChallenge.
func (g *GRPCServer) CreateAuthenticateChallenge(ctx context.Context, req *proto.CreateAuthenticateChallengeRequest) (*proto.MFAAuthenticateChallenge, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := actx.ServerWithRoles.CreateAuthenticateChallenge(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// CreatePrivilegeToken is implemented by AuthService.CreatePrivilegeToken.
func (g *GRPCServer) CreatePrivilegeToken(ctx context.Context, req *proto.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := auth.CreatePrivilegeToken(ctx, req)
	return token, trace.Wrap(err)
}

// CreateRegisterChallenge is implemented by AuthService.CreateRegisterChallenge.
func (g *GRPCServer) CreateRegisterChallenge(ctx context.Context, req *proto.CreateRegisterChallengeRequest) (*proto.MFARegisterChallenge, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := actx.ServerWithRoles.CreateRegisterChallenge(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return res, nil
}

// GenerateCertAuthorityCRL returns a CRL for a CA.
func (g *GRPCServer) GenerateCertAuthorityCRL(ctx context.Context, req *proto.CertAuthorityRequest) (*proto.CRL, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	crl, err := auth.GenerateCertAuthorityCRL(ctx, req.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.CRL{CRL: crl}, nil
}

// ListResources retrieves a paginated list of resources.
func (g *GRPCServer) ListResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := auth.ListResources(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	protoResp := &proto.ListResourcesResponse{
		NextKey:    resp.NextKey,
		Resources:  make([]*proto.PaginatedResource, len(resp.Resources)),
		TotalCount: int32(resp.TotalCount),
	}

	for i, resource := range resp.Resources {
		var protoResource *proto.PaginatedResource
		switch req.ResourceType {
		case types.KindDatabaseServer:
			database, ok := resource.(*types.DatabaseServerV3)
			if !ok {
				return nil, trace.BadParameter("database server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_DatabaseServer{DatabaseServer: database}}
		case types.KindAppServer:
			app, ok := resource.(*types.AppServerV3)
			if !ok {
				return nil, trace.BadParameter("application server has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_AppServer{AppServer: app}}
		case types.KindNode:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.BadParameter("node has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: srv}}
		case types.KindKubeService:
			srv, ok := resource.(*types.ServerV2)
			if !ok {
				return nil, trace.BadParameter("kubernetes service has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubeService{KubeService: srv}}
		case types.KindWindowsDesktop:
			desktop, ok := resource.(*types.WindowsDesktopV3)
			if !ok {
				return nil, trace.BadParameter("windows desktop has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_WindowsDesktop{WindowsDesktop: desktop}}
		case types.KindKubernetesCluster:
			cluster, ok := resource.(*types.KubernetesClusterV3)
			if !ok {
				return nil, trace.BadParameter("kubernetes cluster has invalid type %T", resource)
			}

			protoResource = &proto.PaginatedResource{Resource: &proto.PaginatedResource_KubeCluster{KubeCluster: cluster}}
		default:
			return nil, trace.NotImplemented("resource type %s doesn't support pagination", req.ResourceType)
		}

		protoResp.Resources[i] = protoResource
	}

	return protoResp, nil
}

// CreateSessionTracker creates a tracker resource for an active session.
func (g *GRPCServer) CreateSessionTracker(ctx context.Context, req *proto.CreateSessionTrackerRequest) (*types.SessionTrackerV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var createTracker types.SessionTracker = req.SessionTracker
	// DELETE IN 11.0.0
	// Early v9 versions use a flattened out types.SessionTrackerV1
	if req.SessionTracker == nil {
		spec := types.SessionTrackerSpecV1{
			SessionID:         req.ID,
			Kind:              req.Type,
			State:             types.SessionState_SessionStatePending,
			Reason:            req.Reason,
			Invited:           req.Invited,
			Hostname:          req.Hostname,
			Address:           req.Address,
			ClusterName:       req.ClusterName,
			Login:             req.Login,
			Participants:      []types.Participant{*req.Initiator},
			Expires:           req.Expires,
			KubernetesCluster: req.KubernetesCluster,
			HostUser:          req.HostUser,
		}
		createTracker, err = types.NewSessionTracker(spec)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	tracker, err := auth.ServerWithRoles.CreateSessionTracker(ctx, createTracker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v1, ok := tracker.(*types.SessionTrackerV1)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", tracker)
	}

	return v1, nil
}

// GetSessionTracker returns the current state of a session tracker for an active session.
func (g *GRPCServer) GetSessionTracker(ctx context.Context, req *proto.GetSessionTrackerRequest) (*types.SessionTrackerV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	session, err := auth.ServerWithRoles.GetSessionTracker(ctx, req.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defined, ok := session.(*types.SessionTrackerV1)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", session)
	}

	return defined, nil
}

// GetActiveSessionTrackers returns a list of active session trackers.
func (g *GRPCServer) GetActiveSessionTrackers(_ *empty.Empty, stream proto.AuthService_GetActiveSessionTrackersServer) error {
	ctx := stream.Context()
	auth, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	sessions, err := auth.ServerWithRoles.GetActiveSessionTrackers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, session := range sessions {
		defined, ok := session.(*types.SessionTrackerV1)
		if !ok {
			return trace.BadParameter("unexpected session type %T", session)
		}

		err := stream.Send(defined)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// RemoveSessionTracker removes a tracker resource for an active session.
func (g *GRPCServer) RemoveSessionTracker(ctx context.Context, req *proto.RemoveSessionTrackerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.ServerWithRoles.RemoveSessionTracker(ctx, req.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (g *GRPCServer) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) (*empty.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.ServerWithRoles.UpdateSessionTracker(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &empty.Empty{}, nil
}

// GetDomainName returns local auth domain of the current auth server.
func (g *GRPCServer) GetDomainName(ctx context.Context, req *empty.Empty) (*proto.GetDomainNameResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dn, err := auth.ServerWithRoles.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.GetDomainNameResponse{
		DomainName: dn,
	}, nil
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster
// without signing keys. If the cluster has multiple TLS certs, they will all
// be appended.
func (g *GRPCServer) GetClusterCACert(
	ctx context.Context, req *empty.Empty,
) (*proto.GetClusterCACertResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auth.ServerWithRoles.GetClusterCACert(ctx)
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
	// TraceClient is used to forward spans to the upstream telemetry collector
	TraceClient otlptrace.Client
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
	err := utils.RegisterPrometheusCollectors(heartbeatConnectionsReceived, watcherEventsEmitted, watcherEventSizes, connectedResources)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("GRPC(SERVER): keep alive %v count: %v.", cfg.KeepAlivePeriod, cfg.KeepAliveCount)
	opts := []grpc.ServerOption{
		grpc.Creds(&httplib.TLSCreds{
			Config: cfg.TLS,
		}),
		grpc.ChainUnaryInterceptor(otelgrpc.UnaryServerInterceptor(), cfg.UnaryInterceptor),
		grpc.ChainStreamInterceptor(otelgrpc.StreamServerInterceptor(), cfg.StreamInterceptor),
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
		server:      server,
		traceClient: cfg.TraceClient,
	}
	proto.RegisterAuthServiceServer(server, authServer)
	collectortracepb.RegisterTraceServiceServer(server, authServer)

	// create server with no-op role to pass to JoinService server
	serverWithNopRole, err := serverWithNopRole(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	joinServiceServer := joinserver.NewJoinServiceGRPCServer(serverWithNopRole)
	proto.RegisterJoinServiceServer(server, joinServiceServer)

	return authServer, nil
}

func serverWithNopRole(cfg GRPCServerConfig) (*ServerWithRoles, error) {
	clusterName, err := cfg.AuthServer.Services.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nopRole := BuiltinRole{
		Role:        types.RoleNop,
		Username:    string(types.RoleNop),
		ClusterName: clusterName.GetClusterName(),
	}
	recConfig, err := cfg.AuthServer.Services.GetSessionRecordingConfig(context.Background())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nopCtx, err := contextForBuiltinRole(nopRole, recConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServerWithRoles{
		authServer: cfg.AuthServer,
		context:    *nopCtx,
		sessions:   cfg.SessionService,
		alog:       cfg.AuthServer.IAuditLog,
	}, nil
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
		} else if trace.IsNotFound(err) {
			// user not found, wrap error with access denied
			return nil, trace.Wrap(err, "[10] access denied")
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
