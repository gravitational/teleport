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
	"encoding/json"
	"fmt"
	"log/slog"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2/google"
)

// GCPSQLBeforeConnect returns a gpx BeforeConnect function suitable for GCP
// SQL PostgreSQL with IAM authentication.
func GCPSQLBeforeConnect(ctx context.Context, slog *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	return gcpOAUTHTokenBeforeConnect(ctx, gcpSQLOAuthScope, slog)
}

// GCPAlloyDBBeforeConnect returns a gpx BeforeConnect function suitable for GCP
// AlloyDB (PostgreSQL-compatiable) with IAM authentication.
func GCPAlloyDBBeforeConnect(ctx context.Context, slog *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	return gcpOAUTHTokenBeforeConnect(ctx, gcpAlloyDBOAuthScope, slog)
}

const (
	// gcpSQLOAuthScope is the scope used for GCP SQL IAM authentication.
	// https://developers.google.com/identity/protocols/oauth2/scopes#sqladmin
	gcpSQLOAuthScope = "https://www.googleapis.com/auth/sqlservice.admin"
	// gcpAlloyDBOAuthScope is the scope used for GCP AlloyDB IAM authentication.
	// https://cloud.google.com/alloydb/docs/connect-iam
	gcpAlloyDBOAuthScope = "https://www.googleapis.com/auth/alloydb.login"
)

func gcpOAUTHTokenBeforeConnect(ctx context.Context, scope string, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	defaultCred, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		// google.FindDefaultCredentials gives pretty error descriptions already.
		return nil, trace.Wrap(err)
	}

	defaultServiceAccount, err := getClientEmailFromGCPCredentials(defaultCred, logger)
	if err != nil || defaultServiceAccount == "" {
		// The above check shouldn't fail. Just logging in case.
		logger.Warn("Could not find client email from default google credentials. Teleport will always try to generate access tokens by impersonating the specified GCP service account for IAM authentication.", "err", err, "email", defaultServiceAccount)
	} else {
		logger.Debug("Retreived client email from default google credentials.", "email", defaultServiceAccount)
	}

	return func(ctx context.Context, config *pgx.ConnConfig) error {
		// IAM auth users have the PostgreSQL username of their emails minus
		// the ".gserviceaccount.com" part. So add this part back for the full
		// service account email.
		serviceAccountToAuth := config.User + ".gserviceaccount.com"

		// If the requested db user is for another service account, the
		// "host"/default service account can impersonate the target service
		// account as a Token Creator.
		if defaultServiceAccount != serviceAccountToAuth {
			token, err := genGCPAccessTokenForServiceAccount(ctx, serviceAccountToAuth, scope, logger)
			if err != nil {
				return trace.Wrap(err, "generating GCP access token for %v", serviceAccountToAuth)
			}

			config.Password = token
			return nil
		}

		token, err := getGCPAccessTokenFromCredentials(defaultCred, logger)
		if err != nil {
			return trace.Wrap(err, "obtaining GCP access token from default credentials")
		}

		config.Password = token
		return nil
	}, nil
}

func getClientEmailFromGCPCredentials(credentials *google.Credentials, logger *slog.Logger) (string, error) {
	// When credentials JSON file is provided through either
	// GOOGLE_APPLICATION_CREDENTIALS env var or a well known file.
	if len(credentials.JSON) > 0 {
		content := struct {
			ClientEmail string `json:"client_email"`
		}{}

		err := json.Unmarshal(credentials.JSON, &content)
		return content.ClientEmail, trace.Wrap(err)
	}

	// No JSON credentials but using metadata endpoints when on Google Compute.
	if metadata.OnGCE() {
		email, err := metadata.Email("")
		return email, trace.Wrap(err)
	}

	return "", trace.NotImplemented("unknown scenario for getting client email")
}

func getGCPAccessTokenFromCredentials(defaultCred *google.Credentials, logger *slog.Logger) (string, error) {
	token, err := defaultCred.TokenSource.Token()
	if err != nil {
		return "", trace.Wrap(err)
	}

	logger.Debug("Acquired GCP access token from default credentials.", "ttl", token.Expiry)
	return token.AccessToken, nil
}

func genGCPAccessTokenForServiceAccount(ctx context.Context, serviceAccount, scope string, logger *slog.Logger) (string, error) {
	gcpIAM, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	resp, err := gcpIAM.GenerateAccessToken(
		ctx,
		&credentialspb.GenerateAccessTokenRequest{
			// The resource name of the service account for which the credentials
			// are requested, in the following format:
			// `projects/-/serviceAccounts/{ACCOUNT_EMAIL_OR_UNIQUEID}`.
			Name:  fmt.Sprintf("projects/-/serviceAccounts/%v", serviceAccount),
			Scope: []string{scope},
		},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	logger.Debug("Generated GCP access token.", "ttl", resp.ExpireTime, "sa", serviceAccount)
	return resp.AccessToken, nil
}
