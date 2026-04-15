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
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// GetCertAuthorityOverride reads a CA override resource by ID.
func (c *Cache) GetCertAuthorityOverride(
	ctx context.Context,
	id types.CertAuthorityOverrideID,
) (*subcav1.CertAuthorityOverride, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthorityOverride")
	defer span.End()

	getter := genericGetter[*subcav1.CertAuthorityOverride, certAuthorityOverrideIndex]{
		cache:      c,
		collection: c.collections.certAuthorityOverrides,
		index:      certAuthorityOverrideFullNameIndex,
		upstreamGet: func(ctx context.Context, fullName string) (*subcav1.CertAuthorityOverride, error) {
			id, err := parseCAOverrideFullName(fullName)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			resource, err := c.SubCAService.GetCertAuthorityOverride(ctx, *id)
			return resource, trace.Wrap(err)
		},
	}

	out, err := getter.get(ctx, id.FullName())
	return out, trace.Wrap(err)
}

// ListCertAuthorityOverrides lists all CA overrides.
func (c *Cache) ListCertAuthorityOverrides(ctx context.Context, pageSize int, pageToken string) (_ []*subcav1.CertAuthorityOverride, nextPageToken string, _ error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListCertAuthorityOverrides")
	defer span.End()

	lister := genericLister[*subcav1.CertAuthorityOverride, certAuthorityOverrideIndex]{
		cache:      c,
		collection: c.collections.certAuthorityOverrides,
		index:      certAuthorityOverrideFullNameIndex,
		upstreamList: func(ctx context.Context, pageSize int, pageToken string) ([]*subcav1.CertAuthorityOverride, string, error) {
			out, next, err := c.SubCAService.ListCertAuthorityOverrides(ctx, pageSize, pageToken)
			return out, next, trace.Wrap(err)
		},
		nextToken: caOverrideFullName,
	}

	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

type certAuthorityOverrideIndex string

const (
	// certAuthorityOverrideFullNameIndex indexes by sub_kind+name.
	certAuthorityOverrideFullNameIndex certAuthorityOverrideIndex = "full_name"
)

func newCertAuthorityOverrideCollection(
	upstream services.SubCAServiceGetter,
	watchKind types.WatchKind,
) (*collection[*subcav1.CertAuthorityOverride, certAuthorityOverrideIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter SubCAService")
	}

	return &collection[*subcav1.CertAuthorityOverride, certAuthorityOverrideIndex]{
		store: newStore(
			types.KindCertAuthorityOverride,
			proto.CloneOf[*subcav1.CertAuthorityOverride],
			map[certAuthorityOverrideIndex]func(*subcav1.CertAuthorityOverride) string{
				certAuthorityOverrideFullNameIndex: caOverrideFullName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*subcav1.CertAuthorityOverride, error) {
			out, err := stream.Collect(clientutils.Resources(
				ctx,
				func(ctx context.Context, pageSize int, pageToken string) ([]*subcav1.CertAuthorityOverride, string, error) {
					return upstream.ListCertAuthorityOverrides(ctx, pageSize, pageToken)
				}))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *subcav1.CertAuthorityOverride {
			return &subcav1.CertAuthorityOverride{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				SubKind: hdr.SubKind,
				Metadata: &headerv1.Metadata{
					Name:        hdr.Metadata.Name,
					Description: hdr.Metadata.Description,
				},
			}
		},
		watch: watchKind,
	}, nil
}

func caOverrideFullName(r *subcav1.CertAuthorityOverride) string {
	id := types.CertAuthorityOverrideID{
		ClusterName: r.GetMetadata().GetName(),
		CAType:      r.GetSubKind(),
	}
	return id.FullName()
}

func parseCAOverrideFullName(fullName string) (*types.CertAuthorityOverrideID, error) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return nil, trace.BadParameter("invalid CA override identifier: %q", fullName)
	}
	return &types.CertAuthorityOverrideID{
		CAType:      parts[0],
		ClusterName: parts[1],
	}, nil
}
