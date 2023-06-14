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

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
)

func (c *Cluster) HandleHeadlessAuthentications(ctx context.Context, promptWebauthn func(ctx context.Context, in *api.PromptWebauthnRequest, opts ...grpc.CallOption) (*api.PromptWebauthnResponse, error)) error {
	var proxyClient *client.ProxyClient

	err := addMetadataToRetryableError(ctx, func() error {
		var err error
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	watcher, err := proxyClient.WatchHeadlessAuthentications(ctx)
	if err != nil {
		proxyClient.Close()
		return trace.Wrap(err)
	}

	go func() {
		defer proxyClient.Close()
		for {
			ha, err := watcher.Recv()
			if err != nil {
				return
			}

			go func() {
				if _, err := promptWebauthn(ctx, &api.PromptWebauthnRequest{}); err != nil {
					c.Log.WithError(err).Error("promptWebauthn error.")
				}
			}()

			// TODO: Present modal to user, confirm/deny, prompt for mfa
			if err := c.clusterClient.HeadlessApprove(ctx, ha.GetName(), false); err != nil {
				c.Log.WithError(err).Error("Failed to approve headless authentication state.")
			}
		}
	}()

	return nil
}
