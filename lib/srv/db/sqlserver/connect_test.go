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

package sqlserver

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/sqlserver/protocol"
)

// TestConnectorSelection given a database session, choose correctly which
// connector to use. This test doesn't cover the connection flow, only the
// selection logic.
func TestConnectorSelection(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		desc                 string
		databaseSpec         types.DatabaseSpecV3
		expectAzureConnector bool
		expectTokenConnector bool
		errAssertion         require.ErrorAssertionFunc
	}{
		{
			desc: "Non-Azure database",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
			},
			// When using a non-Azure database, the connector should fail
			// loading Kerberos credentials.
			errAssertion: requireKerberosClientError,
		},
		{
			desc: "Azure database with AD configured",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "name.database.windows.net:1443",
				AD: types.AD{
					Domain: "EXAMPLE.COM",
				},
			},
			// When using an Azure database with AD configuration, the connector
			// should fail loading Kerberos credentials.
			errAssertion: requireKerberosClientError,
		},
		{
			desc: "Azure database without AD configured",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "azure-db.database.windows.net:1443",
			},
			// When using an Azure database without AD configuration, the
			// connector should succeed in selecting the Azure connector.
			expectAzureConnector: true,
			errAssertion:         require.NoError,
		},
		{
			desc: "RDS Proxied database",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "proxy-sqlserver.proxy-000000000000.us-east-1.rds.amazonaws.com:1433",
				AWS: types.AWS{
					RDSProxy: types.RDSProxy{
						Name: "proxy-sqlserver",
					},
				},
			},
			// When using an RDS proxy database, the connector should succeed
			// in selecting the access token connector.
			expectTokenConnector: true,
			errAssertion:         require.NoError,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			var azureCalled, tokenCalled bool

			connector := &connector{
				DBAuth:   &mockDBAuth{},
				kerberos: &mockKerberos{},
				newAzureConnector: func(msdsn.Config) (*mssql.Connector, error) {
					azureCalled = true
					return &mssql.Connector{}, nil
				},
				newSecurityTokenConnector: func(msdsn.Config, func(context.Context) (string, error)) (*mssql.Connector, error) {
					tokenCalled = true
					return &mssql.Connector{}, nil
				},
			}

			database, err := types.NewDatabaseV3(types.Metadata{
				Name: fmt.Sprintf("db-%v", i),
			}, tt.databaseSpec)
			require.NoError(t, err)

			_, err = connector.selectConnector(t.Context(), &common.Session{Database: database}, &protocol.Login7Packet{})
			tt.errAssertion(t, err)

			require.Equal(t, tt.expectAzureConnector, azureCalled, "Azure connector call mismatch")
			require.Equal(t, tt.expectTokenConnector, tokenCalled, "Token connector call mismatch")
		})
	}
}

type mockKerberos struct{}

const unimplementedMessage = "intentionally left unimplemented"

func (m *mockKerberos) GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error) {
	return nil, trace.BadParameter(unimplementedMessage)
}

func requireKerberosClientError(t require.TestingT, err error, msgAndArgs ...any) {
	require.ErrorContains(t, err, unimplementedMessage, msgAndArgs...)
}
