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

	emptyPoolConfig := func(t *testing.T) *pgxpool.Config {
		t.Helper()
		return &pgxpool.Config{}
	}
	gcpCloudSQLPoolConfig := func(t *testing.T) *pgxpool.Config {
		t.Helper()
		config, err := pgxpool.ParseConfig("postgres://user@project.iam@/#gcp_connection_name=project:location:instance")
		require.NoError(t, err)
		// Unset dial func to verify that it will be overwritten.
		config.ConnConfig.DialFunc = nil
		return config
	}

	verifyBeforeConnectIsSet := func(t *testing.T, config *pgxpool.Config) {
		t.Helper()
		require.NotNil(t, config.BeforeConnect)
	}
	verifyDialFuncIsSet := func(t *testing.T, config *pgxpool.Config) {
		t.Helper()
		require.NotNil(t, config.ConnConfig.DialFunc)
	}
	verifyNothingIsSet := func(t *testing.T, config *pgxpool.Config) {
		t.Helper()
		require.Equal(t, emptyPoolConfig(t), config)
	}

	tests := []struct {
		authMode                   AuthMode
		makePoolConfig             func(*testing.T) *pgxpool.Config
		requireCheckError          require.ErrorAssertionFunc
		verifyPoolConfigAfterApply func(*testing.T, *pgxpool.Config)
	}{
		{
			authMode:          AuthMode("unknown-mode"),
			makePoolConfig:    emptyPoolConfig,
			requireCheckError: require.Error,
		},
		{
			authMode:                   StaticAuth,
			makePoolConfig:             emptyPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyNothingIsSet,
		},
		{
			authMode:                   AzureADAuth,
			makePoolConfig:             emptyPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyBeforeConnectIsSet,
		},
		{
			authMode:                   GCPCloudSQLIAMAuth,
			makePoolConfig:             gcpCloudSQLPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyDialFuncIsSet,
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

			if tc.verifyPoolConfigAfterApply != nil {
				configs := []*pgxpool.Config{tc.makePoolConfig(t), tc.makePoolConfig(t), tc.makePoolConfig(t)}
				err = tc.authMode.ApplyToPoolConfigs(ctx, logger, configs...)
				require.NoError(t, err)

				for _, config := range configs {
					tc.verifyPoolConfigAfterApply(t, config)
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
