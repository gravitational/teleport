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
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	gcpcredentials "cloud.google.com/go/iam/credentials/apiv1"
	gcpcredentialspb "cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/aws/aws-sdk-go-v2/aws"
	rdsauth "github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	rss "github.com/aws/aws-sdk-go-v2/service/redshiftserverless"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/oauth2"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	azureutils "github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/cloud"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
	libazure "github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	azureimds "github.com/gravitational/teleport/lib/cloud/imds/azure"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	dbiam "github.com/gravitational/teleport/lib/srv/db/common/iam"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	"github.com/gravitational/teleport/lib/utils/aws/migration"
)

// azureVirtualMachineCacheTTL is the default TTL for Azure virtual machine
// cache entries.
const azureVirtualMachineCacheTTL = 5 * time.Minute

// Auth defines interface for creating auth tokens and TLS configurations.
type Auth interface {
	// GetRDSAuthToken generates RDS/Aurora auth token.
	GetRDSAuthToken(ctx context.Context, database types.Database, databaseUser string) (string, error)
	// GetRedshiftAuthToken generates Redshift auth token.
	GetRedshiftAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error)
	// GetRedshiftServerlessAuthToken generates Redshift Serverless auth token.
	GetRedshiftServerlessAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error)
	// GetElastiCacheRedisToken generates an ElastiCache Redis auth token.
	GetElastiCacheRedisToken(ctx context.Context, database types.Database, databaseUser string) (string, error)
	// GetMemoryDBToken generates a MemoryDB auth token.
	GetMemoryDBToken(ctx context.Context, database types.Database, databaseUser string) (string, error)
	// GetCloudSQLAuthToken generates Cloud SQL auth token.
	GetCloudSQLAuthToken(ctx context.Context, databaseUser string) (string, error)
	// GetSpannerTokenSource returns an oauth token source for GCP Spanner.
	GetSpannerTokenSource(ctx context.Context, databaseUser string) (oauth2.TokenSource, error)
	// GetCloudSQLPassword generates password for a Cloud SQL database user.
	GetCloudSQLPassword(ctx context.Context, database types.Database, databaseUser string) (string, error)
	// GetAzureAccessToken generates Azure database access token.
	GetAzureAccessToken(ctx context.Context) (string, error)
	// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
	GetAzureCacheForRedisToken(ctx context.Context, database types.Database) (string, error)
	// GetTLSConfig builds the client TLS configuration for the session.
	GetTLSConfig(ctx context.Context, certExpiry time.Time, database types.Database, databaseUser string) (*tls.Config, error)
	// GetAuthPreference returns the cluster authentication config.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	// GetAzureIdentityResourceID returns the Azure identity resource ID
	// attached to the current compute instance. If Teleport is not running on
	// Azure VM returns an error.
	GetAzureIdentityResourceID(ctx context.Context, identityName string) (string, error)
	// GetAWSIAMCreds returns the AWS IAM credentials, including access key,
	// secret access key and session token.
	GetAWSIAMCreds(ctx context.Context, database types.Database, databaseUser string) (string, string, string, error)
	// GenerateDatabaseClientKey generates a cryptographic key appropriate for
	// database client connections.
	GenerateDatabaseClientKey(context.Context) (*keys.PrivateKey, error)
	// WithLogger returns a new instance of Auth with updated logger.
	// The callback function receives the current logger and returns a new one.
	WithLogger(getUpdatedLogger func(*slog.Logger) *slog.Logger) Auth
}

// AuthClient is an interface that defines a subset of libauth.Client's
// functions that are required for database auth.
type AuthClient interface {
	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
}

// AccessPoint is an interface that defines a subset of
// authclient.DatabaseAccessPoint that are required for database auth.
type AccessPoint interface {
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
}

// redshiftClient defines a subset of the AWS Redshift client API.
type redshiftClient interface {
	GetClusterCredentialsWithIAM(context.Context, *redshift.GetClusterCredentialsWithIAMInput, ...func(*redshift.Options)) (*redshift.GetClusterCredentialsWithIAMOutput, error)
	GetClusterCredentials(context.Context, *redshift.GetClusterCredentialsInput, ...func(*redshift.Options)) (*redshift.GetClusterCredentialsOutput, error)
}

// rssClient defines a subset of the AWS Redshift Serverless client API.
type rssClient interface {
	GetCredentials(context.Context, *rss.GetCredentialsInput, ...func(*rss.Options)) (*rss.GetCredentialsOutput, error)
}

