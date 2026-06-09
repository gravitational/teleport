package accessgraph

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	_ "google.golang.org/grpc/health"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	authpb "github.com/gravitational/teleport/api/client/proto"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	dbobjectv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobject/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	legacy_header "github.com/gravitational/teleport/api/types/header/convert/legacy"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	"github.com/gravitational/teleport/api/utils/clientutils"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	// supportedActionsKey is the metadata header name that lists the
	// actions supported in EventsStreamV2Response by this code.
	supportedActionsKey       = "supported-actions"
	supportedActionUsageEvent = "usage-event"

	// supportedKindsKey is the metadata header name that lists the
	// kinds supported by access graph.
	supportedKindsKey = "supported-kinds"
)

type (
	eventStream    grpc.BidiStreamingClient[accessgraphv1.EventsStreamV2Request, accessgraphv1.EventsStreamV2Response]
	auditLogStream grpc.BidiStreamingClient[accessgraphv1.AuditLogStreamRequest, accessgraphv1.AuditLogStreamResponse]
)

// initializeAndWatchAccessGraph initializes the access graph service and watches the auth server for events.
// This function acquires a lock on the backend to ensure that only one instance of auth server is sending
// events to the access graph service at a time.
func initializeAndWatchAccessGraph(ctx context.Context, log *slog.Logger, config ServiceClientConfig, getCreds ClientCredentialsGetter, authServer *auth.Server, bk backend.Backend) error {
	// The gauges are process-wide, so clear any stale healthy state before
	// attempting a stream handshake and again when this sync loop exits.
	setAccessGraphConnected(accessGraphMetricStreamEvent, false)
	defer setAccessGraphConnected(accessGraphMetricStreamEvent, false)
	if config.AuditLog.Enabled {
		setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)
		defer setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)
	}

	// Configure health check service to monitor access graph service and
	// automatically reconnect if the connection is lost without
	// relying on new events from the auth server to trigger a reconnect.
	const serviceConfig = `{
		"loadBalancingConfig": [{"round_robin": {}}],
		"healthCheckConfig": {
			"serviceName": ""
		}
	}`

	err := backend.RunWhileLocked(
		ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				LockNameComponents: []string{"accessGraphClient"},
				Backend:            bk,
				TTL:                10 * time.Second,
				RetryInterval:      5 * time.Second,
			},
			ReleaseCtxTimeout:   5 * time.Second,
			RefreshLockInterval: 1 * time.Second,
		},
		func(ctx context.Context) error {
			accessGraphConn, err := NewAccessGraphClient(
				ctx,
				config,
				getCreds,
				grpc.WithDefaultServiceConfig(serviceConfig),
			)
			if err != nil {
				return trace.Wrap(err)
			}
			// Close the connection when the function returns.
			defer accessGraphConn.Close()
			client := accessgraphv1.NewAccessGraphServiceClient(accessGraphConn)

			// Set metadata to inform the other end of what response messages are
			// supported by this teleport client.
			md := metadata.MD{
				supportedActionsKey: []string{supportedActionUsageEvent},
			}
			streamCtx := metadata.NewOutgoingContext(ctx, md)
			eventStream, err := client.EventsStreamV2(streamCtx)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get access graph service events stream", "error", err)
				return trace.Wrap(err)
			}

			ctx, cancel := context.WithCancel(ctx)
			// Start a goroutine to watch the access graph service connection state.
			// If the connection is closed, cancel the context to stop all stream processing.
			go func() {
				defer cancel()
				if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
					log.InfoContext(ctx, "access graph service connection was closed")
				}
				setAccessGraphConnected(accessGraphMetricStreamEvent, false)
				if config.AuditLog.Enabled {
					setAccessGraphConnected(accessGraphMetricStreamAuditLog, false)
				}
			}()

			g, egCtx := errgroup.WithContext(ctx)
			g.Go(func() error {
				return processEventStream(egCtx, log, eventStream, authServer)
			})
			if config.AuditLog.Enabled {
				g.Go(func() error {
					return initiateAndProcessAuditLogStream(egCtx, log, client, authServer, config.AuditLog)
				})
			}
			err = g.Wait()
			if err != nil {
				log.ErrorContext(ctx, "Failed to run access graph service", "error", err)
				return trace.Wrap(err)
			}
			return nil
		})
	return trace.Wrap(err)
}

