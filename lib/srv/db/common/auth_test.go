/*
Copyright 2022 Gravitational, Inc.

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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudazure "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
)

func TestAuthGetAzureCacheForRedisToken(t *testing.T) {
	t.Parallel()

	auth, err := NewAuth(AuthConfig{
		AuthClient: new(authClientMock),
		Clients: &cloud.TestCloudClients{
			AzureRedis: libcloudazure.NewRedisClientByAPI(&libcloudazure.ARMRedisMock{
				Token: "azure-redis-token",
			}),
			AzureRedisEnterprise: libcloudazure.NewRedisEnterpriseClientByAPI(&libcloudazure.ARMRedisEnterpriseDatabaseMock{
				Token: "azure-redis-enterprise-token",
			}),
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		resourceID  string
		expectError bool
		expectToken string
	}{
		{
			name:        "invalid resource ID",
			resourceID:  "/subscriptions/sub-id/resourceGroups/group-name/providers/some-unknown-service/example-teleport",
			expectError: true,
		},
		{
			name:        "Redis (non-Enterprise)",
			resourceID:  "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/Redis/example-teleport",
			expectToken: "azure-redis-token",
		},
		{
			name:        "Redis Enterprise",
			resourceID:  "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport",
			expectToken: "azure-redis-enterprise-token",
		},
		{
			name:        "Redis Enterprise (database resource ID)",
			resourceID:  "/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.Cache/redisEnterprise/example-teleport/databases/default",
			expectToken: "azure-redis-enterprise-token",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token, err := auth.GetAzureCacheForRedisToken(context.TODO(), &Session{
				Database: newAzureRedisDatabase(t, test.resourceID),
			})
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectToken, token)
			}
		})
	}
}

func TestAuthGetTLSConfig(t *testing.T) {
	t.Parallel()

	auth, err := NewAuth(AuthConfig{
		AuthClient: new(authClientMock),
		Clients:    &cloud.TestCloudClients{},
	})
	require.NoError(t, err)

	systemCertPool, err := x509.SystemCertPool()
	require.NoError(t, err)

	// The authClientMock uses fixtures.TLSCACertPEM as the root signing CA.
	defaultCertPool := x509.NewCertPool()
	require.True(t, defaultCertPool.AppendCertsFromPEM([]byte(fixtures.TLSCACertPEM)))

	// Use a different CA to pretend to be CAs for AWS hosted databases.
	awsCertPool := x509.NewCertPool()
	require.True(t, awsCertPool.AppendCertsFromPEM([]byte(fixtures.SAMLOktaCertPEM)))

	tests := []struct {
		name                     string
		sessionDatabase          types.Database
		expectServerName         string
		expectRootCAs            *x509.CertPool
		expectClientCertificates bool
		expectVerifyConnection   bool
		expectInsecureSkipVerify bool
	}{
		{
			name:                     "self-hosted",
			sessionDatabase:          newSelfHostedDatabase(t, "localhost:8888"),
			expectServerName:         "localhost",
			expectRootCAs:            defaultCertPool,
			expectClientCertificates: true,
		},
		{
			name:            "AWS ElastiCache Redis",
			sessionDatabase: newElastiCacheRedisDatabase(t, fixtures.SAMLOktaCertPEM),
			expectRootCAs:   awsCertPool,
		},
		{
			name:             "AWS Redishift",
			sessionDatabase:  newRedshiftDatabase(t, fixtures.SAMLOktaCertPEM),
			expectServerName: "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com",
			expectRootCAs:    awsCertPool,
		},
		{
			name:             "Azure Redis",
			sessionDatabase:  newAzureRedisDatabase(t, "resource-id"),
			expectServerName: "test-database.redis.cache.windows.net",
			expectRootCAs:    systemCertPool,
		},
		{
			name:            "GCP Cloud SQL",
			sessionDatabase: newCloudSQLDatabase(t, "project-id", "instance-id"),
			// RootCAs is empty, and custom VerifyConnection function is provided.
			expectServerName:         "project-id:instance-id",
			expectRootCAs:            x509.NewCertPool(),
			expectInsecureSkipVerify: true,
			expectVerifyConnection:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tlsConfig, err := auth.GetTLSConfig(context.TODO(), &Session{
				Identity:     tlsca.Identity{},
				DatabaseUser: "default",
				Database:     test.sessionDatabase,
			})
			require.NoError(t, err)

			require.Equal(t, test.expectServerName, tlsConfig.ServerName)
			require.Equal(t, test.expectInsecureSkipVerify, tlsConfig.InsecureSkipVerify)

			// nolint:staticcheck
			// TODO x509.CertPool.Subjects() is deprecated. use
			// x509.CertPool.Equal introduced in 1.19 for comparison.
			require.Equal(t, test.expectRootCAs.Subjects(), tlsConfig.RootCAs.Subjects())

			if test.expectClientCertificates {
				require.Len(t, tlsConfig.Certificates, 1)
			} else {
				require.Empty(t, tlsConfig.Certificates)
			}

			if test.expectVerifyConnection {
				require.NotNil(t, tlsConfig.VerifyConnection)
			} else {
				require.Nil(t, tlsConfig.VerifyConnection)
			}
		})
	}
}

func newAzureRedisDatabase(t *testing.T, resourceID string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "rediss://test-database.redis.cache.windows.net:8888",
		Azure: types.Azure{
			ResourceID: resourceID,
		},
	})
	require.NoError(t, err)
	return database
}

func newSelfHostedDatabase(t *testing.T, uri string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      uri,
	})
	require.NoError(t, err)
	return database
}

func newCloudSQLDatabase(t *testing.T, projectID, instanceID string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      "cloudsql:8888",
		GCP: types.GCPCloudSQL{
			ProjectID:  projectID,
			InstanceID: instanceID,
		},
	})
	require.NoError(t, err)
	return database
}

func newElastiCacheRedisDatabase(t *testing.T, ca string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "master.example-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
		TLS: types.DatabaseTLS{
			CACert: ca,
		},
	})
	require.NoError(t, err)
	return database
}

func newRedshiftDatabase(t *testing.T, ca string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
		TLS: types.DatabaseTLS{
			CACert: ca,
		},
	})
	require.NoError(t, err)
	return database
}

// authClientMock is a mock that implements AuthClient interface.
type authClientMock struct {
}

// GenerateDatabaseCert generates a cert using fixtures TLS CA.
func (m *authClientMock) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCACert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromTLSCertificate(tlsCACert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certReq := tlsca.CertificateRequest{
		PublicKey: csr.PublicKey,
		Subject:   csr.Subject,
		NotAfter:  time.Now().Add(req.TTL.Get()),
		DNSNames:  []string{"localhost", "127.0.0.1"},
	}
	cert, err := tlsCA.GenerateCertificate(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.DatabaseCertResponse{
		Cert: cert,
		CACerts: [][]byte{
			[]byte(fixtures.TLSCACertPEM),
		},
	}, nil
}

// GetAuthPreference always returns types.DefaultAuthPreference().
func (m *authClientMock) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}
