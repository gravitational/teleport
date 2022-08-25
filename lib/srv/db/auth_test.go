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
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestAuthTokens verifies that proper IAM auth tokens are used when connecting
// to cloud databases such as RDS, Redshift, Cloud SQL.
func TestAuthTokens(t *testing.T) {
	ctx := context.Background()
	testCtx := setupTestContext(ctx, t,
		withRDSPostgres("postgres-rds-correct-token", rdsAuthToken),
		withRDSPostgres("postgres-rds-incorrect-token", "qwe123"),
		withRedshiftPostgres("postgres-redshift-correct-token", redshiftAuthToken),
		withRedshiftPostgres("postgres-redshift-incorrect-token", "qwe123"),
		withCloudSQLPostgres("postgres-cloudsql-correct-token", cloudSQLAuthToken),
		withCloudSQLPostgres("postgres-cloudsql-incorrect-token", "qwe123"),
		withAzurePostgres("postgres-azure-correct-token", azureAccessToken),
		withAzurePostgres("postgres-azure-incorrect-token", "qwe123"),
		withRDSMySQL("mysql-rds-correct-token", "root", rdsAuthToken),
		withRDSMySQL("mysql-rds-incorrect-token", "root", "qwe123"),
		withCloudSQLMySQL("mysql-cloudsql-correct-token", "root", cloudSQLPassword),
		withCloudSQLMySQL("mysql-cloudsql-incorrect-token", "root", "qwe123"),
		withAzureMySQL("mysql-azure-correct-token", "root", azureAccessToken),
		withAzureMySQL("mysql-azure-incorrect-token", "root", "qwe123"))
	go testCtx.startHandlingConnections()

	testCtx.createUserAndRole(ctx, t, "alice", "admin", []string{types.Wildcard}, []string{types.Wildcard})

	tests := []struct {
		desc     string
		service  string
		protocol string
		err      string
	}{
		{
			desc:     "correct Postgres RDS IAM auth token",
			service:  "postgres-rds-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres RDS IAM auth token",
			service:  "postgres-rds-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "rds-db:connect", // Make sure we print example RDS IAM policy.
		},
		{
			desc:     "correct Postgres Redshift IAM auth token",
			service:  "postgres-redshift-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres Redshift IAM auth token",
			service:  "postgres-redshift-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "invalid auth token",
		},
		{
			desc:     "correct Postgres Cloud SQL IAM auth token",
			service:  "postgres-cloudsql-correct-token",
			protocol: defaults.ProtocolPostgres,
		},
		{
			desc:     "incorrect Postgres Cloud SQL IAM auth token",
			service:  "postgres-cloudsql-incorrect-token",
			protocol: defaults.ProtocolPostgres,
			err:      "invalid auth token",
		},
		{
			desc:     "correct MySQL RDS IAM auth token",
			service:  "mysql-rds-correct-token",
			protocol: defaults.ProtocolMySQL,
		},
		{
			desc:     "incorrect MySQL RDS IAM auth token",
			service:  "mysql-rds-incorrect-token",
			protocol: defaults.ProtocolMySQL,
			err:      "rds-db:connect", // Make sure we print example RDS IAM policy.
		},
		{
			desc:     "correct MySQL Cloud SQL IAM auth token",
			service:  "mysql-cloudsql-correct-token",
			protocol: defaults.ProtocolMySQL,
		},
		{
			desc:     "incorrect MySQL Cloud SQL IAM auth token",
			service:  "mysql-cloudsql-incorrect-token",
			protocol: defaults.ProtocolMySQL,
			err:      "Access denied for user",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			switch test.protocol {
			case defaults.ProtocolPostgres:
				conn, err := testCtx.postgresClient(ctx, "alice", test.service, "postgres", "postgres")
				if test.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.err)
				} else {
					require.NoError(t, err)
					require.NoError(t, conn.Close(ctx))
				}
			case defaults.ProtocolMySQL:
				conn, err := testCtx.mysqlClient("alice", test.service, "root")
				if test.err != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.err)
				} else {
					require.NoError(t, err)
					require.NoError(t, conn.Close())
				}
			default:
				t.Fatalf("unrecognized database protocol in test: %q", test.protocol)
			}
		})
	}
}

