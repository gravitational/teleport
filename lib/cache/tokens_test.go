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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

// TestTokens tests static tokens
func TestStaticTokens(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	// Make sure we get a NotFoundError (and not a panic) when there are no
	// static tokens.
	_, err := p.cache.GetStaticTokens(ctx)
	require.ErrorAs(t, err, new(*trace.NotFoundError))

	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Token:   "static1",
				Roles:   types.SystemRoles{types.RoleAuth, types.RoleNode},
				Expires: time.Now().UTC().Add(time.Hour),
			},
		},
	})
	require.NoError(t, err)

	err = p.clusterConfigS.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	out, err := p.cache.GetStaticTokens(ctx)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(staticTokens, out, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
}

// TestDynamicTokens tests the dynamic tokens cache.
func TestDynamicTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newPackForAuth(t)
	t.Cleanup(p.Close)

	expires := time.Now().Add(10 * time.Hour).Truncate(time.Second).UTC()
	token, err := types.NewProvisionToken("token", types.SystemRoles{types.RoleAuth, types.RoleNode}, expires)
	require.NoError(t, err)

	err = p.provisionerS.UpsertToken(ctx, token)
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	tout, err := p.cache.GetToken(ctx, token.GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(token, tout, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))

	err = p.provisionerS.DeleteToken(ctx, token.GetName())
	require.NoError(t, err)

	select {
	case event := <-p.eventsC:
		require.Equal(t, EventProcessed, event.Type)
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for event")
	}

	_, err = p.cache.GetToken(ctx, token.GetName())
	require.True(t, trace.IsNotFound(err))
}

// TestTokensCache tests that CRUD operations on tokens resources are
// replicated from the backend to the cache.
func TestTokensCache(t *testing.T) {
	t.Parallel()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	testResources(t, p, testFuncs[types.ProvisionToken]{
		newResource: func(key string) (types.ProvisionToken, error) {
			return &types.ProvisionTokenV2{
				Metadata: types.Metadata{
					Name: key,
				},
				Spec: types.ProvisionTokenSpecV2{
					Roles: []types.SystemRole{
						types.RoleAdmin,
					},
				},
			}, nil
		},
		cacheGet: func(ctx context.Context, key string) (types.ProvisionToken, error) {
			return p.cache.GetToken(ctx, key)
		},
		cacheList: func(ctx context.Context, pageSize int, pageToken string) ([]types.ProvisionToken, string, error) {
			return p.cache.ListProvisionTokens(ctx, pageSize, pageToken, nil, "")
		},
		create: func(ctx context.Context, resource types.ProvisionToken) error {
			err := p.provisionerS.CreateToken(ctx, resource)
			return err
		},
		list: func(ctx context.Context, pageSize int, pageToken string) ([]types.ProvisionToken, string, error) {
			return p.provisionerS.ListProvisionTokens(ctx, pageSize, pageToken, nil, "")
		},
		update: func(ctx context.Context, t types.ProvisionToken) error {
			err := p.provisionerS.UpsertToken(ctx, t)
			return err
		},
		deleteAll: func(ctx context.Context) error {
			return p.provisionerS.DeleteAllTokens()
		},
	})
}

// TestTokensCacheFilters tests that cache items are filtered.
func TestTokensCacheFilters(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := newTestPack(t, ForAuth)
	t.Cleanup(p.Close)

	tokens := []struct {
		roles   types.SystemRoles
		botName string
	}{
		{
			roles:   types.SystemRoles{types.RoleAdmin},
			botName: "",
		},
		{
			roles:   types.SystemRoles{types.RoleAdmin, types.RoleNode, types.RoleBot},
			botName: "bot-1",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-2",
		},
		{
			roles:   types.SystemRoles{types.RoleBot},
			botName: "bot-1",
		},
	}

	for i, token := range tokens {
		err := p.provisionerS.CreateToken(ctx, &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: "test-token-" + strconv.Itoa(i+1),
			},
			Spec: types.ProvisionTokenSpecV2{
				Roles:   token.roles,
				BotName: token.botName,
			},
		})
		require.NoError(t, err)
	}

	// Let the cache catch up
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		result, _, err := p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "")
		require.NoError(t, err)
		require.Len(t, result, len(tokens))
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("roles filter", func(t *testing.T) {
		result, _, err := p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode, types.RoleBot}, "")
		require.NoError(t, err)
		assert.Len(t, result, 4)

		result, _, err = p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin, types.RoleNode}, "")
		require.NoError(t, err)
		assert.Len(t, result, 2)

		result, _, err = p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleBot}, "")
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("bot name filter", func(t *testing.T) {
		result, _, err := p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-1")
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "test-token-2", result[0].GetName())

		result, _, err = p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", nil, "bot-2")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test-token-3", result[0].GetName())
	})

	t.Run("combined roles and bot name filters", func(t *testing.T) {
		result, _, err := p.cache.ListProvisionTokens(ctx, defaults.MaxIterationLimit, "", types.SystemRoles{types.RoleAdmin}, "bot-1")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test-token-2", result[0].GetName())
	})
}
