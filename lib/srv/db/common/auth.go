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
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gcpcredentialspb "cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/aws/aws-sdk-go/service/memorydb"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
	"github.com/aws/aws-sdk-go/service/redshift"
	"github.com/aws/aws-sdk-go/service/redshiftserverless"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	libazure "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// azureVirtualMachineCacheTTL is the default TTL for Azure virtual machine
// cache entries.
const azureVirtualMachineCacheTTL = 5 * time.Minute

// Auth defines interface for creating auth tokens and TLS configurations.
type Auth interface {
	// GetRDSAuthToken generates RDS/Aurora auth token.
	GetRDSAuthToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetRedshiftAuthToken generates Redshift auth token.
	GetRedshiftAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error)
	// GetRedshiftServerlessAuthToken generates Redshift Serverless auth token.
	GetRedshiftServerlessAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error)
	// GetElastiCacheRedisToken generates an ElastiCache Redis auth token.
	GetElastiCacheRedisToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetMemoryDBToken generates a MemoryDB auth token.
	GetMemoryDBToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetCloudSQLAuthToken generates Cloud SQL auth token.
	GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetCloudSQLPassword generates password for a Cloud SQL database user.
	GetCloudSQLPassword(ctx context.Context, sessionCtx *Session) (string, error)
	// GetAzureAccessToken generates Azure database access token.
	GetAzureAccessToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
	GetAzureCacheForRedisToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetTLSConfig builds the client TLS configuration for the session.
	GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error)
	// GetAuthPreference returns the cluster authentication config.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	// GetAzureIdentityResourceID returns the Azure identity resource ID
	// attached to the current compute instance. If Teleport is not running on
	// Azure VM returns an error.
	GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error)
	// GetAWSIAMCreds returns the AWS IAM credentials, including access key,
	// secret access key and session token.
	GetAWSIAMCreds(ctx context.Context, sessionCtx *Session) (string, string, string, error)
	// Closer releases all resources used by authenticator.
	io.Closer
}

// AuthClient is an interface that defines a subset of libauth.Client's
// functions that are required for database auth.
type AuthClient interface {
	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
	// GetAuthPreference returns the cluster authentication config.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
}

// AuthConfig is the database access authenticator configuration.
type AuthConfig struct {
	// AuthClient is the cluster auth client.
	AuthClient AuthClient
	// Clients provides interface for obtaining cloud provider clients.
	Clients cloud.Clients
	// Clock is the clock implementation.
	Clock clockwork.Clock
	// Log is used for logging.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *AuthConfig) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.Clients == nil {
		cloudClients, err := cloud.NewClients()
		if err != nil {
			return trace.Wrap(err)
		}
		c.Clients = cloudClients
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, "db:auth")
	}
	return nil
}

// dbAuth provides utilities for creating TLS configurations and
// generating auth tokens when connecting to databases.
type dbAuth struct {
	cfg AuthConfig
	// azureVirtualMachineCache caches the current Azure virtual machine.
	// Avoiding the need to query the metadata server on every database
	// connection.
	azureVirtualMachineCache *utils.FnCache
}

// NewAuth returns a new instance of database access authenticator.
func NewAuth(config AuthConfig) (Auth, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	azureVirtualMachineCache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:   azureVirtualMachineCacheTTL,
		Clock: config.Clock,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &dbAuth{
		cfg:                      config,
		azureVirtualMachineCache: azureVirtualMachineCache,
	}, nil
}

