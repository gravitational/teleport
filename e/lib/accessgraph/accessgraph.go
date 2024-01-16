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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	_ "google.golang.org/grpc/health"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistv1conv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	legacy_header "github.com/gravitational/teleport/api/types/header/convert/legacy"
	headerv1 "github.com/gravitational/teleport/api/types/header/convert/v1"
	"github.com/gravitational/teleport/e/lib/licensefile"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
)

// ServiceClientConfig is the configuration for the access graph service client.
type ServiceClientConfig struct {
	// Addr is the address of the access graph service.
	Addr string
	// CA is the path to the CA certificate used to verify the access graph GRPC connection.
	CA string
	// License is the license file used to authenticate the access graph GRPC connection and share Tenant ID.
	License *licensefile.LicenseFile
	// Insecure is true if the access graph GRPC connection should be insecure.
	// Do not use in production.
	Insecure bool
}

// NewAccessGraphClient returns a new access graph service client.
func NewAccessGraphClient(ctx context.Context, config ServiceClientConfig, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	opt, err := grpcCredentials(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conn, err := grpc.DialContext(ctx, config.Addr, append(opts, opt)...)
	return conn, trace.Wrap(err)
}

// initializeAndWatchAccessGraph initializes the access graph service and watches the auth server for events.
// This function acquires a lock on the backend to ensure that only one instance of auth server is sending
// events to the access graph service at a time.
func initializeAndWatchAccessGraph(ctx context.Context, log logrus.FieldLogger, config ServiceClientConfig, authServer *auth.Server, bk backend.Backend) error {
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
				grpc.WithDefaultServiceConfig(serviceConfig),
			)
			if err != nil {
				return trace.Wrap(err)
			}
			// Close the connection when the function returns.
			defer accessGraphConn.Close()
			client := accessgraphv1.NewAccessGraphServiceClient(accessGraphConn)

			stream, err := client.EventsStream(ctx)
			if err != nil {
				log.WithError(err).Error("Failed to get access graph service stream")
				return trace.Wrap(err)
			}

			newCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			// Start a goroutine to watch the access graph service connection state.
			// If the connection is closed, cancel the context to stop the event watcher
			// before it tries to send any events to the access graph service.
			go func() {
				defer cancel()
				if !accessGraphConn.WaitForStateChange(ctx, connectivity.Ready) {
					log.Info("access graph service connection was closed")
				}
			}()

			eventWatcher := newTagEventWatcher(newCtx, stream)

			errc := make(chan error)
			go func() {
				// Start watching the auth server for events.
				// Subscribe for new events before sending all resources.
				// Otherwise, we might miss some events.
				errc <- startWatching(eventWatcher, authServer)
			}()

			log.Debug("Sending teleport resources to access graph service")
			// Send all teleport resources to the access graph service.
			if err := sendTeleportResources(ctx, stream, authServer); err != nil {
				log.WithError(err).Error("Failed to send teleport resources to access graph service")
				return trace.Wrap(err)
			}

			log.Debug("Done sending teleport resources to access graph service")

			// Marks as ready and send cached resources to TAG
			if err := eventWatcher.MarkReady(); err != nil {
				return trace.Wrap(err)
			}

			err = <-errc
			if errors.Is(err, context.Canceled) {
				log.WithError(err).Info("access graph service connection was closed")
				return trace.Wrap(err)
			} else if err != nil {
				log.WithError(err).Error("Failed to start watching access graph service")
				return trace.Wrap(err)
			}

			return nil
		})
	return trace.Wrap(err)
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
func sendTeleportResources(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, authServer *auth.Server) error {
	if err := sendRoles(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := sendUsers(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := sendAccessRequests(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := pushResourcesViaUnifiedResourcesCache(
		ctx,
		authServer,
		stream,
		types.KindNode,
		types.KindAppServer,
		types.KindDatabaseServer,
		types.KindWindowsDesktop,
		types.KindKubeServer,
	); err != nil {
		return trace.Wrap(err)
	}

	if err := sendAccessLists(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	// Send end event to indicate that initialization is done.
	err := stream.Send(
		&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Sync{
				Sync: &accessgraphv1.SyncOperation{},
			},
		},
	)
	return trace.Wrap(err)
}

// startWatching starts watching the auth server for events and sends them to the access graph service.
func startWatching(eventWatcher *tagEventWatcher, authServer *auth.Server) error {
	observedKinds := []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindAccessRequest},
		{Kind: types.KindKubeServer},
		{Kind: types.KindAppServer},
		{Kind: types.KindDatabaseServer},
		{Kind: types.KindWindowsDesktop},
		{Kind: types.KindAccessListMember},
		{Kind: types.KindAccessList},
	}

	watcher, err := authServer.Services.NewWatcher(
		eventWatcher.Context(),
		types.Watch{
			Kinds: observedKinds,
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

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
func sendUsers(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
	startToken := ""
	limit := apidefaults.DefaultChunkSize

	for {
		users, nextToken, err := authServer.ListUsers(ctx, limit, startToken, false /*withSecrets*/)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := pushUsersToTAG(ctx, stream, users); err != nil {
			return trace.Wrap(err)
		}

		if nextToken == "" {
			break
		}
		startToken = nextToken
	}

	return nil
}

func pushUsersToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, users []types.User) error {
	if len(users) == 0 {
		return nil
	}
	list := &accessgraphv1.ResourceList{}
	for _, user := range users {
		u, ok := user.(*types.UserV2)
		if !ok {
			return trace.BadParameter("expected types.UserV2, got %T", user)
		}
		list.Resources = append(list.Resources, &accessgraphv1.ResourceEntry{
			Resource: &accessgraphv1.ResourceEntry_User{
				User: u,
			},
		})
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_Upsert{
			Upsert: list,
		},
	}))
}

// sendRoles sends all roles to the access graph service.
func sendRoles(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
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

func pushRolesToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, roles []types.Role) error {
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
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_Upsert{
			Upsert: list,
		},
	}))
}

