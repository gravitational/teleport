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

package daemon

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

func (s *Service) StartHeadlessHandler(uri string) error {
	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.startHeadlessHandler(cluster); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

func (s *Service) StartHeadlessHandlers() error {
	clusters, err := s.cfg.Storage.ReadAll()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, c := range clusters {
		if c.Connected() {
			if err := s.startHeadlessHandler(c); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func (s *Service) startHeadlessHandler(cluster *clusters.Cluster) error {
	ctx, cancel := context.WithCancel(s.closeContext)
	if err := cluster.HandlePendingHeadlessAuthentications(ctx, s.pendingHeadlessAuthenticationHandler(cluster.URI.String())); err != nil {
		cancel()
		return trace.Wrap(err)
	}

	s.headlessHandlerClosers[cluster.URI.String()] = cancel
	return nil
}

func (s *Service) StopHeadlessHandler(ctx context.Context, uri string) error {
	if closer, ok := s.headlessHandlerClosers[uri]; ok {
		closer()
		delete(s.headlessHandlerClosers, uri)
	}
	return nil
}

func (s *Service) pendingHeadlessAuthenticationHandler(clusterURI string) func(ctx context.Context, ha *types.HeadlessAuthentication) error {
	return func(ctx context.Context, ha *types.HeadlessAuthentication) error {
		_, err := s.tshdEventsClient.HeadlessAuthentication(ctx, &api.HeadlessAuthenticationRequest{
			ClusterUri: clusterURI,
			HeadlessAuthentication: &api.HeadlessAuthentication{
				Name:            ha.GetName(),
				User:            ha.User,
				ClientIpAddress: ha.ClientIpAddress,
			},
		})
		return trace.Wrap(err)
	}
}
