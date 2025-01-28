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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	libcloudazure "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/imds"
	"github.com/gravitational/teleport/lib/cloud/mocks"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func TestAuthGetAzureCacheForRedisToken(t *testing.T) {
	t.Parallel()

	auth, err := NewAuth(AuthConfig{
		AuthClient:  new(authClientMock),
		AccessPoint: new(accessPointMock),
		Clients: &cloud.TestCloudClients{
			AzureRedis: libcloudazure.NewRedisClientByAPI(&libcloudazure.ARMRedisMock{
				Token: "azure-redis-token",
			}),
			AzureRedisEnterprise: libcloudazure.NewRedisEnterpriseClientByAPI(nil, &libcloudazure.ARMRedisEnterpriseDatabaseMock{
				Token: "azure-redis-enterprise-token",
			}),
		},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
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
			token, err := auth.GetAzureCacheForRedisToken(context.TODO(), newAzureRedisDatabase(t, test.resourceID))
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

	// setup mock aws sessions.
	stsMock := &mocks.STSClient{}
	clock := clockwork.NewFakeClock()
	auth, err := NewAuth(AuthConfig{
		Clock:             clock,
		AuthClient:        new(authClientMock),
		AccessPoint:       new(accessPointMock),
		Clients:           &cloud.TestCloudClients{},
		AWSConfigProvider: &mocks.AWSConfigProvider{STSClient: stsMock},
		awsClients: fakeAWSClients{
			rssClient: &mocks.RedshiftServerlessClient{
				GetCredentialsOutput: mocks.RedshiftServerlessGetCredentialsOutput("IAM:some-user", "some-password", clock),
			},
		},
	})
	require.NoError(t, err)

	dbUser, dbPassword, err := auth.GetRedshiftServerlessAuthToken(context.TODO(),
		newRedshiftServerlessDatabase(t),
		"some-user",
		"some-database",
	)
	require.NoError(t, err)
	require.Equal(t, "IAM:some-user", dbUser)
	require.Equal(t, "some-password", dbPassword)
	require.Equal(t, []string{"arn:aws:iam::123456789012:role/some-user"}, stsMock.GetAssumedRoleARNs())
	require.Equal(t, []string{""}, stsMock.GetAssumedRoleExternalIDs())
}

func TestAuthGetTLSConfig(t *testing.T) {
	t.Parallel()

	auth, err := NewAuth(AuthConfig{
		AuthClient:        new(authClientMock),
		AccessPoint:       new(accessPointMock),
		Clients:           &cloud.TestCloudClients{},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
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
			name:                     "self-hosted with trust_system_cert_pool",
			sessionDatabase:          newSelfHostedDatabaseWithTrustSytemCertPool(t, "postgres.dev.example.com:8888"),
			expectServerName:         "postgres.dev.example.com",
			expectRootCAs:            systemCertPoolWithCA,
			expectClientCertificates: true,
		},
		{
			name:            "AWS ElastiCache Redis",
			sessionDatabase: newElastiCacheRedisDatabase(t, withCA(fixtures.SAMLOktaCertPEM)),
			expectRootCAs:   awsCertPool,
		},
		{
			name:             "AWS Redshift",
			sessionDatabase:  newRedshiftDatabase(t, withCA(fixtures.SAMLOktaCertPEM)),
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
			name:             "GCP Spanner",
			sessionDatabase:  newSpannerDatabase(t, ""),
			expectServerName: "spanner.googleapis.com",
			expectRootCAs:    systemCertPool,
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
			tlsConfig, err := auth.GetTLSConfig(context.TODO(),
				time.Now().Add(time.Hour),
				test.sessionDatabase,
				"defaultUser")
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
				AuthClient:        new(authClientMock),
				AccessPoint:       new(accessPointMock),
				Clients:           tc.clients,
				AWSConfigProvider: &mocks.AWSConfigProvider{},
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
		Clock:       clock,
		AuthClient:  new(authClientMock),
		AccessPoint: new(accessPointMock),
		Clients: &cloud.TestCloudClients{
			InstanceMetadata: &imdsMock{
				id:           "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg/providers/microsoft.compute/virtualmachines/vm",
				instanceType: types.InstanceMetadataTypeAzure,
			},
			AzureVirtualMachines: libcloudazure.NewVirtualMachinesClientByAPI(virtualMachinesMock),
		},
		AWSConfigProvider: &mocks.AWSConfigProvider{},
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
			inputUsername: "arn:aws:iam::123456789012:role/rolename",
			expectRoleARN: "arn:aws:iam::123456789012:role/rolename",
		},
		{
			inputUsername: "arn:aws:iam::123456789012:user/user",
			expectError:   true,
		},
		{
			inputUsername: "arn:aws:not-iam::123456789012:role/rolename",
			expectError:   true,
		},
		{
			inputUsername: "role/rolename",
			expectRoleARN: "arn:aws:iam::123456789012:role/rolename",
		},
		{
			inputUsername: "rolename",
			expectRoleARN: "arn:aws:iam::123456789012:role/rolename",
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

func TestAuthGetAWSTokenWithAssumedRole(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	tests := map[string]struct {
		checkGetAuthFn func(t *testing.T, auth Auth)
		checkSTS       func(t *testing.T, stsMock *mocks.STSClient)
	}{
		"Redshift": {
			checkGetAuthFn: func(t *testing.T, auth Auth) {
				t.Helper()
				databaseUser := "some-user"
				databaseName := "some-database"
				database := newRedshiftDatabase(t,
					withCA(fixtures.SAMLOktaCertPEM),
					withAssumeRole(types.AssumeRole{
						RoleARN:    "arn:aws:iam::123456789012:role/RedshiftRole",
						ExternalID: "externalRedshift",
					}))

				dbUser, dbPassword, err := auth.GetRedshiftAuthToken(ctx, database, databaseUser, databaseName)
				require.NoError(t, err)
				require.Equal(t, "IAM:some-user", dbUser)
				require.Equal(t, "some-password", dbPassword)
			},
			checkSTS: func(t *testing.T, stsMock *mocks.STSClient) {
				t.Helper()
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/RedshiftRole")
				require.Contains(t, stsMock.GetAssumedRoleExternalIDs(), "externalRedshift")
			},
		},
		"Redshift with IAM role": {
			checkGetAuthFn: func(t *testing.T, auth Auth) {
				t.Helper()
				databaseUser := "role/some-role"
				databaseName := "some-database"
				database := newRedshiftDatabase(t,
					withCA(fixtures.SAMLOktaCertPEM),
					withAssumeRole(types.AssumeRole{
						RoleARN:    "arn:aws:iam::123456789012:role/RedshiftRole",
						ExternalID: "externalRedshift",
					}))

				dbUser, dbPassword, err := auth.GetRedshiftAuthToken(ctx, database, databaseUser, databaseName)
				require.NoError(t, err)
				require.Equal(t, "IAM:some-role", dbUser)
				require.Equal(t, "some-password-for-some-role", dbPassword)
			},
			checkSTS: func(t *testing.T, stsMock *mocks.STSClient) {
				t.Helper()
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/RedshiftRole")
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/some-role")
				require.Contains(t, stsMock.GetAssumedRoleExternalIDs(), "externalRedshift")
			},
		},
		"Redshift Serverless": {
			checkGetAuthFn: func(t *testing.T, auth Auth) {
				t.Helper()
				databaseUser := "some-user"
				databaseName := "some-database"
				database := newRedshiftServerlessDatabase(t,
					withAssumeRole(types.AssumeRole{
						RoleARN:    "arn:aws:iam::123456789012:role/RedshiftServerlessRole",
						ExternalID: "externalRedshiftServerless",
					}))

				dbUser, dbPassword, err := auth.GetRedshiftServerlessAuthToken(ctx, database, databaseUser, databaseName)
				require.NoError(t, err)
				require.Equal(t, "IAM:some-user", dbUser)
				require.Equal(t, "some-password", dbPassword)
			},
			checkSTS: func(t *testing.T, stsMock *mocks.STSClient) {
				t.Helper()
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/RedshiftServerlessRole")
				require.Contains(t, stsMock.GetAssumedRoleExternalIDs(), "externalRedshiftServerless")
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/some-user")
			},
		},
		"RDS Proxy": {
			checkGetAuthFn: func(t *testing.T, auth Auth) {
				t.Helper()
				databaseUser := "some-user"
				database := newRDSProxyDatabase(t, "my-proxy.proxy-abcdefghijklmnop.us-east-1.rds.amazonaws.com:5432",
					withAssumeRole(types.AssumeRole{
						RoleARN:    "arn:aws:iam::123456789012:role/RDSProxyRole",
						ExternalID: "externalRDSProxy",
					}))
				token, err := auth.GetRDSAuthToken(ctx, database, databaseUser)
				require.NoError(t, err)
				require.Contains(t, token, "DBUser=some-user")
			},
			checkSTS: func(t *testing.T, stsMock *mocks.STSClient) {
				t.Helper()
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/RDSProxyRole")
				require.Contains(t, stsMock.GetAssumedRoleExternalIDs(), "externalRDSProxy")
			},
		},
		"ElastiCache Redis": {
			checkGetAuthFn: func(t *testing.T, auth Auth) {
				t.Helper()
				databaseUser := "some-user"
				database := newElastiCacheRedisDatabase(t,
					withAssumeRole(types.AssumeRole{
						RoleARN:    "arn:aws:iam::123456789012:role/RedisRole",
						ExternalID: "externalElastiCacheRedis",
					}))
				token, err := auth.GetElastiCacheRedisToken(ctx, database, databaseUser)
				require.NoError(t, err)
				u, err := url.Parse(token)
				require.NoError(t, err)
				require.Equal(t, "example-cluster/", u.Path)
				query := u.Query()
				require.Equal(t, "connect", query.Get("Action"))
				require.Equal(t, "some-user", query.Get("User"))
				require.Equal(t, "host", query.Get("X-Amz-SignedHeaders"))
				require.Equal(t, "token", query.Get("X-Amz-Security-Token"))
				require.Equal(t, "FAKEACCESSKEYID/20010203/ca-central-1/elasticache/aws4_request",
					query.Get("X-Amz-Credential"))
			},
			checkSTS: func(t *testing.T, stsMock *mocks.STSClient) {
				t.Helper()
				require.Contains(t, stsMock.GetAssumedRoleARNs(), "arn:aws:iam::123456789012:role/RedisRole")
				require.Contains(t, stsMock.GetAssumedRoleExternalIDs(), "externalElastiCacheRedis")
			},
		},
	}

	fakeSTS := &mocks.STSClient{}
	clock := clockwork.NewFakeClockAt(time.Date(2001, time.February, 3, 0, 0, 0, 0, time.UTC))
	auth, err := NewAuth(AuthConfig{
		Clock:       clock,
		AuthClient:  new(authClientMock),
		AccessPoint: new(accessPointMock),
		Clients: &cloud.TestCloudClients{
			STS: &fakeSTS.STSClientV1,
		},
		AWSConfigProvider: &mocks.AWSConfigProvider{
			STSClient: fakeSTS,
		},
		awsClients: fakeAWSClients{
			redshiftClient: &mocks.RedshiftClient{
				GetClusterCredentialsOutput:        mocks.RedshiftGetClusterCredentialsOutput("IAM:some-user", "some-password", clock),
				GetClusterCredentialsWithIAMOutput: mocks.RedshiftGetClusterCredentialsWithIAMOutput("IAM:some-role", "some-password-for-some-role", clock),
			},
			rssClient: &mocks.RedshiftServerlessClient{
				GetCredentialsOutput: mocks.RedshiftServerlessGetCredentialsOutput("IAM:some-user", "some-password", clock),
			},
			stsClient: fakeSTS,
		},
	})
	require.NoError(t, err)

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tt.checkGetAuthFn(t, auth)
			tt.checkSTS(t, fakeSTS)
		})
	}
}

func TestGetAWSIAMCreds(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	ctx := context.Background()

	for name, tt := range map[string]struct {
		db                   types.Database
		stsMock              *mocks.STSClient
		username             string
		expectedAssumedRoles []string
		expectedExternalIDs  []string
		wantErrContains      string
	}{
		"username is full role ARN": {
			db:                   newMongoAtlasDatabase(t, types.AWS{}),
			stsMock:              &mocks.STSClient{},
			username:             "arn:aws:iam::123456789012:role/role-name",
			expectedAssumedRoles: []string{"arn:aws:iam::123456789012:role/role-name"},
			expectedExternalIDs:  []string{""},
		},
		"username is partial role ARN": {
			db: newMongoAtlasDatabase(t, types.AWS{}),
			stsMock: &mocks.STSClient{
				STSClientV1: mocks.STSClientV1{
					// This is the role returned by the STS GetCallerIdentity.
					ARN: "arn:aws:iam::222222222222:role/teleport-service-role",
				},
			},
			username:             "role/role-name",
			expectedAssumedRoles: []string{"arn:aws:iam::222222222222:role/role-name"},
			expectedExternalIDs:  []string{""},
		},
		"unable to fetch account ID": {
			db: newMongoAtlasDatabase(t, types.AWS{}),
			stsMock: &mocks.STSClient{
				Unauth: true,
			},
			username:        "role/role-name",
			wantErrContains: "unauthorized",
		},
		"chained IAM role": {
			db: newMongoAtlasDatabase(t, types.AWS{
				ExternalID:    "123123",
				AssumeRoleARN: "arn:aws:iam::222222222222:role/teleport-service-role-external",
			}),
			stsMock: &mocks.STSClient{
				STSClientV1: mocks.STSClientV1{
					ARN: "arn:aws:iam::111111111111:role/teleport-service-role",
				},
			},
			username: "role/role-name",
			expectedAssumedRoles: []string{
				"arn:aws:iam::222222222222:role/teleport-service-role-external",
				"arn:aws:iam::222222222222:role/role-name",
			},
			expectedExternalIDs: []string{"123123", ""},
		},
	} {
		t.Run(name, func(t *testing.T) {
			auth, err := NewAuth(AuthConfig{
				Clock:       clock,
				AuthClient:  new(authClientMock),
				AccessPoint: new(accessPointMock),
				Clients: &cloud.TestCloudClients{
					STS: &tt.stsMock.STSClientV1,
				},
				AWSConfigProvider: &mocks.AWSConfigProvider{},
				awsClients: fakeAWSClients{
					stsClient: tt.stsMock,
				},
			})
			require.NoError(t, err)

			keyId, _, _, err := auth.GetAWSIAMCreds(ctx, tt.db, tt.username)
			if tt.wantErrContains != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErrContains)
				return
			}
			require.Equal(t, "FAKEACCESSKEYID", keyId)
			require.ElementsMatch(t, tt.expectedAssumedRoles, tt.stsMock.GetAssumedRoleARNs())
			require.ElementsMatch(t, tt.expectedExternalIDs, tt.stsMock.GetAssumedRoleExternalIDs())
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

func newSelfHostedDatabaseWithTrustSytemCertPool(t *testing.T, uri string) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMySQL,
		URI:      uri,
		TLS: types.DatabaseTLS{
			TrustSystemCertPool: true,
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

func newMongoAtlasDatabase(t *testing.T, aws types.AWS) types.Database {
	t.Helper()

	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolMongoDB,
		URI:      "test.xxxxxxx.mongodb.net",
		MongoAtlas: types.MongoAtlas{
			Name: "test",
		},
		AWS: aws,
	})
	require.NoError(t, err)
	return database
}

type databaseSpecOpt func(spec *types.DatabaseSpecV3)

func withCA(ca string) databaseSpecOpt {
	return func(spec *types.DatabaseSpecV3) {
		spec.TLS.CACert = ca
	}
}

func withAssumeRole(assumeRole types.AssumeRole) databaseSpecOpt {
	return func(spec *types.DatabaseSpecV3) {
		spec.AWS.AssumeRoleARN = assumeRole.RoleARN
		spec.AWS.ExternalID = assumeRole.ExternalID
	}
}

func newElastiCacheRedisDatabase(t *testing.T, specOpts ...databaseSpecOpt) types.Database {
	t.Helper()

	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolRedis,
		URI:      "master.example-cluster.xxxxxx.cac1.cache.amazonaws.com:6379",
	}
	for _, opt := range specOpts {
		opt(&spec)
	}
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, spec)
	require.NoError(t, err)
	return database
}

func newRedshiftDatabase(t *testing.T, specOpts ...databaseSpecOpt) types.Database {
	t.Helper()

	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "redshift-cluster-1.abcdefghijklmnop.us-east-1.redshift.amazonaws.com:5432",
	}
	for _, opt := range specOpts {
		opt(&spec)
	}
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, spec)
	require.NoError(t, err)
	return database
}