// sendUsers sends all users to the access graph service.
func sendAccessLists(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
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

func pushAccessListsToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, accessLists []*accesslist.AccessList) error {
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
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_Upsert{
			Upsert: list,
		},
	}))
}

func sendAccessListMembers(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient, accessList *accesslist.AccessList) error {
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

func pushAccessListMembersToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, accessListMembers []*accesslist.AccessListMember) error {
	if len(accessListMembers) == 0 {
		return nil
	}
	list := &accessgraphv1.AccessListsMembers{}
	for _, accessListMember := range accessListMembers {
		list.Members = append(
			list.Members, accesslistv1conv.ToMemberProto(accessListMember),
		)
	}
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_AccessListsMembers{
			AccessListsMembers: list,
		},
	}))
}

// sendAccessRequests sends all access requests to the access graph service.
func sendAccessRequests(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
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

func pushAccessRequestToTAG(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, accessRequests []types.AccessRequest) error {
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
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_Upsert{
			Upsert: list,
		},
	}))
}

// grpcCredentials returns a grpc.DialOption configured with TLS credentials.
func grpcCredentials(config ServiceClientConfig) (grpc.DialOption, error) {
	cert, err := tls.X509KeyPair(
		config.License.KeyPair.CertPEM,
		config.License.KeyPair.KeyPEM,
	)
	if err != nil {
		return nil, trace.Wrap(err, "cannot parse License authority key pair")
	}

	var pool *x509.CertPool
	if config.CA != "" {
		pool = x509.NewCertPool()
		caBytes, err := os.ReadFile(config.CA)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !pool.AppendCertsFromPEM(caBytes) {
			return nil, trace.BadParameter("failed to append CA certificate to pool")
		}
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			cert,
		},
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: config.Insecure,
		RootCAs:            pool,
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)), nil
}

type accessGraphSender interface {
	Send(*accessgraphv1.EventsStreamRequest) error
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

// MarkReady marks the watcher as ready to send events.
func (t *tagEventWatcher) MarkReady() error {
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
	deleteEventStreamRequest := func(header *types.ResourceHeader) *accessgraphv1.EventsStreamRequest {
		return &accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Delete{
				Delete: &accessgraphv1.ResourceHeaderList{
					Resources: []*types.ResourceHeader{
						header,
					},
				},
			},
		}
	}
	var req *accessgraphv1.EventsStreamRequest
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
		req = &accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_ExcludeAccessListMembers{
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
	default:
		return trace.BadParameter("unexpected resource type: %T", resource)
	}

	err := t.accessGraphStream.Send(
		req,
	)
	return trace.Wrap(err)
}

func (t *tagEventWatcher) sendPut(event *proto.Event) (err error) {
	putResourceEventStreamRequest := func(resources ...*accessgraphv1.ResourceEntry) *accessgraphv1.EventsStreamRequest {
		return &accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: resources,
				},
			},
		}
	}

	var req *accessgraphv1.EventsStreamRequest
	switch resource := event.Resource.(type) {
	case *proto.Event_User:
		req = putResourceEventStreamRequest(
			&accessgraphv1.ResourceEntry{
				Resource: &accessgraphv1.ResourceEntry_User{
					User: resource.User,
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
		req = &accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_AccessListsMembers{
				AccessListsMembers: &accessgraphv1.AccessListsMembers{
					Members: []*accesslistv1.Member{resource.AccessListMember},
				},
			},
		}
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
func pushResourcesViaUnifiedResourcesCache(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient, kinds ...string) error {
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
func pushResourcesWithLabelsToTAG(resources []types.ResourceWithLabels, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
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
	return trace.Wrap(stream.Send(&accessgraphv1.EventsStreamRequest{
		Operation: &accessgraphv1.EventsStreamRequest_Upsert{
			Upsert: list,
		},
	}))
}
