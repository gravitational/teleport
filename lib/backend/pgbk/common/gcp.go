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
	"fmt"
	"log/slog"
	"time"

	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/gravitational/teleport/lib/utils/gcp"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GCPSQLBeforeConnect returns a pgx BeforeConnect function suitable for GCP
// SQL PostgreSQL with IAM authentication.
func GCPSQLBeforeConnect(ctx context.Context, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	return gcpOAuthAccessTokenBeforeConnect(ctx, gcpAccessTokenGetterImpl{}, gcpSQLOAuthScope, logger)
}

// GCPAlloyDBBeforeConnect returns a pgx BeforeConnect function suitable for GCP
// AlloyDB (PostgreSQL-compatiable) with IAM authentication.
func GCPAlloyDBBeforeConnect(ctx context.Context, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	return gcpOAuthAccessTokenBeforeConnect(ctx, gcpAccessTokenGetterImpl{}, gcpAlloyDBOAuthScope, logger)
}

const (
	// gcpSQLOAuthScope is the scope used for GCP SQL IAM authentication.
	// https://developers.google.com/identity/protocols/oauth2/scopes#sqladmin
	gcpSQLOAuthScope = "https://www.googleapis.com/auth/sqlservice.admin"
	// gcpAlloyDBOAuthScope is the scope used for GCP AlloyDB IAM authentication.
	// https://cloud.google.com/alloydb/docs/connect-iam
	gcpAlloyDBOAuthScope = "https://www.googleapis.com/auth/alloydb.login"

	gcpServiceAccountEmailSuffix = ".gserviceaccount.com"
)

type gcpAccessTokenGetter interface {
	getFromCredentials(ctx context.Context, credentials *google.Credentials) (*oauth2.Token, error)
	generateForServiceAccount(ctx context.Context, serviceAccount, scope string) (string, time.Time, error)
}

func gcpOAuthAccessTokenBeforeConnect(ctx context.Context, tokenGetter gcpAccessTokenGetter, scope string, logger *slog.Logger) (func(context.Context, *pgx.ConnConfig) error, error) {
	defaultCred, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		// google.FindDefaultCredentials gives pretty error descriptions already.
		return nil, trace.Wrap(err)
	}

	// This function tries to capture service account emails from various
	// credentials methods but may fail for some unknown scenarios.
	defaultServiceAccount, err := gcp.GetServiceAccountFromCredentials(defaultCred)
	if err != nil || defaultServiceAccount == "" {
		logger.WarnContext(ctx, "Failed to get service account email from default google credentials. Teleport will assume the database user in the PostgreSQL connection string matches the service account of the default google credentials.", "err", err, "sa", defaultServiceAccount)
	}

	return func(ctx context.Context, config *pgx.ConnConfig) error {
		// IAM auth users have the PostgreSQL username of their emails minus the
		// ".gserviceaccount.com" part. Now add the suffix back for the full
		// service account email.
		serviceAccountToAuth := config.User + gcpServiceAccountEmailSuffix

		// If the requested db user is for another service account, the
		// "host"/default service account can impersonate the target service
		// account as a Token Creator. This is useful when using a different
		// database user for change feed.
		if defaultServiceAccount != "" && defaultServiceAccount != serviceAccountToAuth {
			token, ttl, err := tokenGetter.generateForServiceAccount(ctx, serviceAccountToAuth, scope)
			if err != nil {
				return trace.Wrap(err, "generating GCP access token for %v", serviceAccountToAuth)
			}

			logger.DebugContext(ctx, "Generated GCP access token.", "service_account", serviceAccountToAuth, "ttl", ttl)
			config.Password = token
			return nil
		}

		token, err := tokenGetter.getFromCredentials(ctx, defaultCred)
		if err != nil {
			return trace.Wrap(err, "obtaining GCP access token from default credentials")
		}

		logger.DebugContext(ctx, "Obtained GCP access token from default credentials.", "ttl", token.Expiry, "token_type", token.TokenType)
		config.Password = token.AccessToken
		return nil
	}, nil
}

type gcpAccessTokenGetterImpl struct {
}

func (g gcpAccessTokenGetterImpl) getFromCredentials(ctx context.Context, credentials *google.Credentials) (*oauth2.Token, error) {
	token, err := credentials.TokenSource.Token()
	return token, trace.Wrap(err)
}

func (g gcpAccessTokenGetterImpl) generateForServiceAccount(ctx context.Context, serviceAccount, scope string) (string, time.Time, error) {
	gcpIAM, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	defer func() {
		if err := gcpIAM.Close(); err != nil {
			slog.DebugContext(ctx, "Failed to close GCP IAM Credentials client.", "err", err)
		}
	}()

	resp, err := gcpIAM.GenerateAccessToken(ctx, &credentialspb.GenerateAccessTokenRequest{
		Name:  fmt.Sprintf("projects/-/serviceAccounts/%v", serviceAccount),
		Scope: []string{scope},
	})
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}
	return resp.AccessToken, resp.ExpireTime.AsTime(), nil
}
