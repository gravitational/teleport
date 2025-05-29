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

type samlIdPServiceProviderIndex string

const samlIdPServiceProviderNameIndex samlIdPServiceProviderIndex = "name"

func newSAMLIdPServiceProviderCollection(upstream services.SAMLIdPServiceProviders, w types.WatchKind) (*collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter SAMLIdPServiceProviders")
	}

	return &collection[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		store: newStore(
			types.SAMLIdPServiceProvider.Copy,
			map[samlIdPServiceProviderIndex]func(types.SAMLIdPServiceProvider) string{
				samlIdPServiceProviderNameIndex: types.SAMLIdPServiceProvider.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.SAMLIdPServiceProvider, error) {
			var startKey string
			var sps []types.SAMLIdPServiceProvider
			for {
				var samlProviders []types.SAMLIdPServiceProvider
				var err error
				samlProviders, startKey, err = upstream.ListSAMLIdPServiceProviders(ctx, 0, startKey)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				sps = append(sps, samlProviders...)

				if startKey == "" {
					break
				}
			}

			return sps, nil
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

// ListSAMLIdPServiceProviders returns a paginated list of SAML IdP service provider resources.
func (c *Cache) ListSAMLIdPServiceProviders(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/ListSAMLIdPServiceProviders")
	defer span.End()

	lister := genericLister[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		cache:           c,
		collection:      c.collections.samlIdPServiceProviders,
		index:           samlIdPServiceProviderNameIndex,
		defaultPageSize: 200,
		upstreamList:    c.Config.SAMLIdPServiceProviders.ListSAMLIdPServiceProviders,
		nextToken: func(t types.SAMLIdPServiceProvider) string {
			return t.GetMetadata().Name
		},
	}
	out, next, err := lister.list(ctx, pageSize, pageToken)
	return out, next, trace.Wrap(err)
}

// GetSAMLIdPServiceProvider returns the specified SAML IdP service provider resources.
func (c *Cache) GetSAMLIdPServiceProvider(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSAMLIdPServiceProvider")
	defer span.End()

	getter := genericGetter[types.SAMLIdPServiceProvider, samlIdPServiceProviderIndex]{
		cache:       c,
		collection:  c.collections.samlIdPServiceProviders,
		index:       samlIdPServiceProviderNameIndex,
		upstreamGet: c.Config.SAMLIdPServiceProviders.GetSAMLIdPServiceProvider,
	}
	out, err := getter.get(ctx, name)
	return out, trace.Wrap(err)
}