func processEventStream(ctx context.Context, log *slog.Logger, stream eventStream, authServer *auth.Server) error {
	header, err := stream.Header()
	if err != nil {
		log.ErrorContext(ctx, "Failed to get access graph service stream header", "error", err)
		return trace.Wrap(err)
	}
	supportedKinds := header.Get(supportedKindsKey)
	if len(supportedKinds) == 0 {
		return trace.BadParameter("access graph service did not return supported kinds")
	}
	setAccessGraphConnected(accessGraphMetricStreamEvent, true)
	defer setAccessGraphConnected(accessGraphMetricStreamEvent, false)

	newCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		defer cancel()

		for {
			obj, err := stream.Recv()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					log.InfoContext(ctx, "access graph service connection was closed", "error", err)
				} else {
					log.ErrorContext(ctx, "Failed to receive message from access graph service", "error", err)
				}
				return
			}

			processTAGMessage(ctx, obj, authServer, log)
		}
	}()

	eventWatcherSender := newTagEventWatcher(newCtx, stream)

	var (
		// we use two watchers to watch the auth server for events.
		// one subscribes to the Cache service in order to watch for events
		// that are supported by the cache such as: kube servers, app servers, users, roles...
		// the other subscribes to the Services service in order to watch for events
		// that aren't supported by the cache such as: devices, private keys, authorized keys...
		// noOpWatcher is used to terminate the watcher if the context is canceled. This is used to
		// avoid locks when connection is terminating.
		// The watcher will be replaced with the real watcher if access graph service supports the associated kinds.
		servicesWatcher    types.Watcher = &noOpWatcher{ctx}
		cacheWatcher       types.Watcher = &noOpWatcher{ctx}
		supportedResources []string
	)

	if svcWatchKinds := supportedKindsToWatcherKinds(supportedKinds, servicesWatcherKind); len(svcWatchKinds) > 0 {
		servicesWatcher, err = authServer.Services.NewWatcher(
			eventWatcherSender.Context(),
			types.Watch{
				Kinds:               svcWatchKinds,
				AllowPartialSuccess: true,
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		defer servicesWatcher.Close()
		servicesSupportedResources, err := waitForInit(servicesWatcher)
		if err != nil {
			return trace.Wrap(err)
		}
		supportedResources = append(supportedResources, servicesSupportedResources...)
	}

	if cacheWatchKinds := supportedKindsToWatcherKinds(supportedKinds, cacheWatcherKind); len(cacheWatchKinds) > 0 {
		cacheWatcher, err = authServer.Cache.NewWatcher(
			eventWatcherSender.Context(),
			types.Watch{
				Kinds:               cacheWatchKinds,
				AllowPartialSuccess: true,
			},
		)
		if err != nil {
			return trace.Wrap(err)
		}
		defer cacheWatcher.Close()
		cacheSupportedResources, err := waitForInit(cacheWatcher)
		if err != nil {
			return trace.Wrap(err)
		}
		supportedResources = append(supportedResources, cacheSupportedResources...)
	}

	missingWatchKinds(ctx, log, supportedKinds, supportedResources)
	errc := make(chan error, 1)
	go func() {
		// Start watching the auth server for events.
		// Subscribe for new events before sending all resources.
		// Otherwise, we might miss some events.
		errc <- forwardEventsWatch(cacheWatcher, servicesWatcher, eventWatcherSender)
	}()

	log.DebugContext(ctx, "Sending teleport resources to access graph service")
	// Send all teleport resources to the access graph service.
	if err := sendTeleportResources(ctx, stream, authServer, supportedKinds); err != nil {
		log.ErrorContext(ctx, "Failed to send teleport resources to access graph service", "error", err)
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Done sending teleport resources to access graph service")

	// Marks as ready and send delayed events.
	if err := eventWatcherSender.markReady(); err != nil {
		return trace.Wrap(err)
	}

	err = <-errc
	if errors.Is(err, context.Canceled) {
		log.InfoContext(ctx, "access graph service connection was closed", "error", err)
		return trace.Wrap(err)
	} else if err != nil {
		log.ErrorContext(ctx, "Failed to start watching access graph service", "error", err)
		return trace.Wrap(err)
	}

	return nil
}

type eventSender interface {
	EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error
	AnonymizeAndSubmit(event ...usagereporter.Anonymizable)
}

func processTAGMessage(ctx context.Context, obj *accessgraphv1.EventsStreamV2Response, authServer eventSender, log *slog.Logger) {
	switch obj.WhichAction() {
	case accessgraphv1.EventsStreamV2Response_Event_case:
		event := convertEvent(obj.GetEvent())
		if event == nil {
			log.WarnContext(ctx, "Received unknown event type from access graph service", "event", obj)
			return
		}

		if err := authServer.EmitAuditEvent(ctx, event); err != nil {
			log.ErrorContext(ctx, "Failed to emit Crown Jewel update event")
		}
	case accessgraphv1.EventsStreamV2Response_UsageEvent_case:
		event := convertUsageEvent(obj.GetUsageEvent())
		if event == nil {
			log.WarnContext(ctx, "Received unknown usage event type from access graph service", "event", obj)
			return
		}
		authServer.AnonymizeAndSubmit(event)

	default:
		log.WarnContext(ctx, "Received unknown event type from access graph service", "event", obj)
	}
}

func convertEvent(event *accessgraphv1.AuditEvent) apievents.AuditEvent {
	var tEvent apievents.AuditEvent

	switch event.WhichEvent() {
	case accessgraphv1.AuditEvent_AccessPathChanged_case:
		data := event.GetAccessPathChanged()
		tEvent = &apievents.AccessPathChanged{
			Metadata: apievents.Metadata{
				Type: events.AccessGraphAccessPathChangedEvent,
				Code: events.AccessGraphAccessPathChangedCode,
			},
			ChangeID:               data.GetChangeId(),
			AffectedResourceName:   data.GetAffectedResourceName(),
			AffectedResourceSource: data.GetAffectedResourceSource(),
			AffectedResourceType:   data.GetAffectedResourceKind(),
		}
	default:
		return nil
	}

	return tEvent
}

func convertUsageEvent(event *accessgraphv1.UsageEvent) usagereporter.Anonymizable {
	switch event.WhichEvent() {
	case accessgraphv1.UsageEvent_GraphSize_case:
		if event.GetGraphSize() == nil {
			return nil
		}
		return (*usagereporter.IdentitySecurityGraphSizeEvent)(event.GetGraphSize())
	case accessgraphv1.UsageEvent_AuditLogsIngested_case:
		if event.GetAuditLogsIngested() == nil {
			return nil
		}
		return (*usagereporter.IdentitySecurityAuditLogsIngestedEvent)(event.GetAuditLogsIngested())
	}
	return nil
}

type watcherKind int

const (
	cacheWatcherKind watcherKind = iota + 1
	servicesWatcherKind
)

var servicesWatcherOnlyKinds = []string{
	types.KindAccessGraphSecretAuthorizedKey,
	types.KindAccessGraphSecretPrivateKey,
	types.KindDevice,
}

func supportedKindsToWatcherKinds(supportedKinds []string, wk watcherKind) []types.WatchKind {
	var observedKinds []types.WatchKind
	for _, kind := range supportedKinds {
		switch isServicesWatchOnly := slices.Contains(servicesWatcherOnlyKinds, kind); {
		case wk == servicesWatcherKind && isServicesWatchOnly:
			observedKinds = append(observedKinds, types.WatchKind{Kind: kind})
		case wk == cacheWatcherKind && !isServicesWatchOnly:
			observedKinds = append(observedKinds, types.WatchKind{Kind: kind})
		}
	}
	return observedKinds
}

func missingWatchKinds(ctx context.Context, logger *slog.Logger, expected, actual []string) {
	var missingKinds []string
	actualKinds := make(map[string]struct{})
	for _, kind := range actual {
		actualKinds[kind] = struct{}{}
	}
	for _, expectedKind := range expected {
		if _, found := actualKinds[expectedKind]; !found {
			missingKinds = append(missingKinds, expectedKind)
		}
	}
	if len(missingKinds) > 0 {
		logger.WarnContext(ctx, "Missing kinds in the access graph service", "missing_kinds", missingKinds)
	}
}

// waitForInit waits for the watcher to receive an init event.
func waitForInit(watcher types.Watcher) ([]string, error) {
	for {
		select {
		case event := <-watcher.Events():
			if event.Type != types.OpInit {
				continue
			}
			evt, ok := event.Resource.(types.WatchStatus)
			if !ok {
				return nil, nil
			}
			var kinds []string
			for _, kind := range evt.GetKinds() {
				kinds = append(kinds, kind.Kind)
			}
			return kinds, nil
		case <-watcher.Done():
			return nil, trace.Wrap(watcher.Error())
		}
	}
}

// newTagEventWatcher returns a new tagEventWatcher.
func newTagEventWatcher(ctx context.Context, stream accessGraphSender) *tagEventWatcher {
	return &tagEventWatcher{
		ctx:               ctx,
		accessGraphStream: stream,
		cache:             make([]types.Event, 0),
	}
}

// sendTeleportResources sends all teleport resources to the access graph service.
func sendTeleportResources(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, authServer *auth.Server, serverSupportedKinds []string) error {
	if slices.Contains(serverSupportedKinds, types.KindRole) {
		if err := sendRoles(ctx, authServer.Cache, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindUser) {
		if err := sendUsers(ctx, authServer.Cache, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindAccessRequest) {
		if err := sendAccessRequests(ctx, authServer.Services, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindCrownJewel) {
		if err := sendCrownJewels(ctx, authServer.Cache, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	teleportSupportedKinds := []string{
		types.KindNode,
		types.KindAppServer,
		types.KindDatabaseServer,
		types.KindWindowsDesktop,
		types.KindKubeServer,
	}
	var supportedUnifiedResources []string
	for _, kind := range teleportSupportedKinds {
		if !slices.Contains(serverSupportedKinds, kind) {
			continue
		}
		supportedUnifiedResources = append(supportedUnifiedResources, kind)
	}

	if err := pushResourcesViaUnifiedResourcesCache(
		ctx,
		authServer,
		stream,
		supportedUnifiedResources...,
	); err != nil {
		return trace.Wrap(err)
	}

	if slices.Contains(serverSupportedKinds, types.KindAccessList) {
		if err := sendAccessLists(ctx, authServer.Cache, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindDatabaseObject) {
		if err := sendDatabaseObjects(ctx, authServer.Cache, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindDevice) {
		if err := sendDevices(ctx, authServer.Services, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindAccessGraphSecretAuthorizedKey) {
		if err := sendAuthorizedKeys(ctx, authServer.Services, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	if slices.Contains(serverSupportedKinds, types.KindAccessGraphSecretPrivateKey) {
		if err := sendPrivateKeys(ctx, authServer.Services, stream); err != nil {
			return trace.Wrap(err)
		}
	}

	// Send end event to indicate that initialization is done.
	err := stream.Send(
		accessgraphv1.EventsStreamV2Request_builder{
			Sync: &accessgraphv1.SyncOperation{},
		}.Build(),
	)
	return trace.Wrap(err)
}

// forwardEventsWatch starts watching the auth server for events and sends them to the access graph service.
func forwardEventsWatch(cacheWatcher types.Watcher, servicesWatcher types.Watcher, eventWatcher *tagEventWatcher) error {
	for {
		select {
		case event := <-cacheWatcher.Events():
			if err := eventWatcher.Send(event); err != nil {
				return trace.Wrap(err)
			}
		case event := <-servicesWatcher.Events():
			if err := eventWatcher.Send(event); err != nil {
				return trace.Wrap(err)
			}
		case <-cacheWatcher.Done():
			return trace.Wrap(cacheWatcher.Error())
		case <-servicesWatcher.Done():
			return trace.Wrap(servicesWatcher.Error())
		}
	}
}

func sendUsers(ctx context.Context, authServer interface {
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}, stream accessGraphSender,
) error {
	return sendPaginatedResources(ctx, stream,
		func(ctx context.Context, size int, token string) ([]*types.UserV2, string, error) {
			rsp, err := authServer.ListUsers(ctx, userspb.ListUsersRequest_builder{
				PageSize:    int32(size),
				PageToken:   token,
				WithSecrets: true,
			}.Build())
			if err != nil {
				return nil, "", err
			}
			return rsp.GetUsers(), rsp.GetNextPageToken(), nil
		},
		func(user *types.UserV2) *accessgraphv1.ResourceEntry {
			// If the user's weakest device is not set, set it now.
			if auth := user.GetLocalAuth(); auth != nil &&
				user.GetWeakestDevice() == types.MFADeviceKind_MFA_DEVICE_KIND_UNSPECIFIED {
				user.SetWeakestDevice(local.GetWeakestMFADeviceKind(auth.MFA))
			}
			return accessgraphv1.ResourceEntry_builder{
				// reset local auth to avoid sending secrets to the access graph service
				// we load secrets only to populate the user's MFA status when not set
				// in the database.
				User: user.WithoutSecrets().(*types.UserV2),
			}.Build()
		},
	)
}

func sendCrownJewels(ctx context.Context, authServer interface {
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
}, stream accessGraphSender,
) error {
	return sendPaginatedResources(ctx, stream,
		func(ctx context.Context, size int, token string) ([]*crownjewelv1.CrownJewel, string, error) {
			return authServer.ListCrownJewels(ctx, int64(size), token)
		},
		func(c *crownjewelv1.CrownJewel) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{CrownJewel: proto.ValueOrDefault(c)}.Build()
		},
	)
}

func sendRoles(ctx context.Context, authServer interface {
	ListRoles(context.Context, *authpb.ListRolesRequest) (*authpb.ListRolesResponse, error)
}, stream accessGraphSender,
) error {
	return sendPaginatedResources(ctx, stream,
		func(ctx context.Context, size int, token string) ([]*types.RoleV6, string, error) {
			rsp, err := authServer.ListRoles(ctx, &authpb.ListRolesRequest{
				Limit:    int32(size),
				StartKey: token,
			})
			if err != nil {
				return nil, "", err
			}
			return rsp.GetRoles(), rsp.GetNextKey(), nil
		},
		func(role *types.RoleV6) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{Role: role}.Build()
		},
	)
}

func sendAccessLists(ctx context.Context, authServer interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}, stream accessGraphSender,
) error {
	err := sendPaginatedResources(ctx, stream,
		authServer.ListAccessLists,
		func(al *accesslist.AccessList) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{AccessList: proto.ValueOrDefault(accesslistv1conv.ToProto(al))}.Build()
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(sendAccessListMembers(ctx, authServer, stream))

}

func sendAccessListMembers(ctx context.Context, authServer interface {
	ListAllAccessListMembers(ctx context.Context, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}, stream accessGraphSender,
) error {
	for m, err := range clientutils.Resources(ctx, authServer.ListAllAccessListMembers) {
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(accessgraphv1.EventsStreamV2Request_builder{
			AccessListsMembers: accessgraphv1.AccessListsMembers_builder{Members: []*accesslistv1.Member{accesslistv1conv.ToMemberProto(m)}}.Build(),
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// sendPaginatedResources pages through items using listFn and sends each item individually.
func sendPaginatedResources[T any](
	ctx context.Context,
	stream accessGraphSender,
	listFn func(ctx context.Context, size int, token string) (items []T, nextToken string, err error),
	toEntry func(T) *accessgraphv1.ResourceEntry,
) error {
	for item, err := range clientutils.Resources(ctx, listFn) {
		if err != nil {
			return trace.Wrap(err)
		}
		if err := stream.Send(accessgraphv1.EventsStreamV2Request_builder{
			Upsert: accessgraphv1.ResourceList_builder{Resources: []*accessgraphv1.ResourceEntry{toEntry(item)}}.Build(),
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func sendDatabaseObjects(ctx context.Context, authServer services.DatabaseObjectsGetter, stream accessGraphSender) error {
	return sendPaginatedResources(ctx, stream,
		authServer.ListDatabaseObjects,
		func(o *dbobjectv1.DatabaseObject) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{DatabaseObject: proto.ValueOrDefault(o)}.Build()
		},
	)
}

func sendDevices(ctx context.Context, authServer services.DevicesGetter, stream accessGraphSender) error {
	if authServer == nil {
		return trace.BadParameter("authServer is nil")
	}
	return sendPaginatedResources(ctx, stream,
		func(ctx context.Context, size int, token string) ([]*devicepb.Device, string, error) {
			return authServer.ListDevices(ctx, size, token, devicepb.DeviceView_DEVICE_VIEW_RESOURCE)
		},
		func(d *devicepb.Device) *accessgraphv1.ResourceEntry {
			d.ClearCredential()
			return accessgraphv1.ResourceEntry_builder{Device: proto.ValueOrDefault(d)}.Build()
		},
	)
}

func sendPrivateKeys(ctx context.Context, authServer services.AccessGraphSecretsGetter, stream accessGraphSender) error {
	if authServer == nil {
		return trace.BadParameter("authServer is nil")
	}
	return sendPaginatedResources(ctx, stream,
		authServer.ListAllPrivateKeys,
		func(k *accessgraphsecretsv1pb.PrivateKey) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{PrivateKey: proto.ValueOrDefault(k)}.Build()
		},
	)
}

func sendAuthorizedKeys(ctx context.Context, authServer services.AccessGraphSecretsGetter, stream accessGraphSender) error {
	if authServer == nil {
		return trace.BadParameter("authServer is nil")
	}
	return sendPaginatedResources(ctx, stream,
		authServer.ListAllAuthorizedKeys,
		func(k *accessgraphsecretsv1pb.AuthorizedKey) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{AuthorizedKey: proto.ValueOrDefault(k)}.Build()
		},
	)
}

// sendAccessRequests sends all access requests to the access graph service.
func sendAccessRequests(ctx context.Context, authServer interface {
	ListAccessRequests(context.Context, *authpb.ListAccessRequestsRequest) (*authpb.ListAccessRequestsResponse, error)
}, stream accessGraphSender) error {
	return sendPaginatedResources(ctx, stream,
		func(ctx context.Context, size int, token string) ([]*types.AccessRequestV3, string, error) {
			rsp, err := authServer.ListAccessRequests(ctx, &authpb.ListAccessRequestsRequest{
				Filter:   &types.AccessRequestFilter{},
				Limit:    int32(size),
				StartKey: token,
			})
			if err != nil {
				return nil, "", err
			}
			return rsp.GetAccessRequests(), rsp.GetNextKey(), nil
		},
		func(a *types.AccessRequestV3) *accessgraphv1.ResourceEntry {
			return accessgraphv1.ResourceEntry_builder{AccessRequest: a}.Build()
		},
	)
}

type accessGraphSender interface {
	Send(request *accessgraphv1.EventsStreamV2Request) error
}

type tagEventWatcher struct {
	ctx context.Context
	// ready is set to true when the watcher is ready to send events.
	ready atomic.Bool
	// cache is used to cache events before the watcher is ready.
	cache []types.Event
	// cacheMtx is used to synchronize access to the cache.
	cacheMtx sync.Mutex
	// accessGraphStream is used to send events to the access graph service.
	accessGraphStream accessGraphSender
}

// Context returns the context of the watcher.
func (t *tagEventWatcher) Context() context.Context {
	return t.ctx
}

// Send sends an event to the access graph service.
func (t *tagEventWatcher) Send(event types.Event) error {
	// If the watcher is not ready, cache the event and send it later.
	if !t.ready.Load() {
		t.cacheMtx.Lock()
		defer t.cacheMtx.Unlock()

		// Double check if the watcher is ready, as the order of locking in MarkReady is not the same.
		if t.ready.Load() {
			// Send the events.
			return trace.Wrap(t.send(event))
		}
		// If the watcher is not ready, cache the event and send it later.
		t.cache = append(t.cache, event)

		return nil
	}

	// If the watcher is ready, send the event.
	return trace.Wrap(t.send(event))
}

func (t *tagEventWatcher) send(event types.Event) error {
	switch event.Type {
	case types.OpDelete:
		return trace.Wrap(t.sendDelete(event))
	case types.OpPut:
		return trace.Wrap(t.sendPut(event))
	default:
		// Ignore INIT event type.
		return nil
	}
}

// markReady marks the watcher as ready to send events.
func (t *tagEventWatcher) markReady() error {
	// Send all cached events.
	for {
		t.cacheMtx.Lock()
		// If cache is empty, mark the watcher as ready and return.
		if len(t.cache) == 0 {
			t.ready.Store(true)
			t.cacheMtx.Unlock()
			break
		}
		// Otherwise, send the first event in the cache.
		event := t.cache[0]
		t.cache = t.cache[1:]
		t.cacheMtx.Unlock()

		if err := t.send(event); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (t *tagEventWatcher) sendDelete(event types.Event) error {
	deleteEventStreamRequest := func(header *types.ResourceHeader) *accessgraphv1.EventsStreamV2Request {
		return accessgraphv1.EventsStreamV2Request_builder{
			Delete: accessgraphv1.ResourceHeaderList_builder{
				Resources: []*types.ResourceHeader{
					header,
				},
			}.Build(),
		}.Build()
	}

	deleteEventStreamRequestResource153 := func(resource types.Resource153) *accessgraphv1.EventsStreamV2Request {
		meta := headerv1.FromMetadataProto(resource.GetMetadata())
		return deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:     resource.GetKind(),
				Version:  resource.GetVersion(),
				Metadata: legacy_header.FromHeaderMetadata(meta),
			},
		)
	}

	var req *accessgraphv1.EventsStreamV2Request
	switch resource := event.Resource.(type) {
	case *types.UserV2:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindUser,
				types.V2,
				resource,
			),
		)

	case *types.RoleV6:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindRole,
				resource.Version,
				resource,
			),
		)
	case *types.AccessRequestV3:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindAccessRequest,
				resource.Version,
				resource,
			),
		)
	case *types.ServerV2:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindNode,
				resource.Version,
				resource,
			),
		)
	case *types.KubernetesServerV3:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindKubeServer,
				resource.Version,
				resource,
			),
		)
	case *types.AppServerV3:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindAppServer,
				resource.Version,
				resource,
			),
		)
	case *types.DatabaseServerV3:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindDatabaseServer,
				resource.Version,
				resource,
			),
		)
	case *types.WindowsDesktopV3:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindWindowsDesktop,
				resource.Version,
				resource,
			),
		)
	case *types.DeviceV1:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindDevice,
				resource.Version,
				resource,
			),
		)
	case *types.ResourceHeader:
		if resource.Kind == types.KindAccessListMember {
			// Access list member uses a different object format
			// when it is sent to the access graph service.
			req = accessgraphv1.EventsStreamV2Request_builder{
				ExcludeAccessListMembers: accessgraphv1.ExcludeAccessListsMembers_builder{
					Members: []*accessgraphv1.ExcludeAccessListMember{
						accessgraphv1.ExcludeAccessListMember_builder{
							// access list name comes from the resource header description
							AccessList: resource.GetMetadata().Description,
							Username:   resource.GetName(),
						}.Build(),
					},
				}.Build(),
			}.Build()
			break
		}
		req = deleteEventStreamRequest(resource)
	case *accesslist.AccessList:
		header := headerv1.FromResourceHeaderProto(accesslistv1conv.ToProto(resource).GetHeader())
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:    header.GetKind(),
				Version: header.GetVersion(),
				Metadata: legacy_header.FromHeaderMetadata(
					header.GetMetadata(),
				),
			},
		)
	case *accesslist.AccessListMember:
		// Access list member uses a different header format.
		req = accessgraphv1.EventsStreamV2Request_builder{
			ExcludeAccessListMembers: accessgraphv1.ExcludeAccessListsMembers_builder{
				Members: []*accessgraphv1.ExcludeAccessListMember{
					accessgraphv1.ExcludeAccessListMember_builder{
						AccessList: resource.Spec.AccessList,
						Username:   resource.Spec.Name,
					}.Build(),
				},
			}.Build(),
		}.Build()
	case types.Resource153UnwrapperT[*crownjewelv1.CrownJewel]:
		cj := resource.UnwrapT()
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:     cj.GetKind(),
				Version:  cj.GetVersion(),
				Metadata: fromProtoMetadataToTypes(cj.GetMetadata()),
			},
		)
	case types.Resource153UnwrapperT[*dbobjectv1.DatabaseObject]:
		req = deleteEventStreamRequestResource153(resource.UnwrapT())
	case types.Resource153UnwrapperT[*accessgraphsecretsv1pb.PrivateKey]:
		pk := resource.UnwrapT()
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:    pk.GetKind(),
				Version: pk.GetVersion(),
				Metadata: types.Metadata{
					Name:        pk.GetMetadata().GetName(),
					Description: pk.GetSpec().GetDeviceId(),
				},
			},
		)
	case types.Resource153UnwrapperT[*accessgraphsecretsv1pb.AuthorizedKey]:
		ak := resource.UnwrapT()
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:    ak.GetKind(),
				Version: ak.GetVersion(),
				Metadata: types.Metadata{
					Name:        ak.GetMetadata().GetName(),
					Description: ak.GetSpec().GetHostId(),
				},
			},
		)
	default:
		return trace.BadParameter("unexpected resource type: %T", resource)
	}

	err := t.accessGraphStream.Send(
		req,
	)
	return trace.Wrap(err)
}

