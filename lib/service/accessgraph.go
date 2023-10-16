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

package service

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
)

func initializeAndWatchAccessGraph(ctx context.Context, accessGraphAddr string, authServer *auth.Server) error {
	var opts []grpc.DialOption

	// TODO(jakule): add TLS support
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, err := grpc.Dial(accessGraphAddr, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	accessGraphClient := accessgraphv1.NewAccessGraphServiceClient(conn)

	resp, err := accessGraphClient.SendEvent(ctx, &accessgraphv1.SendEventRequest{
		Event: &proto.Event{
			Type: proto.Operation_INIT,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !resp.GetCacheInitialized() {
		// Order of sending matters here. Roles must go first.
		// TODO(jakule): Order should not matter.
		if err := sendRoles(ctx, authServer, accessGraphClient); err != nil {
			return trace.Wrap(err)
		}

		if err := sendUsers(ctx, authServer, accessGraphClient); err != nil {
			return trace.Wrap(err)
		}

		if err := sendNodes(ctx, authServer, accessGraphClient); err != nil {
			return trace.Wrap(err)
		}

		if err := sendAccessRequests(ctx, authServer, accessGraphClient); err != nil {
			return trace.Wrap(err)
		}

		// Send end event to indicate that initialization is done.
		if _, err := accessGraphClient.SendResource(ctx, &accessgraphv1.SendResourceRequest{
			Resource: &accessgraphv1.SendResourceRequest_End{},
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(startWatching(ctx, accessGraphClient, authServer))
}

func startWatching(ctx context.Context, accessGraphClient accessgraphv1.AccessGraphServiceClient, authServer *auth.Server) error {
	eventWatcher := &tagEventWatcher{
		ctx:               ctx,
		accessGraphClient: accessGraphClient,
	}
	observedKinds := []types.WatchKind{
		{Kind: types.KindNode},
		{Kind: types.KindUser},
		{Kind: types.KindRole},
		{Kind: types.KindAccessRequest},
	}

	return trace.Wrap(auth.WatchEvents(&proto.Watch{Kinds: observedKinds}, eventWatcher, "accessgraph", authServer))
}

func sendUsers(ctx context.Context, authServer *auth.Server, accessGraphClient accessgraphv1.AccessGraphServiceClient) error {
	users, err := authServer.GetUsers(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, user := range users {
		u, ok := user.(*types.UserV2)
		if !ok {
			return trace.BadParameter("expected userV2, got %T", user)
		}

		_, err := accessGraphClient.SendResource(ctx, &accessgraphv1.SendResourceRequest{
			Resource: &accessgraphv1.SendResourceRequest_User{User: u},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func sendRoles(ctx context.Context, authServer *auth.Server, accessGraphClient accessgraphv1.AccessGraphServiceClient) error {
	roles, err := authServer.GetRoles(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, role := range roles {
		r, ok := role.(*types.RoleV6)
		if !ok {
			return trace.BadParameter("expected roleV6, got %T", role)
		}

		_, err := accessGraphClient.SendResource(ctx, &accessgraphv1.SendResourceRequest{
			Resource: &accessgraphv1.SendResourceRequest_Role{Role: r},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func sendAccessRequests(ctx context.Context, authServer *auth.Server, accessGraphClient accessgraphv1.AccessGraphServiceClient) error {
	requests, err := authServer.GetAccessRequests(ctx, types.AccessRequestFilter{})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, request := range requests {
		r, ok := request.(*types.AccessRequestV3)
		if !ok {
			return trace.BadParameter("expected AccessRequestV3, got %T", request)
		}

		_, err := accessGraphClient.SendResource(ctx, &accessgraphv1.SendResourceRequest{
			Resource: &accessgraphv1.SendResourceRequest_AccessRequest{AccessRequest: r},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func sendNodes(ctx context.Context, authServer *auth.Server, accessGraphClient accessgraphv1.AccessGraphServiceClient) error {
	nodes, err := authServer.GetNodes(ctx, apidefaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, server := range nodes {
		s, ok := server.(*types.ServerV2)
		if !ok {
			return trace.BadParameter("expected ServerV2, got %T", server)
		}

		_, err := accessGraphClient.SendResource(ctx, &accessgraphv1.SendResourceRequest{
			Resource: &accessgraphv1.SendResourceRequest_Server{Server: s},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}
