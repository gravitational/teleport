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
		return nil, trace.BadParameter("missing parameter SAMLIdPSession")
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

type samlIdPSessionIndex string

const samlIdPSessionNameIndex samlIdPSessionIndex = "name"

func newSAMLIdPSessionCollection(upstream services.SAMLIdPSession, w types.WatchKind) (*collection[types.WebSession, samlIdPSessionIndex], error) {
	if upstream == nil {
		return nil, trace.BadParameter("missing parameter SAMLIdPSession")
	}

	return &collection[types.WebSession, samlIdPSessionIndex]{
		store: newStore(
			types.WebSession.Copy,
			map[samlIdPSessionIndex]func(types.WebSession) string{
				samlIdPSessionNameIndex: types.WebSession.GetName,
			}),
		fetcher: func(ctx context.Context, loadSecrets bool) ([]types.WebSession, error) {
			var startKey string
			var sessions []types.WebSession
			for {
				webSessions, nextKey, err := upstream.ListSAMLIdPSessions(ctx, 0, startKey, "")
				if err != nil {
					return nil, trace.Wrap(err)
				}

				if !loadSecrets {
					for i := 0; i < len(webSessions); i++ {
						webSessions[i] = webSessions[i].WithoutSecrets()
					}
				}

				sessions = append(sessions, webSessions...)

				if nextKey == "" {
					break
				}
				startKey = nextKey
			}
			return sessions, nil
		},
		headerTransform: func(hdr *types.ResourceHeader) types.WebSession {
			return &types.WebSessionV2{
				Kind:    hdr.Kind,
				SubKind: hdr.SubKind,
				Version: hdr.Version,
				Metadata: types.Metadata{
					Name: hdr.Metadata.Name,
				},
			}
		},
		watch: w,
	}, nil
}

// GetSAMLIdPSession gets a SAML IdP session.
func (c *Cache) GetSAMLIdPSession(ctx context.Context, req types.GetSAMLIdPSessionRequest) (types.WebSession, error) {
	ctx, span := c.Tracer.Start(ctx, "cache/GetSAMLIdPSession")
	defer span.End()

	var upstreamRead bool
	getter := genericGetter[types.WebSession, samlIdPSessionIndex]{
		cache:      c,
		collection: c.collections.samlIdPSessions,
		index:      samlIdPSessionNameIndex,
		upstreamGet: func(ctx context.Context, s string) (types.WebSession, error) {
			upstreamRead = true

			session, err := c.Config.SAMLIdPSession.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest{SessionID: s})
			return session, trace.Wrap(err)
		},
	}
	out, err := getter.get(ctx, req.SessionID)
	if trace.IsNotFound(err) && !upstreamRead {
		// fallback is sane because method is never used
		// in construction of derivative caches.
		if item, err := c.Config.SAMLIdPSession.GetSAMLIdPSession(ctx, req); err == nil {
			return item, nil
		}
	}
	return out, trace.Wrap(err)
}
