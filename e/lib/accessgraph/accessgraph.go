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

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
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

	for {
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

				// Send all teleport resources to the access graph service.
				if err := sendTeleportResources(ctx, stream, authServer); err != nil {
					log.WithError(err).Error("Failed to send teleport resources to access graph service")
					return trace.Wrap(err)
				}

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
		if err != nil {
			log.WithError(err).Error("Failed to run access graph service while locked")
			// Wait a bit before retrying.
			time.Sleep(5 * time.Second)
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
func sendTeleportResources(ctx context.Context, stream accessgraphv1.AccessGraphService_EventsStreamClient, authServer *auth.Server) error {
	// Order of sending matters here. Roles must go first.
	// TODO(jakule): Order should not matter.
	if err := sendRoles(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := sendUsers(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := sendNodes(ctx, authServer, stream); err != nil {
		return trace.Wrap(err)
	}

	if err := sendAccessRequests(ctx, authServer, stream); err != nil {
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
	}

	return trace.Wrap(auth.WatchEvents(&proto.Watch{Kinds: observedKinds}, eventWatcher, "accessgraph", authServer))
}

// sendUsers sends all users to the access graph service.
func sendUsers(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
	users, err := authServer.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, user := range users {
		u, ok := user.(*types.UserV2)
		if !ok {
			return trace.BadParameter("expected userV2, got %T", user)
		}

		// TODO(tigrato): batch these up
		err := stream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_User{
								User: u,
							},
						},
					},
				},
			},
		},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// sendRoles sends all roles to the access graph service.
func sendRoles(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
	roles, err := authServer.GetRoles(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, role := range roles {
		logrus.Infof("sending role: %v", role.GetName())
		r, ok := role.(*types.RoleV6)
		if !ok {
			return trace.BadParameter("expected roleV6, got %T", role)
		}
		// TODO(tigrato): batch these up
		err := stream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_Role{
								Role: r,
							},
						},
					},
				},
			},
		},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// sendAccessRequests sends all access requests to the access graph service.
func sendAccessRequests(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
	requests, err := authServer.GetAccessRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, request := range requests {
		r, ok := request.(*types.AccessRequestV3)
		if !ok {
			return trace.BadParameter("expected AccessRequestV3, got %T", request)
		}

		// TODO(tigrato): batch these up
		err := stream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_AccessRequest{
								AccessRequest: r,
							},
						},
					},
				},
			},
		},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// sendNodes sends all nodes to the access graph service.
func sendNodes(ctx context.Context, authServer *auth.Server, stream accessgraphv1.AccessGraphService_EventsStreamClient) error {
	nodes, err := authServer.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, server := range nodes {
		s, ok := server.(*types.ServerV2)
		if !ok {
			return trace.BadParameter("expected ServerV2, got %T", server)
		}

		// TODO(tigrato): batch these up
		err := stream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_Server{
								Server: s,
							},
						},
					},
				},
			},
		},
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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
	//cacheMtx is used to synchronize access to the cache.
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
	resourceHeader, ok := event.Resource.(*proto.Event_ResourceHeader)
	if !ok {
		return trace.BadParameter("expected resource header, got %T", event.Resource)
	}
	err := t.accessGraphStream.Send(
		&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Delete{
				Delete: &accessgraphv1.ResourceHeaderList{
					Resources: []*types.ResourceHeader{resourceHeader.ResourceHeader},
				},
			},
		},
	)
	return trace.Wrap(err)
}

func (t *tagEventWatcher) sendPut(event *proto.Event) (err error) {
	switch resource := event.Resource.(type) {
	case *proto.Event_User:
		err = t.accessGraphStream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_User{
								User: resource.User,
							},
						},
					},
				},
			},
		},
		)
	case *proto.Event_Role:
		err = t.accessGraphStream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_Role{
								Role: resource.Role,
							},
						},
					},
				},
			},
		},
		)
	case *proto.Event_AccessRequest:
		err = t.accessGraphStream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_AccessRequest{
								AccessRequest: resource.AccessRequest,
							},
						},
					},
				},
			},
		},
		)
	case *proto.Event_Server:
		err = t.accessGraphStream.Send(&accessgraphv1.EventsStreamRequest{
			Operation: &accessgraphv1.EventsStreamRequest_Upsert{
				Upsert: &accessgraphv1.ResourceList{
					Resources: []*accessgraphv1.ResourceEntry{
						{
							Resource: &accessgraphv1.ResourceEntry_Server{
								Server: resource.Server,
							},
						},
					},
				},
			},
		},
		)
	default:
		return trace.BadParameter("unexpected resource type: %T", resource)
	}

	return trace.Wrap(err)
}
