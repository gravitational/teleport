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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type mockGCPServiceAccountImpersonator struct {
	calledForServiceAccount []string
}

func (m *mockGCPServiceAccountImpersonator) makeTokenSource(_ context.Context, serviceAccount string, _ ...string) (oauth2.TokenSource, error) {
	m.calledForServiceAccount = append(m.calledForServiceAccount, serviceAccount)
	return oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: "access_token",
		Expiry:      time.Now().Add(time.Hour),
	}), nil
}

func Test_makeGCPCloudSQLAuthOptionsForServiceAccount(t *testing.T) {
	mustSetGoogleApplicationCredentialsEnv(t)
	ctx := context.Background()
	logger := slog.Default()
	m := &mockGCPServiceAccountImpersonator{}

	t.Run("using default credentials", func(t *testing.T) {
		defaultServiceAccount := "my-service-account@teleport-example-123456.iam.gserviceaccount.com"
		options, err := makeGCPCloudSQLAuthOptionsForServiceAccount(ctx, defaultServiceAccount, m, logger)
		require.NoError(t, err)

		// Cannot validate the actual options. Just check that the count of
		// options is correct and impersonator is NOT called.
		require.Len(t, options, 1)
		require.Empty(t, m.calledForServiceAccount)
	})

	t.Run("impersonate a service account", func(t *testing.T) {
		otherServiceAccount := "my-other-service-account@teleport-example-123456.iam"
		options, err := makeGCPCloudSQLAuthOptionsForServiceAccount(ctx, otherServiceAccount, m, logger)
		require.NoError(t, err)

		// Cannot validate the actual options. Just check that the count of
		// options is correct and impersonator is called twice (once for API
		// client and once for IAM auth).
		require.Len(t, options, 2)
		require.Equal(t,
			[]string{otherServiceAccount, otherServiceAccount},
			m.calledForServiceAccount,
		)
	})
}

func mustSetGoogleApplicationCredentialsEnv(t *testing.T) {
	t.Helper()

	file := filepath.Join(t.TempDir(), uuid.New().String())
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