// GetRDSAuthToken returns authorization token that will be used as a password
// when connecting to RDS and Aurora databases.
func (a *dbAuth) GetRDSAuthToken(ctx context.Context, sessionCtx *Session) (string, error) {
	meta := sessionCtx.Database.GetAWS()
	awsSession, err := a.cfg.Clients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating RDS auth token for %s.", sessionCtx)
	token, err := rdsutils.BuildAuthToken(
		sessionCtx.Database.GetURI(),
		meta.Region,
		sessionCtx.DatabaseUser,
		awsSession.Config.Credentials)
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", trace.AccessDenied(`Could not generate RDS IAM auth token:

  %v

Make sure that Teleport database agent's IAM policy is attached and has "rds-connect"
permissions (note that IAM changes may take a few minutes to propagate):

%v
`, err, policy)
	}
	return token, nil
}

// GetRedshiftAuthToken returns authorization token that will be used as a
// password when connecting to Redshift databases.
func (a *dbAuth) GetRedshiftAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	if awsutils.IsRoleARN(sessionCtx.DatabaseUser) {
		return a.getRedshiftIAMRoleAuthToken(ctx, sessionCtx)
	}

	return a.getRedshiftDBUserAuthToken(ctx, sessionCtx)
}

func (a *dbAuth) getRedshiftIAMRoleAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	meta := sessionCtx.Database.GetAWS()
	roleARN, err := a.buildAWSRoleARNFromDatabaseUser(ctx, sessionCtx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	baseSession, err := a.cfg.Clients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// Assume the configured AWS role before assuming the role we need to get the
	// auth token. This allows cross-account AWS access.
	client, err := a.cfg.Clients.GetAWSRedshiftClient(ctx, meta.Region,
		cloud.WithChainedAssumeRole(baseSession, roleARN, externalIDForChainedAssumeRole(meta)),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM role auth token:

  %v

Make sure that IAM role %q has a trust relationship with Teleport database agent's IAM identity.
`, err, roleARN)
	}

	// Now make the API call to generate the temporary credentials.
	a.cfg.Log.Debugf("Generating Redshift IAM role auth token for %s.", sessionCtx)
	resp, err := client.GetClusterCredentialsWithIAMWithContext(ctx, &redshift.GetClusterCredentialsWithIAMInput{
		ClusterIdentifier: aws.String(meta.Redshift.ClusterID),
		DbName:            aws.String(sessionCtx.DatabaseName),
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocumentForAssumedRole(sessionCtx.Database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM role auth token:

  %v

Make sure that IAM role %q has permissions to generate credentials. Here is a sample IAM policy:

%v
`, err, roleARN, policy)
	}
	return aws.StringValue(resp.DbUser), aws.StringValue(resp.DbPassword), nil
}

func (a *dbAuth) getRedshiftDBUserAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	meta := sessionCtx.Database.GetAWS()
	redshiftClient, err := a.cfg.Clients.GetAWSRedshiftClient(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating Redshift auth token for %s.", sessionCtx)
	resp, err := redshiftClient.GetClusterCredentialsWithContext(ctx, &redshift.GetClusterCredentialsInput{
		ClusterIdentifier: aws.String(meta.Redshift.ClusterID),
		DbUser:            aws.String(sessionCtx.DatabaseUser),
		DbName:            aws.String(sessionCtx.DatabaseName),
		// TODO(r0mant): Do not auto-create database account if DbUser doesn't
		// exist for now, but it may be potentially useful in future.
		AutoCreate: aws.Bool(false),
		// TODO(r0mant): List of additional groups DbUser will join for the
		// session. Do we need to let people control this?
		DbGroups: []*string{},
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(sessionCtx.Database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM auth token:

  %v

Make sure that Teleport database agent's IAM policy is attached and has permissions
to generate Redshift credentials (note that IAM changes may take a few minutes to
propagate):

%v
`, err, policy)
	}
	return aws.StringValue(resp.DbUser), aws.StringValue(resp.DbPassword), nil
}

