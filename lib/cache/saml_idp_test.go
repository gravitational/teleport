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
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestSAMLIdPServiceProviders tests that CRUD operations on SAML IdP service provider resources are
// replicated from the backend to the cache.
func TestSAMLIdPServiceProviders(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.SAMLIdPServiceProvider]{
		newResource: func(name string) (types.SAMLIdPServiceProvider, error) {
			return types.NewSAMLIdPServiceProvider(
				types.Metadata{
					Name: name,
				},
				types.SAMLIdPServiceProviderSpecV1{
					EntityDescriptor: testEntityDescriptor,
					EntityID:         "IAMShowcase",
				})
		},
		create: p.samlIDPServiceProviders.CreateSAMLIdPServiceProvider,
		list: func(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
			results, _, err := p.samlIDPServiceProviders.ListSAMLIdPServiceProviders(ctx, 0, "")
			return results, err
		},
		cacheGet: p.cache.GetSAMLIdPServiceProvider,
		cacheList: func(ctx context.Context) ([]types.SAMLIdPServiceProvider, error) {
			results, _, err := p.cache.ListSAMLIdPServiceProviders(ctx, 0, "")
			return results, err
		},
		update:    p.samlIDPServiceProviders.UpdateSAMLIdPServiceProvider,
		deleteAll: p.samlIDPServiceProviders.DeleteAllSAMLIdPServiceProviders,
	})
}

func TestSAMLIdPSessions(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	for i := 0; i < 31; i++ {
		err := p.samlIdPSessionsS.UpsertSAMLIdPSession(t.Context(), &types.WebSessionV2{
			Kind:    types.KindWebSession,
			SubKind: types.KindSAMLIdPSession,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "saml-session" + strconv.Itoa(i+1),
			},
			Spec: types.WebSessionSpecV2{
				User: "fish",
			},
		})
		require.NoError(t, err)
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for i := 0; i < 31; i++ {
			session, err := p.cache.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest{SessionID: "saml-session" + strconv.Itoa(i+1)})
			assert.NoError(t, err)
			assert.NotNil(t, session)
		}
	}, 15*time.Second, 100*time.Millisecond)

	require.NoError(t, p.samlIdPSessionsS.DeleteAllSAMLIdPSessions(ctx))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		for i := 0; i < 31; i++ {
			session, err := p.cache.GetSAMLIdPSession(ctx, types.GetSAMLIdPSessionRequest{SessionID: "saml-session" + strconv.Itoa(i+1)})
			assert.Error(t, err)
			assert.Nil(t, session)
		}
	}, 15*time.Second, 100*time.Millisecond)
}
