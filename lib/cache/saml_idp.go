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
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/cache/internal"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type samlIdPServiceProviderIndex string

const samlIdPServiceProviderNameIndex samlIdPServiceProviderIndex = "name"

func newSAMLIdPServiceProviderCollection(upstream services.SAMLIdPServiceProviders, w types.WatchKind) (*collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter SAMLIdPServiceProviders")
	}

	return &collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		store: newStore(
			types.KindSAMLIdPServiceProvider,
			types.SAMLIdPServiceProvider.Copy,
			map[samlIdPServiceProviderIndex]func(types.SAMLIdPServiceProvider) string{
				samlIdPServiceProviderNameIndex: types.SAMLIdPServiceProvider.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.SAMLIdPServiceProvider, error) {
			out, err := stream.Collect(clientutils.Resources(ctx, upstream.ListSAMLIdPServiceProviders))
			return out, trace.Wrap(err)
		},
		headerTransform: func(hdr *types.ResourceHeader) types.SAMLIdPServiceProvider {
			return &types.SAMLIdPServiceProviderV1{
				ResourceHeader: types.ResourceHeader{
					Kind:    hdr.Kind,
					Version: hdr.Version,
					Metadata: types.Metadata{
						Name: hdr.Metadata.Name,
					},
				},
			}
		},
		watch: w,
	}, nil
}

// samlIdPServiceProviderCollection provides read access to cached SAML IdP
// service providers. Its exported methods are promoted onto every topology
// cache that embeds it; the reads are implemented exactly once here. It is a
// stateless value assembled inline by each of its consumers so that no
// shared scaffolding couples their lifetimes.
type samlIdPServiceProviderCollection struct {
	engine   *internal.Engine
	tracer   oteltrace.Tracer
	upstream services.SAMLIdPServiceProviders
	col      *collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (c samlIdPServiceProviderCollection) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	ctx, span := c.tracer.Start(ctx, "cache/ListSAMLIdPServiceProviders")
	defer span.End()

	lister := genericLister[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		engine:          c.engine,
		collection:      c.col,
		index:           samlIdPServiceProviderNameIndex,
		defaultPageSize: 200,
		upstreamList:    c.upstream.ListSAMLIdPServiceProviders,
		nextToken: func(t types.SAMLIdPServiceProvider) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (c *Cache) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	return samlIdPServiceProviderCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.SAMLIdPServiceProviders,
		col:      c.collections.samlIdPServiceProviders,
	}.ListSAMLIdPServiceProviders(ctx, pageSize, pageToken)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (c samlIdPServiceProviderCollection) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	ctx, span := c.tracer.Start(ctx, "cache/GetSAMLIdPServiceProvider")
	defer span.End()

	getter := genericGetter[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		engine:      c.engine,
		collection:  c.col,
		index:       samlIdPServiceProviderNameIndex,
		upstreamGet: c.upstream.GetSAMLIdPServiceProvider,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (c *Cache) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	return samlIdPServiceProviderCollection{
		engine:   c.engine,
		tracer:   c.Tracer,
		upstream: c.Config.SAMLIdPServiceProviders,
		col:      c.collections.samlIdPServiceProviders,
	}.GetSAMLIdPServiceProvider(ctx, name)
}
