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
	"time"

	"github.com/gravitational/trace"
	"github.com/jcmturner/gokrb5/v8/client"
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connector := &connector{
		DBAuth:   &mockDBAuth{},
		kerberos: &mockKerberos{},
	}

	for i, tt := range []struct {
		desc         string
		databaseSpec types.DatabaseSpecV3
		errAssertion require.ErrorAssertionFunc
	}{
		{
			desc: "Non-Azure database",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver:1443",
			},
			// When using a non-Azure database, the connector should fail
			// loading Kerberos credentials.
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, unimplementedMessage)
			},
		},
		{
			desc: "Azure database with AD configured",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "name.database.windows.net:1443",
				AD: types.AD{
					// Domain is required for AD authentication.
					Domain: "EXAMPLE.COM",
				},
			},
			// When using a Azure database with AD configuration, the connector
			// should fail loading Kerberos credentials.
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, unimplementedMessage)
			},
		},
		{
			desc: "Azure database without AD configured",
			databaseSpec: types.DatabaseSpecV3{
				Protocol: defaults.ProtocolSQLServer,
				URI:      "random.database.windows.net:1443",
			},
			// When using a Azure database without AD configuration, the
			// connector should fail because it could not connect to the
			// database.
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unable to open tcp connection with host")
			},
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
			// RDS proxies cannot be accessed outside their VPC. So, this test
			// case should not resolve their host.
			errAssertion: func(t require.TestingT, err error, _ ...any) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "no such host")
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: fmt.Sprintf("db-%v", i),
			}, tt.databaseSpec)
			require.NoError(t, err)

			connectorCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			resChan := make(chan error, 1)
			go func() {
				_, _, err = connector.Connect(connectorCtx, &common.Session{Database: database}, &protocol.Login7Packet{})
				resChan <- err
			}()

			// Cancel the context to avoid dialing databases.
			cancel()

			select {
			case err := <-resChan:
				tt.errAssertion(t, err)
			case <-ctx.Done():
				require.Fail(t, "timed out waiting for connector to return")
			}
		})
	}
}

type mockKerberos struct{}

const unimplementedMessage = "intentionally left unimplemented"

func (m *mockKerberos) GetKerberosClient(ctx context.Context, ad types.AD, username string) (*client.Client, error) {
	return nil, trace.BadParameter(unimplementedMessage)
}
