// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cache

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
)

// TestKubernetes tests that CRUD operations on kubernetes clusters resources are
// replicated from the backend to the cache.
func TestKubernetes(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.KubeCluster]{
		newResource: func(name string) (types.KubeCluster, error) {
			return types.NewKubernetesClusterV3(types.Metadata{
				Name: name,
			}, types.KubernetesClusterSpecV3{})
		},
		create:    p.kubernetes.CreateKubernetesCluster,
		list:      p.kubernetes.GetKubernetesClusters,
		cacheGet:  p.cache.GetKubernetesCluster,
		cacheList: p.cache.GetKubernetesClusters,
		update:    p.kubernetes.UpdateKubernetesCluster,
		deleteAll: p.kubernetes.DeleteAllKubernetesClusters,
	})
}

// TestKubernetesServers tests that CRUD operations on kube servers are
// replicated from the backend to the cache.
func TestKubernetesServers(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForProxy)
	t.Cleanup(p.Close)

	t.Run("GetKubernetesServers", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeServer]{
			newResource: func(name string) (types.KubeServer, error) {
				app, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
				require.NoError(t, err)
				return types.NewKubernetesServerV3FromCluster(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertKubernetesServer),
			list: func(ctx context.Context) ([]types.KubeServer, error) {
				return p.presenceS.GetKubernetesServers(ctx)
			},
			cacheList: func(ctx context.Context) ([]types.KubeServer, error) {
				return p.cache.GetKubernetesServers(ctx)
			},
			update: withKeepalive(p.presenceS.UpsertKubernetesServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllKubernetesServers(ctx)
			},
		})
	})

	t.Run("ListResources", func(t *testing.T) {
		testResources(t, p, testFuncs[types.KubeServer]{
			newResource: func(name string) (types.KubeServer, error) {
				app, err := types.NewKubernetesClusterV3(types.Metadata{Name: name}, types.KubernetesClusterSpecV3{})
				require.NoError(t, err)
				return types.NewKubernetesServerV3FromCluster(app, "host", uuid.New().String())
			},
			create: withKeepalive(p.presenceS.UpsertKubernetesServer),
			list: func(ctx context.Context) ([]types.KubeServer, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindKubeServer,
				}

				var out []types.KubeServer
				for {
					resp, err := p.presenceS.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.KubeServer))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			cacheList: func(ctx context.Context) ([]types.KubeServer, error) {
				req := proto.ListResourcesRequest{
					ResourceType: types.KindKubeServer,
				}

				var out []types.KubeServer
				for {
					resp, err := p.cache.ListResources(ctx, req)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					for _, s := range resp.Resources {
						out = append(out, s.(types.KubeServer))
					}

					req.StartKey = resp.NextKey

					if req.StartKey == "" {
						break
					}
				}

				return out, nil
			},
			update: withKeepalive(p.presenceS.UpsertKubernetesServer),
			deleteAll: func(ctx context.Context) error {
				return p.presenceS.DeleteAllKubernetesServers(ctx)
			},
		})
	})

}