func newRedshiftServerlessDatabase(t *testing.T, specOpts ...databaseSpecOpt) types.Database {
	t.Helper()

	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "my-workgroup.123456789012.eu-west-2.redshift-serverless.amazonaws.com:5439",
	}
	for _, opt := range specOpts {
		opt(&spec)
	}
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, spec)
	require.NoError(t, err)
	return database
}

func newRDSProxyDatabase(t *testing.T, uri string, specOpts ...databaseSpecOpt) types.Database {
	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      uri,
		AWS: types.AWS{
			AccountID: "123456789012",
			RDSProxy: types.RDSProxy{
				Name: "test-database",
			},
		},
	}
	for _, opt := range specOpts {
		opt(&spec)
	}
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, spec)
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

func newSpannerDatabase(t *testing.T, uri string, specOpts ...databaseSpecOpt) types.Database {
	spec := types.DatabaseSpecV3{
		Protocol: defaults.ProtocolSpanner,
		URI:      uri,
		GCP: types.GCPCloudSQL{
			ProjectID:  "project-id",
			InstanceID: "instance-id",
		},
	}
	for _, opt := range specOpts {
		opt(&spec)
	}
	database, err := types.NewDatabaseV3(types.Metadata{
		Name: "test-database",
	}, spec)
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
type authClientMock struct{}

// GenerateDatabaseCert generates a cert using fixtures TLS CA.
func (m *authClientMock) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	if req.GetRequesterName() != proto.DatabaseCertRequest_UNSPECIFIED {
		return nil, trace.BadParameter("db agent should not specify requester name")
	}
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

type accessPointMock struct{}

// GetAuthPreference always returns types.DefaultAuthPreference().
func (m accessPointMock) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return types.DefaultAuthPreference(), nil
}

// imdsMock is a mock that implements the [imds.Client] interface.
type imdsMock struct {
	imds.Client
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

type fakeAWSClients struct {
	redshiftClient redshiftClient
	rssClient      rssClient
	stsClient      stsClient
}

func (f fakeAWSClients) getRedshiftClient(aws.Config, ...func(*redshift.Options)) redshiftClient {
	return f.redshiftClient
}

func (f fakeAWSClients) getRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) rssClient {
	return f.rssClient
}

func (f fakeAWSClients) getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient {
	return f.stsClient
}
