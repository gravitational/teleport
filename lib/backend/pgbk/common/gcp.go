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
	"slices"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"

	apiutils "github.com/gravitational/teleport/api/utils"
	gcputils "github.com/gravitational/teleport/lib/utils/gcp"
)

// GCPIPType specifies the type of IP used for GCP connection.
//
// Values are sourced from:
// https://github.com/GoogleCloudPlatform/cloud-sql-go-connector/blob/main/internal/cloudsql/refresh.go
// https://github.com/GoogleCloudPlatform/alloydb-go-connector/blob/main/internal/alloydb/refresh.go
//
// Note that AutoIP is not recommended for Cloud SQL and not present for
// AlloyDB. So we are not supporting AutoIP. Values are also lower-cased for
// simplicity. If not specified, the library defaults to public.
type GCPIPType string

const (
	GCPIPTypeUnspecified           GCPIPType = ""
	GCPIPTypePublicIP              GCPIPType = "public"
	GCPIPTypePrivateIP             GCPIPType = "private"
	GCPIPTypePrivateServiceConnect GCPIPType = "psc"
)

var gcpIPTypes = []GCPIPType{
	GCPIPTypeUnspecified,
	GCPIPTypePublicIP,
	GCPIPTypePrivateIP,
	GCPIPTypePrivateServiceConnect,
}

func (g GCPIPType) check() error {
	if slices.Contains(gcpIPTypes, g) {
		return nil
	}
	return trace.BadParameter("invalid GCP IP type %q, should be one of \"%v\"", g, apiutils.JoinStrings(gcpIPTypes, `", "`))
}

func (g GCPIPType) cloudsqlconnOption() cloudsqlconn.DialOption {
	switch g {
	case GCPIPTypePublicIP:
		return cloudsqlconn.WithPublicIP()
	case GCPIPTypePrivateIP:
		return cloudsqlconn.WithPrivateIP()
	case GCPIPTypePrivateServiceConnect:
		return cloudsqlconn.WithPSC()
	default:
		return nil
	}
}

// GCPCloudSQLDialFunc creates a pgconn.DialFunc to use cloudsqlconn for
// "automatic" IAM database authentication.
//
// https://cloud.google.com/sql/docs/postgres/iam-authentication
func GCPCloudSQLDialFunc(ctx context.Context, config AuthConfig, dbUser string, logger *slog.Logger) (pgconn.DialFunc, error) {
	// IAM auth users have the PostgreSQL username of their emails minus
	// the ".gserviceaccount.com" part. Now add the suffix back for the
	// full service account email.
	targetServiceAccount := dbUser + ".gserviceaccount.com"
	if err := gcputils.ValidateGCPServiceAccountName(targetServiceAccount); err != nil {
		return nil, trace.Wrap(err, "IAM database user for service account should have usernames in format of <service_account_name>@<project_id>.iam but got %s", dbUser)
	}

	iamAuthOptions, err := makeGCPCloudSQLAuthOptionsForServiceAccount(ctx, targetServiceAccount, gcpServiceAccountImpersonatorImpl{}, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dialer, err := cloudsqlconn.NewDialer(ctx, iamAuthOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var dialOptions []cloudsqlconn.DialOption
	if ipTypeOption := config.GCPIPType.cloudsqlconnOption(); ipTypeOption != nil {
		dialOptions = append(dialOptions, ipTypeOption)
	}

	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		// Use connection name and ignore network and host address.
		logger.DebugContext(ctx, "Dialing GCP Cloud SQL.", "connection_name", config.GCPConnectionName, "service_account", targetServiceAccount, "ip_type", config.GCPIPType)
		conn, err := dialer.Dial(ctx, config.GCPConnectionName, dialOptions...)
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
