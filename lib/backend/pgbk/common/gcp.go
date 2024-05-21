/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package pgcommon

import (
	"context"
	"log/slog"
	"net"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"

	gcputils "github.com/gravitational/teleport/lib/utils/gcp"
)

// ConfigureConnectionForGCPCloudSQL configures the provide poolConfig to use
// cloudsqlconn for "automatic" IAM database authentication.
//
// https://cloud.google.com/sql/docs/postgres/iam-authentication
func ConfigureConnectionForGCPCloudSQL(ctx context.Context, logger *slog.Logger, connConfig *pgx.ConnConfig) error {
	if connConfig == nil {
		return trace.BadParameter("missing connection config")
	}

	gcpConfig, err := gcpConfigFromConnConfig(connConfig)
	if err != nil {
		return trace.Wrap(err, "invalid postgresql url %s", connConfig.ConnString())
	}

	dialFunc, err := makeGCPCloudSQLDialFunc(ctx, gcpConfig, logger)
	if err != nil {
		return trace.Wrap(err)
	}

	connConfig.DialFunc = dialFunc
	return nil
}

func makeGCPCloudSQLDialFunc(ctx context.Context, config *gcpConfig, logger *slog.Logger) (pgconn.DialFunc, error) {
	iamAuthOptions, err := makeGCPCloudSQLAuthOptionsForServiceAccount(ctx, config.serviceAccount, gcpServiceAccountImpersonatorImpl{}, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer, err := cloudsqlconn.NewDialer(ctx, iamAuthOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var dialOptions []cloudsqlconn.DialOption
	if ipTypeOption := config.ipType.cloudsqlconnOption(); ipTypeOption != nil {
		dialOptions = append(dialOptions, ipTypeOption)
	}

	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		// Use connection name and ignore network and host address.
		logger.DebugContext(ctx, "Dialing GCP Cloud SQL.", "connection_name", config.connectionName, "service_account", config.serviceAccount, "ip_type", config.ipType)
		conn, err := dialer.Dial(ctx, config.connectionName, dialOptions...)
		return conn, trace.Wrap(err)
	}, nil
}

func makeGCPCloudSQLAuthOptionsForServiceAccount(ctx context.Context, targetServiceAccount string, impersonator gcpServiceAccountImpersonator, logger *slog.Logger) ([]cloudsqlconn.Option, error) {
	defaultCred, err := google.FindDefaultCredentials(ctx)
	if err != nil {
		// google.FindDefaultCredentials gives pretty error descriptions already.
		return nil, trace.Wrap(err)
	}

	// This function tries to capture service account emails from various
	// credentials methods but may fail for some unknown scenarios.
	defaultServiceAccount, err := gcputils.GetServiceAccountFromCredentials(defaultCred)
	if err != nil || defaultServiceAccount == "" {
		logger.WarnContext(ctx, "Failed to get service account email from default google credentials. Teleport will assume the database user in the PostgreSQL connection string matches the service account of the default google credentials.", "err", err, "sa", defaultServiceAccount)
		return []cloudsqlconn.Option{cloudsqlconn.WithIAMAuthN()}, nil
	}

	// If the requested db user is for another service account, the default
	// service account can impersonate the target service account as a Token
	// Creator. This is useful when using a different database user for change
	// feed. Otherwise, let cloudsqlconn use the default credentials.
	if defaultServiceAccount == targetServiceAccount {
		logger.InfoContext(ctx, "Using google default credentials for Cloud SQL backend.")
		return []cloudsqlconn.Option{cloudsqlconn.WithIAMAuthN()}, nil
	}

	// For simplicity, we assume the target service account will be used for
	// both API and IAM auth. See description of
	// cloudsqlconn.WithIAMAuthNTokenSources on the required scopes.
	logger.InfoContext(ctx, "Impersonating a service account for Cloud SQL backend.", "service_account", targetServiceAccount)

	apiTokenSource, err := impersonator.makeTokenSource(ctx, targetServiceAccount, "https://www.googleapis.com/auth/sqlservice.admin")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	iamAuthTokenSource, err := impersonator.makeTokenSource(ctx, targetServiceAccount, "https://www.googleapis.com/auth/sqlservice.login")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []cloudsqlconn.Option{
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithIAMAuthNTokenSources(apiTokenSource, iamAuthTokenSource),
	}, nil
}

type gcpServiceAccountImpersonator interface {
	makeTokenSource(context.Context, string, ...string) (oauth2.TokenSource, error)
}

type gcpServiceAccountImpersonatorImpl struct {
}

func (g gcpServiceAccountImpersonatorImpl) makeTokenSource(ctx context.Context, targetServiceAccount string, scopes ...string) (oauth2.TokenSource, error) {
	tokenSource, err := impersonate.CredentialsTokenSource(
		ctx,
		impersonate.CredentialsConfig{
			TargetPrincipal: targetServiceAccount,
			Scopes:          scopes,
		},
	)
	// tokenSource caches the access token and only refreshes when expired.
	return tokenSource, trace.Wrap(err)
}
