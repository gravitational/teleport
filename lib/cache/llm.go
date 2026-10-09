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
	llmv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/llm/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type llmEndpointQuotaIndex string

const llmEndpointQuotaNameIndex llmEndpointQuotaIndex = "name"

func newLLMEndpointQuotaCollection(upstream services.LLMEndpointQuotaGetter, w types.WatchKind) (*collection[*llmv1.LLMEndpointQuota, llmEndpointQuotaIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter endpoint quota getter")
	}

	return &collection[*llmv1.LLMEndpointQuota, llmEndpointQuotaIndex]{
		store: newStore(
			types.KindLLMEndpointQuota,
			proto.CloneOf[*llmv1.LLMEndpointQuota],
			map[llmEndpointQuotaIndex]func(*llmv1.LLMEndpointQuota) string{
				llmEndpointQuotaNameIndex: func(r *llmv1.LLMEndpointQuota) string {
					return r.GetMetadata().GetName()
				},
			},
		),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]*llmv1.LLMEndpointQuota, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error) {
				return upstream.ListLLMEndpointQuotas(ctx, limit, startKey)
			}))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) *llmv1.LLMEndpointQuota {
			return llmv1.LLMEndpointQuota_builder{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: headerv1.Metadata_builder{
					Name: hdr.Metadata.Name,
				}.Build(),
			}.Build()
		},
		watch: w,
	}, nil
}

// GetLLMEndpointQuota returns the specified [LLMEndpointQuota].
func (c *Cache) GetLLMEndpointQuota(ctx context.Context, name string) (*llmv1.LLMEndpointQuota, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetLLMEndpointQuota")
	defer span.End()

	getter := genericGetter[*llmv1.LLMEndpointQuota, llmEndpointQuotaIndex]{
		cache:       c,
		collection:  c.collections.llmEndpointQuotas,
		index:       llmEndpointQuotaNameIndex,
		upstreamGet: c.Config.LLMEndpointQuotaService.GetLLMEndpointQuota,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// ListLLMEndpointQuotas lists LLMEndpointQuota resources.
func (c *Cache) ListLLMEndpointQuotas(ctx context.Context, limit int, startKey string) ([]*llmv1.LLMEndpointQuota, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListLLMEndpointQuotas")
	defer span.End()

	lister := genericLister[*llmv1.LLMEndpointQuota, llmEndpointQuotaIndex]{
		cache:        c,
		collection:   c.collections.llmEndpointQuotas,
		index:        llmEndpointQuotaNameIndex,
		upstreamList: c.Config.LLMEndpointQuotaService.ListLLMEndpointQuotas,
		nextToken: func(quota *llmv1.LLMEndpointQuota) string {
			return quota.GetMetadata().GetName()
		},
	}
	out, next, err := lister.list(ctx, limit, startKey)
	return out, next, trace.Wrap(err)
}
