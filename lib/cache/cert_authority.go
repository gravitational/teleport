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

func newCertAuthorityCollection(t services.Trust, w types.WatchKind) (*collection[types.CertAuthority, *resourceStore[types.CertAuthority], *caUpstream], error) {
	if t == nil {
		return nil, trace.BadParameter("missing parameter Trust")
	}

	var filter types.CertAuthorityFilter
	filter.FromMap(w.Filter)

	return &collection[types.CertAuthority, *resourceStore[types.CertAuthority], *caUpstream]{
		store: newResourceStoreWithFilter(
			filter.Match,
			map[string]func(types.CertAuthority) string{
				"id": func(ca types.CertAuthority) string {
					return string(ca.GetType()) + "/" + ca.GetID().DomainName
				},
			},
		),
		upstream: &caUpstream{Trust: t, filter: filter},
		watch:    w,
	}, nil
}

type caUpstream struct {
	services.Trust
	// extracted from watch.Filter, to avoid rebuilding on every event
	filter types.CertAuthorityFilter
}

func (c caUpstream) getAll(ctx context.Context, loadSecrets bool) ([]types.CertAuthority, error) {
	var authorities []types.CertAuthority
	for _, caType := range types.CertAuthTypes {
		cas, err := c.Trust.GetCertAuthorities(ctx, caType, loadSecrets)
		// if caType was added in this major version we might get a BadParameter
		// error if we're connecting to an older upstream that doesn't know about it
		if err != nil {
			if !(types.IsUnsupportedAuthorityErr(err) && caType.NewlyAdded()) {
				return nil, trace.Wrap(err)
			}
			continue
		}

		// this can be removed once we get the ability to fetch CAs with a filter,
		// but it should be harmless, and it could be kept as additional safety
		if !c.filter.IsEmpty() {
			filtered := cas[:0]
			for _, ca := range cas {
				if c.filter.Match(ca) {
					filtered = append(filtered, ca)
				}
			}
			cas = filtered
		}

		authorities = append(authorities, cas...)
	}

	return authorities, nil
}

type getCertAuthorityCacheKey struct {
	id types.CertAuthID
}

var _ map[getCertAuthorityCacheKey]struct{} // compile-time hashability check

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Cache) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthority")
	defer span.End()

	collection := c.collections.certAuthorities
	rg, err := acquireReadGuard(c, collection.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		ca, err := collection.store.get("id", string(id.Type)+"/"+id.DomainName)
		if err != nil {
			if trace.IsNotFound(err) {
				if ca, err := c.Config.Trust.GetCertAuthority(ctx, id, loadSigningKeys); err == nil {
					return ca, nil
				}
			}

			return nil, trace.Wrap(err)
		}

		if !loadSigningKeys {
			return ca.Clone().WithoutSecrets().(types.CertAuthority), nil
		}

		return ca.Clone(), nil
	}

	// When signing keys are requested, always read from the upstream.
	if loadSigningKeys {
		ca, err := collection.upstream.GetCertAuthority(ctx, id, loadSigningKeys)
		return ca, err
	}

	// When no keys are requested, use the ca cache to reduce the upstream load.
	cachedCA, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthorityCacheKey{id}, func(ctx context.Context) (types.CertAuthority, error) {
		ca, err := collection.upstream.GetCertAuthority(ctx, id, loadSigningKeys)
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

var _ map[getCertAuthoritiesCacheKey]struct{} // compile-time hashability check

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (c *Cache) GetCertAuthorities(ctx context.Context, caType types.CertAuthType, loadSigningKeys bool) ([]types.CertAuthority, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetCertAuthorities")
	defer span.End()

	collection := c.collections.certAuthorities
	rg, err := acquireReadGuard(c, collection.watch)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rg.Release()

	if rg.ReadCache() {
		var cas []types.CertAuthority
		for ca := range collection.store.iterate("id", string(caType), sortcache.NextKey(string(caType))) {
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
		cas, err := collection.upstream.GetCertAuthorities(ctx, caType, loadSigningKeys)
		return cas, err
	}

	// When no keys are requested, use the ca cache to reduce the upstream load.
	cachedCAs, err := utils.FnCacheGet(ctx, c.fnCache, getCertAuthoritiesCacheKey{caType}, func(ctx context.Context) ([]types.CertAuthority, error) {
		cas, err := collection.upstream.GetCertAuthorities(ctx, caType, loadSigningKeys)
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
