/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	_ "google.golang.org/grpc/health"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	v1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	userspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/users/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	legacy_header "github.com/gravitational/teleport/api/types/header/convert/legacy"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// initializeAndWatchAccessGraph initializes the access graph service and watches the auth server for events.
// This function acquires a lock on the backend to ensure that only one instance of auth server is sending
// events to the access graph service at a time.
func initializeAndWatchAccessGraph(ctx context.Context, log *slog.Logger, config ServiceClientConfig, getCreds ClientCredentialsGetter, authServer *auth.Server, bk backend.Backend) error {
	// Configure health check service to monitor access graph service and
	// automatically reconnect if the connection is lost without
	// relying on new events from the auth server to trigger a reconnect.
	const serviceConfig = `{
		"loadBalancingPolicy": "round_robin",
		"healthCheckConfig": {
			"serviceName": ""
		}
	}`

	err := backend.RunWhileLocked(
		ctx,
		backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				LockName:      "accessGraphClient",
				Backend:       bk,
				TTL:           10 * time.Second,
				RetryInterval: 5 * time.Second,
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

			stream, err := client.EventsStreamV2(ctx)
			if err != nil {
				log.ErrorContext(ctx, "Failed to get access graph service stream", "error", err)
				return trace.Wrap(err)
			}

			header, err := stream.Header()
			if err != nil {
				log.ErrorContext(ctx, "Failed to get access graph service stream header", "error", err)
				return trace.Wrap(err)
			}
			const (
				supportedResourcesKey = "supported-kinds"
			)
			supportedKinds := header.Get(supportedResourcesKey)
			if len(supportedKinds) == 0 {
				return trace.BadParameter("access graph service did not return supported kinds")
			}

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

			// Start a goroutine to watch the access graph service connection state.
			// If the connection is closed, cancel the context to stop the event watcher
			// before it tries to send any events to the access graph service.
			go func() {
				defer cancel()
				if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
					log.InfoContext(ctx, "access graph service connection was closed")
				}
			}()

			eventWatcherSender := newTagEventWatcher(newCtx, stream)
			watcher, err := authServer.Cache.NewWatcher(
				eventWatcherSender.Context(),
				types.Watch{
					Kinds:               supportedKindsToWatcherKinds(supportedKinds),
					AllowPartialSuccess: true,
				},
			)
			if err != nil {
				return trace.Wrap(err)
			}
			defer watcher.Close()
			cacheSupportedResources, err := waitForInit(watcher)
			if err != nil {
				return trace.Wrap(err)
			}
			missingWatchKinds(ctx, log, supportedKinds, cacheSupportedResources)
			errc := make(chan error, 1)
			go func() {
				// Start watching the auth server for events.
				// Subscribe for new events before sending all resources.
				// Otherwise, we might miss some events.
				errc <- forwardEventsWatch(watcher, eventWatcherSender)
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
		})
	return trace.Wrap(err)
}

type eventSender interface {
	EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error
}

func processTAGMessage(ctx context.Context, obj *accessgraphv1.EventsStreamV2Response, authServer eventSender, log *slog.Logger) {
	switch o := obj.Action.(type) {
	case *accessgraphv1.EventsStreamV2Response_Event:
		event := convertEvent(o.Event)
		if event == nil {
			log.WarnContext(ctx, "Received unknown event type from access graph service", "event", obj)
			return
		}

		if err := authServer.EmitAuditEvent(ctx, event); err != nil {
			log.ErrorContext(ctx, "Failed to emit Crown Jewel update event")
		}
	default:
		log.WarnContext(ctx, "Received unknown event type from access graph service", "event", obj)
	}
}

func convertEvent(event *accessgraphv1.AuditEvent) apievents.AuditEvent {
	var tEvent apievents.AuditEvent

	switch e := event.Event.(type) {
	case *accessgraphv1.AuditEvent_AccessPathChanged:
		data := e.AccessPathChanged
		tEvent = &apievents.AccessPathChanged{
			Metadata: apievents.Metadata{
				Type: events.AccessGraphAccessPathChangedEvent,
				Code: events.AccessGraphAccessPathChangedCode,
			},
			ChangeID:               data.ChangeId,
			AffectedResourceName:   data.AffectedResourceName,
			AffectedResourceSource: data.AffectedResourceSource,
		}
	default:
		return nil
	}

	return tEvent
}