// GetRedshiftServerlessAuthToken generates Redshift Serverless auth token.
func (a *dbAuth) GetRedshiftServerlessAuthToken(ctx context.Context, sessionCtx *Session) (string, string, error) {
	// Redshift Serverless maps caller IAM users/roles to database users. For
	// example, an IAM role "arn:aws:iam::1234567890:role/my-role-name" will be
	// mapped to a Postgres user "IAMR:my-role-name" inside the database. So we
	// first need to assume this IAM role before getting auth token.
	meta := sessionCtx.Database.GetAWS()
	roleARN, err := redshiftServerlessUsernameToRoleARN(meta, sessionCtx.DatabaseUser)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	baseSession, err := a.cfg.Clients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// Assume the configured AWS role before assuming the role we need to get the
	// auth token. This allows cross-account AWS access.
	client, err := a.cfg.Clients.GetAWSRedshiftServerlessClient(ctx, meta.Region,
		cloud.WithChainedAssumeRole(baseSession, roleARN, externalIDForChainedAssumeRole(meta)),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.AccessDenied(`Could not generate Redshift Serverless auth token:

  %v

Make sure that IAM role %q has a trust relationship with Teleport database agent's IAM identity.
`, err, roleARN)
	}

	// Now make the API call to generate the temporary credentials.
	a.cfg.Log.Debugf("Generating Redshift Serverless auth token for %s.", sessionCtx)
	resp, err := client.GetCredentialsWithContext(ctx, &redshiftserverless.GetCredentialsInput{
		WorkgroupName: aws.String(meta.RedshiftServerless.WorkgroupName),
		DbName:        aws.String(sessionCtx.DatabaseName),
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocumentForAssumedRole(sessionCtx.Database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", "", trace.AccessDenied(`Could not generate Redshift Serverless auth token:

  %v

Make sure that IAM role %q has permissions to generate credentials. Here is a sample IAM policy:

%v
`, err, roleARN, policy)
	}
	return aws.StringValue(resp.DbUser), aws.StringValue(resp.DbPassword), nil
}

// GetCloudSQLAuthToken returns authorization token that will be used as a
// password when connecting to Cloud SQL databases.
func (a *dbAuth) GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error) {
	gcpIAM, err := a.cfg.Clients.GetGCPIAMClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating GCP auth token for %s.", sessionCtx)

	serviceAccountName := sessionCtx.DatabaseUser
	if !strings.HasSuffix(serviceAccountName, ".gserviceaccount.com") {
		serviceAccountName = serviceAccountName + ".gserviceaccount.com"
	}
	resp, err := gcpIAM.GenerateAccessToken(ctx,
		&gcpcredentialspb.GenerateAccessTokenRequest{
			// From GenerateAccessToken docs:
			//
			// The resource name of the service account for which the credentials
			// are requested, in the following format:
			//   projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}
			Name: fmt.Sprintf("projects/-/serviceAccounts/%v", serviceAccountName),
			// From GenerateAccessToken docs:
			//
			// Code to identify the scopes to be included in the OAuth 2.0 access
			// token:
			//   https://developers.google.com/identity/protocols/oauth2/scopes
			//   https://developers.google.com/identity/protocols/oauth2/scopes#sqladmin
			Scope: []string{
				"https://www.googleapis.com/auth/sqlservice.admin",
			},
		})
	if err != nil {
		return "", trace.AccessDenied(`Could not generate GCP IAM auth token:

  %v

Make sure Teleport db service has "Service Account Token Creator" GCP IAM role,
or "iam.serviceAccounts.getAccessToken" IAM permission.
`, err)
	}
	return resp.AccessToken, nil
}

// GetCloudSQLPassword updates the specified database user's password to a
// random value using GCP Cloud SQL Admin API.
//
// It is used to generate a one-time password when connecting to GCP MySQL
// databases which don't support IAM authentication.
func (a *dbAuth) GetCloudSQLPassword(ctx context.Context, sessionCtx *Session) (string, error) {
	gcpCloudSQL, err := a.cfg.Clients.GetGCPSQLAdminClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating GCP user password for %s.", sessionCtx)
	token, err := utils.CryptoRandomHex(defaults.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// Cloud SQL will return 409 to a user update operation if there is another
	// one in progress, so retry upon encountering it. Also, be nice to the API
	// and retry with a backoff.
	retry, err := retryutils.NewConstant(time.Second)
	if err != nil {
		return "", trace.Wrap(err)
	}
	retryCtx, cancel := context.WithTimeout(ctx, defaults.DatabaseConnectTimeout)
	defer cancel()
	err = retry.For(retryCtx, func() error {
		err := a.updateCloudSQLUser(ctx, sessionCtx, gcpCloudSQL, &sqladmin.User{
			Password: token,
		})
		if err != nil && !trace.IsCompareFailed(ConvertError(err)) { // We only want to retry on 409.
			return retryutils.PermanentRetryError(err)
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

// updateCloudSQLUser makes a request to Cloud SQL API to update the provided user.
func (a *dbAuth) updateCloudSQLUser(ctx context.Context, sessionCtx *Session, gcpCloudSQL gcp.SQLAdminClient, user *sqladmin.User) error {
	err := gcpCloudSQL.UpdateUser(ctx, sessionCtx.Database, sessionCtx.DatabaseUser, user)
	if err != nil {
		// Note that mysql client has a 1024 char limit for displaying errors
		// so we need to keep the message short when possible. This message
		// does get cut off when sessionCtx.DatabaseUser or err is long.
		return trace.AccessDenied(`Could not update Cloud SQL user %q password:

  %v

If the db user uses IAM authentication, please use the full service account email
ID as "--db-user", or grant the Teleport Database Service the
"cloudsql.users.get" IAM permission so it can discover the user type.

If the db user uses passwords, make sure Teleport Database Service has "Cloud
SQL Admin" GCP IAM role, or "cloudsql.users.update" IAM permission.
`, sessionCtx.DatabaseUser, err)
	}
	return nil
}

// GetAzureAccessToken generates Azure database access token.
func (a *dbAuth) GetAzureAccessToken(ctx context.Context, sessionCtx *Session) (string, error) {
	a.cfg.Log.Debugf("Generating Azure access token for %s.", sessionCtx)
	cred, err := a.cfg.Clients.GetAzureCredential()
	if err != nil {
		return "", trace.Wrap(err)
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{
			// Access token scope for connecting to Postgres/MySQL database.
			"https://ossrdbms-aad.database.windows.net/.default",
		},
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token.Token, nil
}

// GetElastiCacheRedisToken generates an ElastiCache Redis auth token.
func (a *dbAuth) GetElastiCacheRedisToken(ctx context.Context, sessionCtx *Session) (string, error) {
	meta := sessionCtx.Database.GetAWS()
	awsSession, err := a.cfg.Clients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating ElastiCache Redis auth token for %s.", sessionCtx)
	tokenReq := &awsRedisIAMTokenRequest{
		// For IAM-enabled ElastiCache users, the username and user id properties must be identical.
		// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-limits
		userID:      sessionCtx.DatabaseUser,
		targetID:    meta.ElastiCache.ReplicationGroupID,
		serviceName: elasticache.ServiceName,
		region:      meta.Region,
		credentials: awsSession.Config.Credentials,
		clock:       a.cfg.Clock,
	}
	token, err := tokenReq.toSignedRequestURI()
	return token, trace.Wrap(err)
}

// GetMemoryDBToken generates a MemoryDB auth token.
func (a *dbAuth) GetMemoryDBToken(ctx context.Context, sessionCtx *Session) (string, error) {
	meta := sessionCtx.Database.GetAWS()
	awsSession, err := a.cfg.Clients.GetAWSSession(ctx, meta.Region,
		cloud.WithAssumeRoleFromAWSMeta(meta),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating MemoryDB auth token for %s.", sessionCtx)
	tokenReq := &awsRedisIAMTokenRequest{
		userID:      sessionCtx.DatabaseUser,
		targetID:    meta.MemoryDB.ClusterName,
		serviceName: strings.ToLower(memorydb.ServiceName),
		region:      meta.Region,
		credentials: awsSession.Config.Credentials,
		clock:       a.cfg.Clock,
	}
	token, err := tokenReq.toSignedRequestURI()
	return token, trace.Wrap(err)
}

// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
func (a *dbAuth) GetAzureCacheForRedisToken(ctx context.Context, sessionCtx *Session) (string, error) {
	resourceID, err := arm.ParseResourceID(sessionCtx.Database.GetAzure().ResourceID)
	if err != nil {
		return "", trace.Wrap(err)
	}

	var client libazure.CacheForRedisClient
	switch resourceID.ResourceType.String() {
	case "Microsoft.Cache/Redis":
		client, err = a.cfg.Clients.GetAzureRedisClient(resourceID.SubscriptionID)
		if err != nil {
			return "", trace.Wrap(err)
		}
	case "Microsoft.Cache/redisEnterprise", "Microsoft.Cache/redisEnterprise/databases":
		client, err = a.cfg.Clients.GetAzureRedisEnterpriseClient(resourceID.SubscriptionID)
		if err != nil {
			return "", trace.Wrap(err)
		}
	default:
		return "", trace.BadParameter("unknown Azure Cache for Redis resource type: %v", resourceID.ResourceType)
	}
	token, err := client.GetToken(ctx, sessionCtx.Database.GetAzure().ResourceID)
	if err != nil {
		// Some Azure error messages are long, multi-lined, and may even
		// contain divider lines like "------". It's unreadable in redis-cli as
		// the message has to be merged to a single line string. Thus logging
		// the original error as debug and returning a more user friendly
		// message.
		a.cfg.Log.WithError(err).Debugf("Failed to get token for Azure Redis %q.", sessionCtx.Database.GetName())
		switch {
		case trace.IsAccessDenied(err):
			return "", trace.AccessDenied("Failed to get token for Azure Redis %q. Please make sure the database agent has the \"listKeys\" permission to the database.", sessionCtx.Database.GetName())
		case trace.IsNotFound(err):
			// Note that Azure Cache for Redis should always have both keys
			// generated at all time. Here just checking in case something
			// wrong with the API.
			return "", trace.AccessDenied("Failed to get token for Azure Redis %q. Please make sure either the primary key or the secondary key is generated.", sessionCtx.Database.GetName())
		default:
			return "", trace.Errorf("Failed to get token for Azure Redis %q.", sessionCtx.Database.GetName())
		}
	}
	return token, nil
}

// GetTLSConfig builds the client TLS configuration for the session.
//
// For RDS/Aurora, the config must contain RDS root certificate as a trusted
// authority. For on-prem we generate a client certificate signed by the host
// CA used to authenticate.
func (a *dbAuth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	dbTLSConfig := sessionCtx.Database.GetTLS()

	// Mode won't be set for older clients. We will default to VerifyFull then - the same as before.
	switch dbTLSConfig.Mode {
	case types.DatabaseTLSMode_INSECURE:
		return a.getTLSConfigInsecure(ctx, sessionCtx)
	case types.DatabaseTLSMode_VERIFY_CA:
		return a.getTLSConfigVerifyCA(ctx, sessionCtx)
	default:
		return a.getTLSConfigVerifyFull(ctx, sessionCtx)
	}
}

// getTLSConfigVerifyFull returns tls.Config with full verification enabled ('verify-full' mode).
// Config also includes database specific adjustment.
func (a *dbAuth) getTLSConfigVerifyFull(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Add CA certificate to the trusted pool if it's present, e.g. when
	// connecting to RDS/Aurora which require AWS CA or when was provided in config file.
	//
	// Some databases may also require the system cert pool, e.g Azure Redis.
	if err := setupTLSConfigRootCAs(tlsConfig, sessionCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	// You connect to Cloud SQL instances by IP and the certificate presented
	// by the instance does not contain IP SANs so the default "full" certificate
	// verification will always fail.
	//
	// In the docs they recommend disabling hostname verification when connecting
	// e.g. with psql (verify-ca mode) reasoning that it's not required since
	// CA is instance-specific:
	//   https://cloud.google.com/sql/docs/postgres/connect-admin-ip
	//
	// They do encode <project-id>:<instance-id> in the CN field, which also
	// wouldn't validate by default since CN has been deprecated and server
	// name verification ignores it starting from Go 1.15.
	//
	// For this reason we're setting ServerName to <project-id>:<instance-id>,
	// disabling default certificate verification and validating it ourselves.
	//
	// See the following Go issue for more context:
	//   https://github.com/golang/go/issues/40748
	if sessionCtx.Database.IsCloudSQL() {
		// Cloud SQL server presented certificates encode instance names as
		// "<project-id>:<instance-id>" in CommonName. This is verified against
		// the ServerName in a custom connection verification step (see below).
		tlsConfig.ServerName = sessionCtx.Database.GetGCP().GetServerName()
		// This just disables default verification.
		tlsConfig.InsecureSkipVerify = true
		// This will verify CN and cert chain on each connection.
		tlsConfig.VerifyConnection = getVerifyCloudSQLCertificate(tlsConfig.RootCAs)
	}

	// Setup server name for verification.
	if err := setupTLSConfigServerName(tlsConfig, sessionCtx); err != nil {
		return nil, trace.Wrap(err)
	}

	// RDS/Aurora/Redshift/ElastiCache and Cloud SQL auth is done with an auth
	// token so don't generate a client certificate and exit here.
	if sessionCtx.Database.IsCloudHosted() {
		return tlsConfig, nil
	}

	// MongoDB Atlas doesn't not require client certificates if is using AWS
	// authentication.
	if awsutils.IsRoleARN(sessionCtx.DatabaseUser) && sessionCtx.Database.GetType() == types.DatabaseTypeMongoAtlas {
		return tlsConfig, nil
	}

	// Otherwise, when connecting to an onprem database, generate a client
	// certificate. The database instance should be configured with
	// Teleport's CA obtained with 'tctl auth sign --type=db'.
	return a.appendClientCert(ctx, sessionCtx, tlsConfig)
}

// getTLSConfigInsecure generates tls.Config when TLS mode is equal to 'insecure'.
// Generated configuration will accept any certificate provided by database.
func (a *dbAuth) getTLSConfigInsecure(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	tlsConfig, err := a.getTLSConfigVerifyFull(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Accept any certificate provided by database.
	tlsConfig.InsecureSkipVerify = true
	// Remove certificate validation if set.
	tlsConfig.VerifyConnection = nil

	return tlsConfig, nil
}

// getTLSConfigVerifyCA generates tls.Config when TLS mode is equal to 'verify-ca'.
// Generated configuration is the same as 'verify-full' except the server name
// verification is disabled.
func (a *dbAuth) getTLSConfigVerifyCA(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	tlsConfig, err := a.getTLSConfigVerifyFull(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Base on https://github.com/golang/go/blob/master/src/crypto/tls/example_test.go#L193-L208
	// Set InsecureSkipVerify to skip the default validation we are
	// replacing. This will not disable VerifyConnection.
	tlsConfig.InsecureSkipVerify = true
	tlsConfig.VerifyConnection = verifyConnectionFunc(tlsConfig.RootCAs)
	// ServerName is irrelevant in this case. Set it to default value to make it explicit.
	tlsConfig.ServerName = ""

	return tlsConfig, nil
}

// appendClientCert generates a client certificate and appends it to the provided tlsConfig.
func (a *dbAuth) appendClientCert(ctx context.Context, sessionCtx *Session, tlsConfig *tls.Config) (*tls.Config, error) {
	cert, cas, err := a.getClientCert(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.Certificates = []tls.Certificate{*cert}
	for _, ca := range cas {
		if !tlsConfig.RootCAs.AppendCertsFromPEM(ca) {
			return nil, trace.BadParameter("failed to append CA certificate to the pool")
		}
	}

	return tlsConfig, nil
}

// setupTLSConfigRootCAs initializes the root CA cert pool for the provided
// tlsConfig based on session context.
func setupTLSConfigRootCAs(tlsConfig *tls.Config, sessionCtx *Session) error {
	// Start with an empty pool or a system cert pool.
	if shouldUseSystemCertPool(sessionCtx) {
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig.RootCAs = systemCertPool
	} else {
		tlsConfig.RootCAs = x509.NewCertPool()
	}

	// If CAs are provided by the database object, add them to the pool.
	if len(sessionCtx.Database.GetCA()) != 0 {
		if !tlsConfig.RootCAs.AppendCertsFromPEM([]byte(sessionCtx.Database.GetCA())) {
			return trace.BadParameter("invalid server CA certificate")
		}
		return nil
	}

	// Done. Client cert may also be added later for non-cloud databases.
	return nil
}

// shouldUseSystemCertPool returns true for database servers presenting
// certificates signed by publicly trusted CAs so a system cert pool can be
// used.
func shouldUseSystemCertPool(sessionCtx *Session) bool {
	switch sessionCtx.Database.GetType() {
	// Azure databases either use Baltimore Root CA or DigiCert Global Root G2.
	//
	// https://docs.microsoft.com/en-us/azure/postgresql/concepts-ssl-connection-security
	// https://docs.microsoft.com/en-us/azure/mysql/howto-configure-ssl
	case types.DatabaseTypeAzure:
		return true

	case types.DatabaseTypeRDSProxy:
		// AWS RDS Proxy uses Amazon Root CAs.
		//
		// https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/rds-proxy.howitworks.html#rds-proxy-security.tls
		return true

	case types.DatabaseTypeOpenSearch:
		// OpenSearch is commonly hosted on AWS and uses Amazon Root CAs.
		return true
	}
	return false
}

// setupTLSConfigServerName initializes the server name for the provided
// tlsConfig based on session context.
func setupTLSConfigServerName(tlsConfig *tls.Config, sessionCtx *Session) error {
	// Use user provided server name if set. Override the current value if needed.
	if dbTLSConfig := sessionCtx.Database.GetTLS(); dbTLSConfig.ServerName != "" {
		tlsConfig.ServerName = dbTLSConfig.ServerName
		return nil
	}

	// If server name is set prior to this function, use that.
	if tlsConfig.ServerName != "" {
		return nil
	}

	switch sessionCtx.Database.GetProtocol() {
	case defaults.ProtocolMongoDB:
		// Don't set the ServerName when connecting to a MongoDB cluster - in case
		// of replica set the driver may dial multiple servers and will set
		// ServerName itself.
		return nil
	case defaults.ProtocolDynamoDB:
		// Don't set the server name for DynamoDB - the engine may dial different endpoints
		// based on the client request and will set ServerName itself.
		return nil
	case defaults.ProtocolRedis:
		// Azure Redis servers always serve the certificates with the proper
		// hostnames. However, OSS cluster mode may redirect to an IP address,
		// and without correct ServerName the handshake will fail as the IPs
		// are not in SANs.
		if sessionCtx.Database.IsAzure() {
			serverName, err := azureutils.GetHostFromRedisURI(sessionCtx.Database.GetURI())
			if err != nil {
				return trace.Wrap(err)
			}

			tlsConfig.ServerName = serverName
			return nil
		}

		// Redis is using custom URI schema.
		return nil
	case defaults.ProtocolClickHouse, defaults.ProtocolClickHouseHTTP:
		u, err := url.Parse(sessionCtx.Database.GetURI())
		if err != nil {
			return trace.Wrap(err)
		}
		addr, err := utils.ParseAddr(u.Host)
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig.ServerName = addr.Host()
		return nil
	default:
		// For other databases we're always connecting to the server specified
		// in URI so set ServerName ourselves.
		addr, err := utils.ParseAddr(sessionCtx.Database.GetURI())
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig.ServerName = addr.Host()
		return nil
	}
}

// verifyConnectionFunc returns a certificate validation function. serverName if empty will skip the hostname validation.
func verifyConnectionFunc(rootCAs *x509.CertPool) func(cs tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return trace.AccessDenied("database didn't present any certificate during initial handshake")
		}

		opts := x509.VerifyOptions{
			Roots:         rootCAs,
			DNSName:       "", // Skip server name validation
			Intermediates: x509.NewCertPool(),
		}
		// From Go Docs:
		// The first element (zero index) is the leaf certificate that the connection is verified against.
		//
		// In order to verify the whole chain we need to add all certificates on pos [1:] as intermediates
		// and call Verify() on the [0] one. Root is provided as an input to this function.
		for _, cert := range cs.PeerCertificates[1:] {
			opts.Intermediates.AddCert(cert)
		}

		_, err := cs.PeerCertificates[0].Verify(opts)
		return trace.Wrap(err)
	}
}

// getClientCert signs an ephemeral client certificate used by this
// server to authenticate with the database instance.
func (a *dbAuth) getClientCert(ctx context.Context, sessionCtx *Session) (cert *tls.Certificate, cas [][]byte, err error) {
	privateKey, err := native.GeneratePrivateKey()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Postgres requires the database username to be encoded as a common
	// name in the client certificate.
	subject := pkix.Name{CommonName: sessionCtx.DatabaseUser}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// TODO(r0mant): Cache database certificates to avoid expensive generate
	// operation on each connection.
	a.cfg.Log.Debugf("Generating client certificate for %s.", sessionCtx)
	resp, err := a.cfg.AuthClient.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
		CSR: csr,
		TTL: proto.Duration(sessionCtx.Identity.Expires.Sub(a.cfg.Clock.Now())),
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clientCert, err := privateKey.TLSCertificate(resp.Cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &clientCert, resp.CACerts, nil
}

// GetAuthPreference returns the cluster authentication config.
func (a *dbAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.cfg.AuthClient.GetAuthPreference(ctx)
}

// GetAzureIdentityResourceID returns the Azure identity resource ID attached to
// the current compute instance.
func (a *dbAuth) GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error) {
	if identityName == "" {
		return "", trace.BadParameter("empty identity name")
	}

	vm, err := utils.FnCacheGet(ctx, a.azureVirtualMachineCache, "", a.getCurrentAzureVM)
	if err != nil {
		return "", trace.Wrap(err)
	}

	for _, identity := range vm.Identities {
		if matchAzureResourceName(identity.ResourceID, identityName) {
			return identity.ResourceID, nil
		}
	}

	return "", trace.NotFound("could not find identity %q attached to the instance", identityName)
}

// getCurrentAzureVM fetches current Azure Virtual Machine struct. If Teleport
// is not running on Azure, returns an error.
func (a *dbAuth) getCurrentAzureVM(ctx context.Context) (*libazure.VirtualMachine, error) {
	metadataClient, err := a.cfg.Clients.GetInstanceMetadataClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if metadataClient.GetType() != types.InstanceMetadataTypeAzure {
		return nil, trace.BadParameter("fetching Azure identity resource ID is only supported on Azure")
	}

	instanceID, err := metadataClient.GetID(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsedInstanceID, err := arm.ParseResourceID(instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vmClient, err := a.cfg.Clients.GetAzureVirtualMachinesClient(parsedInstanceID.SubscriptionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vm, err := vmClient.Get(ctx, instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return vm, nil
}

func (a *dbAuth) buildAWSRoleARNFromDatabaseUser(ctx context.Context, sessionCtx *Session) (string, error) {
	dbAWS := sessionCtx.Database.GetAWS()
	awsAccountID := dbAWS.AccountID

	if awsutils.IsPartialRoleARN(sessionCtx.DatabaseUser) && awsAccountID == "" {
		switch {
		case dbAWS.AssumeRoleARN != "":
			a.cfg.Log.Debugf("Using AWS Account ID from assumed role")
			assumeRoleARN, err := awsutils.ParseRoleARN(dbAWS.AssumeRoleARN)
			if err != nil {
				return "", trace.Wrap(err)
			}

			awsAccountID = assumeRoleARN.AccountID
		default:
			a.cfg.Log.Debugf("Fetching AWS Account ID to build role ARN")
			stsClient, err := a.cfg.Clients.GetAWSSTSClient(ctx, dbAWS.Region, cloud.WithAmbientCredentials())
			if err != nil {
				return "", trace.Wrap(err)
			}

			identity, err := awslib.GetIdentityWithClient(ctx, stsClient)
			if err != nil {
				return "", trace.Wrap(err)
			}

			awsAccountID = identity.GetAccountID()
		}
	}

	arn, err := awsutils.BuildRoleARN(sessionCtx.DatabaseUser, dbAWS.Region, awsAccountID)
	return arn, trace.Wrap(err)
}

// GetAWSIAMCreds returns the AWS IAM credentials, including access key, secret
// access key and session token.
func (a *dbAuth) GetAWSIAMCreds(ctx context.Context, sessionCtx *Session) (string, string, string, error) {
	dbAWS := sessionCtx.Database.GetAWS()
	arn, err := a.buildAWSRoleARNFromDatabaseUser(ctx, sessionCtx)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}

	baseSession, err := a.cfg.Clients.GetAWSSession(ctx, dbAWS.Region,
		cloud.WithAssumeRoleFromAWSMeta(dbAWS),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}

	// ExternalID should only be used once. If the baseSession assumes a role,
	// the chained sessions should have an empty external ID.

	sess, err := a.cfg.Clients.GetAWSSession(ctx, dbAWS.Region,
		cloud.WithChainedAssumeRole(baseSession, arn, externalIDForChainedAssumeRole(dbAWS)),
		cloud.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}

	creds, err := sess.Config.Credentials.Get()
	if err != nil {
		return "", "", "", trace.Wrap(err)
	}

	return creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken, nil
}

// Close releases all resources used by authenticator.
func (a *dbAuth) Close() error {
	return a.cfg.Clients.Close()
}

// getVerifyCloudSQLCertificate returns a function that performs verification
// of server certificate presented by a Cloud SQL database instance.
func getVerifyCloudSQLCertificate(roots *x509.CertPool) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) < 1 {
			return trace.AccessDenied("Cloud SQL instance didn't present a certificate")
		}
		// CN has been deprecated for a while, but Cloud SQL instances still use
		// it to encode instance name in the form of <project-id>:<instance-id>.
		commonName := cs.PeerCertificates[0].Subject.CommonName
		if commonName != cs.ServerName {
			return trace.AccessDenied("Cloud SQL certificate CommonName validation failed: expected %q, got %q", cs.ServerName, commonName)
		}
		opts := x509.VerifyOptions{Roots: roots, Intermediates: x509.NewCertPool()}
		for _, cert := range cs.PeerCertificates[1:] {
			opts.Intermediates.AddCert(cert)
		}
		_, err := cs.PeerCertificates[0].Verify(opts)
		return err
	}
}

// matchAzureResourceName receives a resource ID and checks if the resource name
// matches.
func matchAzureResourceName(resourceID, name string) bool {
	parsedResource, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return false
	}

	return parsedResource.Name == name
}

// redshiftServerlessUsernameToRoleARN converts a database username to AWS role
// ARN for a Redshift Serverless database.
func redshiftServerlessUsernameToRoleARN(aws types.AWS, username string) (string, error) {
	// These are in-database usernames created when logged in as IAM
	// users/roles. We will enforce Teleport users to provide IAM roles
	// instead.
	if strings.HasPrefix(username, "IAM:") || strings.HasPrefix(username, "IAMR:") {
		return "", trace.BadParameter("expecting name or ARN of an AWS IAM role but got %v", username)
	}
	return awsutils.BuildRoleARN(username, aws.Region, aws.AccountID)
}

func externalIDForChainedAssumeRole(meta types.AWS) string {
	// ExternalID should only be used once. If the baseSession assumes a role,
	// the chained sessions should have an empty external ID.
	if meta.AssumeRoleARN != "" {
		return ""
	}
	return meta.ExternalID
}

// awsRedisIAMTokenRequest builds an AWS IAM auth token for ElastiCache
// Redis and MemoryDB.
// Implemented following the AWS examples:
// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-Connecting
// https://docs.aws.amazon.com/memorydb/latest/devguide/auth-iam.html#auth-iam-Connecting
type awsRedisIAMTokenRequest struct {
	// userID is the ElastiCache user ID.
	userID string
	// targetID is the ElastiCache replication group ID or the MemoryDB cluster name.
	targetID string
	// region is the AWS region.
	region string
	// credentials are used to presign with AWS SigV4.
	credentials *credentials.Credentials
	// clock is the clock implementation.
	clock clockwork.Clock
	// serviceName is the AWS service name used for signing.
	serviceName string
}

// checkAndSetDefaults validates config and sets defaults.
func (r *awsRedisIAMTokenRequest) checkAndSetDefaults() error {
	if r.userID == "" {
		return trace.BadParameter("missing user ID")
	}
	if r.targetID == "" {
		return trace.BadParameter("missing target ID for signing")
	}
	if r.region == "" {
		return trace.BadParameter("missing region")
	}
	if r.credentials == nil {
		return trace.BadParameter("missing credentials")
	}
	if r.serviceName == "" {
		return trace.BadParameter("missing service name")
	}
	if r.clock == nil {
		r.clock = clockwork.NewRealClock()
	}
	return nil
}

// toSignedRequestURI creates a new AWS SigV4 pre-signed request URI.
// This pre-signed request URI can then be used to authenticate as an
// ElastiCache Redis or MemoryDB user.
func (r *awsRedisIAMTokenRequest) toSignedRequestURI() (string, error) {
	if err := r.checkAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}
	req, err := r.getSignableRequest()
	if err != nil {
		return "", trace.Wrap(err)
	}
	s := v4.NewSigner(r.credentials)
	_, err = s.Presign(req, nil, r.serviceName, r.region, time.Minute*15, r.clock.Now())
	if err != nil {
		return "", trace.Wrap(err)
	}
	res := url.URL{
		Host:     req.URL.Host,
		Path:     "/",
		RawQuery: req.URL.RawQuery,
	}
	return strings.TrimPrefix(res.String(), "//"), nil
}

// getSignableRequest creates a new request suitable for pre-signing with SigV4.
func (r *awsRedisIAMTokenRequest) getSignableRequest() (*http.Request, error) {
	query := url.Values{
		"Action": {"connect"},
		"User":   {r.userID},
	}
	reqURI := url.URL{
		Scheme:   "http",
		Host:     r.targetID,
		Path:     "/",
		RawQuery: query.Encode(),
	}
	req, err := http.NewRequest(http.MethodGet, reqURI.String(), nil)
	if err != nil {
		return nil, err
	}
	return req, nil
}

type reportingAuth struct {
	Auth
	component string
	db        types.Database
}

// newReportingAuth returns a reporting version of Auth, wrapping the original Auth instance.
func newReportingAuth(db types.Database, auth Auth) *reportingAuth {
	return &reportingAuth{
		Auth:      auth,
		component: "db:auth",
		db:        db,
	}
}

func (r *reportingAuth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	defer methodCallMetrics("GetTLSConfig", r.component, r.db)()
	return r.Auth.GetTLSConfig(ctx, sessionCtx)
}

var _ Auth = (*reportingAuth)(nil)
