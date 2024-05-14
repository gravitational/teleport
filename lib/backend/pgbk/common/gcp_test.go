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
	"net/url"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
)

func Test_gcpOAuthTokenGetter(t *testing.T) {
	mustSetGoogleApplicationCredentialsEnv(t)

	gcp, err := newGCPOAuthTokenGetter(context.Background(), "test-scope", slog.Default())
	require.NoError(t, err)

	// Define some mocks.
	gcp.genAccessTokenForServiceAccount = func(_ context.Context, sa, scope string, _ *slog.Logger) (string, error) {
		return fmt.Sprintf("token-for-%s-with-scope-%s", sa, scope), nil
	}
	gcp.getAccessTokenFromCredentials = func(context.Context, *google.Credentials, *slog.Logger) (string, error) {
		return "token-from-default-credentials", nil
	}

	defaultDBUser := strings.TrimSuffix(mockGoogleServiceAccount, gcpServiceAccountEmailSuffix)
	tests := []struct {
		name         string
		config       *pgx.ConnConfig
		wantUser     string
		wantPassword string
	}{
		{
			name:         "no user in connection string",
			config:       &pgx.ConnConfig{},
			wantUser:     defaultDBUser,
			wantPassword: "token-from-default-credentials",
		},
		{
			name: "default service account as user in connection string",
			config: &pgx.ConnConfig{
				Config: pgconn.Config{
					User: defaultDBUser,
				},
			},
			wantUser:     defaultDBUser,
			wantPassword: "token-from-default-credentials",
		},
		{
			name: "another service account as user in connection string",
			config: &pgx.ConnConfig{
				Config: pgconn.Config{
					User: "another-service-account@teleport-example-123456.iam",
				},
			},
			wantUser:     "another-service-account@teleport-example-123456.iam",
			wantPassword: "token-for-another-service-account@teleport-example-123456.iam.gserviceaccount.com-with-scope-test-scope",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := gcp.beforeConnect(context.Background(), tc.config)
			require.NoError(t, err)
			require.Equal(t, tc.wantUser, tc.config.User)
			require.Equal(t, tc.wantPassword, tc.config.Password)
		})
	}
}

func Test_getClientEmailFromGCPCredentials(t *testing.T) {
	mustSetGoogleApplicationCredentialsEnv(t)

	defaultCred, err := google.FindDefaultCredentials(context.Background(), "")
	require.NoError(t, err)

	email, err := getClientEmailFromGCPCredentials(defaultCred, slog.Default())
	require.NoError(t, err)
	require.Equal(t, mockGoogleServiceAccount, email)
}

func mustSetGoogleApplicationCredentialsEnv(t *testing.T) {
	t.Helper()

	file := path.Join(t.TempDir(), uuid.New().String())
	fileContent := fmt.Sprintf(`{
  "type": "service_account",
  "project_id": "teleport-example-123456",
  "private_key_id": "1234569890abcdef1234567890abcdef12345678",
  "private_key": "fake-private-key",
  "client_email": "%s",
  "client_id": "111111111111111111111",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/%s",
  "universe_domain": "googleapis.com"
}`,
		mockGoogleServiceAccount,
		url.PathEscape(mockGoogleServiceAccount),
	)

	err := os.WriteFile(file, []byte(fileContent), 0644)
	require.NoError(t, err)

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", file)
}

const (
	mockGoogleServiceAccount = "my-service-account@teleport-example-123456.iam.gserviceaccount.com"
)
