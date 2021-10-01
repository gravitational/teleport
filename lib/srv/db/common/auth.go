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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	libauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
	"github.com/aws/aws-sdk-go/service/redshift"

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	gcpcredentialspb "google.golang.org/genproto/googleapis/iam/credentials/v1"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

// Auth defines interface for creating auth tokens and TLS configurations.
type Auth interface {
	// GetRDSAuthToken generates RDS/Aurora auth token.
	GetRDSAuthToken(sessionCtx *Session) (string, error)
	// GetRedshiftAuthToken generates Redshift auth token.
	GetRedshiftAuthToken(sessionCtx *Session) (string, string, error)
	// GetCloudSQLAuthToken generates Cloud SQL auth token.
	GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error)
	// GetCloudSQLPassword generates password for a Cloud SQL database user.
	GetCloudSQLPassword(ctx context.Context, sessionCtx *Session) (string, error)
	// GetTLSConfig builds the client TLS configuration for the session.
	GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error)
	// GetAuthPreference returns the cluster authentication config.
	GetAuthPreference(ctx context.Context) (types.AuthPreference, error)
	// Closes releases all resources used by authenticator.
	io.Closer
}

// AuthConfig is the database access authenticator configuration.
type AuthConfig struct {
	// AuthClient is the cluster auth client.
	AuthClient *libauth.Client
	// Clients provides interface for obtaining cloud provider clients.
	Clients CloudClients
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
		c.Clients = NewCloudClients()
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, "db:auth")
	}
	return nil
}

// dbAuth provides utilities for creating TLS configurations and
// generating auth tokens when connecting to databases.
type dbAuth struct {
	cfg AuthConfig
}

// NewAuth returns a new instance of database access authenticator.
func NewAuth(config AuthConfig) (Auth, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &dbAuth{
		cfg: config,
	}, nil
}

// GetRDSAuthToken returns authorization token that will be used as a password
// when connecting to RDS and Aurora databases.
func (a *dbAuth) GetRDSAuthToken(sessionCtx *Session) (string, error) {
	awsSession, err := a.cfg.Clients.GetAWSSession(sessionCtx.Database.GetAWS().Region)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating RDS auth token for %s.", sessionCtx)
	token, err := rdsutils.BuildAuthToken(
		sessionCtx.Database.GetURI(),
		sessionCtx.Database.GetAWS().Region,
		sessionCtx.DatabaseUser,
		awsSession.Config.Credentials)
	if err != nil {
		return "", trace.AccessDenied(`Could not generate RDS IAM auth token:

  %v

Make sure that Teleport database agent's IAM policy is attached and has "rds-connect"
permissions:

%v
`, err, sessionCtx.Database.GetRDSPolicy())
	}
	return token, nil
}

