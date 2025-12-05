/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package local

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	appauthconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/appauthconfig/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	labelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/label/v1"
	"github.com/gravitational/teleport/api/types/appauthconfig"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestAppAuthConfigService(t *testing.T) {
	t.Parallel()

	t.Run("empty list", func(t *testing.T) {
		svc := setupAppAuthService(t, nil)
		cfgs, _, err := svc.ListAppAuthConfigs(t.Context(), 10, "")
		require.NoError(t, err)
		require.Empty(t, cfgs)
	})

	t.Run("create configs", func(t *testing.T) {
		svc := setupAppAuthService(t, nil)
		_, err := svc.CreateAppAuthConfig(t.Context(), makeAppAuthJWTConfig("jwt1"))
		require.NoError(t, err)
		_, err = svc.CreateAppAuthConfig(t.Context(), makeAppAuthJWTConfig("jwt2"))
		require.NoError(t, err)
	})

	t.Run("create invalid", func(t *testing.T) {
		cfg := appauthconfig.NewAppAuthConfigJWT(
			"jwt-invalid",
			[]*labelv1.Label{{Name: "*", Values: []string{"*"}}},
			&appauthconfigv1.AppAuthConfigJWTSpec{},
		)

		svc := setupAppAuthService(t, nil)
		_, err := svc.CreateAppAuthConfig(t.Context(), cfg)
		require.Error(t, err)
	})

	t.Run("list configs", func(t *testing.T) {
		expectedCfgs := []*appauthconfigv1.AppAuthConfig{
			makeAppAuthJWTConfig("jwt1"),
			makeAppAuthJWTConfig("jwt2"),
		}
		svc := setupAppAuthService(t, expectedCfgs)

		cfgs, _, err := svc.ListAppAuthConfigs(t.Context(), len(expectedCfgs)+1, "")
		require.NoError(t, err)
		requireAppAuthConfigEqual(t, expectedCfgs, cfgs)
	})

	t.Run("list with next key", func(t *testing.T) {
		expectedCfgs := []*appauthconfigv1.AppAuthConfig{
			makeAppAuthJWTConfig("jwt1"),
			makeAppAuthJWTConfig("jwt2"),
		}
		svc := setupAppAuthService(t, expectedCfgs)

		var retrievedCfgs []*appauthconfigv1.AppAuthConfig
		first, nextKey, err := svc.ListAppAuthConfigs(t.Context(), 1, "")
		require.NoError(t, err)
		require.NotEmpty(t, nextKey)
		require.Len(t, first, 1)
		retrievedCfgs = append(retrievedCfgs, first...)

		second, nextKey, err := svc.ListAppAuthConfigs(t.Context(), 1, nextKey)
		require.NoError(t, err)
		require.Empty(t, nextKey)
		require.Len(t, first, 1)
		retrievedCfgs = append(retrievedCfgs, second...)

		requireAppAuthConfigEqual(t, expectedCfgs, retrievedCfgs)
	})

	t.Run("create and retrieve", func(t *testing.T) {
		svc := setupAppAuthService(t, nil)

		cfgCreated, err := svc.CreateAppAuthConfig(t.Context(), makeAppAuthJWTConfig("jwt1"))
		require.NoError(t, err)
		cfgRetrieved, err := svc.GetAppAuthConfig(t.Context(), cfgCreated.Metadata.Name)
		require.NoError(t, err)

		requireAppAuthConfigEqual(t, []*appauthconfigv1.AppAuthConfig{cfgCreated}, []*appauthconfigv1.AppAuthConfig{cfgRetrieved})
	})

	t.Run("delete and retrieve", func(t *testing.T) {
		cfg := makeAppAuthJWTConfig("jwt1")
		svc := setupAppAuthService(t, []*appauthconfigv1.AppAuthConfig{cfg})

		err := svc.DeleteAppAuthConfig(t.Context(), cfg.Metadata.Name)
		require.NoError(t, err)

		// Deleting again should return error.
		err = svc.DeleteAppAuthConfig(t.Context(), cfg.Metadata.Name)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected error to be NotFound but got %T", err)

		_, err = svc.GetAppAuthConfig(t.Context(), cfg.Metadata.Name)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected error to be NotFound but got %T", err)
	})

	t.Run("upsert", func(t *testing.T) {
		baseCfg := makeAppAuthJWTConfig("jwt1")
		svc := setupAppAuthService(t, []*appauthconfigv1.AppAuthConfig{})

		firstCfg, err := svc.UpsertAppAuthConfig(t.Context(), baseCfg)
		require.NoError(t, err)
		firstRev := firstCfg.Metadata.Revision

		secondCfg, err := svc.UpsertAppAuthConfig(t.Context(), baseCfg)
		require.NoError(t, err)
		secondRev := secondCfg.Metadata.Revision

		require.NotEqual(t, firstRev, secondRev)
	})
}

func requireAppAuthConfigEqual(t *testing.T, expected []*appauthconfigv1.AppAuthConfig, actual []*appauthconfigv1.AppAuthConfig) {
	require.Empty(t, cmp.Diff(
		expected,
		actual,
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
		protocmp.Transform(),
	))
}

func makeAppAuthJWTConfig(name string) *appauthconfigv1.AppAuthConfig {
	return appauthconfig.NewAppAuthConfigJWT(
		name,
		[]*labelv1.Label{{Name: "*", Values: []string{"*"}}},
		&appauthconfigv1.AppAuthConfigJWTSpec{
			Audience: "teleport",
			Issuer:   "https://my-issuer",
			KeysSource: &appauthconfigv1.AppAuthConfigJWTSpec_JwksUrl{
				JwksUrl: "https://my-issuer/.well-known/jwks.json",
			},
		},
	)
}

func setupAppAuthService(t *testing.T, cfgs []*appauthconfigv1.AppAuthConfig) *AppAuthConfigService {
	t.Helper()

	backend, err := memory.New(memory.Config{Context: t.Context()})
	require.NoError(t, err)

	svc, err := NewAppAuthConfigService(backend)
	require.NoError(t, err)

	for _, cfg := range cfgs {
		_, err := svc.CreateAppAuthConfig(t.Context(), cfg)
		require.NoError(t, err)
	}

	return svc
}
