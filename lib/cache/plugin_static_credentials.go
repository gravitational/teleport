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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type pluginStaticCredentialsIndex string

const pluginStaticCredentialsNameIndex pluginStaticCredentialsIndex = "name"

func newPluginStaticCredentialsCollection(upstream services.PluginStaticCredentials, w types.WatchKind) (*collection[types.PluginStaticCredentials, pluginStaticCredentialsIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter PluginStaticCredentials")
	}

	return &collection[types.PluginStaticCredentials, pluginStaticCredentialsIndex]{
		store: newStore(
			types.PluginStaticCredentials.Clone,
			map[pluginStaticCredentialsIndex]func(types.PluginStaticCredentials) string{
				pluginStaticCredentialsNameIndex: types.PluginStaticCredentials.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.PluginStaticCredentials, error) {
			creds, err := upstream.GetAllPluginStaticCredentials(ctx)
			return creds, trace.Wrap(err)

		},
		headerTransform: func(hdr *types.ResourceHeader) types.PluginStaticCredentials {
			return &types.PluginStaticCredentialsV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name:        hdr.Metadata.Name,
						Description: hdr.Metadata.Description,
					},
				},
			}
		},
		watch: w,
	}, nil
}

func (c *Cache) GetPluginStaticCredentials(ctx context.Context, name string) (types.PluginStaticCredentials, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPluginStaticCredentials")
	defer span.End()

	getter := genericGetter[types.PluginStaticCredentials, pluginStaticCredentialsIndex]{
		cache:       c,
		collection:  c.collections.pluginStaticCredentials,
		index:       pluginStaticCredentialsNameIndex,
		upstreamGet: c.Config.PluginStaticCredentials.GetPluginStaticCredentials,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

func (c *Cache) GetPluginStaticCredentialsByLabels(ctx context.Context, labels map[string]string) ([]types.PluginStaticCredentials, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetPluginStaticCredentialsByLabels")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.pluginStaticCredentials)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		resp, err := c.Config.PluginStaticCredentials.GetPluginStaticCredentialsByLabels(ctx, labels)
		return resp, trace.Wrap(err)
	}

	var out []types.PluginStaticCredentials
	for cred := range rg.store.resources(pluginStaticCredentialsNameIndex, "", "") {
		if types.MatchLabels(cred, labels) {
			out = append(out, cred.Clone())
		}
	}
	return out, nil
}
