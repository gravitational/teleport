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
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/sortcache"
)

type certAuthorityIndex string

const certAuthorityIDIndex certAuthorityIndex = "id"

func newCertAuthorityCollection(t services.Trust, w types.WatchKind) (*collection[types.CertAuthority, certAuthorityIndex], error) {
	if t == nil {
		return nil, trace.BadParameter("missing parameter Trust")
	}

	var filter types.CertAuthorityFilter
	filter.FromMap(w.Filter)

	return &collection[types.CertAuthority, certAuthorityIndex]{
		store: newStore(
			types.CertAuthority.Clone,
			map[certAuthorityIndex]func(types.CertAuthority) string{
				certAuthorityIDIndex: func(ca types.CertAuthority) string {
					return string(ca.GetType()) + "/" + ca.GetID().DomainName
				},
			}),
		watch:  w,
		filter: filter.Match,
		headerTransform: func(hdr *types.ResourceHeader) types.CertAuthority {
			return &types.CertAuthorityV2{
				Kind:    hdr.Kind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
				Spec: types.CertAuthoritySpecV2{
					Type: types.CertAuthType(hdr.SubKind),
				},
			}
		},
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.CertAuthority, error) {
			var authorities []types.CertAuthority
			for _, caType := range types.CertAuthTypes {
				cas, err := t.GetCertAuthorities(ctx, caType, loadSecrets)
				// if caType was added in this major version we might get a BadParameter
				// error if we're connecting to an older upstream that doesn't know about it
				if err != nil {
					if !types.IsUnsupportedAuthorityErr(err) || !caType.NewlyAdded() {
						return nil, trace.Wrap(err)
					}
					continue
				}

				// this can be removed once we get the ability to fetch CAs with a filter,
				// but it should be harmless, and it could be kept as additional safety
				if !filter.IsEmpty() {
					filtered := cas[:0]
					for _, ca := range cas {
						if filter.Match(ca) {
							filtered = append(filtered, ca)
						}
					}
					cas = filtered
				}

				authorities = append(authorities, cas...)
			}

			return authorities, nil
		},
	}, nil
}

type getCertAuthorityCacheKey struct {
	id types.CertAuthID
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Cache) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthority")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.certAuthorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		ca, err := rg.store.get(certAuthorityIDIndex, string(id.Type)+"/"+id.DomainName)
		if err != nil {
			// release read lock early
			rg.Release()

			// fallback is sane because method is never used
			// in construction of derivative caches.
			if trace.IsNotFound(err) {
				if ca, err := c.Config.Trust.GetCertAuthority(ctx, id, loadSigningKeys); err == nil {
					return ca, nil
				}
			}

			return nil, trace.Wrap(err)
		}

		if !loadSigningKeys {
			return ca.WithoutSecrets().(types.CertAuthority), nil
		}

		return ca.Clone(), nil
	}

	// When signing keys are requested, always read from the upstream.
	if loadSigningKeys {
		ca, err := c.Config.Trust.GetCertAuthority(ctx, id, loadSigningKeys)
		return ca, err
	}

	// When no keys are requested, use the ca cache to reduce the upstream load.
	cachedCA, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthorityCacheKey{id}, func(ctx context.Context) (types.CertAuthority, error) {
		ca, err := c.Config.Trust.GetCertAuthority(ctx, id, loadSigningKeys)
		return ca, err
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cachedCA.Clone(), nil
}

type getCertAuthoritiesCacheKey struct {
	caType types.CertAuthType
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (c *Cache) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadSigningKeys bool) ([]types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthorities")
	defer span.End()

	rg, err := acquireReadGuard(c, c.collections.certAuthorities)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		cas := make([]types.CertAuthority, 0, rg.store.len())
		for ca := range rg.store.resources(certAuthorityIDIndex, string(caType), sortcache.NextKey(string(caType))) {
			if loadSigningKeys {
				cas = append(cas, ca.Clone())
			} else {
				cas = append(cas, ca.WithoutSecrets().(types.CertAuthority))
			}
		}

		return cas, nil
	}

	// When signing keys are requested, always read from the upstream.
	if loadSigningKeys {
		cas, err := c.Config.Trust.GetCertAuthorities(ctx, caType, loadSigningKeys)
		return cas, err
	}

	// When no keys are requested, use the ca cache to reduce the upstream load.
	cachedCAs, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthoritiesCacheKey{caType}, func(ctx context.Context) ([]types.CertAuthority, error) {
		cas, err := c.Config.Trust.GetCertAuthorities(ctx, caType, loadSigningKeys)
		return cas, trace.Wrap(err)
	})
	if err != nil || cachedCAs == nil {
		return nil, trace.Wrap(err)
	}
	cas := make([]types.CertAuthority, 0, len(cachedCAs))
	for _, ca := range cachedCAs {
		cas = append(cas, ca.Clone())
	}
	return cas, nil
}
