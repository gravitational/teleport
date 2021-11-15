/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package db

import (
	"context"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"

	"github.com/stretchr/testify/require"
)

// TestInitCACert verifies automatic download of root certs for cloud databases.
func TestInitCACert(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	selfHosted, err := types.NewDatabaseV3(types.Metadata{
		Name: "self-hosted",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
	})
	require.NoError(t, err)

	rdsWithCert, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds-with-cert",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
		CACert:   "rds-test-cert",
	})
	require.NoError(t, err)

	redshift, err := types.NewDatabaseV3(types.Metadata{
		Name: "redshift",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1", Redshift: types.Redshift{ClusterID: "cluster-id"}},
	})
	require.NoError(t, err)

	cloudSQL, err := types.NewDatabaseV3(types.Metadata{
		Name: "cloud-sql",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		GCP:      types.GCPCloudSQL{ProjectID: "project-id", InstanceID: "instance-id"},
	})
	require.NoError(t, err)

	allDatabases := []types.Database{
		selfHosted, rds, rdsWithCert, redshift, cloudSQL,
	}

	tests := []struct {
		desc     string
		database string
		cert     string
	}{
		{
			desc:     "shouldn't download self-hosted CA",
			database: selfHosted.GetName(),
			cert:     selfHosted.GetCA(),
		},
		{
			desc:     "should download RDS CA when it's not set",
			database: rds.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "shouldn't download RDS CA when it's set",
			database: rdsWithCert.GetName(),
			cert:     rdsWithCert.GetCA(),
		},
		{
			desc:     "should download Redshift CA when it's not set",
			database: redshift.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
		{
			desc:     "should download Cloud SQL CA when it's not set",
			database: cloudSQL.GetName(),
			cert:     fixtures.TLSCACertPEM,
		},
	}

	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: allDatabases,
	})

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			var database types.Database
			for _, db := range databaseServer.getProxiedDatabases() {
				if db.GetName() == test.database {
					database = db
				}
			}
			require.NotNil(t, database)
			require.Equal(t, test.cert, database.GetCA())
		})
	}
}

// TestInitCACertCaching verifies root certificate is not re-downloaded if
// it was already downloaded before.
func TestInitCACertCaching(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t)

	rds, err := types.NewDatabaseV3(types.Metadata{
		Name: "rds",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
		AWS:      types.AWS{Region: "us-east-1"},
	})
	require.NoError(t, err)

	databaseServer := testCtx.setupDatabaseServer(ctx, t, agentParams{
		Databases: []types.Database{rds},
		NoStart:   true,
	})

	// Initialize RDS cert for the first time.
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, 1, databaseServer.cfg.CADownloader.(*fakeDownloader).count)

	// Reset it and initialize again, it should already be downloaded.
	rds.SetStatusCA("")
	require.NoError(t, databaseServer.initCACert(ctx, rds))
	require.Equal(t, 1, databaseServer.cfg.CADownloader.(*fakeDownloader).count)
}

type fakeDownloader struct {
	// cert is the cert to return as downloaded one.
	cert []byte
	// count keeps track of how many times the downloader has been invoked.
	count int
}

func (d *fakeDownloader) Download(context.Context, types.Database) ([]byte, error) {
	d.count++
	return d.cert, nil
}
