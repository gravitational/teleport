/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package pgcommon

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestAuthMode(t *testing.T) {
	mustSetGoogleApplicationCredentialsEnv(t)
	mustSetAzureEnvironmentCredential(t)

	verifyBeforeConnectIsSet := func(t *testing.T, config *pgxpool.Config) {
		t.Helper()
		require.NotNil(t, config.BeforeConnect)
	}
	verifyNothingIsSet := func(t *testing.T, config *pgxpool.Config) {
		t.Helper()
		require.NotNil(t, config)
		require.Equal(t, pgxpool.Config{}, *config)
	}

	tests := []struct {
		authMode                       AuthMode
		requireCheckError              require.ErrorAssertionFunc
		verifyPoolConfigAfterConfigure func(*testing.T, *pgxpool.Config)
	}{
		{
			authMode:          AuthMode("unknown-mode"),
			requireCheckError: require.Error,
		},
		{
			authMode:                       StaticAuth,
			requireCheckError:              require.NoError,
			verifyPoolConfigAfterConfigure: verifyNothingIsSet,
		},
		{
			authMode:                       AzureADAuth,
			requireCheckError:              require.NoError,
			verifyPoolConfigAfterConfigure: verifyBeforeConnectIsSet,
		},
		{
			authMode:                       GCPSQLIAMAuth,
			requireCheckError:              require.NoError,
			verifyPoolConfigAfterConfigure: verifyBeforeConnectIsSet,
		},
		{
			authMode:                       GCPAlloyDBIAMAuth,
			requireCheckError:              require.NoError,
			verifyPoolConfigAfterConfigure: verifyBeforeConnectIsSet,
		},
	}

	ctx := context.Background()
	logger := slog.Default()
	for _, tc := range tests {
		t.Run(string(tc.authMode), func(t *testing.T) {
			err := tc.authMode.Check()
			if err != nil {
				// Just checking out how the error message looks like.
				t.Log(err)
			}
			tc.requireCheckError(t, err)

			if tc.verifyPoolConfigAfterConfigure != nil {
				configs := []*pgxpool.Config{
					&pgxpool.Config{},
					&pgxpool.Config{},
				}

				err := tc.authMode.ConfigurePoolConfigs(ctx, logger, configs...)
				require.NoError(t, err)

				for _, config := range configs {
					tc.verifyPoolConfigAfterConfigure(t, config)
				}
			}
		})
	}
}

func mustSetAzureEnvironmentCredential(t *testing.T) {
	t.Helper()
	t.Setenv("AZURE_TENANT_ID", "teleport-test-tenant-id")
	t.Setenv("AZURE_CLIENT_ID", "teleport-test-client-id")
	t.Setenv("AZURE_CLIENT_SECRET", "teleport-test-client-secret")
}
