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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v3"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudazure "github.com/gravitational/teleport/lib/cloud/azure"
	cloudtest "github.com/gravitational/teleport/lib/cloud/test"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestAuthGetAzureCacheForRedisToken(t *testing.T) {
	t.Parallel()

	auth, err := NewAuth(AuthConfig{
		AuthClient: new(authClientMock),
		Clients: &cloud.TestCloudClients{
			AzureRedis: libcloudazure.NewRedisClientByAPI(&libcloudazure.ARMRedisMock{
				Token: "azure-redis-token",
			}),
			AzureRedisEnterprise: libcloudazure.NewRedisEnterpriseClientByAPI(nil, &libcloudazure.ARMRedisEnterpriseDatabaseMock{
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

func TestAuthGetRedshiftServerlessAuthToken(t *testing.T) {
	t.Parallel()

	clock := clockwork.NewFakeClock()
	auth, err := NewAuth(AuthConfig{
		Clock:      clock,
		AuthClient: new(authClientMock),
		Clients: &cloud.TestCloudClients{
			RedshiftServerless: &cloudtest.RedshiftServerlessMock{
				GetCredentialsOutput: cloudtest.RedshiftServerlessGetCredentialsOutput("IAM:some-user", "some-password", clock),
			},
		},
	})
	require.NoError(t, err)

	dbUser, dbPassword, err := auth.GetRedshiftServerlessAuthToken(context.TODO(), &Session{
		DatabaseUser: "some-user",
		DatabaseName: "some-database",
		Database:     newRedshiftServerlessDatabase(t),
	})
	require.NoError(t, err)
	require.Equal(t, "IAM:some-user", dbUser)
	require.Equal(t, "some-password", dbPassword)
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

	systemCertPoolWithCA := systemCertPool.Clone()
	systemCertPoolWithCA.AppendCertsFromPEM([]byte(fixtures.TLSCACertPEM))

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
			name:             "AWS RDS Proxy",
			sessionDatabase:  newRDSProxyDatabase(t, "my-proxy.proxy-abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432"),
			expectServerName: "my-proxy.proxy-abcdefghijklmnop.us-east-1.rds.amazonaws.com",
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
		{
			name:             "Azure SQL Server",
			sessionDatabase:  newAzureSQLDatabase(t, "resource-id"),
			expectServerName: "test-database.database.windows.net",
			expectRootCAs:    systemCertPool,
		},
		{
			name:             "Azure Postgres with downloaded CA",
			sessionDatabase:  newAzurePostgresDatabaseWithCA(t, fixtures.TLSCACertPEM),
			expectServerName: "my-postgres.postgres.database.azure.com",
			expectRootCAs:    systemCertPoolWithCA,
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
			require.True(t, test.expectRootCAs.Equal(tlsConfig.RootCAs))

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

func TestGetAzureIdentityResourceID(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		desc                string
		identityName        string
		clients             *cloud.TestCloudClients
		errAssertion        require.ErrorAssertionFunc
		resourceIDAssertion require.ValueAssertionFunc
	}{
		{
			desc:         "running on Azure and identity is attached",
			identityName: "identity",
			clients: &cloud.TestCloudClients{
				InstanceMetadata: &imdsMock{
					id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
					instanceType: types.InstanceMetadataTypeAzure,
				},
				AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(&libcloudazure.ARMComputeMock{
					GetResult: generateAzureVM(t, []string{identityResourceID(t, "identity")}),
				}),
			},
			errAssertion: require.NoError,
			resourceIDAssertion: func(requireT require.TestingT, value interface{}, _ ...interface{}) {
				require.Equal(requireT, identityResourceID(t, "identity"), value)
			},
		},
		{
			desc:         "running on Azure without the identity",
			identityName: "random-identity-not-attached",
			clients: &cloud.TestCloudClients{
				InstanceMetadata: &imdsMock{
					id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
					instanceType: types.InstanceMetadataTypeAzure,
				},
				AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(&libcloudazure.ARMComputeMock{
					GetResult: generateAzureVM(t, []string{identityResourceID(t, "identity")}),
				}),
			},
			errAssertion:        require.Error,
			resourceIDAssertion: require.Empty,
		},
		{
			desc:         "running on Azure wrong format identity",
			identityName: "identity",
			clients: &cloud.TestCloudClients{
				InstanceMetadata: &imdsMock{
					id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
					instanceType: types.InstanceMetadataTypeAzure,
				},
				AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(&libcloudazure.ARMComputeMock{
					GetResult: generateAzureVM(t, []string{"identity"}),
				}),
			},
			errAssertion:        require.Error,
			resourceIDAssertion: require.Empty,
		},
		{
			desc:         "running outside of Azure",
			identityName: "identity",
			clients: &cloud.TestCloudClients{
				InstanceMetadata: &imdsMock{
					id:           "i-1234567890abcdef0",
					instanceType: types.InstanceMetadataTypeEC2,
				},
			},
			errAssertion:        require.Error,
			resourceIDAssertion: require.Empty,
		},
		{
			desc:         "running on azure but failed to get VM",
			identityName: "random-identity-not-attached",
			clients: &cloud.TestCloudClients{
				InstanceMetadata: &imdsMock{
					id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
					instanceType: types.InstanceMetadataTypeAzure,
				},
				AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(&libcloudazure.ARMComputeMock{
					GetErr: errors.New("failed to get VM"),
				}),
			},
			errAssertion:        require.Error,
			resourceIDAssertion: require.Empty,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			auth, err := NewAuth(AuthConfig{
				AuthClient: new(authClientMock),
				Clients:    tc.clients,
			})
			require.NoError(t, err)

			resourceID, err := auth.GetAzureIdentityResourceID(ctx, tc.identityName)
			tc.errAssertion(t, err)
			tc.resourceIDAssertion(t, resourceID)
		})
	}
}

func TestGetAzureIdentityResourceIDCache(t *testing.T) {
	ctx := context.Background()
	identityName := "identity"
	virtualMachinesMock := &libcloudazure.ARMComputeMock{
		GetErr: errors.New("failed to fetch VM"),
	}

	clock := clockwork.NewFakeClock()

	auth, err := NewAuth(AuthConfig{
		Clock:      clock,
		AuthClient: new(authClientMock),
		Clients: &cloud.TestCloudClients{
			InstanceMetadata: &imdsMock{
				id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
				instanceType: types.InstanceMetadataTypeAzure,
			},
			AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(virtualMachinesMock),
		},
	})
	require.NoError(t, err)

	// First fetch will return an error.
	resourceID, err := auth.GetAzureIdentityResourceID(ctx, identityName)
	require.Error(t, err)
	require.Empty(t, resourceID)

	// Change mock to return the VM.
	virtualMachinesMock.GetErr = nil
	virtualMachinesMock.GetResult = generateAzureVM(t, []string{identityResourceID(t, "identity")})

	// Advance the clock to force cache expiration.
	clock.Advance(azureVirtualMachineCacheTTL + time.Second)

	// Second fetch succeeds and return the matched identity.
	resourceID, err = auth.GetAzureIdentityResourceID(ctx, identityName)
	require.NoError(t, err)
	require.Equal(t, identityResourceID(t, "identity"), resourceID)

	// Change mock back to return an error.
	virtualMachinesMock.GetErr = errors.New("failed to fetch VM")

	// Third fetch succeeds and return the cached identity.
	resourceID, err = auth.GetAzureIdentityResourceID(ctx, identityName)
	require.NoError(t, err)
	require.Equal(t, identityResourceID(t, "identity"), resourceID)
}

func TestRedshiftServerlessUsernameToRoleARN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputUsername string
		expectRoleARN string
		expectError   bool
	}{
		{
			inputUsername: "arn:aws:iam::1234567890:role/rolename",
			expectRoleARN: "arn:aws:iam::1234567890:role/rolename",
		},
		{
			inputUsername: "arn:aws:iam::1234567890:user/user",
			expectError:   true,
		},
		{
			inputUsername: "arn:aws:not-iam::1234567890:role/rolename",
			expectError:   true,
		},
		{
			inputUsername: "role/rolename",
			expectRoleARN: "arn:aws:iam::1234567890:role/rolename",
		},
		{
			inputUsername: "rolename",
			expectRoleARN: "arn:aws:iam::1234567890:role/rolename",
		},
		{
			inputUsername: "IAM:user",
			expectError:   true,
		},
		{
			inputUsername: "IAMR:rolename",
			expectError:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.inputUsername, func(t *testing.T) {
			actualRoleARN, err := redshiftServerlessUsernameToRoleARN(newRedshiftServerlessDatabase(t).GetAWS(), test.inputUsername)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectRoleARN, actualRoleARN)
			}
		})
	}
}

func newAzureRedisDatabase(t *testing.T, resourceID string) types.Database {
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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
	t.Helper()

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

func newRedshiftServerlessDatabase(t *testing.T) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "my-workgroup.1234567890.eu-west-2.redshift-serverless.amazonaws.com:5439",
	})
	require.NoError(t, err)
	return database
}