// GetRedshiftAuthToken returns authorization token that will be used as a
// password when connecting to Redshift databases.
func (a *dbAuth) GetRedshiftAuthToken(sessionCtx *Session) (string, string, error) {
	awsSession, err := a.cfg.Clients.GetAWSSession(sessionCtx.Database.GetAWS().Region)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating Redshift auth token for %s.", sessionCtx)
	resp, err := redshift.New(awsSession).GetClusterCredentials(&redshift.GetClusterCredentialsInput{
		ClusterIdentifier: aws.String(sessionCtx.Database.GetAWS().Redshift.ClusterID),
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
		return "", "", trace.AccessDenied(`Could not generate Redshift IAM auth token:

  %v

Make sure that Teleport database agent's IAM policy is attached and has permissions
to generate Redshift credentials:

%v
`, err, sessionCtx.Database.GetRedshiftPolicy())
	}
	return *resp.DbUser, *resp.DbPassword, nil
}

// GetCloudSQLAuthToken returns authorization token that will be used as a
// password when connecting to Cloud SQL databases.
func (a *dbAuth) GetCloudSQLAuthToken(ctx context.Context, sessionCtx *Session) (string, error) {
	gcpIAM, err := a.cfg.Clients.GetGCPIAMClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	a.cfg.Log.Debugf("Generating GCP auth token for %s.", sessionCtx)
	resp, err := gcpIAM.GenerateAccessToken(ctx,
		&gcpcredentialspb.GenerateAccessTokenRequest{
			// From GenerateAccessToken docs:
			//
			// The resource name of the service account for which the credentials
			// are requested, in the following format:
			//   projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}
			Name: fmt.Sprintf("projects/-/serviceAccounts/%v.gserviceaccount.com", sessionCtx.DatabaseUser),
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
	token, err := utils.CryptoRandomHex(libauth.TokenLenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// Cloud SQL will return 409 to a user update operation if there is another
	// one in progress, so retry upon encountering it. Also, be nice to the API
	// and retry with a backoff.
	retry, err := utils.NewConstant(time.Second)
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
			return utils.PermanentRetryError(err)
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

// updateCloudSQLUser makes a request to Cloud SQL API to update the provided user.
func (a *dbAuth) updateCloudSQLUser(ctx context.Context, sessionCtx *Session, gcpCloudSQL *sqladmin.Service, user *sqladmin.User) error {
	_, err := gcpCloudSQL.Users.Update(
		sessionCtx.Database.GetGCP().ProjectID,
		sessionCtx.Database.GetGCP().InstanceID,
		user).Name(sessionCtx.DatabaseUser).Host("%").Context(ctx).Do()
	if err != nil {
		return trace.AccessDenied(`Could not update Cloud SQL user %q password:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role, or
"cloudsql.users.update" IAM permission.
`, sessionCtx.DatabaseUser, err)
	}
	return nil
}

// GetTLSConfig builds the client TLS configuration for the session.
//
// For RDS/Aurora, the config must contain RDS root certificate as a trusted
// authority. For onprem we generate a client certificate signed by the host
// CA used to authenticate.
func (a *dbAuth) GetTLSConfig(ctx context.Context, sessionCtx *Session) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
	}
	// Don't set the ServerName when connecting to a MongoDB cluster - in case
	// of replica set the driver may dial multiple servers and will set
	// ServerName itself. For Postgres/MySQL we're always connecting to the
	// server specified in URI so set ServerName ourselves.
	if sessionCtx.Database.GetProtocol() != defaults.ProtocolMongoDB {
		addr, err := utils.ParseAddr(sessionCtx.Database.GetURI())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConfig.ServerName = addr.Host()
	}
	// Add CA certificate to the trusted pool if it's present, e.g. when
	// connecting to RDS/Aurora which require AWS CA.
	if len(sessionCtx.Database.GetCA()) != 0 {
		if !tlsConfig.RootCAs.AppendCertsFromPEM([]byte(sessionCtx.Database.GetCA())) {
			return nil, trace.BadParameter("invalid server CA certificate")
		}
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
		tlsConfig.ServerName = fmt.Sprintf("%v:%v", sessionCtx.Database.GetGCP().ProjectID, sessionCtx.Database.GetGCP().InstanceID)
		// This just disables default verification.
		tlsConfig.InsecureSkipVerify = true
		// This will verify CN and cert chain on each connection.
		tlsConfig.VerifyConnection = getVerifyCloudSQLCertificate(tlsConfig.RootCAs)
	}
	// RDS/Aurora/Redshift and Cloud SQL auth is done with an auth token so
	// don't generate a client certificate and exit here.
	if sessionCtx.Database.IsRDS() || sessionCtx.Database.IsRedshift() || sessionCtx.Database.IsCloudSQL() {
		return tlsConfig, nil
	}
	// Otherwise, when connecting to an onprem database, generate a client
	// certificate. The database instance should be configured with
	// Teleport's CA obtained with 'tctl auth sign --type=db'.
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

// getClientCert signs an ephemeral client certificate used by this
// server to authenticate with the database instance.
func (a *dbAuth) getClientCert(ctx context.Context, sessionCtx *Session) (cert *tls.Certificate, cas [][]byte, err error) {
	privateBytes, _, err := native.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Postgres requires the database username to be encoded as a common
	// name in the client certificate.
	subject := pkix.Name{CommonName: sessionCtx.DatabaseUser}
	csr, err := tlsca.GenerateCertificateRequestPEM(subject, privateBytes)
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
	clientCert, err := tls.X509KeyPair(resp.Cert, privateBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return &clientCert, resp.CACerts, nil
}

// GetAuthPreference returns the cluster authentication config.
func (a *dbAuth) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return a.cfg.AuthClient.GetAuthPreference(ctx)
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
