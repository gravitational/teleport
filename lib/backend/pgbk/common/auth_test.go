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

func TestAuthConfig(t *testing.T) {
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
		name                       string
		authConfig                 AuthConfig
		makePoolConfig             func(*testing.T) *pgxpool.Config
		requireCheckError          require.ErrorAssertionFunc
		verifyPoolConfigAfterApply func(*testing.T, *pgxpool.Config)
	}{
		{
			name: "unknown mode",
			authConfig: AuthConfig{
				AuthMode: AuthMode("unknown-mode"),
			},
			makePoolConfig:    emptyPoolConfig,
			requireCheckError: require.Error,
		},
		{
			name: "static auth",
			authConfig: AuthConfig{
				AuthMode: StaticAuth,
			},
			makePoolConfig:             emptyPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyNothingIsSet,
		},
		{
			name: "Azure AD Auth",
			authConfig: AuthConfig{
				AuthMode: AzureADAuth,
			},
			makePoolConfig:             emptyPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyBeforeConnectIsSet,
		},
		{
			name: "GCP IAM Auth",
			authConfig: AuthConfig{
				AuthMode:          GCPCloudSQLIAMAuth,
				GCPConnectionName: "project:location:instance",
			},
			makePoolConfig:             gcpCloudSQLPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyDialFuncIsSet,
		},
		{
			name: "GCP IAM Auth with IP type",
			authConfig: AuthConfig{
				AuthMode:          GCPCloudSQLIAMAuth,
				GCPConnectionName: "project:location:instance",
				GCPIPType:         GCPIPTypePrivateIP,
			},
			makePoolConfig:             gcpCloudSQLPoolConfig,
			requireCheckError:          require.NoError,
			verifyPoolConfigAfterApply: verifyDialFuncIsSet,
		},
		{
			name: "missing GCP connection name",
			authConfig: AuthConfig{
				AuthMode: GCPCloudSQLIAMAuth,
			},
			makePoolConfig:    gcpCloudSQLPoolConfig,
			requireCheckError: require.Error,
		},
		{
			name: "invalid GCP IP Type",
			authConfig: AuthConfig{
				AuthMode:          GCPCloudSQLIAMAuth,
				GCPConnectionName: "project:location:instance",
				GCPIPType:         GCPIPType("unknown-ip-type"),
			},
			makePoolConfig:    gcpCloudSQLPoolConfig,
			requireCheckError: require.Error,
		},
	}

	ctx := context.Background()
	logger := slog.Default()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("check", func(t *testing.T) {
				err := tc.authConfig.Check()
				if err != nil {
					// Just checking out how the error message looks like.
					t.Log(err)
				}
				tc.requireCheckError(t, err)
			})

			if tc.verifyPoolConfigAfterApply != nil {
				t.Run("ApplyToPoolConfigs", func(t *testing.T) {
					configs := []*pgxpool.Config{tc.makePoolConfig(t), tc.makePoolConfig(t), tc.makePoolConfig(t)}
					err := tc.authConfig.ApplyToPoolConfigs(ctx, logger, configs...)
					require.NoError(t, err)

					for _, config := range configs {
						tc.verifyPoolConfigAfterApply(t, config)
					}
				})
			}
		})
	}
}

func TestGCPIPType(t *testing.T) {
	tests := []struct {
		ipTypeStr                 string
		requireCheck              require.ErrorAssertionFunc
		requireCloudSQLConnOption require.ValueAssertionFunc
	}{
		{
			ipTypeStr:                 "",
			requireCheck:              require.NoError,
			requireCloudSQLConnOption: require.Nil,
		},
		{
			ipTypeStr:                 "unknown",
			requireCheck:              require.Error,
			requireCloudSQLConnOption: require.Nil,
		},
		{
			ipTypeStr:                 "public",
			requireCheck:              require.NoError,
			requireCloudSQLConnOption: require.NotNil,
		},
		{
			ipTypeStr:                 "private",
			requireCheck:              require.NoError,
			requireCloudSQLConnOption: require.NotNil,
		},
		{
			ipTypeStr:                 "psc",
			requireCheck:              require.NoError,
			requireCloudSQLConnOption: require.NotNil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.ipTypeStr, func(t *testing.T) {
			ipType := GCPIPType(tc.ipTypeStr)
			tc.requireCheck(t, ipType.check())
			tc.requireCloudSQLConnOption(t, ipType.cloudsqlconnOption())
		})
	}
}

func mustSetAzureEnvironmentCredential(t *testing.T) {
	t.Helper()
	t.Setenv("AZURE_TENANT_ID", "teleport-test-tenant-id")
	t.Setenv("AZURE_CLIENT_ID", "teleport-test-client-id")
	t.Setenv("AZURE_CLIENT_SECRET", "teleport-test-client-secret")
}