func newRDSProxyDatabase(t *testing.T, uri string) types.Database {
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      uri,
		AWS: types.AWS{
			Region: "us-east-1",
			RDSProxy: types.RDSProxy{
				Name: "test-database",
			},
		},
	})
	require.NoError(t, err)
	return database
}

func newAzurePostgresDatabaseWithCA(t *testing.T, ca string) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "my-postgres.postgres.database.azure.com:5432",
	})
	require.NoError(t, err)

	database.SetStatusCA(ca)
	return database
}

func newAzureSQLDatabase(t *testing.T, resourceID string) types.Database {
	t.Helper()
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolSQLServer,
		URI:      "test-database.database.windows.net:1433",
		Azure: types.Azure{
			ResourceID: resourceID,
		},
	})
	require.NoError(t, err)
	return database
}

// identityResourceID generates full resource ID of the Azure user identity.
func identityResourceID(t *testing.T, identityName string) string {
	t.Helper()
	return fmt.Sprintf("/subscriptions/sub-id/resourceGroups/group-name/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s", identityName)
}

// generateAzureVM generates Azure VM resource.
func generateAzureVM(t *testing.T, identities []string) armcompute.VirtualMachine {
	t.Helper()

	identitiesMap := make(map[string]*armcompute.UserAssignedIdentitiesValue)
	for _, identity := range identities {
		identitiesMap[identity] = &armcompute.UserAssignedIdentitiesValue{}
	}

	return armcompute.VirtualMachine{
		ID:   to.Ptr("/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm"),
		Name: to.Ptr("vm"),
		Identity: &armcompute.VirtualMachineIdentity{
			PrincipalID:            to.Ptr("00000000-0000-0000-0000-000000000000"),
			UserAssignedIdentities: identitiesMap,
		},
	}
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

// imdsMock is a mock that implements InstanceMetadata interface.
type imdsMock struct {
	cloud.InstanceMetadata
	// GetID mocks.
	id    string
	idErr error
	// GetType mocks.
	instanceType types.InstanceMetadataType
}

func (m *imdsMock) GetID(_ context.Context) (string, error) {
	return m.id, m.idErr
}

func (m *imdsMock) GetType() types.InstanceMetadataType {
	return m.instanceType
}
