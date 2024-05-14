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
	"strings"

	"cloud.google.com/go/compute/metadata"
	credentials "cloud.google.com/go/iam/credentials/apiv1"
	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2/google"
)

// GCPSQLBeforeConnect returns a pgx BeforeConnect function suitable for GCP
// SQL PostgreSQL with IAM authentication.
func GCPSQLBeforeConnect(ctx context.Context, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	gcp, err := newGCPOAuthTokenGetter(ctx, gcpSQLOAuthScope, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gcp.beforeConnect, nil
}

// GCPAlloyDBBeforeConnect returns a pgx BeforeConnect function suitable for GCP
// AlloyDB (PostgreSQL-compatiable) with IAM authentication.
func GCPAlloyDBBeforeConnect(ctx context.Context, logger *slog.Logger) (func(ctx context.Context, config *pgx.ConnConfig) error, error) {
	gcp, err := newGCPOAuthTokenGetter(ctx, gcpAlloyDBOAuthScope, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gcp.beforeConnect, nil
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

type gcpOAuthTokenGetter struct {
	defaultCred           *google.Credentials
	defaultServiceAccount string
	scope                 string
	logger                *slog.Logger

	// genAccessTokenForServiceAccount defaults to getGCPAccessTokenFromCredentials but
	// can be overridden for unit test.
	genAccessTokenForServiceAccount func(context.Context, string, string, *slog.Logger) (string, error)
	// getAccessTokenFromCredentials defaults to getGCPAccessTokenFromCredentials but
	// can be overridden for unit test.
	getAccessTokenFromCredentials func(context.Context, *google.Credentials, *slog.Logger) (string, error)
}

func newGCPOAuthTokenGetter(ctx context.Context, scope string, logger *slog.Logger) (*gcpOAuthTokenGetter, error) {
	defaultCred, err := google.FindDefaultCredentials(ctx, scope)
	if err != nil {
		// google.FindDefaultCredentials gives pretty error descriptions already.
		return nil, trace.Wrap(err)
	}

	// Find the full service account email. This call should not fail but
	// checking just in case.
	defaultServiceAccount, err := getClientEmailFromGCPCredentials(defaultCred, logger)
	if err != nil {
		return nil, trace.Wrap(err, "finding client email from default google credentials")
	} else if defaultServiceAccount == "" {
		return nil, trace.NotFound("could not find client email from default google credentials")
	}

	return &gcpOAuthTokenGetter{
		defaultCred:                     defaultCred,
		defaultServiceAccount:           defaultServiceAccount,
		scope:                           scope,
		logger:                          logger,
		genAccessTokenForServiceAccount: genGCPAccessTokenForServiceAccount,
		getAccessTokenFromCredentials:   getGCPAccessTokenFromCredentials,
	}, nil
}

func (g *gcpOAuthTokenGetter) beforeConnect(ctx context.Context, config *pgx.ConnConfig) error {
	// Use the default service account if user is not specified in
	// connection string. IAM auth users have the PostgreSQL username of
	// their emails minus the ".gserviceaccount.com" part.
	if config.User == "" {
		config.User = strings.TrimSuffix(g.defaultServiceAccount, gcpServiceAccountEmailSuffix)
	}

	// Now add the suffix back for the full service account email.
	serviceAccountToAuth := config.User + gcpServiceAccountEmailSuffix

	// If the requested db user is for another service account, the
	// "host"/default service account can impersonate the target service
	// account as a Token Creator.
	if g.defaultServiceAccount != serviceAccountToAuth {
		token, err := g.genAccessTokenForServiceAccount(ctx, serviceAccountToAuth, g.scope, g.logger)
		if err != nil {
			return trace.Wrap(err, "generating GCP access token for %v", serviceAccountToAuth)
		}

		config.Password = token
		return nil
	}

	token, err := g.getAccessTokenFromCredentials(ctx, g.defaultCred, g.logger)
	if err != nil {
		return trace.Wrap(err, "obtaining GCP access token from default credentials")
	}

	config.Password = token
	return nil
}

func getClientEmailFromGCPCredentials(credentials *google.Credentials, logger *slog.Logger) (string, error) {
	// When credentials JSON file is provided through either
	// GOOGLE_APPLICATION_CREDENTIALS env var or a well known file.
	if len(credentials.JSON) > 0 {
		content := struct {
			ClientEmail string `json:"client_email"`
		}{}

		if err := json.Unmarshal(credentials.JSON, &content); err != nil {
			return "", trace.Wrap(err)
		}

		logger.Debug("Retreived client email from default google application credentials.", "email", content.ClientEmail)
		return content.ClientEmail, nil
	}

	// No credentials from JSON files but using metadata endpoints when on
	// Google Compute Engine.
	if metadata.OnGCE() {
		email, err := metadata.Email("")
		if err != nil {
			return "", trace.Wrap(err)
		}

		logger.Debug("Retreived client email from GCE metadata.", "email", email)
		return email, nil
	}

	return "", trace.NotImplemented("unknown scenario for getting client email")
}

func getGCPAccessTokenFromCredentials(ctx context.Context, defaultCred *google.Credentials, logger *slog.Logger) (string, error) {
	token, err := defaultCred.TokenSource.Token()
	if err != nil {
		return "", trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Acquired GCP access token from default credentials.", "ttl", token.Expiry)
	return token.AccessToken, nil
}

func genGCPAccessTokenForServiceAccount(ctx context.Context, serviceAccount, scope string, logger *slog.Logger) (string, error) {
	gcpIAM, err := credentials.NewIamCredentialsClient(ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}

	defer func() {
		if err := gcpIAM.Close(); err != nil {
			logger.DebugContext(ctx, "Failed to close GCP IAM Credentials client", "err", err)
		}
	}()

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

	logger.DebugContext(ctx, "Generated GCP access token.", "ttl", resp.ExpireTime, "service_account", serviceAccount)
	return resp.AccessToken, nil
}