func fromProtoMetadataToTypes(metadata *v1.Metadata) types.Metadata {
	return types.Metadata{
		Name:        metadata.GetName(),
		Namespace:   metadata.GetNamespace(),
		Description: metadata.GetDescription(),
		Labels:      metadata.GetLabels(),
		Revision:    metadata.GetRevision(),
		Expires:     timePtr(metadata.GetExpires().AsTime()),
	}
}

func timePtr(asTime time.Time) *time.Time {
	if asTime.IsZero() {
		return nil
	}
	return &asTime
}

func (t *tagEventWatcher) sendPut(event types.Event) (err error) {
	putResourceEventStreamRequest := func(resources ...*accessgraphv1.ResourceEntry) *accessgraphv1.EventsStreamV2Request {
		return accessgraphv1.EventsStreamV2Request_builder{
			Upsert: accessgraphv1.ResourceList_builder{
				Resources: resources,
			}.Build(),
		}.Build()
	}

	var req *accessgraphv1.EventsStreamV2Request
	switch resource := event.Resource.(type) {
	case *types.UserV2:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				User: resource,
			}.Build(),
		)

	case *types.RoleV6:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				Role: resource,
			}.Build(),
		)
	case *types.AccessRequestV3:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				AccessRequest: resource,
			}.Build(),
		)
	case *types.ServerV2:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				Server: resource,
			}.Build(),
		)
	case *types.KubernetesServerV3:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				KubernetesServer: resource,
			}.Build(),
		)
	case *types.AppServerV3:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				AppServer: resource,
			}.Build(),
		)
	case *types.DatabaseServerV3:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				DatabaseServer: resource,
			}.Build(),
		)
	case *types.WindowsDesktopV3:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				WindowsDesktop: resource,
			}.Build(),
		)
	case *accesslist.AccessList:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				AccessList: proto.ValueOrDefault(accesslistv1conv.ToProto(resource)),
			}.Build(),
		)
	case *accesslist.AccessListMember:
		req = accessgraphv1.EventsStreamV2Request_builder{
			AccessListsMembers: accessgraphv1.AccessListsMembers_builder{
				Members: []*accesslistv1.Member{accesslistv1conv.ToMemberProto(resource)},
			}.Build(),
		}.Build()
	case *types.DeviceV1:
		device, err := types.DeviceFromResource(resource)
		if err != nil {
			return trace.Wrap(err)
		}
		// reset device credentials before sending to access graph
		device.ClearCredential()
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				Device: proto.ValueOrDefault(device),
			}.Build(),
		)

	case types.Resource153UnwrapperT[*dbobjectv1.DatabaseObject]:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				DatabaseObject: proto.ValueOrDefault(resource.UnwrapT()),
			}.Build(),
		)
	case types.Resource153UnwrapperT[*crownjewelv1.CrownJewel]:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				CrownJewel: proto.ValueOrDefault(resource.UnwrapT()),
			}.Build(),
		)
	case types.Resource153UnwrapperT[*accessgraphsecretsv1pb.PrivateKey]:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				PrivateKey: proto.ValueOrDefault(resource.UnwrapT()),
			}.Build(),
		)
	case types.Resource153UnwrapperT[*accessgraphsecretsv1pb.AuthorizedKey]:
		req = putResourceEventStreamRequest(
			accessgraphv1.ResourceEntry_builder{
				AuthorizedKey: proto.ValueOrDefault(resource.UnwrapT()),
			}.Build(),
		)
	default:
		return trace.BadParameter("unexpected resource type: %T", resource)
	}

	err = t.accessGraphStream.Send(
		req,
	)
	return trace.Wrap(err)
}

