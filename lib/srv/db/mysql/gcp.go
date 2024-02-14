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

package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// TODO
func isDBUserGCPServiceAccount(dbUser string) bool {
	if strings.Contains(dbUser, "@") {
		if strings.HasSuffix(dbUser, ".iam") || strings.HasSuffix(dbUser, ".iam.gserviceaccount.com") {
			return true
		}
	}
	return false
}

// TODO
func getInDatabaseUserFromGCPServiceAccount(serviceAccountName string) string {
	user, _, _ := strings.Cut(serviceAccountName, "@")
	return user
}

func makeGCPServiceAccountFromInDatabaseUser(sessionCtx *common.Session) string {
	return fmt.Sprintf("%s@%s.iam.gserviceaccount.com", sessionCtx.DatabaseUser, sessionCtx.Database.GetGCP().ProjectID)
}

func (e *Engine) getGCPUserAndPassword(ctx context.Context, sessionCtx *common.Session, gcpClient gcp.SQLAdminClient) (string, string, error) {
	// If `--db-user` is an service account email, use IAM Auth.
	if isDBUserGCPServiceAccount(sessionCtx.DatabaseUser) {
		return e.getGCPIAMUserAndPassword(ctx, sessionCtx)
	}

	// Get user info to decide how to authenticate.
	user, err := gcpClient.GetUser(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		// GetUser permission is new for IAM auth. If no permission, assume legacy password user.
		if trace.IsAccessDenied(err) {
			return e.getGCPPasswordUserAndPassword(ctx, sessionCtx)
		}
		return "", "", trace.Wrap(err)
	}

	// Possible values (copied from SDK):
	//   "BUILT_IN" - The database's built-in user type.
	//   "CLOUD_IAM_USER" - Cloud IAM user.
	//   "CLOUD_IAM_SERVICE_ACCOUNT" - Cloud IAM service account.
	//   "CLOUD_IAM_GROUP" - Cloud IAM group non-login user.
	//   "CLOUD_IAM_GROUP_USER" - Cloud IAM group login user.
	//   "CLOUD_IAM_GROUP_SERVICE_ACCOUNT" - Cloud IAM group service account.
	//
	// In practice, type can also be empty for built-in user.
	switch user.Type {
	case "",
		"BUILT_IN":
		return e.getGCPPasswordUserAndPassword(ctx, sessionCtx)

	case "CLOUD_IAM_SERVICE_ACCOUNT",
		"CLOUD_IAM_GROUP_SERVICE_ACCOUNT":
		return e.getGCPIAMUserAndPassword(
			ctx,
			sessionCtx.WithUser(makeGCPServiceAccountFromInDatabaseUser(sessionCtx)),
		)

	default:
		return "", "", trace.BadParameter("GCP MySQL user type %q not supported")
	}
}

func (e *Engine) getGCPIAMUserAndPassword(ctx context.Context, sessionCtx *common.Session) (string, string, error) {
	e.Log.WithField("session", sessionCtx).Debug("Authenticating GCP MySQL with IAM auth.")

	password, err := e.Auth.GetCloudSQLAuthToken(ctx, sessionCtx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	return getInDatabaseUserFromGCPServiceAccount(sessionCtx.DatabaseUser), password, nil
}

func (e *Engine) getGCPPasswordUserAndPassword(ctx context.Context, sessionCtx *common.Session) (string, string, error) {
	e.Log.WithField("session", sessionCtx).Debug("Authenticating GCP MySQL with password auth.")

	// For Cloud SQL MySQL legacy auth, we use one-time passwords by resetting
	// the database user password for each connection. Thus, acquire a lock to
	// make sure all connection attempts to the same database and user are
	// serialized.
	retryCtx, cancel := context.WithTimeout(ctx, defaults.DatabaseConnectTimeout)
	defer cancel()
	lease, err := services.AcquireSemaphoreWithRetry(retryCtx, e.makeAcquireSemaphoreConfig(sessionCtx))
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	// Only release the semaphore after the connection has been established
	// below. If the semaphore fails to release for some reason, it will
	// expire in a minute on its own.
	defer func() {
		err := e.AuthClient.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			e.Log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()
	password, err := e.Auth.GetCloudSQLPassword(ctx, sessionCtx)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	return sessionCtx.DatabaseUser, password, nil
}
