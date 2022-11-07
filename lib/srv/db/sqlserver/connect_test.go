// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlserver

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
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

	connector := &connector{Auth: &mockAuth{}}

	for _, tt := range []struct {
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
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, err, os.ErrNotExist)
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
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.ErrorIs(t, err, os.ErrNotExist)
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
			errAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unable to open tcp connection with host")
			},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			database, err := types.NewDatabaseV3(types.Metadata{
				Name: uuid.NewString(),
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