func resourceHeaderFromMetadata(kind, version string, t interface{ GetMetadata() types.Metadata }) *types.ResourceHeader {
	return &types.ResourceHeader{
		Kind:     kind,
		Version:  version,
		Metadata: t.GetMetadata(),
	}
}

func resourceWithLabelsToEntry(resource types.ResourceWithLabels) (*accessgraphv1.ResourceEntry, error) {
	switch r := resource.(type) {
	case *types.ServerV2:
		return accessgraphv1.ResourceEntry_builder{Server: r}.Build(), nil
	case *types.KubernetesServerV3:
		return accessgraphv1.ResourceEntry_builder{KubernetesServer: r}.Build(), nil
	case *types.AppServerV3:
		return accessgraphv1.ResourceEntry_builder{AppServer: r}.Build(), nil
	case *types.DatabaseServerV3:
		return accessgraphv1.ResourceEntry_builder{DatabaseServer: r}.Build(), nil
	case *types.WindowsDesktopV3:
		return accessgraphv1.ResourceEntry_builder{WindowsDesktop: r}.Build(), nil
	default:
		return nil, trace.BadParameter("unexpected resource type: %T", resource)
	}
}

// pushResourcesViaUnifiedResourcesCache pushes resources to the access graph service via the unified resources cache.
// It iterates over all resources in the unified resources cache whose kinds match [kinds] and pushes them to the access graph service.
func pushResourcesViaUnifiedResourcesCache(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, kinds ...string) error {
	for resource, err := range authServer.UnifiedResourceCache.Resources(ctx, "", types.SortBy{Field: types.ResourceKind}, kinds...) {
		if err != nil {
			return trace.Wrap(err)
		}

		entry, err := resourceWithLabelsToEntry(resource)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := stream.Send(accessgraphv1.EventsStreamV2Request_builder{
			Upsert: accessgraphv1.ResourceList_builder{Resources: []*accessgraphv1.ResourceEntry{entry}}.Build(),
		}.Build()); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// noOpWatcher is a watcher that does not send any events.
// We use it to replace a real watcher if Access Graph server does not support any of the resource types
// they are supposed to watch.
// This way we have a watcher that does not send any events, but is automatically released when the underlying
// context is canceled - i.e. when connection is lost.
type noOpWatcher struct {
	context.Context
}

func (f *noOpWatcher) Events() <-chan types.Event {
	return nil
}

func (f *noOpWatcher) Close() error {
	return nil
}

func (f *noOpWatcher) Error() error {
	return nil
}
