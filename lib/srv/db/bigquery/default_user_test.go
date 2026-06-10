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

package bigquery

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

func TestResolveDefaultDatabaseUser(t *testing.T) {
	ctx := context.Background()
	log := slog.Default()

	t.Run("user already set is not overwritten", func(t *testing.T) {
		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: "test-project"},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "existing-user",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.NoError(t, err)
		require.Equal(t, "existing-user", sessionCtx.DatabaseUser)
	})

	t.Run("auto-detect from credentials JSON", func(t *testing.T) {
		credsJSON := `{
			"type": "service_account",
			"project_id": "test-project",
			"client_email": "my-service-account@test-project.iam.gserviceaccount.com"
		}`
		tmpFile := filepath.Join(t.TempDir(), "creds.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(credsJSON), 0600))
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile)

		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: "test-project"},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.NoError(t, err)
		require.Equal(t, "my-service-account", sessionCtx.DatabaseUser)
	})

	t.Run("no credentials available returns actionable error", func(t *testing.T) {
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/nonexistent/path/creds.json")

		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: "test-project"},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.Error(t, err)
		require.Contains(t, err.Error(), "either provide --db-user or ensure GOOGLE_APPLICATION_CREDENTIALS")
	})

	t.Run("empty client_email in valid JSON returns actionable error", func(t *testing.T) {
		credsJSON := `{
			"type": "service_account",
			"project_id": "test-project",
			"client_email": ""
		}`
		tmpFile := filepath.Join(t.TempDir(), "creds.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(credsJSON), 0600))
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile)

		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: "test-project"},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.Error(t, err)
		require.Contains(t, err.Error(), "provide --db-user explicitly")
	})

	t.Run("resolved name round-trips through databaseUserToGCPServiceAccount", func(t *testing.T) {
		projectID := "my-project"
		originalEmail := "my-sa@my-project.iam.gserviceaccount.com"
		credsJSON := fmt.Sprintf(`{
			"type": "service_account",
			"project_id": "%s",
			"client_email": "%s"
		}`, projectID, originalEmail)
		tmpFile := filepath.Join(t.TempDir(), "creds.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(credsJSON), 0600))
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile)

		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: projectID},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.NoError(t, err)

		// Verify: databaseUserToGCPServiceAccount reconstructs the original email.
		reconstructed := fmt.Sprintf("%s@%s.iam.gserviceaccount.com",
			sessionCtx.DatabaseUser, projectID)
		require.Equal(t, originalEmail, reconstructed)
	})

	t.Run("authorized_user credentials without client_email returns error", func(t *testing.T) {
		credsJSON := `{
			"type": "authorized_user",
			"client_id": "12345.apps.googleusercontent.com",
			"client_secret": "secret",
			"refresh_token": "token"
		}`
		tmpFile := filepath.Join(t.TempDir(), "creds.json")
		require.NoError(t, os.WriteFile(tmpFile, []byte(credsJSON), 0600))
		t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile)

		db, err := types.NewDatabaseV3(types.Metadata{Name: "test-bq"}, types.DatabaseSpecV3{
			Protocol: defaults.ProtocolBigQuery,
			URI:      "bigquery.googleapis.com:443",
			GCP:      types.GCPCloudSQL{ProjectID: "test-project"},
		})
		require.NoError(t, err)

		sessionCtx := &common.Session{
			DatabaseUser: "",
			Database:     db,
		}

		err = resolveDefaultDatabaseUser(ctx, sessionCtx, log)
		require.Error(t, err)
		require.Contains(t, err.Error(), "provide --db-user explicitly")
	})
}
