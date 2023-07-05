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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
)

func (c *Cluster) HandlePendingHeadlessAuthentications(ctx context.Context, handler func(ctx context.Context, ha *types.HeadlessAuthentication) error) error {
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

	watcher, err := proxyClient.WatchPendingHeadlessAuthentications(ctx)
	if err != nil {
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
			if err := handler(ctx, ha); err != nil {
				return trace.Wrap(err)
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())
		}
	}
}