func supportedKindsToWatcherKinds(supportedKinds []string) []types.WatchKind {
	var observedKinds []types.WatchKind
	for _, kind := range supportedKinds {
		observedKinds = append(observedKinds, types.WatchKind{Kind: kind})
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
		cache:             make([]*proto.Event, 0),
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
		if err := sendAccessRequests(ctx, authServer, stream); err != nil {
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

	// Send end event to indicate that initialization is done.
	err := stream.Send(
		&accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_Sync{
				Sync: &accessgraphv1.SyncOperation{},
			},
		},
	)
	return trace.Wrap(err)
}

// forwardEventsWatch starts watching the auth server for events and sends them to the access graph service.
func forwardEventsWatch(watcher types.Watcher, eventWatcher *tagEventWatcher) error {
	for {
		select {
		case event := <-watcher.Events():
			out, err := client.EventToGRPC(event)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := eventWatcher.Send(out); err != nil {
				return trace.Wrap(err)
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		}
	}
}

// sendUsers sends all users to the access graph service.
func sendUsers(ctx context.Context, authServer interface {
	ListUsers(ctx context.Context, req *userspb.ListUsersRequest) (*userspb.ListUsersResponse, error)
}, stream accessgraphv1.AccessGraphService_EventsStreamV2Client,
) error {
	req := userspb.ListUsersRequest{
		PageSize: apidefaults.DefaultChunkSize,
	}

	for {
		rsp, err := authServer.ListUsers(ctx, &req)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := pushUsersToTAG(ctx, stream, rsp.Users); err != nil {
			return trace.Wrap(err)
		}

		req.PageToken = rsp.NextPageToken
		if req.PageToken == "" {
			break
		}
	}

	return nil
}

func sendCrownJewels(ctx context.Context, authServer interface {
	ListCrownJewels(ctx context.Context, pageSize int64, nextToken string) ([]*crownjewelv1.CrownJewel, string, error)
}, stream accessgraphv1.AccessGraphService_EventsStreamV2Client,
) error {
	nextToken := ""

	for {
		crownJewels, token, err := authServer.ListCrownJewels(ctx, 0, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := pushCrownJewelsToTAG(ctx, stream, crownJewels); err != nil {
			return trace.Wrap(err)
		}

		if token == "" {
			break
		}

		nextToken = token
	}

	return nil
}

func pushUsersToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, users []*types.UserV2) error {
	if len(users) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, user := range users {
		list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
			Resource: &accessgraphv1.ResourceEntry_User{
				User: user,
			},
		})
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}

func pushCrownJewelsToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, crownJewels []*crownjewelv1.CrownJewel) error {
	if len(crownJewels) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, crownJewel := range crownJewels {
		list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
			Resource: &accessgraphv1.ResourceEntry_CrownJewel{
				CrownJewel: crownJewel,
			},
		})
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}

// sendRoles sends all roles to the access graph service.
func sendRoles(ctx context.Context, authServer interface {
	GetRoles(context.Context) ([]types.Role, error)
}, stream accessgraphv1.AccessGraphService_EventsStreamV2Client,
) error {
	// Get all roles.
	// Auth server does not support pagination for roles, so we have to get all roles at once
	// and we chunk them after.
	roles, err := authServer.GetRoles(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	chunkSize := apidefaults.DefaultChunkSize
	for i := 0; i < len(roles); i += chunkSize {
		end := i + chunkSize
		if end > len(roles) {
			end = len(roles)
		}

		if err := pushRolesToTAG(ctx, stream, roles[i:end]); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func pushRolesToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, roles []types.Role) error {
	if len(roles) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, role := range roles {
		r, ok := role.(*types.RoleV6)
		if !ok {
			return trace.BadParameter("expected *types.RoleV6, got %T", role)
		}
		list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
			Resource: &accessgraphv1.ResourceEntry_Role{
				Role: r,
			},
		})
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}

// sendUsers sends all users to the access graph service.
func sendAccessLists(ctx context.Context, authServer interface {
	ListAccessLists(context.Context, int, string) ([]*accesslist.AccessList, string, error)
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}, stream accessgraphv1.AccessGraphService_EventsStreamV2Client,
) error {
	startToken := ""
	limit := 0 // use default limit

	for {
		accessLists, nextToken, err := authServer.ListAccessLists(ctx, limit, startToken)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := pushAccessListsToTAG(ctx, stream, accessLists); err != nil {
			return trace.Wrap(err)
		}

		for _, accessList := range accessLists {
			if err := sendAccessListMembers(ctx, authServer, stream, accessList); err != nil {
				return trace.Wrap(err)
			}
		}

		if nextToken == "" {
			break
		}
		startToken = nextToken
	}

	return nil
}

func pushAccessListsToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, accessLists []*accesslist.AccessList) error {
	if len(accessLists) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, accessList := range accessLists {
		list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
			Resource: &accessgraphv1.ResourceEntry_AccessList{
				AccessList: accesslistv1conv.ToProto(accessList),
			},
		})
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}

