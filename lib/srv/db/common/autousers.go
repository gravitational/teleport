/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"context"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// UserProvisioner handles automatic database user creation.
type UserProvisioner struct {
	// CloseContext is the database server close context.
	CloseContext context.Context
	// AuthClient is the cluster auth server client.
	AuthClient authclient.ClientI
	// Backend is the particular database implementation.
	Backend AutoUsers
	// Log is the logger.
	Log *slog.Logger
	// Clock is the clock to use.
	Clock clockwork.Clock
}

// Activate creates or enables a database user.
//
// Returns a cleanup function that the caller must call once the connection to
// database has been established to release the cluster lock acquired by this
// function to make sure no 2 processes run user activation simultaneously.
func (a *UserProvisioner) Activate(ctx context.Context, sessionCtx *Session) (func(), error) {
	if !sessionCtx.AutoCreateUserMode.IsEnabled() {
		return func() {}, nil
	}

	if !sessionCtx.Database.SupportsAutoUsers() {
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning " +
				"but this database doesn't support it, contact your Teleport " +
				"administrator")
	}

	if sessionCtx.Database.GetAdminUser().Name == "" {
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning " +
				"but this database doesn't have admin user configured, contact " +
				"your Teleport administrator")
	}

	// Observe.
	defer methodCallMetrics("UserProvisioner:Activate", teleport.ComponentDatabase, sessionCtx.Database)()

	a.Log.DebugContext(ctx, "Activating database user", "user", sessionCtx.DatabaseUser)
	lock, release, err := a.acquireUserLock(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = a.Backend.ActivateUser(lock, sessionCtx)
	if err != nil {
		release()
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning "+
				"but an attempt to activate database user %q failed due to the "+
				"following error: %v", sessionCtx.DatabaseUser, err)
	}

	return release, nil
}

// Teardown chooses and call the auto provisioner method used to cleanup a
// database user.
func (a *UserProvisioner) Teardown(sessionCtx *Session) error {
	// Cleanup must survive session cancellation, but a stalled database must
	// not keep the user lock alive indefinitely.
	ctx, cancel := context.WithTimeout(a.CloseContext, defaults.DatabaseConnectTimeout)
	defer cancel()

	var err error
	switch sessionCtx.AutoCreateUserMode {
	case types.CreateDatabaseUserMode_DB_USER_MODE_KEEP:
		err = a.deactivate(ctx, sessionCtx)
	case types.CreateDatabaseUserMode_DB_USER_MODE_BEST_EFFORT_DROP:
		err = a.delete(ctx, sessionCtx)
	}

	return trace.Wrap(err)
}

// deactivate disables a database user.
func (a *UserProvisioner) deactivate(ctx context.Context, sessionCtx *Session) error {
	defer methodCallMetrics("UserProvisioner:Deactivate", teleport.ComponentDatabase, sessionCtx.Database)()
	a.Log.DebugContext(ctx, "Deactivating database user", "user", sessionCtx.DatabaseUser)

	lock, release, err := a.acquireUserLock(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	return trace.Wrap(a.Backend.DeactivateUser(lock, sessionCtx))
}

// delete deletes a database user.
func (a *UserProvisioner) delete(ctx context.Context, sessionCtx *Session) error {
	// Observe.
	defer methodCallMetrics("UserProvisioner:Delete", teleport.ComponentDatabase, sessionCtx.Database)()
	a.Log.DebugContext(ctx, "Deleting database user", "user", sessionCtx.DatabaseUser)

	lock, release, err := a.acquireUserLock(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer release()

	return trace.Wrap(a.Backend.DeleteUser(lock, sessionCtx))
}

// acquireUserLock bounds acquisition and keeps the lease alive until release.
func (a *UserProvisioner) acquireUserLock(ctx context.Context, sessionCtx *Session) (*services.SemaphoreLock, func(), error) {
	lockCtx, cancel := context.WithCancelCause(ctx)
	timer := time.AfterFunc(defaults.DatabaseConnectTimeout, func() { cancel(context.DeadlineExceeded) })
	// Stop the acquisition timeout after acquiring the lock so long-running
	// operations can continue while their lease is renewed.
	defer timer.Stop()
	lock, err := services.AcquireSemaphoreLockWithRetry(lockCtx, a.makeAcquireSemaphoreConfig(sessionCtx))
	if err != nil {
		if cause := context.Cause(lockCtx); cause != nil {
			err = cause
		}
		cancel(nil)
		return nil, nil, trace.Wrap(err)
	}
	release := func() {
		lock.Stop()
		cancel(nil)
		if err := lock.Wait(); err != nil {
			a.Log.ErrorContext(ctx, "Failed to release database user lock", "error", err)
		}
	}
	return lock, release, nil
}

func (a *UserProvisioner) makeAcquireSemaphoreConfig(sessionCtx *Session) services.SemaphoreLockConfigWithRetry {
	return services.SemaphoreLockConfigWithRetry{
		SemaphoreLockConfig: services.SemaphoreLockConfig{
			Service: a.AuthClient,
			// The semaphore will serialize connections to the database as specific
			// user. If we fail to release the lock for some reason, it will expire
			// in a minute anyway.
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: "db-auto-users",
				// The name of the sempahore is encoded to prevent any invalid characters
				// in a user's name from being rejected by the backend when creating the semaphore.
				SemaphoreName: hex.EncodeToString([]byte(sessionCtx.Database.GetName() + "-" + sessionCtx.DatabaseUser)),
				MaxLeases:     1,
			},
			Expiry: time.Minute,
			Clock:  a.Clock,
		},
		// If multiple connections are being established simultaneously to the
		// same database as the same user, retry for a few seconds.
		Retry: retryutils.LinearConfig{
			Step:  time.Second,
			Max:   time.Second,
			Clock: a.Clock,
		},
	}
}
