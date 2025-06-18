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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type installerIndex string

const installerNameIndex installerIndex = "name"

func newInstallerCollection(upstream services.ClusterConfiguration, w types.WatchKind) (*collection[types.Installer, installerIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter ClusterConfiguration")
	}

	return &collection[types.Installer, installerIndex]{
		store: newStore(
			types.Installer.Clone,
			map[installerIndex]func(types.Installer) string{
				installerNameIndex: types.Installer.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Installer, error) {
			installers, err := upstream.GetInstallers(ctx)
			return installers, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.Installer {
			return &types.InstallerV1{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetInstaller gets the installer script resource for the cluster
func (c *Cache) GetInstaller(ctx context.Context, name string) (types.Installer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInstaller")
	defer span.End()

	getter := genericGetter[types.Installer, installerIndex]{
		cache:       c,
		collection:  c.collections.installers,
		index:       installerNameIndex,
		upstreamGet: c.Config.ClusterConfig.GetInstaller,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetInstallers gets all the installer script resources for the cluster
func (c *Cache) GetInstallers(ctx context.Context) ([]types.Installer, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetInstallers")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.installers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if !rg.ReadCache() {
		users, err := c.Config.ClusterConfig.GetInstallers(ctx)
		return users, trace.Wrap(err)
	}

	installers := make([]types.Installer, 0, rg.store.len())
	for i := range rg.store.resources(installerNameIndex, "", "") {
		installers = append(installers, i.Clone())
	}

	return installers, nil
}