func sendAccessListMembers(ctx context.Context, authServer interface {
	ListAccessListMembers(ctx context.Context, accessListName string, pageSize int, pageToken string) (members []*accesslist.AccessListMember, nextToken string, err error)
}, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, accessList *accesslist.AccessList,
) error {
	startToken := ""
	limit := 0 // use default limit

	for {

		accessListMembers, nextToken, err := authServer.ListAccessListMembers(ctx, accessList.GetName(), limit, startToken)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := pushAccessListMembersToTAG(ctx, stream, accessListMembers); err != nil {
			return trace.Wrap(err)
		}

		if nextToken == "" {
			break
		}
		startToken = nextToken
	}

	return nil
}

func pushAccessListMembersToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, accessListMembers []*accesslist.AccessListMember) error {
	if len(accessListMembers) == 0 {
		return nil
	}
	list := &accessgraphv1.AccessListsMembers{}
	for _, accessListMember := range accessListMembers {
		list.Members = append(
			list.Members, accesslistv1conv.ToMemberProto(accessListMember),
		)
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_AccessListsMembers{
			AccessListsMembers: list,
		},
	}))
}

func sendDatabaseObjects(ctx context.Context, authServer services.DatabaseObjectsGetter, stream accessgraphv1.AccessGraphService_EventsStreamV2Client) error {
	startToken := ""
	limit := 0 // use default limit

	for {
		objects, nextToken, err := authServer.ListDatabaseObjects(ctx, limit, startToken)
		if err != nil {
			return trace.Wrap(err)
		}

		list := &accessgraphv1.ResourceList{}
		for _, object := range objects {
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_DatabaseObject{
					DatabaseObject: object,
				},
			})
		}

		err = stream.Send(&accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
				Upsert: list,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if nextToken == "" {
			break
		}

		startToken = nextToken
	}

	return nil
}

