/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	_ "google.golang.org/grpc/encoding/gzip" // gzip compressor for gRPC.
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	authpb "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	accessmonitoringrules "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	clusterconfigv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	crownjewelv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	dbobjectimportrulev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	discoveryconfigv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	dynamicwindowsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/dynamicwindows/v1"
	gitserverv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/gitserver/v1"
	identitycenterv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/identitycenter/v1"
	integrationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	kubewaitingcontainerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/kubewaitingcontainer/v1"
	loginrulev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	mfav1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	notificationsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	presencev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/presence/v1"
	provisioningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/provisioning/v1"
	stableunixusersv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/stableunixusers/v1"
	trustv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	userloginstatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userloginstate/v1"
	userprovisioningv2pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	usersv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	usertaskv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	vnetv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	userpreferencesv1pb "github.com/gravitational/teleport/api/gen/proto/go/userpreferences/v1"
	"github.com/gravitational/teleport/api/internalutils/stream"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/trail"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/installers"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/accessmonitoringrules/accessmonitoringrulesv1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/autoupdate/autoupdatev1"
	"github.com/gravitational/teleport/lib/auth/clusterconfig/clusterconfigv1"
	"github.com/gravitational/teleport/lib/auth/crownjewel/crownjewelv1"
	"github.com/gravitational/teleport/lib/auth/dbobject/dbobjectv1"
	"github.com/gravitational/teleport/lib/auth/dbobjectimportrule/dbobjectimportrulev1"
	"github.com/gravitational/teleport/lib/auth/discoveryconfig/discoveryconfigv1"
	"github.com/gravitational/teleport/lib/auth/dynamicwindows/dynamicwindowsv1"
	"github.com/gravitational/teleport/lib/auth/gitserver/gitserverv1"
	"github.com/gravitational/teleport/lib/auth/integration/integrationv1"
	"github.com/gravitational/teleport/lib/auth/kubewaitingcontainer/kubewaitingcontainerv1"
	"github.com/gravitational/teleport/lib/auth/loginrule/loginrulev1"
	"github.com/gravitational/teleport/lib/auth/machineid/machineidv1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/lib/auth/notifications/notificationsv1"
	"github.com/gravitational/teleport/lib/auth/presence/presencev1"
	"github.com/gravitational/teleport/lib/auth/stableunixusers"
	"github.com/gravitational/teleport/lib/auth/trust/trustv1"
	"github.com/gravitational/teleport/lib/auth/userloginstate/userloginstatev1"
	"github.com/gravitational/teleport/lib/auth/userpreferences/userpreferencesv1"
	"github.com/gravitational/teleport/lib/auth/userprovisioning/userprovisioningv2"
	"github.com/gravitational/teleport/lib/auth/users/usersv1"
	"github.com/gravitational/teleport/lib/auth/usertasks/usertasksv1"
	"github.com/gravitational/teleport/lib/auth/vnetconfig/vnetconfigv1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/decision/decisionv1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/joinserver"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/metrics"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/server/installer"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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

	createAuditStreamAcceptedTotalMetric = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "unstable_createauditstream_accepted_total",
		Help:      "CreateAuditStream RPCs accepted by the concurrency limiter",
	})

	createAuditStreamRejectedTotalMetric = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "unstable_createauditstream_rejected_total",
		Help:      "CreateAuditStream RPCs rejected by the concurrency limiter",
	})

	createAuditStreamLimitMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: teleport.MetricNamespace,
		Name:      "unstable_createauditstream_limit",
		Help:      "Configured limit of in-flight CreateAuditStream RPCs",
	})
)

// GRPCServer is gRPC Auth Server API
type GRPCServer struct {
	authpb.UnimplementedAuthServiceServer
	auditlogpb.UnimplementedAuditLogServiceServer
	logger *slog.Logger
	APIConfig
	server *grpc.Server

	// TraceServiceServer exposes the exporter server so that the auth server may
	// collect and forward spans
	collectortracepb.TraceServiceServer

	// createAuditStreamSemaphore, if not nil, is used to limit the amount of
	// in-flight CreateAuditStream RPCs, by sending a value in at the beginning
	// of the RPC and pulling one out before returning.
	createAuditStreamSemaphore chan struct{}
}

// Export forwards OTLP traces to the upstream collector configured in the tracing service. This allows for
// tsh, tctl, etc to be able to export traces without having to know how to connect to the upstream collector
// for the cluster.
func (g *GRPCServer) Export(ctx context.Context, req *collectortracepb.ExportTraceServiceRequest) (*collectortracepb.ExportTraceServiceResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.ResourceSpans) == 0 {
		return &collectortracepb.ExportTraceServiceResponse{}, nil
	}

	return auth.Export(ctx, req)
}

// GetServer returns an instance of grpc server
func (g *GRPCServer) GetServer() (*grpc.Server, error) {
	if g.server == nil {
		return nil, trace.BadParameter("grpc server has not been initialized")
	}

	return g.server, nil
}

// EmitAuditEvent emits audit event
func (g *GRPCServer) EmitAuditEvent(ctx context.Context, req *apievents.OneOf) (*emptypb.Empty, error) {
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
	return &emptypb.Empty{}, nil
}

var connectedResourceGauges = map[string]prometheus.Gauge{
	constants.KeepAliveNode:                  connectedResources.WithLabelValues(constants.KeepAliveNode),
	constants.KeepAliveKube:                  connectedResources.WithLabelValues(constants.KeepAliveKube),
	constants.KeepAliveApp:                   connectedResources.WithLabelValues(constants.KeepAliveApp),
	constants.KeepAliveDatabase:              connectedResources.WithLabelValues(constants.KeepAliveDatabase),
	constants.KeepAliveDatabaseService:       connectedResources.WithLabelValues(constants.KeepAliveDatabaseService),
	constants.KeepAliveWindowsDesktopService: connectedResources.WithLabelValues(constants.KeepAliveWindowsDesktopService),
}

// SendKeepAlives allows node to send a stream of keep alive requests
func (g *GRPCServer) SendKeepAlives(stream authpb.AuthService_SendKeepAlivesServer) error {
	defer stream.SendAndClose(&emptypb.Empty{})
	firstIteration := true
	for {
		// Authenticate within the loop to block locked-out nodes from heartbeating.
		auth, err := g.authenticate(stream.Context())
		if err != nil {
			return trace.Wrap(err)
		}
		keepAlive, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			g.logger.DebugContext(stream.Context(), "Connection closed")
			return nil
		}
		if err != nil {
			g.logger.DebugContext(stream.Context(), "Failed to receive heartbeat", "error", err)
			return trace.Wrap(err)
		}
		err = auth.KeepAliveServer(stream.Context(), *keepAlive)
		if err != nil {
			return trace.Wrap(err)
		}
		if firstIteration {
			g.logger.DebugContext(stream.Context(), "Got heartbeat connection",
				"heartbeat_type", keepAlive.GetType(),
				"identity", auth.User.GetName(),
			)
			heartbeatConnectionsReceived.Inc()

			metric, ok := connectedResourceGauges[keepAlive.GetType()]
			if ok {
				metric.Inc()
				defer metric.Dec()
			} else {
				g.logger.WarnContext(stream.Context(), "missing connected resources gauge for keep alive (this is a bug)", "heartbeat_type", keepAlive.GetType())
			}

			firstIteration = false
		}
	}
}

// CreateAuditStream creates or resumes audit event stream
func (g *GRPCServer) CreateAuditStream(stream authpb.AuthService_CreateAuditStreamServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	if sem := g.createAuditStreamSemaphore; sem != nil {
		select {
		case sem <- struct{}{}:
			createAuditStreamAcceptedTotalMetric.Inc()
			defer func() { <-sem }()
		default:
			createAuditStreamRejectedTotalMetric.Inc()
			// [trace.ConnectionProblemError] is rendered with a gRPC
			// "unavailable" error code, which is the correct error if the
			// client can just back off and retry with no further changes to the
			// request
			return trace.ConnectionProblem(nil, "too many concurrent CreateAuditStream operations, try again later")
		}
	}

	var eventStream apievents.Stream
	var sessionID session.ID
	g.logger.DebugContext(stream.Context(), "CreateAuditStream connection", "identity", auth.User.GetName())
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
					g.logger.DebugContext(stream.Context(), "Failed to send status update", "error", err)
				}
			}
		}
	}

	closeStream := func(eventStream apievents.Stream) {
		if err := eventStream.Close(auth.CloseContext()); err != nil {
			if auth.CloseContext().Err() == nil {
				g.logger.WarnContext(stream.Context(), "Failed to flush close the stream", "error", err)
			}
		} else {
			g.logger.DebugContext(stream.Context(), "Flushed and closed the stream")
		}
	}

	for {
		request, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			if stream.Context().Err() == nil {
				g.logger.DebugContext(stream.Context(), "Failed to receive stream request", "error", err)
			}
			return trace.Wrap(err)
		}
		if create := request.GetCreateStream(); create != nil {
			if eventStream != nil {
				return trace.BadParameter("stream is already created or resumed")
			}
			eventStream, err = auth.CreateAuditStream(stream.Context(), session.ID(create.SessionID))
			if err != nil {
				// Log the reason why audit stream creation failed. This will
				// surface things like AWS/GCP/MinIO credential/configuration
				// errors.
				g.logger.ErrorContext(stream.Context(), "Failed to create audit stream", "error", err)
				return trace.Wrap(err)
			}
			sessionID = session.ID(create.SessionID)
			g.logger.DebugContext(stream.Context(), "Created stream for session", "session_id", sessionID)
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
			g.logger.DebugContext(stream.Context(), "Resumed stream for session", "session_id", resume.SessionID)
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
				// TODO(zmb3): this may result in duplicate upload events, as the upload
				// completer will emit its own session.upload
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
			g.logger.DebugContext(stream.Context(), "Completed stream for session", "session_id", sessionID)
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
				g.logger.DebugContext(stream.Context(), "Failed to decode event", "error", err)
				return trace.Wrap(err)
			}
			// Currently only api/client.auditStreamer calls with an event
			// and it always sends an events.PreparedSessionEvent, so
			// this event can safely be assumed to be prepared. Use a
			// events.NoOpPreparer to simply convert the event.
			setter := &events.NoOpPreparer{}
			start := time.Now()
			preparedEvent, _ := setter.PrepareSessionEvent(event)
			var errors []error
			errors = append(errors, eventStream.RecordEvent(stream.Context(), preparedEvent))

			// v13 clients expect this request to also emit the event, so emit here
			// just for them.
			switch event.GetType() {
			// Don't emit really verbose events.
			case events.ResizeEvent, events.SessionDiskEvent, events.SessionPrintEvent, events.AppSessionRequestEvent, "":
			default:
				clientVersion, versionExists := metadata.ClientVersionFromContext(stream.Context())
				if versionExists && semver.New(clientVersion).Major <= 13 {
					errors = append(errors, auth.EmitAuditEvent(stream.Context(), event))
				}
			}

			err = trace.NewAggregate(errors...)
			if err != nil {
				switch {
				case events.IsPermanentEmitError(err):
					g.logger.ErrorContext(stream.Context(), "Failed to EmitAuditEvent due to a permanent error, event wil be omitted",
						slog.Any("error", err),
						slog.Group("event",
							slog.String("type", event.GetType()),
							slog.String("code", event.GetCode()),
							slog.String("id", event.GetID()),
							slog.Int64("index", event.GetIndex()),
						),
					)
					continue
				default:
					return trace.Wrap(err)
				}
			}

			processed += int64(event.Size())
			seconds := time.Since(streamStart) / time.Second
			counter++
			if counter%logInterval == 0 {
				if seconds > 0 {
					kbytes := float64(processed) / 1000
					g.logger.DebugContext(stream.Context(), "Processed events", "event_count", counter, "tx_rate", kbytes/float64(seconds))
				}
			}
			diff := time.Since(start)
			if diff > 100*time.Millisecond {
				g.logger.WarnContext(stream.Context(), "RecordEvent took longer than 100ms",
					"event_type", event.GetType(),
					"duration", time.Since(event.GetTime()),
				)
			}
		} else {
			g.logger.ErrorContext(stream.Context(), "Rejecting unsupported stream request", "request", request)
			return trace.BadParameter("unsupported stream request")
		}
	}
}

// logInterval is used to log stats after this many events
const logInterval = 10000

// WatchEvents returns a new stream of cluster events
func (g *GRPCServer) WatchEvents(watch *authpb.Watch, stream authpb.AuthService_WatchEventsServer) (err error) {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(WatchEvents(watch, stream, auth.User.GetName(), auth))
}

// WatchEvent is a stream interface for sending events.
type WatchEvent interface {
	Context() context.Context
	Send(*authpb.Event) error
}

type Watcher interface {
	NewStream(ctx context.Context, watch types.Watch) (stream.Stream[types.Event], error)
}

// WatchEvents watches for events and streams them to the provided stream.
func WatchEvents(watch *authpb.Watch, stream WatchEvent, componentName string, auth Watcher) error {
	servicesWatch := types.Watch{
		Name:                componentName,
		Kinds:               watch.Kinds,
		AllowPartialSuccess: watch.AllowPartialSuccess,
	}

	// KindNamespace is being removed but v17 agents will still try to include
	// it in their cache and they will occasionally do a GetNamespace, so we
	// pretend to support it as a resource kind here; it's sound to do so
	// because there will never be any events coming, and the GetNamespace and
	// GetNamespaces APIs return static data
	//
	// TODO(espadolini): remove in v19
	var removedNamespaceWatch bool
	filteredKinds := watch.Kinds[:0]
	for _, k := range watch.Kinds {
		if k.Kind == types.KindNamespace {
			removedNamespaceWatch = true
			continue
		}
		filteredKinds = append(filteredKinds, k)
	}
	watch.Kinds = filteredKinds

	events, err := auth.NewStream(stream.Context(), servicesWatch)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		serr := events.Done()
		if err == nil {
			err = serr
		}
	}()

	for events.Next() {
		event := events.Item()
		// TODO(espadolini): remove in v19
		if removedNamespaceWatch {
			if status, ok := event.Resource.(*types.WatchStatusV1); ok {
				status.Spec.Kinds = append(status.Spec.Kinds, types.WatchKind{Kind: types.KindNamespace})
			}
			// there's only exactly one event of type OpInit and WatchStatus
			// meta-resource (at the beginning of the stream), so we don't need
			// to keep checking
			removedNamespaceWatch = false
		}
		if role, ok := event.Resource.(*types.RoleV6); ok {
			downgraded, err := maybeDowngradeRole(stream.Context(), role)
			if err != nil {
				return trace.Wrap(err)
			}
			event.Resource = downgraded
		}
		out, err := client.EventToGRPC(event)
		if err != nil {
			return trace.Wrap(err)
		}

		size := float64(proto.Size(out))
		watcherEventsEmitted.WithLabelValues(resourceLabel(event)).Observe(size)
		watcherEventSizes.Observe(size)

		if err := stream.Send(out); err != nil {
			return trace.Wrap(err)
		}
	}

	// deferred cleanup func will inject stream error if needed
	return nil
}

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

