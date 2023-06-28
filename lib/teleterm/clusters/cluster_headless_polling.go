// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusters

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
)

func (c *Cluster) HandleHeadlessAuthentications(ctx context.Context, promptMFA func(ctx context.Context, in *api.PromptMFARequest, opts ...grpc.CallOption) (*api.PromptMFAResponse, error)) error {
	var proxyClient *client.ProxyClient

	err := addMetadataToRetryableError(ctx, func() error {
		var err error
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	watcher, err := proxyClient.WatchHeadlessAuthentications(ctx)
	if err != nil {
		proxyClient.Close()
		return trace.Wrap(err)
	}
	defer watcher.Close()

	for {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case event := <-watcher.Events():
			ha, ok := event.Resource.(*types.HeadlessAuthentication)
			if !ok {
				return trace.BadParameter("unexpected resource type %T", event.Resource)
			}

			rootClient, err := proxyClient.ConnectToRootCluster(ctx)
			if err != nil {
				return trace.Wrap(err)
			}

			if _, err := promptMFA(ctx, &api.PromptMFARequest{
				Request: &api.PromptMFARequest_HeadlessRequest{
					HeadlessRequest: &api.HeadlessRequest{
						HeadlessAuthentication: &api.HeadlessAuthentication{
							User:            ha.User,
							ClientIpAddress: ha.ClientIpAddress,
						},
					},
				},
			}); err != nil {
				c.Log.WithError(err).Error("promptMFA error.")
			}

			chall, err := rootClient.CreateAuthenticateChallenge(ctx, &proto.CreateAuthenticateChallengeRequest{
				Request: &proto.CreateAuthenticateChallengeRequest_ContextUser{
					ContextUser: &proto.ContextUser{},
				},
			})
			if err != nil {
				return trace.Wrap(err)
			}

			resp, err := c.clusterClient.PromptMFAChallenge(ctx, tc.WebProxyAddr, chall, nil)
			if err != nil {
				return trace.Wrap(err)
			}

			err = rootClient.UpdateHeadlessAuthenticationState(ctx, ha.GetName(), types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED, resp)
			return trace.Wrap(err)

		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		}
	}
}