// sendAccessRequests sends all access requests to the access graph service.
func sendAccessRequests(ctx context.Context, authServer services.AccessRequestGetter, stream accessgraphv1.AccessGraphService_EventsStreamV2Client) error {
	requests, err := authServer.GetAccessRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	chunkSize := apidefaults.DefaultChunkSize
	for i := 0; i < len(requests); i += chunkSize {
		end := i + chunkSize
		if end > len(requests) {
			end = len(requests)
		}

		if err := pushAccessRequestToTAG(ctx, stream, requests[i:end]); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func pushAccessRequestToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, accessRequests []types.AccessRequest) error {
	if len(accessRequests) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, accessRequest := range accessRequests {
		a, ok := accessRequest.(*types.AccessRequestV3)
		if !ok {
			return trace.BadParameter("expected *types.AccessRequestV3, got %T", accessRequest)
		}
		list.Resources = append(
			list.Resources,
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_AccessRequest{
					AccessRequest: a,
				},
			},
		)
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}

type accessGraphSender interface {
	Send(request *accessgraphv1.EventsStreamV2Request) error
}

type tagEventWatcher struct {
	ctx context.Context
	// ready is set to true when the watcher is ready to send events.
	ready atomic.Bool
	// cache is used to cache events before the watcher is ready.
	cache []*proto.Event
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
func (t *tagEventWatcher) Send(event *proto.Event) error {
	// If the watcher is not ready, cache the event and send it later.
	if !t.ready.Load() {
		t.cacheMtx.Lock()
		defer t.cacheMtx.Unlock()

		// Double check if the watcher is ready, as the oder of locking in MarkReady is not the same.
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

func (t *tagEventWatcher) send(event *proto.Event) error {
	switch event.Type {
	case proto.Operation_DELETE:
		return trace.Wrap(t.sendDelete(event))
	case proto.Operation_PUT:
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

func (t *tagEventWatcher) sendDelete(event *proto.Event) error {
	deleteEventStreamRequest := func(header *types.ResourceHeader) *accessgraphv1.EventsStreamV2Request {
		return &accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_Delete{
				Delete: &accessgraphv1.ResourceHeaderList{
					Resources: []*types.ResourceHeader{
						header,
					},
				},
			},
		}
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
	case *proto.Event_User:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindUser,
				types.V2,
				resource.User,
			),
		)

	case *proto.Event_Role:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindRole,
				resource.Role.Version,
				resource.Role,
			),
		)
	case *proto.Event_AccessRequest:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindAccessRequest,
				resource.AccessRequest.Version,
				resource.AccessRequest,
			),
		)
	case *proto.Event_Server:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindNode,
				resource.Server.Version,
				resource.Server,
			),
		)
	case *proto.Event_KubernetesServer:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindKubeServer,
				resource.KubernetesServer.Version,
				resource.KubernetesServer,
			),
		)
	case *proto.Event_AppServer:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindAppServer,
				resource.AppServer.Version,
				resource.AppServer,
			),
		)
	case *proto.Event_DatabaseServer:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindDatabaseServer,
				resource.DatabaseServer.Version,
				resource.DatabaseServer,
			),
		)
	case *proto.Event_WindowsDesktop:
		req = deleteEventStreamRequest(
			resourceHeaderFromMetadata(
				types.KindWindowsDesktop,
				resource.WindowsDesktop.Version,
				resource.WindowsDesktop,
			),
		)
	case *proto.Event_ResourceHeader:
		if resource.ResourceHeader.Kind == types.KindAccessListMember {
			// Access list member uses a different object format
			// when it is sent to the access graph service.
			req = &accessgraphv1.EventsStreamV2Request{
				Operation: &accessgraphv1.EventsStreamV2Request_ExcludeAccessListMembers{
					ExcludeAccessListMembers: &accessgraphv1.ExcludeAccessListsMembers{
						Members: []*accessgraphv1.ExcludeAccessListMember{
							{
								// access list name comes from the resource header description
								AccessList: resource.ResourceHeader.GetMetadata().Description,
								Username:   resource.ResourceHeader.GetName(),
							},
						},
					},
				},
			}
			break
		}
		req = deleteEventStreamRequest(resource.ResourceHeader)
	case *proto.Event_AccessList:
		header := headerv1.FromResourceHeaderProto(resource.AccessList.GetHeader())
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:    header.GetKind(),
				Version: header.GetVersion(),
				Metadata: legacy_header.FromHeaderMetadata(
					header.GetMetadata(),
				),
			},
		)
	case *proto.Event_AccessListMember:
		// Access list member uses a different header format.
		req = &accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_ExcludeAccessListMembers{
				ExcludeAccessListMembers: &accessgraphv1.ExcludeAccessListsMembers{
					Members: []*accessgraphv1.ExcludeAccessListMember{
						{
							AccessList: resource.AccessListMember.GetSpec().GetAccessList(),
							Username:   resource.AccessListMember.GetSpec().GetName(),
						},
					},
				},
			},
		}
	case *proto.Event_DatabaseObject:
		req = deleteEventStreamRequestResource153(resource.DatabaseObject)
	case *proto.Event_CrownJewel:
		req = deleteEventStreamRequest(
			&types.ResourceHeader{
				Kind:     resource.CrownJewel.Kind,
				Version:  resource.CrownJewel.Version,
				Metadata: fromProtoMetadataToTypes(resource.CrownJewel.Metadata),
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

func (t *tagEventWatcher) sendPut(event *proto.Event) (err error) {
	putResourceEventStreamRequest := func(resources ...*accessgraphv1.ResourceEntry) *accessgraphv1.EventsStreamV2Request {
		return &accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: resources,
				},
			},
		}
	}

	var req *accessgraphv1.EventsStreamV2Request
	switch resource := event.Resource.(type) {
	case *proto.Event_User:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_User{
					User: resource.User,
				},
			},
		)

	case *proto.Event_CrownJewel:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_CrownJewel{
					CrownJewel: resource.CrownJewel,
				},
			},
		)
	case *proto.Event_Role:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_Role{
					Role: resource.Role,
				},
			},
		)
	case *proto.Event_AccessRequest:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_AccessRequest{
					AccessRequest: resource.AccessRequest,
				},
			},
		)
	case *proto.Event_Server:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_Server{
					Server: resource.Server,
				},
			},
		)
	case *proto.Event_KubernetesServer:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_KubernetesServer{
					KubernetesServer: resource.KubernetesServer,
				},
			},
		)
	case *proto.Event_AppServer:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_AppServer{
					AppServer: resource.AppServer,
				},
			},
		)
	case *proto.Event_DatabaseServer:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_DatabaseServer{
					DatabaseServer: resource.DatabaseServer,
				},
			},
		)
	case *proto.Event_WindowsDesktop:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_WindowsDesktop{
					WindowsDesktop: resource.WindowsDesktop,
				},
			},
		)
	case *proto.Event_AccessList:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_AccessList{
					AccessList: resource.AccessList,
				},
			},
		)
	case *proto.Event_AccessListMember:
		req = &accessgraphv1.EventsStreamV2Request{
			Operation: &accessgraphv1.EventsStreamV2Request_AccessListsMembers{
				AccessListsMembers: &accessgraphv1.AccessListsMembers{
					Members: []*accesslistv1.Member{resource.AccessListMember},
				},
			},
		}
	case *proto.Event_DatabaseObject:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_DatabaseObject{
					DatabaseObject: resource.DatabaseObject,
				},
			},
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