// stsClient defines a subset of the AWS STS client API.
type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// awsClientProvider is an AWS SDK client provider.
type awsClientProvider interface {
	getRedshiftClient(cfg aws.Config, optFns ...func(*redshift.Options)) redshiftClient
	getRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) rssClient
	getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient
}

type defaultAWSClients struct{}

func (defaultAWSClients) getRedshiftClient(cfg aws.Config, optFns ...func(*redshift.Options)) redshiftClient {
	return redshift.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getRedshiftServerlessClient(cfg aws.Config, optFns ...func(*rss.Options)) rssClient {
	return rss.NewFromConfig(cfg, optFns...)
}

func (defaultAWSClients) getSTSClient(cfg aws.Config, optFns ...func(*sts.Options)) stsClient {
	return sts.NewFromConfig(cfg, optFns...)
}

// AuthConfig is the database access authenticator configuration.
type AuthConfig struct {
	// AuthClient is the cluster auth client.
	AuthClient AuthClient
	// AccessPoint is a caching client connected to the Auth Server.
	AccessPoint AccessPoint
	// Clients provides interface for obtaining cloud provider clients.
	Clients cloud.Clients
	// Clock is the clock implementation.
	Clock clockwork.Clock
	// Logger is used for logging.
	Logger *slog.Logger
	// AWSConfigProvider provides [aws.Config] for AWS SDK service clients.
	AWSConfigProvider awsconfig.Provider

	// awsClients is an SDK client provider.
	awsClients awsClientProvider
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *AuthConfig) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("missing AccessPoint")
	}
	if c.Clients == nil {
		return trace.BadParameter("missing Clients")
	}
	if c.AWSConfigProvider == nil {
		return trace.BadParameter("missing AWSConfigProvider")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, "db:auth")
	}

	if c.awsClients == nil {
		c.awsClients = defaultAWSClients{}
	}
	return nil
}

func (c *AuthConfig) withLogger(getUpdatedLogger func(*slog.Logger) *slog.Logger) AuthConfig {
	cfg := *c
	cfg.Logger = getUpdatedLogger(c.Logger)
	return cfg
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

// NewAuthForSession returns a copy of Auth with session-specific logging.
func NewAuthForSession(auth Auth, sessionCtx *Session) Auth {
	return auth.WithLogger(func(logger *slog.Logger) *slog.Logger {
		return logger.With(
			"session_id", sessionCtx.ID,
			"database", sessionCtx.Database.GetName(),
		)
	})
}

// WithLogger returns a new instance of Auth with updated logger.
// The callback function receives the current logger and returns a new one.
func (a *dbAuth) WithLogger(getUpdatedLogger func(*slog.Logger) *slog.Logger) Auth {
	return &dbAuth{
		cfg:                      a.cfg.withLogger(getUpdatedLogger),
		azureVirtualMachineCache: a.azureVirtualMachineCache,
	}
}

// GetRDSAuthToken returns authorization token that will be used as a password
// when connecting to RDS and Aurora databases.
func (a *dbAuth) GetRDSAuthToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	meta := database.GetAWS()
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Generating RDS auth token",
		"database", database,
		"database_user", databaseUser,
	)
	token, err := rdsauth.BuildAuthToken(
		ctx,
		database.GetURI(),
		meta.Region,
		databaseUser,
		awsCfg.Credentials,
	)
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(database)
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
func (a *dbAuth) GetRedshiftAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	if awsutils.IsRoleARN(databaseUser) {
		return a.getRedshiftIAMRoleAuthToken(ctx, database, databaseUser, databaseName)
	}

	return a.getRedshiftDBUserAuthToken(ctx, database, databaseUser, databaseName)
}

func (a *dbAuth) getRedshiftIAMRoleAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	meta := database.GetAWS()
	roleARN, err := a.buildAWSRoleARNFromDatabaseUser(ctx, database, databaseUser)
	if err != nil {
		return "", "", trace.Wrap(err)
	}

	// Assume the configured AWS role before assuming the role we need to get the
	// auth token. This allows cross-account AWS access.
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAssumeRole(roleARN, externalIDForChainedAssumeRole(meta)),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM role auth token:

  %v

