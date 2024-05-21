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
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func Test_gcpConfig(t *testing.T) {
	tests := []struct {
		name                  string
		inputConnectionString string
		errorContains         string
		wantConfig            gcpConfig
	}{
		{
			name:                  "valid",
			inputConnectionString: "postgres://user@project.iam@/dbname#gcp_connection_name=project:location:instance",
			wantConfig: gcpConfig{
				connectionName: "project:location:instance",
				serviceAccount: "user@project.iam.gserviceaccount.com",
			},
		},
		{
			name:                  "valid with ip type",
			inputConnectionString: "postgres://user@project.iam@/dbname#gcp_connection_name=project:location:instance&gcp_ip_type=PRIVATE",
			wantConfig: gcpConfig{
				connectionName: "project:location:instance",
				ipType:         gcpIPTypePrivateIP,
				serviceAccount: "user@project.iam.gserviceaccount.com",
			},
		},
		{
			name:                  "missing connection name",
			inputConnectionString: "postgres://user@project.iam@/dbname",
			errorContains:         "missing #gcp_connection_name",
		},
		{
			name:                  "invalid ip type",
			inputConnectionString: "postgres://user@project.iam@/dbname#gcp_connection_name=project:location:instance&gcp_ip_type=unknown",
			errorContains:         "invalid gcp_ip_type",
		},
		{
			name:                  "invalid service account",
			inputConnectionString: "postgres://not-iam-user-for-service-account@/dbname#gcp_connection_name=project:location:instance",
			errorContains:         "invalid service account",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			connConfig, err := pgx.ParseConfig(tc.inputConnectionString)
			require.NoError(t, err)

			gcpConfig, err := gcpConfigFromConnConfig(connConfig)
			if tc.errorContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errorContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, gcpConfig)
				require.Equal(t, tc.wantConfig, *gcpConfig)
			}
		})
	}
}

func Test_gcpIPType(t *testing.T) {
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
			ipType := gcpIPType(tc.ipTypeStr)
			tc.requireCheck(t, ipType.check())
			tc.requireCloudSQLConnOption(t, ipType.cloudsqlconnOption())
		})
	}
}