// pushResourcesViaUnifiedResourcesCache pushes resources to the access graph service via the unified resources cache.
// It iterates over all resources in the unified resources cache whose kinds match [kinds] and pushes them to the access graph service.
func pushResourcesViaUnifiedResourcesCache(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamV2Client, kinds ...string) error {
	set := utils.StringsSet(kinds)
	req := &proto.ListUnifiedResourcesRequest{
		Kinds: kinds,
		Limit: apidefaults.DefaultChunkSize,
		SortBy: types.SortBy{
			Field: types.ResourceKind,
		},
	}
	if err := req.CheckAndSetDefaults(); err != nil {
		panic(err)
	}

	for {
		resources, nextKey, err := authServer.UnifiedResourceCache.IterateUnifiedResources(
			ctx,
			func(rwl types.ResourceWithLabels) (bool, error) {
				_, ok := set[rwl.GetKind()]
				return ok, nil
			},
			req,
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := pushResourcesWithLabelsToTAG(resources, stream); err != nil {
			return trace.Wrap(err)
		}
		if nextKey == "" {
			break
		}
		req.StartKey = nextKey
	}
	return nil
}

// pushResourcesWithLabelsToTAG pushes resources with labels to the access graph service.
func pushResourcesWithLabelsToTAG(resources []types.ResourceWithLabels, stream accessgraphv1.AccessGraphService_EventsStreamV2Client) error {
	if len(resources) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, resource := range resources {
		switch resource := resource.(type) {
		case *types.ServerV2:
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_Server{
					Server: resource,
				},
			})
		case *types.KubernetesServerV3:
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_KubernetesServer{
					KubernetesServer: resource,
				},
			})
		case *types.AppServerV3:
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_AppServer{
					AppServer: resource,
				},
			})
		case *types.DatabaseServerV3:
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_DatabaseServer{
					DatabaseServer: resource,
				},
			})
		case *types.WindowsDesktopV3:
			list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_WindowsDesktop{
					WindowsDesktop: resource,
				},
			})
		default:
			return trace.BadParameter("unexpected resource type: %T", resource)
		}
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamV2Request{
		Operation: &accessgraphv1.EventsStreamV2Request_Upsert{
			Upsert: list,
		},
	}))
}
