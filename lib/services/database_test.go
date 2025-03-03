/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package services

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDatabaseUnmarshal verifies a database resource can be unmarshaled.
func TestDatabaseUnmarshal(t *testing.T) {
	t.Parallel()
	tlsModes := map[string]types.DatabaseTLSMode{
		"":            types.DatabaseTLSMode_VERIFY_FULL,
		"verify-full": types.DatabaseTLSMode_VERIFY_FULL,
		"verify-ca":   types.DatabaseTLSMode_VERIFY_CA,
		"insecure":    types.DatabaseTLSMode_INSECURE,
	}
	for tlsModeName, tlsModeValue := range tlsModes {
		t.Run("tls mode "+tlsModeName, func(t *testing.T) {
			expected, err := types.NewDatabaseV3(types.Metadata{
				Name:        "test-database",
				Description: "Test description",
				Labels:      map[string]string{"env": "dev"},
			}, types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				CACert:   fixtures.TLSCACertPEM,
				TLS: types.DatabaseTLS{
					Mode: tlsModeValue,
				},
			})
			require.NoError(t, err)
			caCert := indent(fixtures.TLSCACertPEM, 4)

			// verify it works with string tls mode.
			data, err := utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, tlsModeName, caCert)))
			require.NoError(t, err)
			actual, err := UnmarshalDatabase(data)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))

			// verify it works with integer tls mode.
			data, err = utils.ToJSON([]byte(fmt.Sprintf(databaseYAML, int32(tlsModeValue), caCert)))
			require.NoError(t, err)
			actual, err = UnmarshalDatabase(data)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(expected, actual))
		})
	}
}

// TestDatabaseMarshal verifies a marshaled database resource can be unmarshaled back.
func TestDatabaseMarshal(t *testing.T) {
	expected, err := types.NewDatabaseV3(types.Metadata{
		Name:        "test-database",
		Description: "Test description",
		Labels:      map[string]string{"env": "dev"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		CACert:   fixtures.TLSCACertPEM,
	})
	require.NoError(t, err)
	data, err := MarshalDatabase(expected)
	require.NoError(t, err)
	actual, err := UnmarshalDatabase(data)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(expected, actual))
}

func TestValidateDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputName   string
		inputSpec   types.DatabaseSpecV3
		expectError bool
	}{
		{
			inputName: "invalid-database-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: "unknown",
				URI:      "localhost:5432",
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "missing-port",
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-assume-role-arn",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolDynamoDB,
				AWS: types.AWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "foobar",
				},
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-assume-role-arn-resource-type",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolDynamoDB,
				AWS: types.AWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:sts::123456789012:federated-user/Alice",
				},
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-assume-role-arn-account-id-mismatch",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolDynamoDB,
				AWS: types.AWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:iam::111222333444:federated-user/Alice",
				},
			},
			expectError: true,
		},
		{
			inputName: "invalid-database-CA-cert",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				TLS: types.DatabaseTLS{
					CACert: "bad-cert",
				},
			},
			expectError: true,
		},
		{
			inputName: "valid-mongodb",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb://mongo-1:27017,mongo-2:27018/?replicaSet=rs0",
			},
			expectError: false,
		},
		{
			inputName: "valid-mongodb-srv",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb+srv://valid.but.cannot.be.resolved.com",
			},
			expectError: false,
		},
		{
			inputName: "invalid-mongodb-srv",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb+srv://valid.but.cannot.be.resolved.com/?readpreference=unknown",
			},
			expectError: true,
		},
		{
			inputName: "invalid-mongodb-missing-username",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb://mongo-1:27017/?authmechanism=plain",
			},
			expectError: true,
		},
		{
			inputName: "valid-redis",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "rediss://redis.example.com:6379",
			},
			expectError: false,
		},
		{
			inputName: "invalid-redis-incorrect-mode",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolRedis,
				URI:      "rediss://redis.example.com:6379?mode=unknown",
			},
			expectError: true,
		},
		{
			inputName: "valid-snowflake",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSnowflake,
				URI:      "test.snowflakecomputing.com",
			},
			expectError: false,
		},
		{
			inputName: "invalid-snowflake",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSnowflake,
				URI:      "not.snow.flake.com",
			},
			expectError: true,
		},
		{
			inputName: "valid-cassandra-without-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolCassandra,
				AWS: types.AWS{
					Region:    "us-east-1",
					AccountID: "123456789012",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-dynamodb-without-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolDynamoDB,
				AWS: types.AWS{
					Region:    "us-east-1",
					AccountID: "123456789012",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-spanner-without-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSpanner,
				GCP: types.GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-spanner-with-uri",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSpanner,
				URI:      "spanner.googleapis.com:443",
				GCP: types.GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			expectError: false,
		},
		{
			inputName: "invalid-spanner-uri-host",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSpanner,
				URI:      "foo.googleapis.com:443",
				GCP: types.GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			expectError: true,
		},
		{
			inputName: "invalid-spanner-uri-missing-port",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSpanner,
				URI:      "spanner.googleapis.com",
				GCP: types.GCPCloudSQL{
					ProjectID:  "project-id",
					InstanceID: "instance-id",
				},
			},
			expectError: true,
		},
		{
			inputName: "invalid-mssql-without-ad",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.goteleport.com:1433",
				AD:       types.AD{},
			},
			expectError: true,
		},
		{
			inputName: "valid-mssql-kerberos-keytabfile",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.goteleport.com:1433",
				AD: types.AD{
					KeytabFile: "path-to.keytab",
					Krb5File:   "path-to.krb5",
					Domain:     "domain.goteleport.com",
					SPN:        "MSSQLSvc/sqlserver.goteleport.com:1433",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-mssql-kerberos-kdchostname",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.goteleport.com:1433",
				AD: types.AD{
					KDCHostName: "DOMAIN-CONTROLLER.domain.goteleport.com",
					Krb5File:    "path-to.krb5",
					Domain:      "domain.goteleport.com",
					SPN:         "MSSQLSvc/sqlserver.goteleport.com:1433",
					LDAPCert:    "-----BEGIN CERTIFICATE-----",
				},
			},
			expectError: false,
		},
		{
			inputName: "invalid-mssql-kerberos-kdchostname-without-ldapcert",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.goteleport.com:1433",
				AD: types.AD{
					KDCHostName: "DOMAIN-CONTROLLER.domain.goteleport.com",
					Krb5File:    "path-to.krb5",
					Domain:      "domain.goteleport.com",
					SPN:         "MSSQLSvc/sqlserver.goteleport.com:1433",
				},
			},
			expectError: true,
		},
		{
			inputName: "valid-mssql-azure-kerberos",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.database.windows.net:1433",
				AD: types.AD{
					KeytabFile: "path-to.keytab",
					Krb5File:   "path-to.krb5",
					Domain:     "domain.goteleport.com",
					SPN:        "MSSQLSvc/sqlserver.database.windows.net:1433",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-mssql-azure-ad",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.database.windows.net:1433",
				AD:       types.AD{},
			},
			expectError: false,
		},
		{
			inputName: "valid-mssql-rds-kerberos-keytab",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.rds.amazonaws.com:1433",
				AD: types.AD{
					KeytabFile: "path-to.keytab",
					Krb5File:   "path-to.krb5",
					Domain:     "domain.goteleport.com",
					SPN:        "MSSQLSvc/sqlserver.rds.amazonaws.com:1433",
				},
			},
			expectError: false,
		},
		{
			inputName: "valid-mssql-aws-rds-proxy",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.rds.amazonaws.com:1433",
				AWS: types.AWS{
					RDSProxy: types.RDSProxy{
						Name: "sqlserver-proxy",
					},
				},
			},
			expectError: false,
		},
		{
			inputName: "invalid-mssql-rds-kerberos-without-ad",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.rds.amazonaws.com:1433",
				AD:       types.AD{},
			},
			expectError: true,
		},
		{
			inputName: "invalid-mssql-aws-rds-proxy-kerberos-without-spn",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.rds.amazonaws.com:1433",
				AWS: types.AWS{
					RDSProxy: types.RDSProxy{
						Name: "sqlserver-proxy",
					},
				},
				AD: types.AD{
					KeytabFile: "path-to.keytab",
					Krb5File:   "path-to.krb5",
					Domain:     "domain.goteleport.com",
				},
			},
			expectError: true,
		},
		{
			inputName: "valid-clickhouse-uri-http-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouseHTTP,
				URI:      "https://localhost:1234",
			},
			expectError: false,
		},
		{
			inputName: "clickhouse-uri-without-schema-http-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouseHTTP,
				URI:      "localhost:1234",
			},
			expectError: false,
		},
		{
			inputName: "clickhouse-uri-without-schema-native-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouse,
				URI:      "localhost:1234",
			},
			expectError: false,
		},
		{
			inputName: "invalid-schema-for-native-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouse,
				URI:      "https://localhost:1234",
			},
			expectError: true,
		},
		{
			inputName: "invalid-schema-for-http-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouseHTTP,
				URI:      "clickhouse://localhost:1234",
			},
			expectError: true,
		},
		{
			inputName: "valid-clickhouse-uri-native-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouse,
				URI:      "clickhouse://localhost:1234",
			},
			expectError: false,
		},
		{
			inputName: "uri-without-schema-native-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouse,
				URI:      "localhost:1234",
			},
			expectError: false,
		},
		{
			inputName: "uri-without-schema-http-protocol",
			inputSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolClickHouseHTTP,
				URI:      "localhost:1234",
			},
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.inputName, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: test.inputName,
			}, test.inputSpec)
			require.NoError(t, err)

			err = ValidateDatabase(database)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSQLServerDatabaseURI(t *testing.T) {
	for _, test := range []struct {
		uri       string
		assertErr require.ErrorAssertionFunc
	}{
		{"mssql://computer.domain.com:1433", require.NoError},
		{"computer.domain.com:1433", require.NoError},
		{"computer.ad.domain.com:1433", require.NoError},
		{"computer.ad.domain.com:1433/hello", require.Error},
		{"mssql://computer.domain.com:1433/hello", require.Error},
		{"computer.domain.com", require.Error},
		{"computer.com:1433", require.Error},
		{"0.0.0.0:1433", require.Error},
		{"mssql://", require.Error},
		{"http://computer.domain.com:1433", require.Error},
	} {
		t.Run(test.uri, func(t *testing.T) {
			test.assertErr(t, ValidateSQLServerURI(test.uri))
		})
	}
}

// indent returns the string where each line is indented by the specified
// number of spaces.
func indent(s string, spaces int) string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		lines = append(lines, fmt.Sprintf("%v%v", strings.Repeat(" ", spaces), scanner.Text()))
	}
	return strings.Join(lines, "\n")
}

var databaseYAML = `---
kind: db
version: v3
metadata:
  name: test-database
  description: "Test description"
  labels:
    env: dev
spec:
  protocol: "postgres"
  uri: "localhost:5432"
  tls:
    mode: %v
  ca_cert: |-
%v`
