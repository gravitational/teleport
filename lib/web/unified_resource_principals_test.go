/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package web

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

// mockLoginGetter implements the GetAllowedLoginsForResource method for testing.
type mockLoginGetter struct {
	services.AccessChecker
	logins []string
	err    error
}

func (m *mockLoginGetter) GetAllowedLoginsForResource(resource services.AccessCheckable) ([]string, error) {
	return m.logins, m.err
}

func TestPrincipalsForUnifiedResource_SSH(t *testing.T) {
	t.Parallel()

	server, err := types.NewServer("test-node", types.KindNode, types.ServerSpecV2{
		Hostname: "test-node",
		Addr:     "1.2.3.4:22",
	})
	require.NoError(t, err)

	t.Run("default mode filters by cert principals", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: server,
				Logins:             []string{"root", "ubuntu", "admin"},
			},
			CertPrincipals: []string{"ubuntu", "admin"},
		})
		require.NoError(t, err)
		require.NotNil(t, result.Logins)
		require.Nil(t, result.AWSRoleARNs)

		require.Equal(t, set.New("ubuntu", "admin"), result.Logins.All)
		require.Equal(t, result.Logins.All, result.Logins.Granted)
	})

	t.Run("UseSearchAsRoles returns enriched logins unfiltered", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: server,
				Logins:             []string{"root", "ubuntu", "admin"},
			},
			CertPrincipals:   []string{"ubuntu"},
			UseSearchAsRoles: true,
		})
		require.NoError(t, err)

		// All logins returned regardless of cert principals.
		require.Equal(t, set.New("root", "ubuntu", "admin"), result.Logins.All)
		// Granted == All when IncludeRequestable is false.
		require.Equal(t, result.Logins.All, result.Logins.Granted)
	})

	t.Run("IncludeRequestable splits granted from requestable", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: server,
				Logins:             []string{"root", "ubuntu", "admin"},
			},
			CertPrincipals:     []string{"ubuntu", "root"},
			AccessChecker:      &mockLoginGetter{logins: []string{"ubuntu"}},
			IncludeRequestable: true,
		})
		require.NoError(t, err)

		// All includes everything from enriched logins.
		require.Equal(t, set.New("root", "ubuntu", "admin"), result.Logins.All)
		// Granted is the intersection of AccessChecker result and cert principals.
		require.Equal(t, set.New("ubuntu"), result.Logins.Granted)
	})
}

func TestPrincipalsForUnifiedResource_App(t *testing.T) {
	t.Parallel()

	appServer := &types.AppServerV3{
		Spec: types.AppServerSpecV3{
			App: &types.AppV3{
				Metadata: types.Metadata{Name: "my-app"},
			},
		},
	}

	t.Run("uses enriched logins when available", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: appServer,
				Logins:             []string{"arn:aws:iam::111:role/Admin", "arn:aws:iam::111:role/ReadOnly"},
			},
			AccessChecker: &mockLoginGetter{logins: []string{"should-not-be-called"}},
		})
		require.NoError(t, err)
		require.Nil(t, result.Logins)
		require.NotNil(t, result.AWSRoleARNs)

		require.Equal(t, set.New("arn:aws:iam::111:role/Admin", "arn:aws:iam::111:role/ReadOnly"), result.AWSRoleARNs.All)
		require.Equal(t, result.AWSRoleARNs.All, result.AWSRoleARNs.Granted)
	})

	t.Run("falls back to AccessChecker when no enriched logins", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: appServer,
			},
			AccessChecker: &mockLoginGetter{logins: []string{"arn:aws:iam::111:role/Fallback"}},
		})
		require.NoError(t, err)

		require.Equal(t, set.New("arn:aws:iam::111:role/Fallback"), result.AWSRoleARNs.All)
		require.Equal(t, result.AWSRoleARNs.All, result.AWSRoleARNs.Granted)
	})

	t.Run("no enriched logins and no fallback", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: appServer,
			},
			AccessChecker: &mockLoginGetter{},
		})
		require.NoError(t, err)

		require.Empty(t, result.AWSRoleARNs.All)
		require.Equal(t, result.AWSRoleARNs.All, result.AWSRoleARNs.Granted)
	})

	t.Run("IncludeRequestable splits granted from requestable", func(t *testing.T) {
		t.Parallel()

		result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
			Resource: &types.EnrichedResource{
				ResourceWithLabels: appServer,
				Logins:             []string{"arn:aws:iam::111:role/Admin", "arn:aws:iam::111:role/ReadOnly"},
			},
			AccessChecker:      &mockLoginGetter{logins: []string{"arn:aws:iam::111:role/ReadOnly"}},
			IncludeRequestable: true,
		})
		require.NoError(t, err)

		require.Equal(t,
			set.New("arn:aws:iam::111:role/Admin", "arn:aws:iam::111:role/ReadOnly"),
			result.AWSRoleARNs.All,
		)
		require.Equal(t,
			set.New("arn:aws:iam::111:role/ReadOnly"),
			result.AWSRoleARNs.Granted,
		)
	})
}

func TestPrincipalsForUnifiedResource_UnknownKind(t *testing.T) {
	t.Parallel()

	result, err := PrincipalsForUnifiedResource(PrincipalsForUnifiedResourceOpts{
		Resource: &types.EnrichedResource{
			ResourceWithLabels: &types.DatabaseV3{},
		},
	})
	require.NoError(t, err)
	require.Nil(t, result.Logins)
	require.Nil(t, result.AWSRoleARNs)
}

func TestFilterByIdentityPrincipals(t *testing.T) {
	t.Parallel()

	t.Run("returns intersection", func(t *testing.T) {
		t.Parallel()
		result := filterByIdentityPrincipals(
			[]string{"root", "ubuntu", "admin"},
			[]string{"ubuntu", "admin", "other"},
		)
		require.Equal(t, set.New("ubuntu", "admin"), result)
	})

	t.Run("empty identity principals", func(t *testing.T) {
		t.Parallel()
		result := filterByIdentityPrincipals(nil, []string{"ubuntu"})
		require.Empty(t, result)
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()
		result := filterByIdentityPrincipals([]string{"root"}, []string{"ubuntu"})
		require.Empty(t, result)
	})
}