func (g *GRPCServer) GenerateUserCerts(ctx context.Context, req *authpb.UserCertsRequest) (*authpb.Certs, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateUserCertsRequest(auth, req); err != nil {
		g.logger.DebugContext(ctx, "Validation of user certs request failed", "error", err)
		return nil, trace.Wrap(err)
	}

	if req.Purpose == authpb.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS {
		certs, err := g.generateUserSingleUseCerts(ctx, auth, req)
		return certs, trace.Wrap(err)
	}

	certs, err := auth.ServerWithRoles.GenerateUserCerts(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return certs, nil
}

func validateUserCertsRequest(actx *grpcContext, req *authpb.UserCertsRequest) error {
	switch req.Usage {
	case authpb.UserCertsRequest_All:
		if req.Purpose == authpb.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS {
			return trace.BadParameter("single-use certificates cannot be issued for all purposes")
		}
	case authpb.UserCertsRequest_App:
		if req.RouteToApp.Name == "" {
			return trace.BadParameter("missing app Name field in an app-only UserCertsRequest")
		}
	case authpb.UserCertsRequest_SSH:
		if req.NodeName == "" {
			return trace.BadParameter("missing NodeName field in a ssh-only UserCertsRequest")
		}
	case authpb.UserCertsRequest_Kubernetes:
		if req.KubernetesCluster == "" {
			return trace.BadParameter("missing KubernetesCluster field in a kubernetes-only UserCertsRequest")
		}
	case authpb.UserCertsRequest_Database:
		if req.RouteToDatabase.ServiceName == "" {
			return trace.BadParameter("missing ServiceName field in a database-only UserCertsRequest")
		}
	case authpb.UserCertsRequest_WindowsDesktop:
		if req.RouteToWindowsDesktop.WindowsDesktop == "" {
			return trace.BadParameter("missing WindowsDesktop field in a windows-desktop-only UserCertsRequest")
		}
	default:
		return trace.BadParameter("unknown certificate Usage %q", req.Usage)
	}

	if req.Purpose != authpb.UserCertsRequest_CERT_PURPOSE_SINGLE_USE_CERTS {
		return nil
	}

	// Single-use certs require current user.
	if err := actx.currentUserAction(req.Username); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// generateUserSingleUseCerts issues single-use user certificates.
func (g *GRPCServer) generateUserSingleUseCerts(ctx context.Context, actx *grpcContext, req *authpb.UserCertsRequest) (*authpb.Certs, error) {
	setUserSingleUseCertsTTL(actx, req)

	// We don't do MFA requirement validations here.
	// Callers are supposed to use either use
	// CreateAuthenticateChallengeRequest.MFARequiredCheck or call IsMFARequired,
	// as appropriate for their scenario.
	//
	// If the request has an MFAAuthenticateResponse, then the caller gets a cert
	// with device extensions. Otherwise, they don't.

	// Generate the cert
	singleUseCert, err := userSingleUseCertsGenerate(
		ctx,
		actx,
		*req)
	if err != nil {
		g.logger.WarnContext(ctx, "Failed to generate single-use cert", "error", err)
		return nil, trace.Wrap(err)
	}

	return singleUseCert, nil
}

func (g *GRPCServer) GenerateHostCerts(ctx context.Context, req *authpb.HostCertsRequest) (*authpb.Certs, error) {
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

func (g *GRPCServer) GenerateOpenSSHCert(ctx context.Context, req *authpb.OpenSSHCertRequest) (*authpb.OpenSSHCert, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := auth.ServerWithRoles.GenerateOpenSSHCert(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cert, nil
}

// AssertSystemRole is used by agents to prove that they have a given system role when their credentials
// originate from multiple separate join tokens so that they can be issued an instance certificate that
// encompasses all of their capabilities. This method will be deprecated once we have a more comprehensive
// model for join token joining/replacement.
func (g *GRPCServer) AssertSystemRole(ctx context.Context, req *authpb.SystemRoleAssertion) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.AssertSystemRole(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &emptypb.Empty{}, nil
}

// icsServicesToMetricName is a helper for translating service names to keepalive names for control-stream
// purposes. When new services switch to using control-stream based heartbeats, they should be added here.
var icsServiceToMetricName = map[types.SystemRole]string{
	types.RoleNode: constants.KeepAliveNode,
	types.RoleApp:  constants.KeepAliveApp,
}

func (g *GRPCServer) InventoryControlStream(stream authpb.AuthService_InventoryControlStreamServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trail.ToGRPC(err)
	}

	p, ok := peer.FromContext(stream.Context())
	if !ok {
		return trace.BadParameter("unable to find peer")
	}

	ics := client.NewUpstreamInventoryControlStream(stream, p.Addr.String())

	hello, err := auth.RegisterInventoryControlStream(ics)
	if err != nil {
		return trail.ToGRPC(err)
	}

	// we use a different name for a service in our metrics than we do in certs/hellos. the subset of
	// services that currently use ics for heartbeats are registered in the icsServiceToMetricName
	// mapping for translation.
	var metricServices []string
	for _, service := range hello.Services {
		if name, ok := icsServiceToMetricName[service]; ok {
			metricServices = append(metricServices, name)
		}
	}

	// the heartbeatConnectionsReceived metric counts individual services as individual connections.
	heartbeatConnectionsReceived.Add(float64(len(metricServices)))

	// hold open the stream until it completes
	<-ics.Done()

	if errors.Is(ics.Error(), io.EOF) {
		return nil
	}

	return trail.ToGRPC(ics.Error())
}

func (g *GRPCServer) GetInventoryStatus(ctx context.Context, req *authpb.InventoryStatusRequest) (*authpb.InventoryStatusSummary, error) {
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

// GetInventoryConnectedServiceCounts returns the counts of each connected service seen in the inventory.
func (g *GRPCServer) GetInventoryConnectedServiceCounts(ctx context.Context, _ *authpb.InventoryConnectedServiceCountsRequest) (*authpb.InventoryConnectedServiceCounts, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	rsp, err := auth.GetInventoryConnectedServiceCounts()
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &rsp, nil
}

func (g *GRPCServer) PingInventory(ctx context.Context, req *authpb.InventoryPingRequest) (*authpb.InventoryPingResponse, error) {
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

func (g *GRPCServer) GetInstances(filter *types.InstanceFilter, stream authpb.AuthService_GetInstancesServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	instances := auth.GetInstances(stream.Context(), *filter)

	for instances.Next() {
		instance, ok := instances.Item().(*types.InstanceV1)
		if !ok {
			g.logger.WarnContext(stream.Context(), "Skipping unexpected instance type",
				"instance_type", logutils.TypeAttr(instances.Item()),
				"expected_instance_type", logutils.TypeAttr(instance),
			)
			continue
		}
		if err := stream.Send(instance); err != nil {
			instances.Done()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(instances.Done())
}

func (g *GRPCServer) GetClusterAlerts(ctx context.Context, query *types.GetClusterAlertsRequest) (*authpb.GetClusterAlertsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	alerts, err := auth.GetClusterAlerts(ctx, *query)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &authpb.GetClusterAlertsResponse{
		Alerts: alerts,
	}, nil
}

func (g *GRPCServer) UpsertClusterAlert(ctx context.Context, req *authpb.UpsertClusterAlertRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.UpsertClusterAlert(ctx, req.Alert); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) CreateAlertAck(ctx context.Context, ack *types.AlertAcknowledgement) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.CreateAlertAck(ctx, *ack); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) GetAlertAcks(ctx context.Context, _ *authpb.GetAlertAcksRequest) (*authpb.GetAlertAcksResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	acks, err := auth.GetAlertAcks(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &authpb.GetAlertAcksResponse{
		Acks: acks,
	}, nil
}

func (g *GRPCServer) ClearAlertAcks(ctx context.Context, req *authpb.ClearAlertAcksRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err := auth.ClearAlertAcks(ctx, *req); err != nil {
		return nil, trail.ToGRPC(err)
	}

	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) GetCurrentUserRoles(_ *emptypb.Empty, stream authpb.AuthService_GetCurrentUserRolesServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	roles, err := auth.ServerWithRoles.GetCurrentUserRoles(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	for _, role := range roles {
		v6, ok := role.(*types.RoleV6)
		if !ok {
			g.logger.WarnContext(stream.Context(), "expected type RoleV6, got unexpected for role type",
				"role_type", logutils.TypeAttr(role),
				"role", role.GetName(),
			)
			return trace.Errorf("encountered unexpected role type")
		}
		if err := stream.Send(v6); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (g *GRPCServer) GetAccessRequestsV2(f *types.AccessRequestFilter, stream authpb.AuthService_GetAccessRequestsV2Server) error {
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

func (g *GRPCServer) ListAccessRequests(ctx context.Context, req *authpb.ListAccessRequestsRequest) (*authpb.ListAccessRequestsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := auth.ServerWithRoles.ListAccessRequests(ctx, req)

	return rsp, trace.Wrap(err)
}

func (g *GRPCServer) CreateAccessRequest(ctx context.Context, req *types.AccessRequestV3) (*emptypb.Empty, error) {
	return nil, trace.NotImplemented("access request creation API has changed, please update your client to v14 or newer")
}

func (g *GRPCServer) CreateAccessRequestV2(ctx context.Context, req *types.AccessRequestV3) (*types.AccessRequestV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateAccessRequestClusterNames(g.AuthServer, req); err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := auth.ServerWithRoles.CreateAccessRequestV2(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r, ok := out.(*types.AccessRequestV3)
	if !ok {
		return nil, trace.Wrap(trace.BadParameter("unexpected access request type %T", r))
	}

	return r, nil
}

func (g *GRPCServer) DeleteAccessRequest(ctx context.Context, id *authpb.RequestID) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteAccessRequest(ctx, id.ID); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) SetAccessRequestState(ctx context.Context, req *authpb.RequestStateSetter) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.Delegator != "" {
		ctx = authz.WithDelegator(ctx, req.Delegator)
	}
	if err := auth.ServerWithRoles.SetAccessRequestState(ctx, types.AccessRequestUpdate{
		RequestID:       req.ID,
		State:           req.State,
		Reason:          req.Reason,
		Annotations:     req.Annotations,
		Roles:           req.Roles,
		AssumeStartTime: req.AssumeStartTime,
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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

func (g *GRPCServer) GetAccessRequestAllowedPromotions(ctx context.Context, request *authpb.AccessRequestAllowedPromotionRequest) (*authpb.AccessRequestAllowedPromotionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessRequest, err := auth.GetAccessRequests(ctx, types.AccessRequestFilter{
		ID: request.AccessRequestID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(accessRequest) != 1 {
		return nil, trace.NotFound("access request not found")
	}

	allowedPromotions, err := auth.ServerWithRoles.GetAccessRequestAllowedPromotions(ctx, accessRequest[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authpb.AccessRequestAllowedPromotionResponse{
		AllowedPromotions: allowedPromotions,
	}, nil
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

func (g *GRPCServer) CreateResetPasswordToken(ctx context.Context, req *authpb.CreateResetPasswordTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req == nil {
		req = &authpb.CreateResetPasswordTokenRequest{}
	}

	token, err := auth.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
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

func (g *GRPCServer) GetResetPasswordToken(ctx context.Context, req *authpb.GetResetPasswordTokenRequest) (*types.UserTokenV3, error) {
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

// GetPluginData loads all plugin data matching the supplied filter.
func (g *GRPCServer) GetPluginData(ctx context.Context, filter *types.PluginDataFilter) (*authpb.PluginDataSeq, error) {
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
	return &authpb.PluginDataSeq{
		PluginData: seq,
	}, nil
}

// UpdatePluginData updates a per-resource PluginData entry.
func (g *GRPCServer) UpdatePluginData(ctx context.Context, params *types.PluginDataUpdateParams) (*emptypb.Empty, error) {
	// TODO(fspmarshall): Implement rate-limiting to prevent misbehaving plugins from
	// consuming too many server resources.
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.UpdatePluginData(ctx, *params); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) Ping(ctx context.Context, req *authpb.PingRequest) (*authpb.PingResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := auth.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// attempt to set remote addr.
	if p, ok := peer.FromContext(ctx); ok {
		rsp.RemoteAddr = p.Addr.String()
	}

	return &rsp, nil
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
func (g *GRPCServer) KeepAliveSemaphoreLease(ctx context.Context, req *types.SemaphoreLease) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.KeepAliveSemaphoreLease(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// CancelSemaphoreLease cancels semaphore lease early.
func (g *GRPCServer) CancelSemaphoreLease(ctx context.Context, req *types.SemaphoreLease) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CancelSemaphoreLease(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetSemaphores returns a list of all semaphores matching the supplied filter.
func (g *GRPCServer) GetSemaphores(ctx context.Context, req *types.SemaphoreFilter) (*authpb.Semaphores, error) {
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
	return &authpb.Semaphores{
		Semaphores: ss,
	}, nil
}

// DeleteSemaphore deletes a semaphore matching the supplied filter.
func (g *GRPCServer) DeleteSemaphore(ctx context.Context, req *types.SemaphoreFilter) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteSemaphore(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpsertDatabaseServer registers a new database proxy server.
func (g *GRPCServer) UpsertDatabaseServer(ctx context.Context, req *authpb.UpsertDatabaseServerRequest) (*types.KeepAlive, error) {
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
func (g *GRPCServer) DeleteDatabaseServer(ctx context.Context, req *authpb.DeleteDatabaseServerRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteDatabaseServer(ctx, req.GetNamespace(), req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllDatabaseServers removes all registered database proxy servers.
func (g *GRPCServer) DeleteAllDatabaseServers(ctx context.Context, req *authpb.DeleteAllDatabaseServersRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllDatabaseServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpsertDatabaseService registers a new database service.
func (g *GRPCServer) UpsertDatabaseService(ctx context.Context, req *authpb.UpsertDatabaseServiceRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	keepAlive, err := auth.UpsertDatabaseService(ctx, req.Service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteDatabaseService removes the specified DatabaseService.
func (g *GRPCServer) DeleteDatabaseService(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteDatabaseService(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllDatabaseServices removes all registered DatabaseServices.
func (g *GRPCServer) DeleteAllDatabaseServices(ctx context.Context, _ *authpb.DeleteAllDatabaseServicesRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.DeleteAllDatabaseServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SignDatabaseCSR generates a client certificate used by proxy when talking
// to a remote database service.
func (g *GRPCServer) SignDatabaseCSR(ctx context.Context, req *authpb.DatabaseCSRRequest) (*authpb.DatabaseCSRResponse, error) {
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

// GenerateDatabaseCert generates a client certificate used by a database
// service to authenticate with the database instance, or a server certificate
// for configuring a self-hosted database, depending on the requester_name.
func (g *GRPCServer) GenerateDatabaseCert(ctx context.Context, req *authpb.DatabaseCertRequest) (*authpb.DatabaseCertResponse, error) {
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
func (g *GRPCServer) GenerateSnowflakeJWT(ctx context.Context, req *authpb.SnowflakeJWTRequest) (*authpb.SnowflakeJWTResponse, error) {
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

// UpsertApplicationServer registers an application server.
func (g *GRPCServer) UpsertApplicationServer(ctx context.Context, req *authpb.UpsertApplicationServerRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	server := req.GetServer()
	app := server.GetApp()

	// Only allow app servers with Okta origins if coming from an Okta role. App servers sourced from
	// Okta are redirected differently which could create unpredictable or insecure behavior if applied
	// to non-Okta apps.
	hasOktaOrigin := server.Origin() == types.OriginOkta || app.Origin() == types.OriginOkta
	if !authz.HasBuiltinRole(auth.context, string(types.RoleOkta)) {
		if hasOktaOrigin {
			return nil, trace.BadParameter("only the Okta role can create app servers and apps with an Okta origin")
		}
	}

	keepAlive, err := auth.UpsertApplicationServer(ctx, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteApplicationServer deletes an application server.
func (g *GRPCServer) DeleteApplicationServer(ctx context.Context, req *authpb.DeleteApplicationServerRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteApplicationServer(ctx, req.GetNamespace(), req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllApplicationServers deletes all registered application servers.
func (g *GRPCServer) DeleteAllApplicationServers(ctx context.Context, req *authpb.DeleteAllApplicationServersRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllApplicationServers(ctx, req.GetNamespace())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAppSession gets an application web session.
func (g *GRPCServer) GetAppSession(ctx context.Context, req *authpb.GetAppSessionRequest) (*authpb.GetAppSessionResponse, error) {
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

	return &authpb.GetAppSessionResponse{
		Session: sess,
	}, nil
}

// ListAppSessions gets a paginated list of application web sessions.
func (g *GRPCServer) ListAppSessions(ctx context.Context, req *authpb.ListAppSessionsRequest) (*authpb.ListAppSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, token, err := auth.ListAppSessions(ctx, int(req.PageSize), req.PageToken, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]*types.WebSessionV2, 0, len(sessions))
	for _, sess := range sessions {
		s, ok := sess.(*types.WebSessionV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", sess)
		}
		out = append(out, s)
	}

	return &authpb.ListAppSessionsResponse{Sessions: out, NextPageToken: token}, nil
}

func (g *GRPCServer) GetSnowflakeSession(ctx context.Context, req *authpb.GetSnowflakeSessionRequest) (*authpb.GetSnowflakeSessionResponse, error) {
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

	return &authpb.GetSnowflakeSessionResponse{
		Session: sess,
	}, nil
}

func (g *GRPCServer) GetSnowflakeSessions(ctx context.Context, e *emptypb.Empty) (*authpb.GetSnowflakeSessionsResponse, error) {
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

	return &authpb.GetSnowflakeSessionsResponse{
		Sessions: out,
	}, nil
}

// GetSAMLIdPSession gets a SAML IdPsession.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) GetSAMLIdPSession(ctx context.Context, req *authpb.GetSAMLIdPSessionRequest) (*authpb.GetSAMLIdPSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	samlSession, err := auth.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest{SessionID: req.GetSessionID()})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := samlSession.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected session type %T", samlSession)
	}

	return &authpb.GetSAMLIdPSessionResponse{
		Session: sess,
	}, nil
}

// ListSAMLIdPSessions gets a paginated list of SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) ListSAMLIdPSessions(ctx context.Context, req *authpb.ListSAMLIdPSessionsRequest) (*authpb.ListSAMLIdPSessionsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sessions, token, err := auth.ListSAMLIdPSessions(ctx, int(req.PageSize), req.PageToken, req.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make([]*types.WebSessionV2, 0, len(sessions))
	for _, sess := range sessions {
		s, ok := sess.(*types.WebSessionV2)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", sess)
		}
		out = append(out, s)
	}

	return &authpb.ListSAMLIdPSessionsResponse{Sessions: out, NextPageToken: token}, nil
}

func (g *GRPCServer) DeleteSnowflakeSession(ctx context.Context, req *authpb.DeleteSnowflakeSessionRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteSnowflakeSession(ctx, types.DeleteSnowflakeSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) DeleteAllSnowflakeSessions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAllSnowflakeSessions(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// CreateAppSession creates an application web session. Application web
// sessions represent a browser session the client holds.
func (g *GRPCServer) CreateAppSession(ctx context.Context, req *authpb.CreateAppSessionRequest) (*authpb.CreateAppSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := auth.CreateAppSession(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := session.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", session)
	}

	return &authpb.CreateAppSessionResponse{
		Session: sess,
	}, nil
}

func (g *GRPCServer) CreateSnowflakeSession(ctx context.Context, req *authpb.CreateSnowflakeSessionRequest) (*authpb.CreateSnowflakeSessionResponse, error) {
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

	return &authpb.CreateSnowflakeSessionResponse{
		Session: sess,
	}, nil
}

// CreateSAMLIdPSession creates a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) CreateSAMLIdPSession(ctx context.Context, req *authpb.CreateSAMLIdPSessionRequest) (*authpb.CreateSAMLIdPSessionResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := auth.CreateSAMLIdPSession(ctx, types.CreateSAMLIdPSessionRequest{
		SessionID:   req.GetSessionID(),
		Username:    req.GetUsername(),
		SAMLSession: req.GetSAMLSession(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sess, ok := session.(*types.WebSessionV2)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", session)
	}

	return &authpb.CreateSAMLIdPSessionResponse{
		Session: sess,
	}, nil
}

// DeleteAppSession removes an application web session.
func (g *GRPCServer) DeleteAppSession(ctx context.Context, req *authpb.DeleteAppSessionRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAppSession(ctx, types.DeleteAppSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllAppSessions removes all application web sessions.
func (g *GRPCServer) DeleteAllAppSessions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAllAppSessions(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteUserAppSessions removes user's all application web sessions.
func (g *GRPCServer) DeleteUserAppSessions(ctx context.Context, req *authpb.DeleteUserAppSessionsRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteUserAppSessions(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteSAMLIdPSession removes a SAML IdP session.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) DeleteSAMLIdPSession(ctx context.Context, req *authpb.DeleteSAMLIdPSessionRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteSAMLIdPSession(ctx, types.DeleteSAMLIdPSessionRequest{
		SessionID: req.GetSessionID(),
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllSAMLIdPSessions removes all SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) DeleteAllSAMLIdPSessions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteAllSAMLIdPSessions(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteUserSAMLIdPSessions removes all of a user's SAML IdP sessions.
// TODO(Joerger): DELETE IN v18.0.0
func (g *GRPCServer) DeleteUserSAMLIdPSessions(ctx context.Context, req *authpb.DeleteUserSAMLIdPSessionsRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteUserSAMLIdPSessions(ctx, req.Username); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GenerateAppToken creates a JWT token with application access.
func (g *GRPCServer) GenerateAppToken(ctx context.Context, req *authpb.GenerateAppTokenRequest) (*authpb.GenerateAppTokenResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	traits := wrappers.Traits{}
	for traitName, traitValues := range req.Traits {
		traits[traitName] = traitValues.Values
	}
	token, err := auth.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username: req.Username,
		Roles:    req.Roles,
		Traits:   traits,
		URI:      req.URI,
		Expires:  req.Expires,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authpb.GenerateAppTokenResponse{
		Token: token,
	}, nil
}

// GetWebSession gets a web session.
func (g *GRPCServer) GetWebSession(ctx context.Context, req *types.GetWebSessionRequest) (*authpb.GetWebSessionResponse, error) {
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

	return &authpb.GetWebSessionResponse{
		Session: sess,
	}, nil
}

// GetWebSessions gets all web sessions.
func (g *GRPCServer) GetWebSessions(ctx context.Context, _ *emptypb.Empty) (*authpb.GetWebSessionsResponse, error) {
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

	return &authpb.GetWebSessionsResponse{
		Sessions: out,
	}, nil
}

// DeleteWebSession removes the web session given with req.
func (g *GRPCServer) DeleteWebSession(ctx context.Context, req *types.DeleteWebSessionRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebSessions().Delete(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllWebSessions removes all web sessions.
func (g *GRPCServer) DeleteAllWebSessions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebSessions().DeleteAll(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GetWebToken gets a web token.
func (g *GRPCServer) GetWebToken(ctx context.Context, req *types.GetWebTokenRequest) (*authpb.GetWebTokenResponse, error) {
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

	return &authpb.GetWebTokenResponse{
		Token: token,
	}, nil
}

// GetWebTokens gets all web tokens.
func (g *GRPCServer) GetWebTokens(ctx context.Context, _ *emptypb.Empty) (*authpb.GetWebTokensResponse, error) {
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

	return &authpb.GetWebTokensResponse{
		Tokens: out,
	}, nil
}

// DeleteWebToken removes the web token given with req.
func (g *GRPCServer) DeleteWebToken(ctx context.Context, req *types.DeleteWebTokenRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebTokens().Delete(ctx, *req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteAllWebTokens removes all web tokens.
func (g *GRPCServer) DeleteAllWebTokens(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.WebTokens().DeleteAll(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpsertKubernetesServer registers an kubernetes server.
func (g *GRPCServer) UpsertKubernetesServer(ctx context.Context, req *authpb.UpsertKubernetesServerRequest) (*types.KeepAlive, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keepAlive, err := auth.UpsertKubernetesServer(ctx, req.GetServer())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keepAlive, nil
}

// DeleteKubernetesServer deletes a kubernetes server.
func (g *GRPCServer) DeleteKubernetesServer(ctx context.Context, req *authpb.DeleteKubernetesServerRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteKubernetesServer(ctx, req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllKubernetesServers deletes all registered kubernetes servers.
func (g *GRPCServer) DeleteAllKubernetesServers(ctx context.Context, req *authpb.DeleteAllKubernetesServersRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllKubernetesServers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// maybeDowngradeRole tests the client version passed through the gRPC metadata,
// and if the client version is unknown or less than the minimum supported
// version for some features of the role returns a shallow copy of the given
// role downgraded for compatibility with the older version.
func maybeDowngradeRole(ctx context.Context, role *types.RoleV6) (*types.RoleV6, error) {
	clientVersionString, ok := metadata.ClientVersionFromContext(ctx)
	if !ok {
		// This client is not reporting its version via gRPC metadata. Teleport
		// clients have been reporting their version for long enough that older
		// clients won't even support v6 roles at all, so this is likely a
		// third-party client, and we shouldn't assume that downgrading the role
		// will do more good than harm.
		return role, nil
	}

	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		return nil, trace.BadParameter("unrecognized client version: %s is not a valid semver", clientVersionString)
	}

	role = maybeDowngradeRoleSSHPortForwarding(role, clientVersion)
	return role, nil
}

var minSupportedSSHPortForwardingVersion = semver.Version{Major: 17, Minor: 1, Patch: 0}

func maybeDowngradeRoleSSHPortForwarding(role *types.RoleV6, clientVersion *semver.Version) *types.RoleV6 {
	sshPortForwarding := role.GetOptions().SSHPortForwarding
	if sshPortForwarding == nil || (sshPortForwarding.Remote == nil && sshPortForwarding.Local == nil) {
		return role
	}

	if supported, err := utils.MinVerWithoutPreRelease(
		clientVersion.String(),
		minSupportedSSHPortForwardingVersion.String()); supported || err != nil {
		return role
	}

	role = apiutils.CloneProtoMsg(role)
	options := role.GetOptions()

	//nolint:staticcheck // this field is preserved for backwards compatibility
	options.PortForwarding = types.NewBoolOption(services.RoleSet{role}.CanPortForward())
	role.SetOptions(options)
	reason := fmt.Sprintf(`Client version %q does not support granular SSH port forwarding. Role %q will be downgraded `+
		`to simple port forwarding rules instead. In order to support granular SSH port forwarding, all clients must be `+
		`updated to version %q or higher.`, clientVersion, role.GetName(), minSupportedSSHPortForwardingVersion)
	if role.Metadata.Labels == nil {
		role.Metadata.Labels = make(map[string]string, 1)
	}
	role.Metadata.Labels[types.TeleportDowngradedLabel] = reason
	return role
}

// GetRole retrieves a role by name.
func (g *GRPCServer) GetRole(ctx context.Context, req *authpb.GetRoleRequest) (*types.RoleV6, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roleI, err := auth.ServerWithRoles.GetRole(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, ok := roleI.(*types.RoleV6)
	if !ok {
		return nil, trace.Errorf("encountered unexpected role type: %T", role)
	}

	downgraded, err := maybeDowngradeRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return downgraded, nil
}

// GetRoles retrieves all roles.
func (g *GRPCServer) GetRoles(ctx context.Context, _ *emptypb.Empty) (*authpb.GetRolesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rolesI, err := auth.ServerWithRoles.GetRoles(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var roles []*types.RoleV6
	for _, r := range rolesI {
		role, ok := r.(*types.RoleV6)
		if !ok {
			return nil, trace.BadParameter("unexpected type %T", r)
		}
		downgraded, err := maybeDowngradeRole(ctx, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, downgraded)
	}
	return &authpb.GetRolesResponse{
		Roles: roles,
	}, nil
}

// ListRoles is a paginated role getter.
func (g *GRPCServer) ListRoles(ctx context.Context, req *authpb.ListRolesRequest) (*authpb.ListRolesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := auth.ServerWithRoles.ListRoles(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	downgradedRoles := rsp.Roles[:0]
	for _, role := range rsp.Roles {
		downgraded, err := maybeDowngradeRole(ctx, role)
		if err != nil {
			g.logger.WarnContext(ctx, "Failed to downgrade role, this is a bug and may result in spurious access denied errors",
				"role", role.GetName(),
				"error", err,
			)
			continue
		}
		downgradedRoles = append(downgradedRoles, downgraded)
	}
	rsp.Roles = downgradedRoles

	return rsp, nil
}

// CreateRole creates a new role.
func (g *GRPCServer) CreateRole(ctx context.Context, req *authpb.CreateRoleRequest) (*types.RoleV6, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This check *must* happen at the RPC layer rather than somewhere like ValidateRole or CheckAndSetDefaults. We want to prevent role
	// creation and updates from defining both port_forwarding and ssh_port_forwarding for the same role. However, when making effective
	// roles available to nodes it should be possible for both fields to be assigned in order to maintain backwards compatibility with older
	// agents (similar to a role downgrade).
	//nolint:staticcheck // this field is preserved for backwards compatibility, but shouldn't be used going forward
	if req.Role.GetOptions().SSHPortForwarding != nil && req.Role.GetOptions().PortForwarding != nil {
		return nil, trace.BadParameter("options define both 'port_forwarding' and 'ssh_port_forwarding', only one can be set")
	}

	if err = services.ValidateRole(req.Role); err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := auth.ServerWithRoles.CreateRole(ctx, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	g.logger.DebugContext(ctx, "role upserted", "role_name", req.Role.GetName())

	v6, ok := created.(*types.RoleV6)
	if !ok {
		g.logger.WarnContext(ctx, "expected type RoleV6, got unexpected type",
			"role_type", logutils.TypeAttr(created),
			"role", created.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected role type")
	}

	return v6, nil
}

// UpdateRole updates an existing  role.
func (g *GRPCServer) UpdateRole(ctx context.Context, req *authpb.UpdateRoleRequest) (*types.RoleV6, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This check *must* happen at the RPC layer rather than somewhere like ValidateRole or CheckAndSetDefaults. We want to prevent role
	// creation and updates from defining both port_forwarding and ssh_port_forwarding for the same role. However, when making effective
	// roles available to nodes it should be possible for both fields to be assigned in order to maintain backwards compatibility with older
	// agents (similar to a role downgrade).
	//nolint:staticcheck // this field is preserved for backwards compatibility, but shouldn't be used going forward
	if req.Role.GetOptions().SSHPortForwarding != nil && req.Role.GetOptions().PortForwarding != nil {
		return nil, trace.BadParameter("options define both 'port_forwarding' and 'ssh_port_forwarding', only one can be set")
	}

	if err = services.ValidateRole(req.Role); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := auth.ServerWithRoles.UpdateRole(ctx, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	g.logger.DebugContext(ctx, "role upserted", "role", req.Role.GetName())

	v6, ok := updated.(*types.RoleV6)
	if !ok {
		g.logger.WarnContext(ctx, "expected type RoleV6, got unexpected type",
			"role_type", logutils.TypeAttr(updated),
			"role", updated.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected role type")
	}

	return v6, nil
}

// UpsertRoleV2 upserts a role.
func (g *GRPCServer) UpsertRoleV2(ctx context.Context, req *authpb.UpsertRoleRequest) (*types.RoleV6, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This check *must* happen at the RPC layer rather than somewhere like ValidateRole or CheckAndSetDefaults. We want to prevent role
	// creation and updates from defining both port_forwarding and ssh_port_forwarding for the same role. However, when making effective
	// roles available to nodes it should be possible for both fields to be assigned in order to maintain backwards compatibility with older
	// agents (similar to a role downgrade).
	//nolint:staticcheck // this field is preserved for backwards compatibility, but shouldn't be used going forward
	if req.Role.GetOptions().SSHPortForwarding != nil && req.Role.GetOptions().PortForwarding != nil {
		return nil, trace.BadParameter("options define both 'port_forwarding' and 'ssh_port_forwarding', only one can be set")
	}

	if err = services.ValidateRole(req.Role); err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := auth.ServerWithRoles.UpsertRole(ctx, req.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	g.logger.DebugContext(ctx, "role upserted", "role", req.Role.GetName())

	v6, ok := upserted.(*types.RoleV6)
	if !ok {
		g.logger.WarnContext(ctx, "expected type RoleV6, got unexpected type",
			"role_type", logutils.TypeAttr(upserted),
			"role", upserted.GetName(),
		)
		return nil, trace.BadParameter("encountered unexpected role type")
	}

	return v6, nil
}

// UpsertRole upserts a role.
func (g *GRPCServer) UpsertRole(ctx context.Context, role *types.RoleV6) (*emptypb.Empty, error) {
	_, err := g.UpsertRoleV2(ctx, &authpb.UpsertRoleRequest{Role: role})
	return &emptypb.Empty{}, trace.Wrap(err)
}

// DeleteRole deletes a role by name.
func (g *GRPCServer) DeleteRole(ctx context.Context, req *authpb.DeleteRoleRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteRole(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	g.logger.DebugContext(ctx, "role deleted", "role", req.GetName())

	return &emptypb.Empty{}, nil
}

// doMFAPresenceChallenge conducts an MFA presence challenge over a stream
// and updates the users presence for a given session.
//
// This function bypasses the `ServerWithRoles` RBAC layer. This is not
// usually how the gRPC layer accesses the underlying auth server API's but it's done
// here to avoid bloating the [authclient.ClientI]  interface with special logic that isn't designed to be touched
// by anyone external to this process. This is not the norm and caution should be taken
// when looking at or modifying this function. This is the same approach taken by other MFA
// related gRPC API endpoints.
func doMFAPresenceChallenge(ctx context.Context, actx *grpcContext, stream authpb.AuthService_MaintainSessionPresenceServer, challengeReq *authpb.PresenceMFAChallengeRequest) error {
	user := actx.User.GetName()

	chalExt := &mfav1pb.ChallengeExtensions{Scope: mfav1pb.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION}
	authChallenge, err := actx.authServer.mfaAuthChallenge(ctx, user, challengeReq.SSOClientRedirectURL, chalExt)
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

	if _, err := actx.authServer.ValidateMFAAuthResponse(ctx, challengeResp, user, chalExt); err != nil {
		return trace.Wrap(err)
	}

	err = actx.authServer.UpdatePresence(ctx, challengeReq.SessionID, user)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// MaintainSessionPresence establishes a channel used to continuously verify the presence for a session.
func (g *GRPCServer) MaintainSessionPresence(stream authpb.AuthService_MaintainSessionPresenceServer) error {
	ctx := stream.Context()
	actx, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
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

// Deprecated: Use AddMFADeviceSync instead.
func (g *GRPCServer) AddMFADevice(stream authpb.AuthService_AddMFADeviceServer) error {
	return trace.NotImplemented("method AddMFADevice is deprecated, use AddMFADeviceSync instead")
}

// Deprecated: Use DeleteMFADeviceSync instead.
func (g *GRPCServer) DeleteMFADevice(stream authpb.AuthService_DeleteMFADeviceServer) error {
	return trace.NotImplemented("method DeleteMFADevice is deprecated, use DeleteMFADeviceSync instead")
}

func mfaDeviceEventMetadata(d *types.MFADevice) apievents.MFADeviceMetadata {
	return apievents.MFADeviceMetadata{
		DeviceName: d.Metadata.Name,
		DeviceID:   d.Id,
		DeviceType: d.MFAType(),
	}
}

// AddMFADeviceSync is implemented by AuthService.AddMFADeviceSync.
func (g *GRPCServer) AddMFADeviceSync(ctx context.Context, req *authpb.AddMFADeviceSyncRequest) (*authpb.AddMFADeviceSyncResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := actx.ServerWithRoles.AddMFADeviceSync(ctx, req)
	return res, trace.Wrap(err)
}

// DeleteMFADeviceSync is implemented by AuthService.DeleteMFADeviceSync.
func (g *GRPCServer) DeleteMFADeviceSync(ctx context.Context, req *authpb.DeleteMFADeviceSyncRequest) (*emptypb.Empty, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := actx.ServerWithRoles.DeleteMFADeviceSync(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) GetMFADevices(ctx context.Context, req *authpb.GetMFADevicesRequest) (*authpb.GetMFADevicesResponse, error) {
	actx, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	devs, err := actx.ServerWithRoles.GetMFADevices(ctx, req)
	return devs, trace.Wrap(err)
}

// Deprecated: Use GenerateUserCerts instead.
func (g *GRPCServer) GenerateUserSingleUseCerts(stream authpb.AuthService_GenerateUserSingleUseCertsServer) error {
	return trace.NotImplemented("method GenerateUserSingleUseCerts is deprecated, use GenerateUserCerts instead")
}

func setUserSingleUseCertsTTL(actx *grpcContext, req *authpb.UserCertsRequest) {
	if isLocalProxyCertReq(req) {
		// don't limit the cert expiry to 1 minute for db local proxy tunnel or kube local proxy,
		// because the certs will be kept in-memory by the client to protect
		// against cert/key exfiltration. When MFA is required, cert expiration
		// time is bounded by the lifetime of the local proxy process or the mfa verification interval.
		return
	}

	maxExpiry := actx.authServer.GetClock().Now().Add(teleport.UserSingleUseCertTTL)
	if req.Expires.After(maxExpiry) {
		req.Expires = maxExpiry
	}
}

// isLocalProxyCertReq returns whether a cert request is for a local proxy cert.
func isLocalProxyCertReq(req *authpb.UserCertsRequest) bool {
	return (req.Usage == authpb.UserCertsRequest_Database &&
		req.RequesterName == authpb.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL) ||
		(req.Usage == authpb.UserCertsRequest_Kubernetes &&
			(req.RequesterName == authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY || req.RequesterName == authpb.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_HEADLESS)) ||
		(req.Usage == authpb.UserCertsRequest_App &&
			req.RequesterName == authpb.UserCertsRequest_TSH_APP_LOCAL_PROXY)
}

func userSingleUseCertsGenerate(ctx context.Context, actx *grpcContext, req authpb.UserCertsRequest) (*authpb.Certs, error) {
	// Get the client IP.
	clientPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.BadParameter("no peer info in gRPC stream, can't get client IP")
	}
	clientIP, _, err := net.SplitHostPort(clientPeer.Addr.String())
	if err != nil {
		return nil, trace.BadParameter("can't parse client IP from peer info: %v", err)
	}

	// MFA certificates are supposed to be always pinned to IP, but it was decided to turn this off until
	// IP pinning comes out of preview. Here we would add option to pin the cert, see commit of this comment for restoring.
	opts := []certRequestOption{
		certRequestPreviousIdentityExpires(actx.Identity.GetIdentity().Expires),
		certRequestLoginIP(clientIP),
		certRequestDeviceExtensions(actx.Identity.GetIdentity().DeviceExtensions),
	}

	// Generate the cert.
	certs, err := actx.generateUserCerts(ctx, req, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Defensively forward only the expected certificate, according to the
	// requested usage.
	resp := &authpb.Certs{}
	switch req.Usage {
	case authpb.UserCertsRequest_SSH:
		resp.SSH = certs.SSH
	case authpb.UserCertsRequest_Kubernetes, authpb.UserCertsRequest_Database, authpb.UserCertsRequest_WindowsDesktop, authpb.UserCertsRequest_App:
		resp.TLS = certs.TLS
	default:
		return nil, trace.BadParameter("unknown certificate usage %q", req.Usage)
	}
	return resp, nil
}

func (g *GRPCServer) IsMFARequired(ctx context.Context, req *authpb.IsMFARequiredRequest) (*authpb.IsMFARequiredResponse, error) {
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

// GetOIDCConnectors retrieves valid OIDC connectors, errors from individual connectors are not forwarded.
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

// CreateOIDCConnector creates a new OIDC connector.
func (g *GRPCServer) CreateOIDCConnector(ctx context.Context, req *authpb.CreateOIDCConnectorRequest) (*types.OIDCConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := auth.ServerWithRoles.CreateOIDCConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v3, ok := created.(*types.OIDCConnectorV3)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected OIDC connector type: %T", created)
	}

	return v3, nil
}

// UpdateOIDCConnector updates an existing OIDC connector.
func (g *GRPCServer) UpdateOIDCConnector(ctx context.Context, req *authpb.UpdateOIDCConnectorRequest) (*types.OIDCConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := auth.ServerWithRoles.UpdateOIDCConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v3, ok := updated.(*types.OIDCConnectorV3)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected OIDC connector type: %T", updated)
	}

	return v3, nil
}

// UpsertOIDCConnectorV2 creates a new or replaces an existing OIDC connector.
func (g *GRPCServer) UpsertOIDCConnectorV2(ctx context.Context, req *authpb.UpsertOIDCConnectorRequest) (*types.OIDCConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upserted, err := auth.ServerWithRoles.UpsertOIDCConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v3, ok := upserted.(*types.OIDCConnectorV3)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected OIDC connector type: %T", upserted)
	}

	return v3, nil
}

// UpsertOIDCConnector creates a new or replaces an existing OIDC connector.
// Deprecated: Use [GRPCServer.UpsertOIDCConnectorV2] instead.
func (g *GRPCServer) UpsertOIDCConnector(ctx context.Context, oidcConnector *types.OIDCConnectorV3) (*emptypb.Empty, error) {
	if _, err := g.UpsertOIDCConnectorV2(ctx, &authpb.UpsertOIDCConnectorRequest{Connector: oidcConnector}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteOIDCConnector deletes an OIDC connector by name.
func (g *GRPCServer) DeleteOIDCConnector(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteOIDCConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetOIDCAuthRequest(ctx context.Context, req *authpb.GetOIDCAuthRequestRequest) (*types.OIDCAuthRequest, error) {
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

// GetSAMLConnectors retrieves valid SAML connectors, errors from individual connectors are not forwarded.
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

// CreateSAMLConnector creates a new SAML connector.
func (g *GRPCServer) CreateSAMLConnector(ctx context.Context, req *authpb.CreateSAMLConnectorRequest) (*types.SAMLConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := auth.ServerWithRoles.CreateSAMLConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v2, ok := created.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected SAML connector type: %T", created)
	}

	return v2, nil
}

// UpdateSAMLConnector updates an existing SAML connector.
func (g *GRPCServer) UpdateSAMLConnector(ctx context.Context, req *authpb.UpdateSAMLConnectorRequest) (*types.SAMLConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := auth.ServerWithRoles.UpdateSAMLConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v2, ok := updated.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected SAML connector type: %T", updated)
	}

	return v2, nil
}

// UpsertSAMLConnectorV2 creates a new or replaces an existing SAML connector.
func (g *GRPCServer) UpsertSAMLConnectorV2(ctx context.Context, req *authpb.UpsertSAMLConnectorRequest) (*types.SAMLConnectorV2, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := auth.ServerWithRoles.UpsertSAMLConnector(ctx, req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	v2, ok := upserted.(*types.SAMLConnectorV2)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected SAML connector type: %T", upserted)
	}

	return v2, nil
}

// UpsertSAMLConnector upserts a SAML connector.
// Deprecated: Use [GRPCServer.UpsertSAMLConnectorV2] instead.
func (g *GRPCServer) UpsertSAMLConnector(ctx context.Context, samlConnector *types.SAMLConnectorV2) (*emptypb.Empty, error) {
	if _, err := g.UpsertSAMLConnectorV2(ctx, &authpb.UpsertSAMLConnectorRequest{Connector: samlConnector}); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteSAMLConnector deletes a SAML connector by name.
func (g *GRPCServer) DeleteSAMLConnector(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteSAMLConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetSAMLAuthRequest(ctx context.Context, req *authpb.GetSAMLAuthRequestRequest) (*types.SAMLAuthRequest, error) {
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
		return nil, trace.Errorf("encountered unexpected GitHub connector type: %T", gc)
	}
	return githubConnectorV3, nil
}

// GetGithubConnectors retrieves valid GitHub connectors, errors from individual connectors are not forwarded.
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
			return nil, trace.Errorf("encountered unexpected GitHub connector type: %T", gc)
		}
	}
	return &types.GithubConnectorV3List{
		GithubConnectors: githubConnectorsV3,
	}, nil
}

// UpsertGithubConnectorV2 creates a new or replaces an existing Github connector.
func (g *GRPCServer) UpsertGithubConnectorV2(ctx context.Context, req *authpb.UpsertGithubConnectorRequest) (*types.GithubConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubConnector, err := services.InitGithubConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upserted, err := auth.ServerWithRoles.UpsertGithubConnector(ctx, githubConnector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubConnectorV3, err := services.ConvertGithubConnector(upserted)
	return githubConnectorV3, trace.Wrap(err)
}

// UpsertGithubConnector creates a new or replaces an existing Github connector.
// Deprecated: Use [GRPCServer.UpsertGithubConnectorV2] instead.
func (g *GRPCServer) UpsertGithubConnector(ctx context.Context, connector *types.GithubConnectorV3) (*emptypb.Empty, error) {
	if _, err := g.UpsertGithubConnectorV2(ctx, &authpb.UpsertGithubConnectorRequest{Connector: connector}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateGithubConnector updates an existing Github connector.
func (g *GRPCServer) UpdateGithubConnector(ctx context.Context, req *authpb.UpdateGithubConnectorRequest) (*types.GithubConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubConnector, err := services.InitGithubConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := auth.ServerWithRoles.UpdateGithubConnector(ctx, githubConnector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubConnectorV3, err := services.ConvertGithubConnector(updated)
	return githubConnectorV3, trace.Wrap(err)
}

// CreateGithubConnector creates a new  Github connector.
func (g *GRPCServer) CreateGithubConnector(ctx context.Context, req *authpb.CreateGithubConnectorRequest) (*types.GithubConnectorV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	githubConnector, err := services.InitGithubConnector(req.Connector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	created, err := auth.ServerWithRoles.CreateGithubConnector(ctx, githubConnector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	githubConnectorV3, err := services.ConvertGithubConnector(created)
	return githubConnectorV3, trace.Wrap(err)
}

// DeleteGithubConnector deletes a Github connector by name.
func (g *GRPCServer) DeleteGithubConnector(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteGithubConnector(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetGithubAuthRequest(ctx context.Context, req *authpb.GetGithubAuthRequestRequest) (*types.GithubAuthRequest, error) {
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
func (g *GRPCServer) GetSSODiagnosticInfo(ctx context.Context, req *authpb.GetSSODiagnosticInfoRequest) (*types.SSODiagnosticInfo, error) {
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

// GetServerInfos returns a stream of ServerInfos.
func (g *GRPCServer) GetServerInfos(_ *emptypb.Empty, stream authpb.AuthService_GetServerInfosServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	infos := auth.GetServerInfos(stream.Context())
	for infos.Next() {
		si, ok := infos.Item().(*types.ServerInfoV1)
		if !ok {
			g.logger.WarnContext(stream.Context(), "expected type ServerInfoV1, got unexpected type",
				"server_info_type", logutils.TypeAttr(infos.Item()),
				"server_info_name", infos.Item().GetName(),
			)
		}
		if err := stream.Send(si); err != nil {
			infos.Done()
			if errors.Is(err, io.EOF) {
				return nil
			}
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(infos.Done())
}

// GetServerInfo returns a ServerInfo by name.
func (g *GRPCServer) GetServerInfo(ctx context.Context, req *types.ResourceRequest) (*types.ServerInfoV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	si, err := auth.ServerWithRoles.GetServerInfo(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverInfoV1, ok := si.(*types.ServerInfoV1)
	if !ok {
		return nil, trace.BadParameter("encountered unexpected Server Info type %T", si)
	}
	return serverInfoV1, nil
}

// UpsertServerInfo upserts a ServerInfo.
func (g *GRPCServer) UpsertServerInfo(ctx context.Context, si *types.ServerInfoV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.UpsertServerInfo(ctx, si); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteServerInfo deletes a ServerInfo by name.
func (g *GRPCServer) DeleteServerInfo(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteServerInfo(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllServerInfos deletes all ServerInfos.
func (g *GRPCServer) DeleteAllServerInfos(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteAllServerInfos(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetTrustedClusters(ctx context.Context, _ *emptypb.Empty) (*types.TrustedClusterV2List, error) {
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
//
// Deprecated: Use UpsertTrustedClusterV2 instead.
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
func (g *GRPCServer) DeleteTrustedCluster(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteTrustedCluster(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetTokens(ctx context.Context, _ *emptypb.Empty) (*types.ProvisionTokenV2List, error) {
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

// UpsertTokenV2 upserts a token.
func (g *GRPCServer) UpsertTokenV2(ctx context.Context, req *authpb.UpsertTokenV2Request) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// When new token versions are introduced, this can be exchanged for a
	// switch statement.
	token := req.GetV2()
	if token == nil {
		return nil, trail.ToGRPC(
			trace.BadParameter("token not provided in request"),
		)
	}
	if err = auth.ServerWithRoles.UpsertToken(ctx, token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// CreateTokenV2 creates a token.
func (g *GRPCServer) CreateTokenV2(ctx context.Context, req *authpb.CreateTokenV2Request) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// When new token versions are introduced, this can be exchanged for a
	// switch statement.
	token := req.GetV2()
	if token == nil {
		return nil, trail.ToGRPC(
			trace.BadParameter("token not provided in request"),
		)
	}
	if err = auth.ServerWithRoles.CreateToken(ctx, token); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteToken deletes a token by name.
func (g *GRPCServer) DeleteToken(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ServerWithRoles.DeleteToken(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) DeleteNode(ctx context.Context, req *types.ResourceInNamespaceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.DeleteNode(ctx, req.Namespace, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllNodes deletes all nodes in a given namespace.
func (g *GRPCServer) DeleteAllNodes(ctx context.Context, req *types.ResourcesInNamespaceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.DeleteAllNodes(ctx, req.Namespace); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetClusterAuditConfig gets cluster audit configuration.
func (g *GRPCServer) GetClusterAuditConfig(ctx context.Context, _ *emptypb.Empty) (*types.ClusterAuditConfigV2, error) {
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
func (g *GRPCServer) GetClusterNetworkingConfig(ctx context.Context, _ *emptypb.Empty) (*types.ClusterNetworkingConfigV2, error) {
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
func (g *GRPCServer) SetClusterNetworkingConfig(ctx context.Context, netConfig *types.ClusterNetworkingConfigV2) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	netConfig.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetClusterNetworkingConfig(ctx, netConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// ResetClusterNetworkingConfig resets cluster networking configuration to defaults.
func (g *GRPCServer) ResetClusterNetworkingConfig(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetClusterNetworkingConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetSessionRecordingConfig gets session recording configuration.
func (g *GRPCServer) GetSessionRecordingConfig(ctx context.Context, _ *emptypb.Empty) (*types.SessionRecordingConfigV2, error) {
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
func (g *GRPCServer) SetSessionRecordingConfig(ctx context.Context, recConfig *types.SessionRecordingConfigV2) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recConfig.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetSessionRecordingConfig(ctx, recConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// ResetSessionRecordingConfig resets session recording configuration to defaults.
func (g *GRPCServer) ResetSessionRecordingConfig(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetSessionRecordingConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetAuthPreference gets cluster auth preference.
func (g *GRPCServer) GetAuthPreference(ctx context.Context, _ *emptypb.Empty) (*types.AuthPreferenceV2, error) {
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
// Deprecated: Use Update/UpsertAuthPreference where appropriate.
func (g *GRPCServer) SetAuthPreference(ctx context.Context, authPref *types.AuthPreferenceV2) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authPref.SetOrigin(types.OriginDynamic)
	if err = auth.ServerWithRoles.SetAuthPreference(ctx, authPref); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// ResetAuthPreference resets cluster auth preference to defaults.
func (g *GRPCServer) ResetAuthPreference(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = auth.ServerWithRoles.ResetAuthPreference(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// StreamSessionEvents streams all events from a given session recording. An error is returned on the first
// channel if one is encountered. Otherwise the event channel is closed when the stream ends.
// The event channel is not closed on error to prevent race conditions in downstream select statements.
func (g *GRPCServer) StreamSessionEvents(req *authpb.StreamSessionEventsRequest, stream authpb.AuthService_StreamSessionEventsServer) error {
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
func (g *GRPCServer) GetNetworkRestrictions(ctx context.Context, _ *emptypb.Empty) (*types.NetworkRestrictionsV4, error) {
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
func (g *GRPCServer) SetNetworkRestrictions(ctx context.Context, nr *types.NetworkRestrictionsV4) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err = auth.ServerWithRoles.SetNetworkRestrictions(ctx, nr); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteNetworkRestrictions deletes the network restrictions.
func (g *GRPCServer) DeleteNetworkRestrictions(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trail.ToGRPC(err)
	}

	if err = auth.ServerWithRoles.DeleteNetworkRestrictions(ctx); err != nil {
		return nil, trail.ToGRPC(err)
	}
	return &emptypb.Empty{}, nil
}

// GetEvents searches for events on the backend and sends them back in a response.
func (g *GRPCServer) GetEvents(ctx context.Context, req *authpb.GetEventsRequest) (*authpb.Events, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastkey, err := auth.ServerWithRoles.SearchEvents(ctx, events.SearchEventsRequest{
		From:       req.StartDate,
		To:         req.EndDate,
		EventTypes: req.EventTypes,
		Limit:      int(req.Limit),
		Order:      types.EventOrder(req.Order),
		StartKey:   req.StartKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res *authpb.Events = &authpb.Events{}

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
func (g *GRPCServer) GetSessionEvents(ctx context.Context, req *authpb.GetSessionEventsRequest) (*authpb.Events, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastkey, err := auth.ServerWithRoles.SearchSessionEvents(ctx, events.SearchSessionEventsRequest{
		From:     req.StartDate,
		To:       req.EndDate,
		Limit:    int(req.Limit),
		Order:    types.EventOrder(req.Order),
		StartKey: req.StartKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var res *authpb.Events = &authpb.Events{}

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
func (g *GRPCServer) GetLock(ctx context.Context, req *authpb.GetLockRequest) (*types.LockV2, error) {
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
func (g *GRPCServer) GetLocks(ctx context.Context, req *authpb.GetLocksRequest) (*authpb.GetLocksResponse, error) {
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
	return &authpb.GetLocksResponse{
		Locks: lockV2s,
	}, nil
}

// UpsertLock upserts a lock.
func (g *GRPCServer) UpsertLock(ctx context.Context, lock *types.LockV2) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertLock(ctx, lock); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteLock deletes a lock.
func (g *GRPCServer) DeleteLock(ctx context.Context, req *authpb.DeleteLockRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteLock(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// ReplaceRemoteLocks replaces the set of locks associated with a remote cluster.
func (g *GRPCServer) ReplaceRemoteLocks(ctx context.Context, req *authpb.ReplaceRemoteLocksRequest) (*emptypb.Empty, error) {
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
	return &emptypb.Empty{}, nil
}

// CreateApp creates a new application resource.
func (g *GRPCServer) CreateApp(ctx context.Context, app *types.AppV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if app.Origin() == "" {
		app.SetOrigin(types.OriginDynamic)
	}
	if err := auth.CreateApp(ctx, app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateApp updates existing application resource.
func (g *GRPCServer) UpdateApp(ctx context.Context, app *types.AppV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if app.Origin() == "" {
		app.SetOrigin(types.OriginDynamic)
	}
	if err := auth.UpdateApp(ctx, app); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetApps(ctx context.Context, _ *emptypb.Empty) (*types.AppV3List, error) {
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
func (g *GRPCServer) DeleteApp(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteApp(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllApps removes all application resources.
func (g *GRPCServer) DeleteAllApps(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllApps(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// CreateDatabase creates a new database resource.
func (g *GRPCServer) CreateDatabase(ctx context.Context, database *types.DatabaseV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if database.Origin() == "" {
		database.SetOrigin(types.OriginDynamic)
	}
	if err := services.ValidateDatabase(database); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CreateDatabase(ctx, database); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateDatabase updates existing database resource.
func (g *GRPCServer) UpdateDatabase(ctx context.Context, database *types.DatabaseV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if database.Origin() == "" {
		database.SetOrigin(types.OriginDynamic)
	}
	if err := services.ValidateDatabase(database); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpdateDatabase(ctx, database); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
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
func (g *GRPCServer) GetDatabases(ctx context.Context, _ *emptypb.Empty) (*types.DatabaseV3List, error) {
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
func (g *GRPCServer) DeleteDatabase(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteDatabase(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllDatabases removes all databases.
func (g *GRPCServer) DeleteAllDatabases(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllDatabases(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetWindowsDesktopServices returns all registered Windows desktop services.
func (g *GRPCServer) GetWindowsDesktopServices(ctx context.Context, req *emptypb.Empty) (*authpb.GetWindowsDesktopServicesResponse, error) {
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
	return &authpb.GetWindowsDesktopServicesResponse{
		Services: services,
	}, nil
}

// GetWindowsDesktopService returns a registered Windows desktop service by name.
func (g *GRPCServer) GetWindowsDesktopService(ctx context.Context, req *authpb.GetWindowsDesktopServiceRequest) (*authpb.GetWindowsDesktopServiceResponse, error) {
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
	return &authpb.GetWindowsDesktopServiceResponse{
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
	clientAddr, err := authz.ClientSrcAddrFromContext(ctx)
	if err != nil {
		g.logger.WarnContext(ctx, "error getting client address from context", "error", err)
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
func (g *GRPCServer) DeleteWindowsDesktopService(ctx context.Context, req *authpb.DeleteWindowsDesktopServiceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteWindowsDesktopService(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllWindowsDesktopServices removes all registered Windows desktop services.
func (g *GRPCServer) DeleteAllWindowsDesktopServices(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllWindowsDesktopServices(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetWindowsDesktops returns all registered Windows desktop hosts.
func (g *GRPCServer) GetWindowsDesktops(ctx context.Context, filter *types.WindowsDesktopFilter) (*authpb.GetWindowsDesktopsResponse, error) {
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
	return &authpb.GetWindowsDesktopsResponse{
		Desktops: desktops,
	}, nil
}

// CreateWindowsDesktop registers a new Windows desktop host.
func (g *GRPCServer) CreateWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.CreateWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateWindowsDesktop updates an existing Windows desktop host.
func (g *GRPCServer) UpdateWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpdateWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// UpsertWindowsDesktop updates a Windows desktop host, creating it if it doesn't exist.
func (g *GRPCServer) UpsertWindowsDesktop(ctx context.Context, desktop *types.WindowsDesktopV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.UpsertWindowsDesktop(ctx, desktop); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteWindowsDesktop removes the specified windows desktop host.
// Note: unlike GetWindowsDesktops, this will delete at-most one desktop.
// Passing an empty host ID will not trigger "delete all" behavior. To delete
// all desktops, use DeleteAllWindowsDesktops.
func (g *GRPCServer) DeleteWindowsDesktop(ctx context.Context, req *authpb.DeleteWindowsDesktopRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteWindowsDesktop(ctx, req.GetHostID(), req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllWindowsDesktops removes all registered Windows desktop hosts.
func (g *GRPCServer) DeleteAllWindowsDesktops(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.DeleteAllWindowsDesktops(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GenerateWindowsDesktopCert generates client certificate for Windows RDP
// authentication.
func (g *GRPCServer) GenerateWindowsDesktopCert(ctx context.Context, req *authpb.WindowsDesktopCertRequest) (*authpb.WindowsDesktopCertResponse, error) {
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

func (g *GRPCServer) GetDesktopBootstrapScript(ctx context.Context, _ *emptypb.Empty) (*authpb.DesktopBootstrapScriptResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := auth.GetDesktopBootstrapScript(ctx)
	return res, trace.Wrap(err)
}

// ChangeUserAuthentication implements AuthService.ChangeUserAuthentication.
func (g *GRPCServer) ChangeUserAuthentication(ctx context.Context, req *authpb.ChangeUserAuthenticationRequest) (*authpb.ChangeUserAuthenticationResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := auth.ServerWithRoles.ChangeUserAuthentication(ctx, req)
	return res, trace.Wrap(err)
}

// StartAccountRecovery is implemented by AuthService.StartAccountRecovery.
func (g *GRPCServer) StartAccountRecovery(ctx context.Context, req *authpb.StartAccountRecoveryRequest) (*types.UserTokenV3, error) {
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
func (g *GRPCServer) VerifyAccountRecovery(ctx context.Context, req *authpb.VerifyAccountRecoveryRequest) (*types.UserTokenV3, error) {
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
func (g *GRPCServer) CompleteAccountRecovery(ctx context.Context, req *authpb.CompleteAccountRecoveryRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = auth.ServerWithRoles.CompleteAccountRecovery(ctx, req)
	return &emptypb.Empty{}, trace.Wrap(err)
}

// CreateAccountRecoveryCodes is implemented by AuthService.CreateAccountRecoveryCodes.
func (g *GRPCServer) CreateAccountRecoveryCodes(ctx context.Context, req *authpb.CreateAccountRecoveryCodesRequest) (*authpb.RecoveryCodes, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := auth.ServerWithRoles.CreateAccountRecoveryCodes(ctx, req)
	return res, trace.Wrap(err)
}

// GetAccountRecoveryToken is implemented by AuthService.GetAccountRecoveryToken.
func (g *GRPCServer) GetAccountRecoveryToken(ctx context.Context, req *authpb.GetAccountRecoveryTokenRequest) (*types.UserTokenV3, error) {
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
func (g *GRPCServer) GetAccountRecoveryCodes(ctx context.Context, req *authpb.GetAccountRecoveryCodesRequest) (*authpb.RecoveryCodes, error) {
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
func (g *GRPCServer) CreateAuthenticateChallenge(ctx context.Context, req *authpb.CreateAuthenticateChallengeRequest) (*authpb.MFAAuthenticateChallenge, error) {
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
func (g *GRPCServer) CreatePrivilegeToken(ctx context.Context, req *authpb.CreatePrivilegeTokenRequest) (*types.UserTokenV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := auth.CreatePrivilegeToken(ctx, req)
	return token, trace.Wrap(err)
}

// CreateRegisterChallenge is implemented by AuthService.CreateRegisterChallenge.
func (g *GRPCServer) CreateRegisterChallenge(ctx context.Context, req *authpb.CreateRegisterChallengeRequest) (*authpb.MFARegisterChallenge, error) {
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
func (g *GRPCServer) GenerateCertAuthorityCRL(ctx context.Context, req *authpb.CertAuthorityRequest) (*authpb.CRL, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	crl, err := auth.GenerateCertAuthorityCRL(ctx, req.Type)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authpb.CRL{CRL: crl}, nil
}

// ListUnifiedResources retrieves a paginated list of unified resources.
func (g *GRPCServer) ListUnifiedResources(ctx context.Context, req *authpb.ListUnifiedResourcesRequest) (*authpb.ListUnifiedResourcesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return auth.ListUnifiedResources(ctx, req)
}

// ListResources retrieves a paginated list of resources.
func (g *GRPCServer) ListResources(ctx context.Context, req *authpb.ListResourcesRequest) (*authpb.ListResourcesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := auth.ListResources(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	paginatedResources, err := services.MakePaginatedResources(ctx, req.ResourceType, resp.Resources, nil /* requestable map */)
	if err != nil {
		return nil, trace.Wrap(err, "making paginated resources")
	}
	protoResp := &authpb.ListResourcesResponse{
		NextKey:    resp.NextKey,
		Resources:  paginatedResources,
		TotalCount: int32(resp.TotalCount),
	}

	return protoResp, nil
}

func (g *GRPCServer) GetSSHTargets(ctx context.Context, req *authpb.GetSSHTargetsRequest) (*authpb.GetSSHTargetsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := auth.ServerWithRoles.GetSSHTargets(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// ResolveSSHTarget gets a server that would match an equivalent ssh dial request.
func (g *GRPCServer) ResolveSSHTarget(ctx context.Context, req *authpb.ResolveSSHTargetRequest) (*authpb.ResolveSSHTargetResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := auth.ServerWithRoles.ResolveSSHTarget(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rsp, nil
}

// CreateSessionTracker creates a tracker resource for an active session.
func (g *GRPCServer) CreateSessionTracker(ctx context.Context, req *authpb.CreateSessionTrackerRequest) (*types.SessionTrackerV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.SessionTracker == nil {
		g.logger.ErrorContext(ctx, "Missing SessionTracker in CreateSessionTrackerRequest, this can be caused by an outdated Teleport node running against your cluster")
		return nil, trace.BadParameter("missing SessionTracker from CreateSessionTrackerRequest")
	}

	tracker, err := auth.ServerWithRoles.CreateSessionTracker(ctx, req.SessionTracker)
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
func (g *GRPCServer) GetSessionTracker(ctx context.Context, req *authpb.GetSessionTrackerRequest) (*types.SessionTrackerV1, error) {
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
func (g *GRPCServer) GetActiveSessionTrackers(_ *emptypb.Empty, stream authpb.AuthService_GetActiveSessionTrackersServer) error {
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

// GetActiveSessionTrackersWithFilter returns a list of active sessions filtered by a filter.
func (g *GRPCServer) GetActiveSessionTrackersWithFilter(filter *types.SessionTrackerFilter, stream authpb.AuthService_GetActiveSessionTrackersWithFilterServer) error {
	ctx := stream.Context()
	auth, err := g.authenticate(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	sessions, err := auth.ServerWithRoles.GetActiveSessionTrackersWithFilter(ctx, filter)
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
func (g *GRPCServer) RemoveSessionTracker(ctx context.Context, req *authpb.RemoveSessionTrackerRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.ServerWithRoles.RemoveSessionTracker(ctx, req.SessionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateSessionTracker updates a tracker resource for an active session.
func (g *GRPCServer) UpdateSessionTracker(ctx context.Context, req *authpb.UpdateSessionTrackerRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = auth.ServerWithRoles.UpdateSessionTracker(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetDomainName returns local auth domain of the current auth server.
func (g *GRPCServer) GetDomainName(ctx context.Context, req *emptypb.Empty) (*authpb.GetDomainNameResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dn, err := auth.ServerWithRoles.GetDomainName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authpb.GetDomainNameResponse{
		DomainName: dn,
	}, nil
}

// GetClusterCACert returns the PEM-encoded TLS certs for the local cluster
// without signing keys. If the cluster has multiple TLS certs, they will all
// be appended.
func (g *GRPCServer) GetClusterCACert(
	ctx context.Context, req *emptypb.Empty,
) (*authpb.GetClusterCACertResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return auth.ServerWithRoles.GetClusterCACert(ctx)
}

// GetConnectionDiagnostic reads a connection diagnostic.
func (g *GRPCServer) GetConnectionDiagnostic(ctx context.Context, req *authpb.GetConnectionDiagnosticRequest) (*types.ConnectionDiagnosticV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := auth.ServerWithRoles.GetConnectionDiagnostic(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnosticV1, ok := connectionDiagnostic.(*types.ConnectionDiagnosticV1)
	if !ok {
		return nil, trace.BadParameter("unexpected connection diagnostic type %T", connectionDiagnostic)
	}

	return connectionDiagnosticV1, nil
}

// CreateConnectionDiagnostic creates a connection diagnostic
func (g *GRPCServer) CreateConnectionDiagnostic(ctx context.Context, connectionDiagnostic *types.ConnectionDiagnosticV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.CreateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SetInstaller sets the installer script resource
func (g *GRPCServer) SetInstaller(ctx context.Context, req *types.InstallerV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.SetInstaller(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) SetUIConfig(ctx context.Context, req *types.UIConfigV1) (*emptypb.Empty, error) {
	// TODO (avatus) send an audit event when SetUIConfig is called
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.SetUIConfig(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) GetUIConfig(ctx context.Context, _ *emptypb.Empty) (*types.UIConfigV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiconfig, err := auth.ServerWithRoles.GetUIConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiconfigv1, ok := uiconfig.(*types.UIConfigV1)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", uiconfig)
	}
	return uiconfigv1, nil
}

func (g *GRPCServer) DeleteUIConfig(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteUIConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetInstaller retrieves the installer script resource
func (g *GRPCServer) GetInstaller(ctx context.Context, req *types.ResourceRequest) (*types.InstallerV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := auth.GetInstaller(ctx, req.Name)
	if err != nil {
		if trace.IsNotFound(err) {
			switch req.Name {
			case installers.InstallerScriptName:
				return installer.DefaultInstaller, nil
			case installers.InstallerScriptNameAgentless:
				return installers.DefaultAgentlessInstaller, nil
			}
		}
		return nil, trace.Wrap(err)
	}
	inst, ok := res.(*types.InstallerV1)
	if !ok {
		return nil, trace.BadParameter("unexpected installer type %T", res)
	}
	return inst, nil
}

// GetInstallers returns all installer script resources registered in the cluster.
func (g *GRPCServer) GetInstallers(ctx context.Context, _ *emptypb.Empty) (*types.InstallerV1List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := auth.GetInstallers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var installersV1 []*types.InstallerV1
	defaultInstallers := map[string]*types.InstallerV1{
		types.DefaultInstallerScriptName:        installer.DefaultInstaller,
		installers.InstallerScriptNameAgentless: installers.DefaultAgentlessInstaller,
	}

	for _, inst := range res {
		instV1, ok := inst.(*types.InstallerV1)
		if !ok {
			return nil, trace.BadParameter("unsupported installer type %T", inst)
		}
		delete(defaultInstallers, inst.GetName())
		installersV1 = append(installersV1, instV1)
	}

	for _, inst := range defaultInstallers {
		installersV1 = append(installersV1, inst)
	}

	return &types.InstallerV1List{
		Installers: installersV1,
	}, nil
}

// DeleteInstaller sets the installer script resource to its default
func (g *GRPCServer) DeleteInstaller(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteInstaller(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteALlInstallers deletes all the installers
func (g *GRPCServer) DeleteAllInstallers(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllInstallers(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateConnectionDiagnostic updates a connection diagnostic
func (g *GRPCServer) UpdateConnectionDiagnostic(ctx context.Context, connectionDiagnostic *types.ConnectionDiagnosticV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.ServerWithRoles.UpdateConnectionDiagnostic(ctx, connectionDiagnostic); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// AppendDiagnosticTrace updates a connection diagnostic
func (g *GRPCServer) AppendDiagnosticTrace(ctx context.Context, in *authpb.AppendDiagnosticTraceRequest) (*types.ConnectionDiagnosticV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnostic, err := auth.ServerWithRoles.AppendDiagnosticTrace(ctx, in.Name, in.Trace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	connectionDiagnosticV1, ok := connectionDiagnostic.(*types.ConnectionDiagnosticV1)
	if !ok {
		return nil, trace.BadParameter("unexpected connection diagnostic type %T", connectionDiagnostic)
	}

	return connectionDiagnosticV1, nil
}

// GetKubernetesCluster returns the specified kubernetes cluster resource.
func (g *GRPCServer) GetKubernetesCluster(ctx context.Context, req *types.ResourceRequest) (*types.KubernetesClusterV3, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeCluster, err := auth.GetKubernetesCluster(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeClusterV3, ok := kubeCluster.(*types.KubernetesClusterV3)
	if !ok {
		return nil, trace.BadParameter("unsupported kubernetes cluster type %T", kubeCluster)
	}
	return kubeClusterV3, nil
}

// CreateKubernetesCluster creates a new kubernetes cluster resource.
func (g *GRPCServer) CreateKubernetesCluster(ctx context.Context, cluster *types.KubernetesClusterV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if origin is not set, force it to be dynamic.
	if len(cluster.Origin()) == 0 {
		cluster.SetOrigin(types.OriginDynamic)
	}
	if err := auth.CreateKubernetesCluster(ctx, cluster); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// UpdateKubernetesCluster updates existing kubernetes cluster resource.
func (g *GRPCServer) UpdateKubernetesCluster(ctx context.Context, cluster *types.KubernetesClusterV3) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if origin is not set, force it to be dynamic.
	if len(cluster.Origin()) == 0 {
		cluster.SetOrigin(types.OriginDynamic)
	}
	if err := auth.UpdateKubernetesCluster(ctx, cluster); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetKubernetesClusters returns all kubernetes cluster resources.
func (g *GRPCServer) GetKubernetesClusters(ctx context.Context, _ *emptypb.Empty) (*types.KubernetesClusterV3List, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters, err := auth.GetKubernetesClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeClusters := make([]*types.KubernetesClusterV3, 0, len(clusters))
	for _, cluster := range clusters {
		clusterV3, ok := cluster.(*types.KubernetesClusterV3)
		if !ok {
			return nil, trace.BadParameter("unsupported kube cluster type %T", cluster)
		}
		kubeClusters = append(kubeClusters, clusterV3)
	}
	return &types.KubernetesClusterV3List{
		KubernetesClusters: kubeClusters,
	}, nil
}

// DeleteKubernetesCluster removes the specified kubernetes cluster.
func (g *GRPCServer) DeleteKubernetesCluster(ctx context.Context, req *types.ResourceRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteKubernetesCluster(ctx, req.Name); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// DeleteAllKubernetesClusters removes all kubernetes cluster.
func (g *GRPCServer) DeleteAllKubernetesClusters(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.DeleteAllKubernetesClusters(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

func (g *GRPCServer) ChangePassword(ctx context.Context, req *authpb.ChangePasswordRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.ChangePassword(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// SubmitUsageEvent submits an external usage event.
func (g *GRPCServer) SubmitUsageEvent(ctx context.Context, req *authpb.SubmitUsageEventRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := auth.SubmitUsageEvent(ctx, req); err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, nil
}

// GetLicense returns the license used to start the auth server.
func (g *GRPCServer) GetLicense(ctx context.Context, req *authpb.GetLicenseRequest) (*authpb.GetLicenseResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	license, err := auth.GetLicense(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &authpb.GetLicenseResponse{
		License: []byte(license),
	}, nil
}

// ListReleases returns a list of Teleport Enterprise releases.
func (g *GRPCServer) ListReleases(ctx context.Context, req *authpb.ListReleasesRequest) (*authpb.ListReleasesResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	releases, err := auth.ListReleases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &authpb.ListReleasesResponse{
		Releases: releases,
	}, nil
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (g *GRPCServer) ListSAMLIdPServiceProviders(ctx context.Context, req *authpb.ListSAMLIdPServiceProvidersRequest) (*authpb.ListSAMLIdPServiceProvidersResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceProviders, nextKey, err := auth.ListSAMLIdPServiceProviders(ctx, int(req.GetLimit()), req.GetNextKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceProvidersV1 := make([]*types.SAMLIdPServiceProviderV1, len(serviceProviders))
	for i, sp := range serviceProviders {
		v1, ok := sp.(*types.SAMLIdPServiceProviderV1)
		if !ok {
			return nil, trace.BadParameter("unexpected SAML IdP service provider type %T", sp)
		}
		serviceProvidersV1[i] = v1
	}

	return &authpb.ListSAMLIdPServiceProvidersResponse{
		ServiceProviders: serviceProvidersV1,
		NextKey:          nextKey,
	}, nil
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (g *GRPCServer) GetSAMLIdPServiceProvider(ctx context.Context, req *authpb.GetSAMLIdPServiceProviderRequest) (*types.SAMLIdPServiceProviderV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sp, err := auth.GetSAMLIdPServiceProvider(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceProviderV1, ok := sp.(*types.SAMLIdPServiceProviderV1)
	if !ok {
		return nil, trace.BadParameter("unexpected SAML IdP service provider type %T", sp)
	}

	return serviceProviderV1, nil
}

// CreateSAMLIdPServiceProvider creates a new SAML IdP service provider resource.
func (g *GRPCServer) CreateSAMLIdPServiceProvider(ctx context.Context, sp *types.SAMLIdPServiceProviderV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.CreateSAMLIdPServiceProvider(ctx, sp))
}

// UpdateSAMLIdPServiceProvider updates an existing SAML IdP service provider resource.
func (g *GRPCServer) UpdateSAMLIdPServiceProvider(ctx context.Context, sp *types.SAMLIdPServiceProviderV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.UpdateSAMLIdPServiceProvider(ctx, sp))
}

// DeleteSAMLIdPServiceProvider removes the specified SAML IdP service provider resource.
func (g *GRPCServer) DeleteSAMLIdPServiceProvider(ctx context.Context, req *authpb.DeleteSAMLIdPServiceProviderRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteSAMLIdPServiceProvider(ctx, req.GetName()))
}

// DeleteAllSAMLIdPServiceProviders removes all SAML IdP service providers.
func (g *GRPCServer) DeleteAllSAMLIdPServiceProviders(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteAllSAMLIdPServiceProviders(ctx))
}

// ListUserGroups returns a paginated list of user group resources.
func (g *GRPCServer) ListUserGroups(ctx context.Context, req *authpb.ListUserGroupsRequest) (*authpb.ListUserGroupsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userGroups, nextKey, err := auth.ListUserGroups(ctx, int(req.GetLimit()), req.GetNextKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userGroupsV1 := make([]*types.UserGroupV1, len(userGroups))
	for i, g := range userGroups {
		v1, ok := g.(*types.UserGroupV1)
		if !ok {
			return nil, trace.BadParameter("unexpected user group type %T", g)
		}
		userGroupsV1[i] = v1
	}

	return &authpb.ListUserGroupsResponse{
		UserGroups: userGroupsV1,
		NextKey:    nextKey,
	}, nil
}

// GetUserGroup returns the specified user group resources.
func (g *GRPCServer) GetUserGroup(ctx context.Context, req *authpb.GetUserGroupRequest) (*types.UserGroupV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sp, err := auth.GetUserGroup(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceProviderV1, ok := sp.(*types.UserGroupV1)
	if !ok {
		return nil, trace.BadParameter("unexpected user group type %T", sp)
	}

	return serviceProviderV1, nil
}

// CreateUserGroup creates a new user group resource.
func (g *GRPCServer) CreateUserGroup(ctx context.Context, sp *types.UserGroupV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.CreateUserGroup(ctx, sp))
}

// UpdateUserGroup updates an existing user group resource.
func (g *GRPCServer) UpdateUserGroup(ctx context.Context, sp *types.UserGroupV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.UpdateUserGroup(ctx, sp))
}

// DeleteUserGroup removes the specified user group resource.
func (g *GRPCServer) DeleteUserGroup(ctx context.Context, req *authpb.DeleteUserGroupRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteUserGroup(ctx, req.GetName()))
}

// DeleteAllUserGroups removes all user groups.
func (g *GRPCServer) DeleteAllUserGroups(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &emptypb.Empty{}, trace.Wrap(auth.DeleteAllUserGroups(ctx))
}

// UpdateHeadlessAuthenticationState updates a headless authentication state.
func (g *GRPCServer) UpdateHeadlessAuthenticationState(ctx context.Context, req *authpb.UpdateHeadlessAuthenticationStateRequest) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return &emptypb.Empty{}, trace.Wrap(err)
	}

	err = auth.UpdateHeadlessAuthenticationState(ctx, req.Id, req.State, req.MfaResponse)
	return &emptypb.Empty{}, trace.Wrap(err)
}

// GetHeadlessAuthentication retrieves a headless authentication.
func (g *GRPCServer) GetHeadlessAuthentication(ctx context.Context, req *authpb.GetHeadlessAuthenticationRequest) (*types.HeadlessAuthentication, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// First, try to retrieve the headless authentication directly if it already exists.
	if ha, err := auth.GetHeadlessAuthentication(ctx, req.Id); err == nil {
		return ha, nil
	} else if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// If the headless authentication doesn't exist yet, the headless login process may be waiting
	// for the user to create a stub to authorize the insert.
	if err := auth.UpsertHeadlessAuthenticationStub(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	// Force a short request timeout to prevent GetHeadlessAuthenticationFromWatcher
	// from waiting indefinitely for a nonexistent headless authentication. This is
	// useful for cases when the headless link/command is copied incorrectly or is
	// run with the wrong user.
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait for the login process to insert the actual headless authentication details.
	authReq, err := auth.GetHeadlessAuthenticationFromWatcher(ctx, req.Id)
	return authReq, trace.Wrap(err)
}

// WatchPendingHeadlessAuthentications watches the backend for pending headless authentication requests for the user.
func (g *GRPCServer) WatchPendingHeadlessAuthentications(_ *emptypb.Empty, stream authpb.AuthService_WatchPendingHeadlessAuthenticationsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := auth.WatchPendingHeadlessAuthentications(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	stubErr := make(chan error)
	go func() {
		stubErr <- auth.MaintainHeadlessAuthenticationStub(stream.Context())
	}()

	for {
		select {
		case err := <-stubErr:
			return trace.Wrap(err)
		case <-stream.Context().Done():
			return nil
		case <-watcher.Done():
			return watcher.Error()
		case event := <-watcher.Events():
			out, err := client.EventToGRPC(event)
			if err != nil {
				return trace.Wrap(err)
			}

			size := float64(proto.Size(out))
			watcherEventsEmitted.WithLabelValues(resourceLabel(event)).Observe(size)
			watcherEventSizes.Observe(size)

			if err := stream.Send(out); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// ExportUpgradeWindows is used to load derived upgrade window values for agents that
// need to export schedules to external upgraders.
func (g *GRPCServer) ExportUpgradeWindows(ctx context.Context, req *authpb.ExportUpgradeWindowsRequest) (*authpb.ExportUpgradeWindowsResponse, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, err := auth.ExportUpgradeWindows(ctx, *req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &rsp, nil
}

// GetClusterMaintenanceConfig gets the current maintenance config singleton.
func (g *GRPCServer) GetClusterMaintenanceConfig(ctx context.Context, _ *emptypb.Empty) (*types.ClusterMaintenanceConfigV1, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmc, err := auth.GetClusterMaintenanceConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rsp, ok := cmc.(*types.ClusterMaintenanceConfigV1)
	if !ok {
		return nil, trace.BadParameter("unexpected maintenance config type %T", cmc)
	}

	return rsp, nil
}

// UpdateClusterMaintenanceConfig updates the current maintenance config singleton.
func (g *GRPCServer) UpdateClusterMaintenanceConfig(ctx context.Context, cmc *types.ClusterMaintenanceConfigV1) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.UpdateClusterMaintenanceConfig(ctx, cmc); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteClusterMaintenanceConfig deletes the current maintenance config singleton.
func (g *GRPCServer) DeleteClusterMaintenanceConfig(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := auth.DeleteClusterMaintenanceConfig(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &emptypb.Empty{}, nil
}

// GetBackend returns the backend from the underlying auth server.
func (g *GRPCServer) GetBackend() backend.Backend {
	return g.AuthServer.bk
}

// GRPCServerConfig specifies gRPC server configuration
type GRPCServerConfig struct {
	// APIConfig is gRPC server API configuration
	APIConfig
	// TLS is gRPC server config
	TLS *tls.Config
	// Middleware is the request TLS client authenticator
	Middleware *Middleware
	// UnaryInterceptors is the gRPC unary interceptor chain.
	UnaryInterceptors []grpc.UnaryServerInterceptor
	// StreamInterceptors is the gRPC stream interceptor chain.
	StreamInterceptors []grpc.StreamServerInterceptor
}

// CheckAndSetDefaults checks and sets default values
func (cfg *GRPCServerConfig) CheckAndSetDefaults() error {
	if cfg.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if cfg.UnaryInterceptors == nil {
		return trace.BadParameter("missing parameter UnaryInterceptors")
	}
	if cfg.StreamInterceptors == nil {
		return trace.BadParameter("missing parameter StreamInterceptors")
	}
	if cfg.Middleware == nil {
		return trace.BadParameter("missing parameter Middleware")
	}
	return nil
}

// NewGRPCServer returns a new instance of gRPC server
func NewGRPCServer(cfg GRPCServerConfig) (*GRPCServer, error) {
	err := metrics.RegisterPrometheusCollectors(heartbeatConnectionsReceived, watcherEventsEmitted, watcherEventSizes, connectedResources)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	logger := slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, teleport.ComponentGRPC))

	logger.DebugContext(context.Background(), "creating gRPC server",
		"keep_alive_period", cfg.KeepAlivePeriod,
		"keep_alive_count", cfg.KeepAliveCount,
	)

	// httplib.TLSCreds are explicitly used instead of credentials.NewTLS because the latter
	// modifies the tls.Config.NextProtos which causes problems due to multiplexing on the auth
	// listener.
	creds, err := NewTransportCredentials(TransportCredentialsConfig{
		TransportCredentials: &httplib.TLSCreds{Config: cfg.TLS},
		UserGetter:           cfg.Middleware,
		GetAuthPreference:    cfg.AuthServer.Cache.GetAuthPreference,
		Clock:                cfg.AuthServer.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server := grpc.NewServer(
		grpc.Creds(creds),
		grpc.ChainUnaryInterceptor(cfg.UnaryInterceptors...),
		grpc.ChainStreamInterceptor(cfg.StreamInterceptors...),
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
		grpc.MaxConcurrentStreams(defaults.GRPCMaxConcurrentStreams),
	)

	usersService, err := usersv1.NewService(usersv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Cache:      cfg.AuthServer.Cache,
		Backend:    cfg.AuthServer.Services,
		Emitter:    cfg.Emitter,
		Reporter:   cfg.AuthServer.Services.UsageReporter,
		Clock:      cfg.AuthServer.GetClock(),
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "users.service"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	usersv1pb.RegisterUsersServiceServer(server, usersService)

	presenceService, err := presencev1.NewService(presencev1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		AuthServer: cfg.AuthServer,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
		Emitter:    cfg.Emitter,
		Reporter:   cfg.AuthServer.Services.UsageReporter,
		Clock:      cfg.AuthServer.GetClock(),
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "presence.service"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	presencev1pb.RegisterPresenceServiceServer(server, presenceService)

	botService, err := machineidv1.NewBotService(machineidv1.BotServiceConfig{
		Authorizer: cfg.Authorizer,
		Cache:      cfg.AuthServer.Cache,
		Backend:    cfg.AuthServer.Services,
		Reporter:   cfg.AuthServer.Services.UsageReporter,
		Emitter:    cfg.Emitter,
		Clock:      cfg.AuthServer.GetClock(),
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "bot.service"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating bot service")
	}
	machineidv1pb.RegisterBotServiceServer(server, botService)

	botInstanceService, err := machineidv1.NewBotInstanceService(machineidv1.BotInstanceServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services.BotInstance,
		Clock:      cfg.AuthServer.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating bot instance service")
	}
	machineidv1pb.RegisterBotInstanceServiceServer(server, botInstanceService)

	workloadIdentityService, err := machineidv1.NewWorkloadIdentityService(machineidv1.WorkloadIdentityServiceConfig{
		Authorizer: cfg.Authorizer,
		Cache:      cfg.AuthServer.Cache,
		Reporter:   cfg.AuthServer.Services.UsageReporter,
		Emitter:    cfg.Emitter,
		Clock:      cfg.AuthServer.GetClock(),
		KeyStore:   cfg.AuthServer.keyStore,
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "workload-identity.service"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating workload identity service")
	}
	machineidv1pb.RegisterWorkloadIdentityServiceServer(server, workloadIdentityService)

	spiffeFederationService, err := machineidv1.NewSPIFFEFederationService(machineidv1.SPIFFEFederationServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services.SPIFFEFederations,
		Cache:      cfg.AuthServer.Cache,
		Clock:      cfg.AuthServer.GetClock(),
		Emitter:    cfg.Emitter,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating SPIFFE federation service")
	}
	machineidv1pb.RegisterSPIFFEFederationServiceServer(server, spiffeFederationService)

	workloadIdentityResourceService, err := workloadidentityv1.NewResourceService(&workloadidentityv1.ResourceServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services.WorkloadIdentities,
		Cache:      cfg.AuthServer.Cache,
		Emitter:    cfg.Emitter,
		Clock:      cfg.AuthServer.GetClock(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating workload identity resource service")
	}
	workloadidentityv1pb.RegisterWorkloadIdentityResourceServiceServer(server, workloadIdentityResourceService)

	clusterName, err := cfg.AuthServer.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err, "getting cluster name")
	}
	workloadIdentityIssuanceService, err := workloadidentityv1.NewIssuanceService(&workloadidentityv1.IssuanceServiceConfig{
		Authorizer:  cfg.Authorizer,
		Cache:       cfg.AuthServer.Cache,
		Emitter:     cfg.Emitter,
		Clock:       cfg.AuthServer.GetClock(),
		KeyStore:    cfg.AuthServer.keyStore,
		ClusterName: clusterName.GetClusterName(),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating workload identity issuance service")
	}
	workloadidentityv1pb.RegisterWorkloadIdentityIssuanceServiceServer(server, workloadIdentityIssuanceService)

	workloadIdentityRevocationService, err := workloadidentityv1.NewRevocationService(&workloadidentityv1.RevocationServiceConfig{
		Authorizer: cfg.Authorizer,
		Emitter:    cfg.Emitter,
		Clock:      cfg.AuthServer.GetClock(),
		Store:      cfg.AuthServer.Services.WorkloadIdentityX509Revocations,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating workload identity issuance service")
	}
	workloadidentityv1pb.RegisterWorkloadIdentityRevocationServiceServer(server, workloadIdentityRevocationService)

	dbObjectImportRuleService, err := dbobjectimportrulev1.NewDatabaseObjectImportRuleService(dbobjectimportrulev1.DatabaseObjectImportRuleServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "db_obj_import_rule"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating database objectImportRule service")
	}
	dbobjectimportrulev1pb.RegisterDatabaseObjectImportRuleServiceServer(server, dbObjectImportRuleService)

	dbObjectService, err := dbobjectv1.NewDatabaseObjectService(dbobjectv1.DatabaseObjectServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "db_object"),
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating database object service")
	}
	dbobjectv1pb.RegisterDatabaseObjectServiceServer(server, dbObjectService)

	stableUNIXUsersServiceServer, err := stableunixusers.New(stableunixusers.Config{
		Authorizer: cfg.Authorizer,
		Emitter:    cfg.Emitter,
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "stable_unix_users"),

		Backend:       cfg.AuthServer.bk,
		ReadOnlyCache: cfg.AuthServer.ReadOnlyCache,

		StableUNIXUsers:      cfg.AuthServer.Services.StableUNIXUsersInternal,
		ClusterConfiguration: cfg.AuthServer.Services.ClusterConfigurationInternal,

		CacheClock:   cfg.AuthServer.clock,
		CacheContext: cfg.AuthServer.closeCtx,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating stable UNIX user service")
	}
	stableunixusersv1.RegisterStableUNIXUsersServiceServer(
		server,
		stableUNIXUsersServiceServer,
	)

	authServer := &GRPCServer{
		APIConfig: cfg.APIConfig,
		logger:    logger,
		server:    server,
	}

	if en := os.Getenv("TELEPORT_UNSTABLE_CREATEAUDITSTREAM_INFLIGHT_LIMIT"); en != "" {
		inflightLimit, err := strconv.ParseInt(en, 10, 64)
		if err != nil {
			logger.ErrorContext(context.Background(), "Failed to parse the TELEPORT_UNSTABLE_CREATEAUDITSTREAM_INFLIGHT_LIMIT envvar, limit will not be enforced")
			inflightLimit = -1
		}
		if inflightLimit == 0 {
			logger.WarnContext(context.Background(), "TELEPORT_UNSTABLE_CREATEAUDITSTREAM_INFLIGHT_LIMIT is set to 0, no CreateAuditStream RPCs will be allowed")
		}
		metrics.RegisterPrometheusCollectors(
			createAuditStreamAcceptedTotalMetric,
			createAuditStreamRejectedTotalMetric,
			createAuditStreamLimitMetric,
		)
		createAuditStreamLimitMetric.Set(float64(inflightLimit))
		if inflightLimit >= 0 {
			authServer.createAuditStreamSemaphore = make(chan struct{}, inflightLimit)
		}
	}

	authpb.RegisterAuthServiceServer(server, authServer)
	collectortracepb.RegisterTraceServiceServer(server, authServer)
	auditlogpb.RegisterAuditLogServiceServer(server, authServer)

	dynamicWindows, err := dynamicwindowsv1.NewService(dynamicwindowsv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dynamicwindowsv1pb.RegisterDynamicWindowsServiceServer(server, dynamicWindows)

	trust, err := trustv1.NewService(&trustv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Cache:      cfg.AuthServer.Cache,
		Backend:    cfg.AuthServer.Services,
		AuthServer: cfg.AuthServer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustv1pb.RegisterTrustServiceServer(server, trust)

	joinServiceServer := joinserver.NewJoinServiceGRPCServer(cfg.AuthServer)
	authpb.RegisterJoinServiceServer(server, joinServiceServer)

	integrationServiceServer, err := integrationv1.NewService(&integrationv1.ServiceConfig{
		Authorizer:      cfg.Authorizer,
		Backend:         cfg.AuthServer.Services,
		Cache:           cfg.AuthServer.Cache,
		KeyStoreManager: cfg.AuthServer.GetKeyStore(),
		Clock:           cfg.AuthServer.clock,
		Emitter:         cfg.Emitter,
		Logger:          cfg.AuthServer.logger.With(teleport.ComponentKey, "integrations.service"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integrationv1pb.RegisterIntegrationServiceServer(server, integrationServiceServer)

	integrationAWSOIDCServiceServer, err := integrationv1.NewAWSOIDCService(&integrationv1.AWSOIDCServiceConfig{
		Authorizer:            cfg.Authorizer,
		IntegrationService:    integrationServiceServer,
		Cache:                 cfg.AuthServer,
		ProxyPublicAddrGetter: cfg.AuthServer.getProxyPublicAddr,
		Clock:                 cfg.AuthServer.clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	integrationv1pb.RegisterAWSOIDCServiceServer(server, integrationAWSOIDCServiceServer)

	userTask, err := usertasksv1.NewService(usertasksv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
		// This must be a function because cfg.AuthServer.UsageReporter is changed after `NewGRPCServer` is called.
		// It starts as a DiscardUsageReporter, but when running in Cloud, gets replaced by a real reporter.
		UsageReporter: func() usagereporter.UsageReporter { return cfg.AuthServer.UsageReporter },
		Emitter:       cfg.Emitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	usertaskv1pb.RegisterUserTaskServiceServer(server, userTask)

	discoveryConfig, err := discoveryconfigv1.NewService(discoveryconfigv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Clock:      cfg.AuthServer.clock,
		Emitter:    cfg.Emitter,
		Logger:     cfg.AuthServer.logger.With(teleport.ComponentKey, "discoveryconfig_crud_service"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	discoveryconfigv1pb.RegisterDiscoveryConfigServiceServer(server, discoveryConfig)

	crownJewel, err := crownjewelv1.NewService(crownjewelv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Reader:     cfg.AuthServer.Cache,
		Emitter:    cfg.Emitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	crownjewelv1pb.RegisterCrownJewelServiceServer(server, crownJewel)

	// Initialize and register the user preferences service.
	userPreferencesSrv, err := userpreferencesv1.NewService(&userpreferencesv1.ServiceConfig{
		Backend:    cfg.AuthServer.Services,
		Authorizer: cfg.Authorizer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userpreferencesv1pb.RegisterUserPreferencesServiceServer(server, userPreferencesSrv)

	// Initialize and register the user login state service.
	userLoginState, err := local.NewUserLoginStateService(cfg.AuthServer.bk)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userLoginStateServer, err := userloginstatev1.NewService(userloginstatev1.ServiceConfig{
		Authorizer:      cfg.Authorizer,
		UserLoginStates: userLoginState,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userloginstatev1pb.RegisterUserLoginStateServiceServer(server, userLoginStateServer)

	clusterConfigService, err := clusterconfigv1.NewService(clusterconfigv1.ServiceConfig{
		Cache:      cfg.AuthServer.Cache,
		Backend:    cfg.AuthServer.Services,
		Authorizer: cfg.Authorizer,
		Emitter:    cfg.Emitter,
		AccessGraph: clusterconfigv1.AccessGraphConfig{
			Enabled:  cfg.APIConfig.AccessGraph.Enabled,
			CA:       cfg.APIConfig.AccessGraph.CA,
			Address:  cfg.APIConfig.AccessGraph.Address,
			Insecure: cfg.APIConfig.AccessGraph.Insecure,
		},
		ReadOnlyCache: cfg.AuthServer.ReadOnlyCache,
		SignatureAlgorithmSuiteParams: types.SignatureAlgorithmSuiteParams{
			FIPS:          cfg.AuthServer.fips,
			UsingHSMOrKMS: cfg.AuthServer.keyStore.UsingHSMOrKMS(),
			Cloud:         modules.GetModules().Features().Cloud,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterconfigv1pb.RegisterClusterConfigServiceServer(server, clusterConfigService)

	// Initialize and register the Kubernetes waiting container service.
	kubeWaitingContsServer, err := kubewaitingcontainerv1.NewService(kubewaitingcontainerv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubewaitingcontainerv1pb.RegisterKubeWaitingContainersServiceServer(server, kubeWaitingContsServer)

	accessMonitoringRuleServer, err := accessmonitoringrulesv1.NewService(&accessmonitoringrulesv1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	accessmonitoringrules.RegisterAccessMonitoringRulesServiceServer(server, accessMonitoringRuleServer)

	// Initialize and register the notifications service.
	notificationsServer, err := notificationsv1.NewService(notificationsv1.ServiceConfig{
		Authorizer:              cfg.Authorizer,
		Backend:                 cfg.AuthServer.Services,
		Clock:                   cfg.AuthServer.GetClock(),
		UserNotificationCache:   cfg.AuthServer.UserNotificationCache,
		GlobalNotificationCache: cfg.AuthServer.GlobalNotificationCache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	notificationsv1pb.RegisterNotificationServiceServer(server, notificationsServer)

	vnetConfigStorage, err := local.NewVnetConfigService(cfg.AuthServer.bk)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vnetConfigServiceServer := vnetconfigv1.NewService(vnetConfigStorage, cfg.Authorizer)
	vnetv1pb.RegisterVnetConfigServiceServer(server, vnetConfigServiceServer)

	staticHostUserServer, err := userprovisioningv2.NewService(userprovisioningv2.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Emitter:    cfg.Emitter,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userprovisioningv2pb.RegisterStaticHostUsersServiceServer(server, staticHostUserServer)

	autoUpdateServiceServer, err := autoupdatev1.NewService(autoupdatev1.ServiceConfig{
		Authorizer: cfg.Authorizer,
		Emitter:    cfg.Emitter,
		Backend:    cfg.AuthServer.Services,
		Cache:      cfg.AuthServer.Cache,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	autoupdatev1pb.RegisterAutoUpdateServiceServer(server, autoUpdateServiceServer)

	identityCenterService, err := local.NewIdentityCenterService(local.IdentityCenterServiceConfig{
		Backend: cfg.AuthServer.bk,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identitycenterv1.RegisterIdentityCenterServiceServer(server, identityCenterService)

	provisioningStateService, err := local.NewProvisioningStateService(cfg.AuthServer.bk)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provisioningv1.RegisterProvisioningServiceServer(server, provisioningStateService)

	gitServerService, err := gitserverv1.NewService(gitserverv1.Config{
		Authorizer:               cfg.Authorizer,
		Backend:                  cfg.AuthServer.Services,
		ProxyPublicAddrGetter:    cfg.AuthServer.getProxyPublicAddr,
		GitHubAuthRequestCreator: cfg.AuthServer.CreateGithubAuthRequest,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gitserverv1pb.RegisterGitServerServiceServer(server, gitServerService)

	// Only register the service if this is an open source build. Enterprise builds
	// register the actual service via an auth plugin, if we register here then all
	// Enterprise builds would fail with a duplicate service registered error.
	if cfg.PluginRegistry == nil || !cfg.PluginRegistry.IsRegistered("auth.enterprise") {
		loginrulev1pb.RegisterLoginRuleServiceServer(server, loginrulev1.NotImplementedService{})
	}

	decisionService, err := decisionv1.NewService(decisionv1.ServiceConfig{
		DecisionService: cfg.AuthServer.pdp,
		Authorizer:      cfg.Authorizer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	decisionpb.RegisterDecisionServiceServer(server, decisionService)

	return authServer, nil
}

type grpcContext struct {
	*authz.Context
	*ServerWithRoles
}

// authenticate extracts authentication context and returns initialized auth server
func (g *GRPCServer) authenticate(ctx context.Context) (*grpcContext, error) {
	// HTTPS server expects auth context to be set by the auth middleware
	authContext, err := g.Authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &grpcContext{
		Context: authContext,
		ServerWithRoles: &ServerWithRoles{
			authServer: g.AuthServer,
			context:    *authContext,
			alog:       g.AuthServer,
		},
	}, nil
}

// GetUnstructuredEvents searches for events on the backend and sends them back in an unstructured format.
func (g *GRPCServer) GetUnstructuredEvents(ctx context.Context, req *auditlogpb.GetUnstructuredEventsRequest) (*auditlogpb.EventsUnstructured, error) {
	auth, err := g.authenticate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rawEvents, lastkey, err := auth.ServerWithRoles.SearchEvents(ctx, events.SearchEventsRequest{
		From:       req.StartDate.AsTime(),
		To:         req.EndDate.AsTime(),
		EventTypes: req.EventTypes,
		Limit:      int(req.Limit),
		Order:      types.EventOrder(req.Order),
		StartKey:   req.StartKey,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	unstructuredEvents := make([]*auditlogpb.EventUnstructured, 0, len(rawEvents))
	for _, event := range rawEvents {
		unstructuredEvent, err := apievents.ToUnstructured(event)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		unstructuredEvents = append(unstructuredEvents, unstructuredEvent)
	}

	return &auditlogpb.EventsUnstructured{
		Items:   unstructuredEvents,
		LastKey: lastkey,
	}, nil
}

// ExportUnstructuredEvents exports events from a given event chunk returned by GetEventExportChunks. This API prioritizes
// performance over ordering and filtering, and is intended for bulk export of events.
func (g *GRPCServer) ExportUnstructuredEvents(req *auditlogpb.ExportUnstructuredEventsRequest, stream auditlogpb.AuditLogService_ExportUnstructuredEventsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	events := auth.ServerWithRoles.ExportUnstructuredEvents(stream.Context(), req)

	for events.Next() {
		if err := stream.Send(events.Item()); err != nil {
			events.Done()
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(events.Done())
}

// GetEventExportChunks returns a stream of event chunks that can be exported via ExportUnstructuredEvents. The returned
// list isn't ordered and polling for new chunks requires re-consuming the entire stream from the beginning.
func (g *GRPCServer) GetEventExportChunks(req *auditlogpb.GetEventExportChunksRequest, stream auditlogpb.AuditLogService_GetEventExportChunksServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	chunks := auth.ServerWithRoles.GetEventExportChunks(stream.Context(), req)

	for chunks.Next() {
		if err := stream.Send(chunks.Item()); err != nil {
			chunks.Done()
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(chunks.Done())
}

// StreamUnstructuredSessionEvents streams all events from a given session recording as an unstructured format.
func (g *GRPCServer) StreamUnstructuredSessionEvents(req *auditlogpb.StreamUnstructuredSessionEventsRequest, stream auditlogpb.AuditLogService_StreamUnstructuredSessionEventsServer) error {
	auth, err := g.authenticate(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}

	c, e := auth.ServerWithRoles.StreamSessionEvents(stream.Context(), session.ID(req.SessionId), int64(req.StartIndex))

	for {
		select {
		case event, more := <-c:
			if !more {
				return nil
			}
			// convert event to JSON
			eventJson, err := apievents.ToUnstructured(event)
			if err != nil {
				return trail.ToGRPC(trace.Wrap(err))
			}
			if err := stream.Send(eventJson); err != nil {
				return trail.ToGRPC(trace.Wrap(err))
			}
		case err := <-e:
			return trail.ToGRPC(trace.Wrap(err))
		}
	}
}