Make sure that IAM role %q has a trust relationship with Teleport database agent's IAM identity.
`, err, roleARN)
	}

	// Now make the API call to generate the temporary credentials.
	a.cfg.Logger.DebugContext(ctx, "Generating Redshift IAM role auth token",
		"database", database,
		"database_user", databaseUser,
		"database_name", databaseName,
	)
	client := a.cfg.awsClients.getRedshiftClient(awsCfg)
	resp, err := client.GetClusterCredentialsWithIAM(ctx, &redshift.GetClusterCredentialsWithIAMInput{
		ClusterIdentifier: aws.String(meta.Redshift.ClusterID),
		DbName:            aws.String(databaseName),
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocumentForAssumedRole(database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM role auth token:

  %v

Make sure that IAM role %q has permissions to generate credentials. Here is a sample IAM policy:

%v
`, err, roleARN, policy)
	}
	return aws.ToString(resp.DbUser), aws.ToString(resp.DbPassword), nil
}

func (a *dbAuth) getRedshiftDBUserAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	meta := database.GetAWS()
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Generating Redshift auth token",
		"database", database,
		"database_user", databaseUser,
		"database_name", databaseName,
	)
	clt := a.cfg.awsClients.getRedshiftClient(awsCfg)
	resp, err := clt.GetClusterCredentials(ctx, &redshift.GetClusterCredentialsInput{
		ClusterIdentifier: aws.String(meta.Redshift.ClusterID),
		DbUser:            aws.String(databaseUser),
		DbName:            aws.String(databaseName),
		// TODO(r0mant): Do not auto-create database account if DbUser doesn't
		// exist for now, but it may be potentially useful in future.
		AutoCreate: aws.Bool(false),
		// TODO(r0mant): List of additional groups DbUser will join for the
		// session. Do we need to let people control this?
		DbGroups: []string{},
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocument(database)
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
	return aws.ToString(resp.DbUser), aws.ToString(resp.DbPassword), nil
}

// GetRedshiftServerlessAuthToken generates Redshift Serverless auth token.
func (a *dbAuth) GetRedshiftServerlessAuthToken(ctx context.Context, database types.Database, databaseUser string, databaseName string) (string, string, error) {
	// Redshift Serverless maps caller IAM users/roles to database users. For
	// example, an IAM role "arn:aws:iam::1234567890:role/my-role-name" will be
	// mapped to a Postgres user "IAMR:my-role-name" inside the database. So we
	// first need to assume this IAM role before getting auth token.
	meta := database.GetAWS()
	roleARN, err := redshiftServerlessUsernameToRoleARN(meta, databaseUser)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// Assume the configured AWS role before assuming the role we need to get the
	// auth token. This allows cross-account AWS access.
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAssumeRole(roleARN, externalIDForChainedAssumeRole(meta)),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", "", trace.AccessDenied(`Could not generate Redshift Serverless auth token:

  %v

Make sure that IAM role %q has a trust relationship with Teleport database agent's IAM identity.
`, err, roleARN)
	}
	clt := a.cfg.awsClients.getRedshiftServerlessClient(awsCfg)

	// Now make the API call to generate the temporary credentials.
	a.cfg.Logger.DebugContext(ctx, "Generating Redshift Serverless auth token",
		"database", database,
		"database_user", databaseUser,
		"database_name", databaseName,
	)
	resp, err := clt.GetCredentials(ctx, &rss.GetCredentialsInput{
		WorkgroupName: aws.String(meta.RedshiftServerless.WorkgroupName),
		DbName:        aws.String(databaseName),
	})
	if err != nil {
		policy, getPolicyErr := dbiam.GetReadableAWSPolicyDocumentForAssumedRole(database)
		if getPolicyErr != nil {
			policy = fmt.Sprintf("failed to generate IAM policy: %v", getPolicyErr)
		}
		return "", "", trace.AccessDenied(`Could not generate Redshift Serverless auth token:

  %v

Make sure that IAM role %q has permissions to generate credentials. Here is a sample IAM policy:

%v
`, err, roleARN, policy)
	}
	return aws.ToString(resp.DbUser), aws.ToString(resp.DbPassword), nil
}

// GetCloudSQLAuthToken returns authorization token that will be used as a
// password when connecting to Cloud SQL databases.
func (a *dbAuth) GetCloudSQLAuthToken(ctx context.Context, databaseUser string) (string, error) {
	//   https://developers.google.com/identity/protocols/oauth2/scopes#sqladmin
	scopes := []string{
		"https://www.googleapis.com/auth/sqlservice.admin",
	}
	ts, err := a.getCloudTokenSource(ctx, databaseUser, scopes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tok, err := ts.Token()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return tok.AccessToken, nil
}

// GetSpannerTokenSource returns an oauth token source for GCP Spanner.
func (a *dbAuth) GetSpannerTokenSource(ctx context.Context, databaseUser string) (oauth2.TokenSource, error) {
	// https://developers.google.com/identity/protocols/oauth2/scopes#spanner
	scopes := []string{
		"https://www.googleapis.com/auth/spanner.data",
	}
	ts, err := a.getCloudTokenSource(ctx, databaseUser, scopes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// refreshes the credentials as needed.
	return oauth2.ReuseTokenSource(nil, ts), nil
}

func (a *dbAuth) getCloudTokenSource(ctx context.Context, databaseUser string, scopes []string) (*cloudTokenSource, error) {
	gcpIAM, err := a.cfg.Clients.GetGCPIAMClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceAccountName := databaseUser
	if !strings.HasSuffix(serviceAccountName, ".gserviceaccount.com") {
		serviceAccountName = serviceAccountName + ".gserviceaccount.com"
	}
	return &cloudTokenSource{
		ctx:            ctx,
		client:         gcpIAM,
		log:            a.cfg.Logger.With("database_user", databaseUser),
		serviceAccount: serviceAccountName,
		scopes:         scopes,
	}, nil
}

// cloudTokenSource implements [oauth2.TokenSource] and logs each time it's
// used to fetch a token.
type cloudTokenSource struct {
	ctx            context.Context
	client         *gcpcredentials.IamCredentialsClient
	log            *slog.Logger
	serviceAccount string
	scopes         []string
}

// Token returns a token or an error.
// Token must be safe for concurrent use by multiple goroutines.
// The returned Token must not be modified.
func (l *cloudTokenSource) Token() (*oauth2.Token, error) {
	l.log.DebugContext(l.ctx, "Generating GCP auth token")
	resp, err := l.client.GenerateAccessToken(l.ctx,
		&gcpcredentialspb.GenerateAccessTokenRequest{
			// From GenerateAccessToken docs:
			//
			// The resource name of the service account for which the credentials
			// are requested, in the following format:
			//   projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}
			Name: fmt.Sprintf("projects/-/serviceAccounts/%v", l.serviceAccount),
			// From GenerateAccessToken docs:
			//
			// Code to identify the scopes to be included in the OAuth 2.0 access
			// token:
			//   https://developers.google.com/identity/protocols/oauth2/scopes
			Scope: l.scopes,
		})
	if err != nil {
		return nil, trace.AccessDenied(`Could not generate GCP IAM auth token:

  %v

Make sure Teleport db service has "Service Account Token Creator" GCP IAM role,
or "iam.serviceAccounts.getAccessToken" IAM permission.
`, err)
	}
	return &oauth2.Token{
		AccessToken: resp.AccessToken,
		Expiry:      resp.ExpireTime.AsTime(),
	}, nil
}

// GetCloudSQLPassword updates the specified database user's password to a
// random value using GCP Cloud SQL Admin API.
//
// It is used to generate a one-time password when connecting to GCP MySQL
// databases which don't support IAM authentication.
func (a *dbAuth) GetCloudSQLPassword(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	gcpCloudSQL, err := a.cfg.Clients.GetGCPSQLAdminClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Generating GCP user password",
		"database", database,
		"database_user", databaseUser,
	)
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
		err := a.updateCloudSQLUser(ctx, database, databaseUser, gcpCloudSQL, &sqladmin.User{
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
func (a *dbAuth) updateCloudSQLUser(ctx context.Context, database types.Database, databaseUser string, gcpCloudSQL gcp.SQLAdminClient, user *sqladmin.User) error {
	err := gcpCloudSQL.UpdateUser(ctx, database, databaseUser, user)
	if err != nil {
		// Note that mysql client has a 1024 char limit for displaying errors
		// so we need to keep the message short when possible. This message
		// does get cut off when databaseUser or err is long.
		return trace.AccessDenied(`Could not update Cloud SQL user %q password:

  %v

If the db user uses IAM authentication, please use the full service account email
ID as "--db-user", or grant the Teleport Database Service the
"cloudsql.users.get" IAM permission so it can discover the user type.

If the db user uses passwords, make sure Teleport Database Service has "Cloud
SQL Admin" GCP IAM role, or "cloudsql.users.update" IAM permission.
`, databaseUser, err)
	}
	return nil
}

// GetAzureAccessToken generates Azure database access token.
func (a *dbAuth) GetAzureAccessToken(ctx context.Context) (string, error) {
	a.cfg.Logger.DebugContext(ctx, "Generating Azure access token")
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
func (a *dbAuth) GetElastiCacheRedisToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	meta := database.GetAWS()
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Generating ElastiCache Redis auth token",
		"database", database,
		"database_user", databaseUser,
	)
	tokenReq := &awsRedisIAMTokenRequest{
		// For IAM-enabled ElastiCache users, the username and user id properties must be identical.
		// https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/auth-iam.html#auth-iam-limits
		userID:      databaseUser,
		targetID:    meta.ElastiCache.ReplicationGroupID,
		serviceName: "elasticache",
		region:      meta.Region,
		credentials: migration.NewCredentialsAdapter(awsCfg.Credentials),
		clock:       a.cfg.Clock,
	}
	token, err := tokenReq.toSignedRequestURI()
	return token, trace.Wrap(err)
}

// GetMemoryDBToken generates a MemoryDB auth token.
func (a *dbAuth) GetMemoryDBToken(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	meta := database.GetAWS()
	awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, meta.Region,
		awsconfig.WithAssumeRole(meta.AssumeRoleARN, meta.ExternalID),
		awsconfig.WithAmbientCredentials(),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Logger.DebugContext(ctx, "Generating MemoryDB auth token",
		"database", database,
		"database_user", databaseUser,
	)
	tokenReq := &awsRedisIAMTokenRequest{
		userID:      databaseUser,
		targetID:    meta.MemoryDB.ClusterName,
		serviceName: "memorydb",
		region:      meta.Region,
		credentials: migration.NewCredentialsAdapter(awsCfg.Credentials),
		clock:       a.cfg.Clock,
	}
	token, err := tokenReq.toSignedRequestURI()
	return token, trace.Wrap(err)
}

// GetAzureCacheForRedisToken retrieves auth token for Azure Cache for Redis.
func (a *dbAuth) GetAzureCacheForRedisToken(ctx context.Context, database types.Database) (string, error) {
	resourceID, err := arm.ParseResourceID(database.GetAzure().ResourceID)
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
	token, err := client.GetToken(ctx, database.GetAzure().ResourceID)
	if err != nil {
		// Some Azure error messages are long, multi-lined, and may even
		// contain divider lines like "------". It's unreadable in redis-cli as
		// the message has to be merged to a single line string. Thus logging
		// the original error as debug and returning a more user friendly
		// message.
		a.cfg.Logger.DebugContext(ctx, "Failed to get token for Azure Redis",
			"error", err,
			"database", database,
		)
		switch {
		case trace.IsAccessDenied(err):
			return "", trace.AccessDenied("Failed to get token for Azure Redis %q. Please make sure the database agent has the \"listKeys\" permission to the database.", database.GetName())
		case trace.IsNotFound(err):
			// Note that Azure Cache for Redis should always have both keys
			// generated at all time. Here just checking in case something
			// wrong with the API.
			return "", trace.AccessDenied("Failed to get token for Azure Redis %q. Please make sure either the primary key or the secondary key is generated.", database.GetName())
		default:
			return "", trace.Errorf("Failed to get token for Azure Redis %q.", database.GetName())
		}
	}
	return token, nil
}

// GetTLSConfig builds the client TLS configuration for the session.
//
// For RDS/Aurora, the config must contain RDS root certificate as a trusted
// authority. For on-prem we generate a client certificate signed by the host
// CA used to authenticate.
func (a *dbAuth) GetTLSConfig(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	dbTLSConfig := database.GetTLS()

	// Mode won't be set for older clients. We will default to VerifyFull then - the same as before.
	switch dbTLSConfig.Mode {
	case types.DatabaseTLSMode_INSECURE:
		return a.getTLSConfigInsecure(ctx, expiry, database, databaseUser)
	case types.DatabaseTLSMode_VERIFY_CA:
		return a.getTLSConfigVerifyCA(ctx, expiry, database, databaseUser)
	default:
		return a.getTLSConfigVerifyFull(ctx, expiry, database, databaseUser)
	}
}

// getTLSConfigVerifyFull returns tls.Config with full verification enabled ('verify-full' mode).
// Config also includes database specific adjustment.
func (a *dbAuth) getTLSConfigVerifyFull(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	// Add CA certificate to the trusted pool if it's present, e.g. when
	// connecting to RDS/Aurora which require AWS CA or when was provided in config file.
	//
	// Some databases may also require the system cert pool, e.g Azure Redis.
	if err := setupTLSConfigRootCAs(tlsConfig, database); err != nil {
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
	if database.IsCloudSQL() {
		// Cloud SQL server presented certificates encode instance names as
		// "<project-id>:<instance-id>" in CommonName. This is verified against
		// the ServerName in a custom connection verification step (see below).
		tlsConfig.ServerName = database.GetGCP().GetServerName()
		// This just disables default verification.
		tlsConfig.InsecureSkipVerify = true
		// This will verify CN and cert chain on each connection.
		tlsConfig.VerifyConnection = getVerifyCloudSQLCertificate(tlsConfig.RootCAs)
	}

	// Setup server name for verification.
	if err := setupTLSConfigServerName(tlsConfig, database); err != nil {
		return nil, trace.Wrap(err)
	}

	// RDS/Aurora/Redshift/ElastiCache and Cloud SQL auth is done with an auth
	// token so don't generate a client certificate and exit here.
	if database.IsCloudHosted() {
		return tlsConfig, nil
	}

	// MongoDB Atlas doesn't not require client certificates if is using AWS
	// authentication.
	if awsutils.IsRoleARN(databaseUser) && database.GetType() == types.DatabaseTypeMongoAtlas {
		return tlsConfig, nil
	}

	// Otherwise, when connecting to an onprem database, generate a client
	// certificate. The database instance should be configured with
	// Teleport's CA obtained with 'tctl auth sign --type=db'.
	return a.appendClientCert(ctx, expiry, databaseUser, tlsConfig)
}

// getTLSConfigInsecure generates tls.Config when TLS mode is equal to 'insecure'.
// Generated configuration will accept any certificate provided by database.
func (a *dbAuth) getTLSConfigInsecure(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	tlsConfig, err := a.getTLSConfigVerifyFull(ctx, expiry, database, databaseUser)
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
func (a *dbAuth) getTLSConfigVerifyCA(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	tlsConfig, err := a.getTLSConfigVerifyFull(ctx, expiry, database, databaseUser)
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
func (a *dbAuth) appendClientCert(ctx context.Context, expiry time.Time, databaseUser string, tlsConfig *tls.Config) (*tls.Config, error) {
	cert, cas, err := a.getClientCert(ctx, expiry, databaseUser)
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
func setupTLSConfigRootCAs(tlsConfig *tls.Config, database types.Database) error {
	// Start with an empty pool or a system cert pool.
	if shouldUseSystemCertPool(database) {
		systemCertPool, err := x509.SystemCertPool()
		if err != nil {
			return trace.Wrap(err)
		}
		tlsConfig.RootCAs = systemCertPool
	} else {
		tlsConfig.RootCAs = x509.NewCertPool()
	}

	// If CAs are provided by the database object, add them to the pool.
	if len(database.GetCA()) != 0 {
		if !tlsConfig.RootCAs.AppendCertsFromPEM([]byte(database.GetCA())) {
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
func shouldUseSystemCertPool(database types.Database) bool {
	if database.GetTLS().TrustSystemCertPool {
		return true
	}

	switch database.GetType() {
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
	case types.DatabaseTypeSpanner:
		// Spanner is hosted on GCP.
		return true
	}
	return false
}

// setupTLSConfigServerName initializes the server name for the provided
// tlsConfig based on session context.
func setupTLSConfigServerName(tlsConfig *tls.Config, database types.Database) error {
	// Use user provided server name if set. Override the current value if needed.
	if dbTLSConfig := database.GetTLS(); dbTLSConfig.ServerName != "" {
		tlsConfig.ServerName = dbTLSConfig.ServerName
		return nil
	}

	// If server name is set prior to this function, use that.
	if tlsConfig.ServerName != "" {
		return nil
	}

	switch database.GetProtocol() {
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
		if database.IsAzure() {
			serverName, err := azureutils.GetHostFromRedisURI(database.GetURI())
			if err != nil {
				return trace.Wrap(err)
			}

			tlsConfig.ServerName = serverName
			return nil
		}

		// Redis is using custom URI schema.
		return nil
	case defaults.ProtocolClickHouse, defaults.ProtocolClickHouseHTTP:
		u, err := url.Parse(database.GetURI())
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
		addr, err := utils.ParseAddr(database.GetURI())
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
func (a *dbAuth) getClientCert(ctx context.Context, expiry time.Time, databaseUser string) (cert *tls.Certificate, cas [][]byte, err error) {
	privateKey, err := a.GenerateDatabaseClientKey(ctx)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Postgres requires the database username to be encoded as a common
	// name in the client certificate.
	subject := pkix.Name{CommonName: databaseUser}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// TODO(r0mant): Cache database certificates to avoid expensive generate
	// operation on each connection.
	a.cfg.Logger.DebugContext(ctx, "Generating client certificate", "database_user", databaseUser)

	resp, err := a.cfg.AuthClient.GenerateDatabaseCert(ctx, &proto.DatabaseCertRequest{
		CSR: csr,
		TTL: proto.Duration(expiry.Sub(a.cfg.Clock.Now())),
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

// GenerateDatabaseClientKey generates a cryptographic key appropriate for
// database client connections.
func (a *dbAuth) GenerateDatabaseClientKey(ctx context.Context) (*keys.PrivateKey, error) {
	signer, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(a), cryptosuites.DatabaseClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, err := keys.NewSoftwarePrivateKey(signer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return privateKey, nil
}

// GetAuthPreference returns the cluster authentication config.
func (a *dbAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.cfg.AccessPoint.GetAuthPreference(ctx)
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

	azureClient, ok := metadataClient.(*azureimds.InstanceMetadataClient)
	if !ok {
		return nil, trace.BadParameter("failed to fetch Azure IMDS client")
	}

	info, err := azureClient.GetInstanceInfo(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parsedInstanceID, err := arm.ParseResourceID(info.ResourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vmClient, err := a.cfg.Clients.GetAzureVirtualMachinesClient(parsedInstanceID.SubscriptionID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if info.ScaleSetName != "" {
		vm, err := vmClient.GetByVMID(ctx, info.VMID, libazure.WithVMScaleSetName(info.ScaleSetName))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return vm, nil
	}

	vm, err := vmClient.Get(ctx, info.ResourceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return vm, nil
}

func (a *dbAuth) buildAWSRoleARNFromDatabaseUser(ctx context.Context, database types.Database, databaseUser string) (string, error) {
	dbAWS := database.GetAWS()
	awsAccountID := dbAWS.AccountID

	if awsutils.IsPartialRoleARN(databaseUser) && awsAccountID == "" {
		switch {
		case dbAWS.AssumeRoleARN != "":
			a.cfg.Logger.DebugContext(ctx, "Using AWS Account ID from assumed role")
			assumeRoleARN, err := awsutils.ParseRoleARN(dbAWS.AssumeRoleARN)
			if err != nil {
				return "", trace.Wrap(err)
			}

			awsAccountID = assumeRoleARN.AccountID
		default:
			a.cfg.Logger.DebugContext(ctx, "Fetching AWS Account ID to build role ARN")
			awsCfg, err := a.cfg.AWSConfigProvider.GetConfig(ctx, dbAWS.Region, awsconfig.WithAmbientCredentials())
			if err != nil {
				return "", trace.Wrap(err)
			}
			clt := a.cfg.awsClients.getSTSClient(awsCfg)

			identity, err := awslib.GetIdentityWithClientV2(ctx, clt)
			if err != nil {
				return "", trace.Wrap(err)
			}

			awsAccountID = identity.GetAccountID()
		}
	}

	arn, err := awsutils.BuildRoleARN(databaseUser, dbAWS.Region, awsAccountID)
	return arn, trace.Wrap(err)
}

// GetAWSIAMCreds returns the AWS IAM credentials, including access key, secret
// access key and session token.
func (a *dbAuth) GetAWSIAMCreds(ctx context.Context, database types.Database, databaseUser string) (string, string, string, error) {
	dbAWS := database.GetAWS()
	arn, err := a.buildAWSRoleARNFromDatabaseUser(ctx, database, databaseUser)
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
func newReportingAuth(db types.Database, auth Auth) Auth {
	return &reportingAuth{
		Auth:      auth,
		component: "db:auth",
		db:        db,
	}
}

func (r *reportingAuth) GetTLSConfig(ctx context.Context, expiry time.Time, database types.Database, databaseUser string) (*tls.Config, error) {
	defer methodCallMetrics("GetTLSConfig", r.component, r.db)()
	return r.Auth.GetTLSConfig(ctx, expiry, database, databaseUser)
}
