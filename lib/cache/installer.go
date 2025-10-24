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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
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
			types.KindInstaller,
			types.Installer.Clone,
			map[installerIndex]func(types.Installer) string{
				installerNameIndex: types.Installer.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.Installer, error) {
			// TODO(lokraszewski): DELETE IN v21.0.0 replace by regular clientutils.Resources
			out, err := clientutils.CollectWithFallback(ctx, upstream.ListInstallers, upstream.GetInstallers)
			return out, trace.Wrap(err)
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

// ListInstallers returns a page of installer script resources.
func (c *Cache) ListInstallers(ctx context.Context, limit int, start string) ([]types.Installer, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListInstallers")
	defer span.End()

	lister := genericLister[types.Installer, installerIndex]{
		cache:        c,
		collection:   c.collections.installers,
		index:        installerNameIndex,
		upstreamList: c.Config.ClusterConfig.ListInstallers,
		nextToken:    types.Installer.GetName,
	}
	out, next, err := lister.list(ctx, limit, start)
	return out, next, trace.Wrap(err)
}

// RangeInstallers returns installer script resources within the range [start, end).
func (c *Cache) RangeInstallers(ctx context.Context, start, end string) iter.Seq2[types.Installer, error] {
	lister := genericLister[types.Installer, installerIndex]{
		cache:        c,
		collection:   c.collections.installers,
		index:        installerNameIndex,
		upstreamList: c.Config.ClusterConfig.ListInstallers,
		nextToken:    types.Installer.GetName,
		// TODO(lokraszewski): DELETE IN v21.0.0
		fallbackGetter: c.Config.ClusterConfig.GetInstallers,
	}

	return func(yield func(types.Installer, error) bool) {
		ctx, span := c.Tracer.Start(ctx, "cache/RangeInstallers")
		defer span.End()

		for inst, err := range lister.RangeWithFallback(ctx, start, end) {
			if !yield(inst, err) {
				return
			}

			if err != nil {
				return
			}
		}
	}
}
