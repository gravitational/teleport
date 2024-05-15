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
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type fakeGCPAccessTokenGetter struct {
}

func (f fakeGCPAccessTokenGetter) getFromCredentials(context.Context, *google.Credentials) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "token-from-default-credentials",
		Expiry:      time.Now().Add(time.Hour),
	}, nil
}

func (f fakeGCPAccessTokenGetter) generateForServiceAccount(ctx context.Context, serviceAccount, scope string) (string, time.Time, error) {
	return fmt.Sprintf("token-for-%s-with-scope-%s", serviceAccount, scope), time.Now().Add(time.Hour), nil
}

func Test_gcpOAuthAccessTokenBeforeConnect(t *testing.T) {
	mustSetGoogleApplicationCredentialsEnv(t)

	ctx := context.Background()
	tokenGetter := fakeGCPAccessTokenGetter{}
	tests := []struct {
		name         string
		config       *pgx.ConnConfig
		wantUser     string
		wantPassword string
	}{
		{
			name: "default service account as user in connection string",
			config: &pgx.ConnConfig{
				Config: pgconn.Config{
					User: "my-service-account@teleport-example-123456.iam",
				},
			},
			wantUser:     "my-service-account@teleport-example-123456.iam",
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
			bc, err := gcpOAuthAccessTokenBeforeConnect(ctx, tokenGetter, "test-scope", slog.Default())
			require.NoError(t, err)

			err = bc(context.Background(), tc.config)
			require.NoError(t, err)
			require.Equal(t, tc.wantUser, tc.config.User)
			require.Equal(t, tc.wantPassword, tc.config.Password)
		})
	}
}

func mustSetGoogleApplicationCredentialsEnv(t *testing.T) {
	t.Helper()

	file := path.Join(t.TempDir(), uuid.New().String())
	err := os.WriteFile(file, []byte(fakeServiceAccountCredentialsJSON), 0644)
	require.NoError(t, err)

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", file)
}

const (
	fakeServiceAccountCredentialsJSON = `{
  "type": "service_account",
  "project_id": "teleport-example-123456",
  "private_key_id": "1234569890abcdef1234567890abcdef12345678",
  "private_key": "fake-private-key",
  "client_email": "my-service-account@teleport-example-123456.iam.gserviceaccount.com",
  "client_id": "111111111111111111111",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-service-account%40teleport-example-123456.iam.gserviceaccount.com",
  "universe_domain": "googleapis.com"
}`
)
