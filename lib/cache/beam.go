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
	"encoding/base32"
	"iter"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"rsc.io/ordered"

	"github.com/gravitational/teleport/api/defaults"
	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type beamIndex string
type bytestring = string

const (
	beamNameIndex    beamIndex = "name"
	beamAliasIndex   beamIndex = "alias"
	beamUserIndex    beamIndex = "user"
	beamExpiresIndex beamIndex = "expires"
)

func newBeamCollection(upstream services.BeamReader, w types.WatchKind) (*collection[*beamsv1.Beam, beamIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter Beams")
	}

	return &collection[*beamsv1.Beam, beamIndex]{
		store: newStore(
			types.KindBeam,
			proto.CloneOf[*beamsv1.Beam],
			map[beamIndex]func(*beamsv1.Beam) string{
				beamNameIndex:    keyForBeamNameIndex,
				beamAliasIndex:   keyForBeamAliasIndex,
				beamUserIndex:    keyForBeamUserIndex,
				beamExpiresIndex: keyForBeamExpiresIndex,
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*beamsv1.Beam, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error) {
				return upstream.ListBeamsV2(ctx, limit, startKey, nil)
			}))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *beamsv1.Beam {
			return &beamsv1.Beam{
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

// GetBeam returns the specified beam resource.
func (c *Cache) GetBeam(ctx context.Context, name string) (*beamsv1.Beam, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBeam")
	defer span.End()

	getter := genericGetter[*beamsv1.Beam, beamIndex]{
		cache:       c,
		collection:  c.collections.beams,
		index:       beamNameIndex,
		upstreamGet: c.Config.Beams.GetBeam,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetBeamByAlias returns the specified beam resource by alias.
func (c *Cache) GetBeamByAlias(ctx context.Context, alias string) (*beamsv1.Beam, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetBeamByAlias")
	defer span.End()

	getter := genericGetter[*beamsv1.Beam, beamIndex]{
		cache:       c,
		collection:  c.collections.beams,
		index:       beamAliasIndex,
		upstreamGet: c.Config.Beams.GetBeamByAlias,
	}
	out, err := getter.get(ctx, alias)
	return out, trace.Wrap(err)
}

// ListBeams lists beams with pagination.
func (c *Cache) ListBeams(ctx context.Context, pageSize int, pageToken string) ([]*beamsv1.Beam, string, error) {
	return c.ListBeamsV2(ctx, pageSize, pageToken, nil)
}

// ListBeamsV2 lists beams with pagination, sorting and filtering.
func (c *Cache) ListBeamsV2(ctx context.Context, pageSize int, pageToken string, options *services.ListBeamsRequestOptions) ([]*beamsv1.Beam, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListBeams")
	defer span.End()

	if pageSize <= 0 {
		pageSize = defaults.DefaultChunkSize
	}

	index, keyFn, encodeFn, decodeFn := beamIndexForSortField(options.GetSortField())
	isDesc := options.GetSortOrder() == beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING

	// Decode the PageToken
	if decodeFn != nil {
		var err error
		pageToken, err = decodeFn(pageToken)
		if err != nil {
			return nil, "", trace.BadParameter("invalid page token: %v", err)
		}
	}

	lister := genericLister[*beamsv1.Beam, beamIndex]{
		cache:      c,
		collection: c.collections.beams,
		index:      index,
		isDesc:     isDesc,
		upstreamList: func(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error) {
			return c.Config.Beams.ListBeamsV2(ctx, limit, startKey, options)
		},
		filter:    services.MakeBeamFilterFunc(options),
		nextToken: keyFn,
	}

	out, next, err := lister.list(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	// Encode the next page token
	if encodeFn != nil {
		next = encodeFn(next)
	}

	return out, next, nil
}

// IterateBeams returns a sequence of beams starting from the given pageToken.
func (c *Cache) IterateBeams(ctx context.Context, pageToken string) iter.Seq2[*beamsv1.Beam, error] {
	return c.IterateBeamsV2(ctx, pageToken, nil)
}

// IterateBeamsV2 returns a sequence of beams starting from the given pageToken
// with sorting and filtering.
func (c *Cache) IterateBeamsV2(ctx context.Context, pageToken string, options *services.ListBeamsRequestOptions) iter.Seq2[*beamsv1.Beam, error] {
	index, keyFn, _, _ := beamIndexForSortField(options.GetSortField())
	isDesc := options.GetSortOrder() == beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING

	lister := genericLister[*beamsv1.Beam, beamIndex]{
		cache:      c,
		collection: c.collections.beams,
		index:      index,
		isDesc:     isDesc,
		upstreamList: func(ctx context.Context, limit int, startKey string) ([]*beamsv1.Beam, string, error) {
			return c.Config.Beams.ListBeamsV2(ctx, limit, startKey, options)
		},
		filter:    services.MakeBeamFilterFunc(options),
		nextToken: keyFn,
	}
	return lister.Range(ctx, pageToken, "")
}

// keyForBeamNameIndex should return a key which matches the backend's storage
// keys to allow consistent paging
func keyForBeamNameIndex(beam *beamsv1.Beam) string {
	return beam.GetMetadata().GetName()
}

func keyForBeamAliasIndex(beam *beamsv1.Beam) string {
	return beam.GetStatus().GetAlias()
}

func keyForBeamUserIndex(r *beamsv1.Beam) bytestring {
	user := r.GetStatus().GetUser()
	name := r.GetMetadata().GetName()
	return string(ordered.Encode(user, name))
}

func keyForBeamExpiresIndex(r *beamsv1.Beam) bytestring {
	expires := r.GetSpec().GetExpires()
	name := r.GetMetadata().GetName()
	if expires == nil {
		return string(ordered.Encode(ordered.Inf, name))
	}
	return string(ordered.Encode(expires.AsTime().UnixMilli(), name))
}

func beamIndexForSortField(sortField beamsv1.BeamSortField) (beamIndex, func(*beamsv1.Beam) string, func(raw string) (encoded string), func(encoded string) (string, error)) {
	switch sortField {
	case beamsv1.BeamSortField_BEAM_SORT_FIELD_ALIAS:
		return beamAliasIndex, keyForBeamAliasIndex, nil, nil // No encoding/decoding required for string key
	case beamsv1.BeamSortField_BEAM_SORT_FIELD_USER:
		return beamUserIndex, keyForBeamUserIndex, base32Encode, base32Decode
	case beamsv1.BeamSortField_BEAM_SORT_FIELD_EXPIRES:
		return beamExpiresIndex, keyForBeamExpiresIndex, base32Encode, base32Decode
	default:
		return beamNameIndex, keyForBeamNameIndex, nil, nil // No encoding/decoding to match backend keys
	}
}

// base32Encode encodes the input in base32hex format.
func base32Encode(raw string) string {
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(raw))
}

// base32Decode secode the input from base32hex format.
func base32Decode(encoded string) (string, error) {
	decoded, err := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(encoded)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(decoded), nil
}
