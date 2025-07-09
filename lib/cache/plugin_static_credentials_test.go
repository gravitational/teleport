// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestPluginStaticCredentials(t *testing.T) {
	t.Parallel()

	p, err := newPack(t.TempDir(), ForAuth)
	require.NoError(t, err)
	t.Cleanup(p.Close)

	makeLabels := func(name string) map[string]string {
		return map[string]string{
			"resource-label": "label-for-" + name,
		}
	}

	cacheGets := []struct {
		name string
		fn   func(context.Context, string) (types.PluginStaticCredentials, error)
	}{
		{
			name: "GetPluginStaticCredentials",
			fn:   p.cache.GetPluginStaticCredentials,
		},
		{
			name: "GetPluginStaticCredentialsByLabels",
			fn: func(ctx context.Context, name string) (types.PluginStaticCredentials, error) {
				creds, err := p.cache.GetPluginStaticCredentialsByLabels(ctx, makeLabels(name))
				if err != nil {
					return nil, trace.Wrap(err)
				}
				if len(creds) != 1 {
					return nil, trace.CompareFailed("expecting one creds for this test but got %v", len(creds))
				}
				return creds[0], nil
			},
		},
	}

	for _, cacheGet := range cacheGets {
		t.Run(cacheGet.name, func(t *testing.T) {
			// Empty backend before the test.
			err := p.pluginStaticCredentials.DeleteAllPluginStaticCredentials(context.Background())
			require.NoError(t, err)

			testResources(t, p, testFuncs[types.PluginStaticCredentials]{
				newResource: func(name string) (types.PluginStaticCredentials, error) {
					return types.NewPluginStaticCredentials(
						types.Metadata{
							Name:   name,
							Labels: makeLabels(name),
						},
						types.PluginStaticCredentialsSpecV1{
							Credentials: &types.PluginStaticCredentialsSpecV1_APIToken{
								APIToken: "some-token",
							},
						})
				},
				create: p.pluginStaticCredentials.CreatePluginStaticCredentials,
				list:   p.pluginStaticCredentials.GetAllPluginStaticCredentials,
				update: func(ctx context.Context, cred types.PluginStaticCredentials) error {
					_, err := p.pluginStaticCredentials.UpdatePluginStaticCredentials(ctx, cred)
					return err
				},
				deleteAll: p.pluginStaticCredentials.DeleteAllPluginStaticCredentials,
				cacheList: func(ctx context.Context) ([]types.PluginStaticCredentials, error) {
					var out []types.PluginStaticCredentials
					for cred := range p.cache.collections.pluginStaticCredentials.store.resources(pluginStaticCredentialsNameIndex, "", "") {
						out = append(out, cred.Clone())
					}
					return out, nil
				},
				cacheGet: cacheGet.fn,
				changeResource: func(cred types.PluginStaticCredentials) {
					// types.PluginStaticCredentials does not support Expires. Let's
					// use labels.
					labels := cred.GetStaticLabels()
					labels["now"] = time.Now().String()
					cred.SetStaticLabels(labels)
				},
			})
		})
	}
}