// testAuth mocks cloud provider auth tokens generation for use in tests.
type testAuth struct {
	// Auth is the wrapped "real" auth that handles everything except for
	// cloud auth tokens generation.
	common.Auth
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

func newTestAuth(ac common.AuthConfig) (*testAuth, error) {
	auth, err := common.NewAuth(ac)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &testAuth{
		Auth:        auth,
		FieldLogger: logrus.WithField(trace.Component, "auth:test"),
	}, nil
}

const (
	// rdsAuthToken is a mock RDS IAM auth token.
	rdsAuthToken = "rds-auth-token"
	// redshiftAuthUser is a mock Redshift IAM auth user.
	redshiftAuthUser = "redshift-auth-user"
	// redshiftAuthToken is a mock Redshift IAM auth token.
	redshiftAuthToken = "redshift-auth-token"
	// cloudSQLAuthToken is a mock Cloud SQL IAM auth token.
	cloudSQLAuthToken = "cloudsql-auth-token"
	// cloudSQLPassword is a mock Cloud SQL user password.
	cloudSQLPassword = "cloudsql-password"
	// azureAccessToken is a mock Azure access token.
	azureAccessToken = "azure-access-token"
)

// GetRDSAuthToken generates RDS/Aurora auth token.
func (a *testAuth) GetRDSAuthToken(sessionCtx *common.Session) (string, error) {
	a.Infof("Generating RDS auth token for %v.", sessionCtx)
	return rdsAuthToken, nil
}

// GetRedshiftAuthToken generates Redshift auth token.
func (a *testAuth) GetRedshiftAuthToken(sessionCtx *common.Session) (string, string, error) {
	a.Infof("Generating Redshift auth token for %v.", sessionCtx)
	return redshiftAuthUser, redshiftAuthToken, nil
}

// GetCloudSQLAuthToken generates Cloud SQL auth token.
func (a *testAuth) GetCloudSQLAuthToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Cloud SQL auth token for %v.", sessionCtx)
	return cloudSQLAuthToken, nil
}

// GetCloudSQLPassword generates Cloud SQL user password.
func (a *testAuth) GetCloudSQLPassword(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Cloud SQL user password %v.", sessionCtx)
	return cloudSQLPassword, nil
}

// GetAzureAccessToken generates Azure access token.
func (a *testAuth) GetAzureAccessToken(ctx context.Context, sessionCtx *common.Session) (string, error) {
	a.Infof("Generating Azure access token for %v.", sessionCtx)
	return azureAccessToken, nil
}

func TestDBCertSigning(t *testing.T) {
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Clock:       clockwork.NewFakeClockAt(time.Now()),
		ClusterName: "local.me",
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, authServer.Close()) })

	ctx := context.Background()

	privateKey, _, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	csr, err := tlsca.GenerateCertificateRequestPEM(pkix.Name{
		CommonName: "localhost",
	}, privateKey)
	require.NoError(t, err)

	// Set rotation to init phase. New CA will be generated.
	// DB service should still use old key to sign certificates.
	// tctl should use new key to sign certificates.
	err = authServer.AuthServer.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.DatabaseCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	dbCAs, err := authServer.AuthServer.GetCertAuthorities(ctx, types.DatabaseCA, false)
	require.NoError(t, err)
	require.Len(t, dbCAs, 1)
	require.NotNil(t, dbCAs[0].GetActiveKeys().TLS)
	require.NotNil(t, dbCAs[0].GetAdditionalTrustedKeys().TLS)

	tests := []struct {
		name      string
		requester proto.DatabaseCertRequest_Requester
		getCertFn func(dbCAs []types.CertAuthority) []byte
	}{
		{
			name:      "sign from DB service",
			requester: proto.DatabaseCertRequest_UNSPECIFIED, // default behavior
			getCertFn: func(dbCAs []types.CertAuthority) []byte {
				return dbCAs[0].GetActiveKeys().TLS[0].Cert
			},
		},
		{
			name:      "sign from tctl",
			requester: proto.DatabaseCertRequest_TCTL,
			getCertFn: func(dbCAs []types.CertAuthority) []byte {
				return dbCAs[0].GetAdditionalTrustedKeys().TLS[0].Cert
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			certResp, err := authServer.AuthServer.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
				CSR:           csr,
				ServerName:    "localhost",
				TTL:           proto.Duration(time.Hour),
				RequesterName: tt.requester,
			})
			require.NoError(t, err)
			require.NotNil(t, certResp.Cert)
			require.Len(t, certResp.CACerts, 2)

			dbCert, err := tlsca.ParseCertificatePEM(certResp.Cert)
			require.NoError(t, err)

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM(tt.getCertFn(dbCAs))
			require.True(t, ok)

			opts := x509.VerifyOptions{
				Roots: certPool,
			}

			// Verify if the generated certificate can be verified with the correct CA.
			_, err = dbCert.Verify(opts)
			require.NoError(t, err)
		})
	}
}
