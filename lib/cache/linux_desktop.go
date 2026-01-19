// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	linuxdesktopv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/linuxdesktop/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type linuxDesktopIndex string

const linuxDesktopNameIndex linuxDesktopIndex = "linux-desktop"

func newLinuxDesktopCollection(upstream services.LinuxDesktops, w types.WatchKind) (*collection[*linuxdesktopv1.LinuxDesktop, linuxDesktopIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter LinuxDesktop")
	}

	s := newStore(
		types.KindLinuxDesktop,
		proto.CloneOf[*linuxdesktopv1.LinuxDesktop],
		map[linuxDesktopIndex]func(*linuxdesktopv1.LinuxDesktop) string{
			linuxDesktopNameIndex: func(r *linuxdesktopv1.LinuxDesktop) string {
				return r.GetMetadata().GetName()
			},
		})
	return &collection[*linuxdesktopv1.LinuxDesktop, linuxDesktopIndex]{
		store: s,
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*linuxdesktopv1.LinuxDesktop, error) {
			var out []*linuxdesktopv1.LinuxDesktop
			var startKey string
			for {
				page, nextKey, err := upstream.ListLinuxDesktops(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				out = append(out, page...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}

			return out, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) *linuxdesktopv1.LinuxDesktop {
			return &linuxdesktopv1.LinuxDesktop{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: &headerv1.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// ListLinuxDesktops lists linuxdesktops with pagination.
func (c *Cache) ListLinuxDesktops(ctx context.Context, pageSize int, nextToken string) ([]*linuxdesktopv1.LinuxDesktop, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListLinuxDesktops")
	defer span.End()

	lister := genericLister[*linuxdesktopv1.LinuxDesktop, linuxDesktopIndex]{
		cache:        c,
		collection:   c.collections.linuxDesktops,
		index:        linuxDesktopNameIndex,
		upstreamList: c.Config.LinuxDesktops.ListLinuxDesktops,
		nextToken: func(t *linuxdesktopv1.LinuxDesktop) string {
			return t.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, pageSize, nextToken)
	return out, next, trace.Wrap(err)
}

// GetLinuxDesktop fetches a linuxdesktop by name.
func (c *Cache) GetLinuxDesktop(ctx context.Context, name string) (*linuxdesktopv1.LinuxDesktop, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetLinuxDesktop")
	defer span.End()

	getter := genericGetter[*linuxdesktopv1.LinuxDesktop, linuxDesktopIndex]{
		cache:       c,
		collection:  c.collections.linuxDesktops,
		index:       linuxDesktopNameIndex,
		upstreamGet: c.Config.LinuxDesktops.GetLinuxDesktop,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
