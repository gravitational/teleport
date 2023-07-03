/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
)

// UserProvisioner handles automatic database user creation.
type UserProvisioner struct {
	// AuthClient is the cluster auth server client.
	AuthClient *auth.Client
	// Backend is the particular database implementation.
	Backend AutoUsers
	// Log is the logger.
	Log logrus.FieldLogger
	// Clock is the clock to use.
	Clock clockwork.Clock
}

// Activate creates or enables a database user.
//
// Returns a cleanup function that the caller must call once the connection to
// database has been established to release the cluster lock acquired by this
// function to make sure no 2 processes run user activation simultaneously.
func (a *UserProvisioner) Activate(ctx context.Context, sessionCtx *Session) (func(), error) {
	if !sessionCtx.AutoCreateUser {
		return func() {}, nil
	}

	if !sessionCtx.Database.SupportsAutoUsers() {
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning " +
				"but this database doesn't support it, contact your Teleport " +
				"administrator")
	}

	if sessionCtx.Database.GetAdminUser() == "" {
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning " +
				"but this database doesn't have admin user configured, contact " +
				"your Teleport administrator")
	}

	retryCtx, cancel := context.WithTimeout(ctx, defaults.DatabaseConnectTimeout)
	defer cancel()

	lease, err := services.AcquireSemaphoreWithRetry(retryCtx, a.makeAcquireSemaphoreConfig(sessionCtx))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	release := func() {
		err := a.AuthClient.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			a.Log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}

	err = a.Backend.ActivateUser(ctx, sessionCtx)
	if err != nil {
		release()
		return nil, trace.BadParameter(
			"your Teleport role requires automatic database user provisioning "+
				"but an attempt to activate database user %q failed due to the "+
				"following error: %v", sessionCtx.DatabaseUser, err)
	}

	return release, nil
}

// Deactivate disables a database user.
func (a *UserProvisioner) Deactivate(ctx context.Context, sessionCtx *Session) error {
	if !sessionCtx.AutoCreateUser {
		return nil
	}

	retryCtx, cancel := context.WithTimeout(ctx, defaults.DatabaseConnectTimeout)
	defer cancel()

	lease, err := services.AcquireSemaphoreWithRetry(retryCtx, a.makeAcquireSemaphoreConfig(sessionCtx))
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		err := a.AuthClient.CancelSemaphoreLease(ctx, *lease)
		if err != nil {
			a.Log.WithError(err).Errorf("Failed to cancel lease: %v.", lease)
		}
	}()

	err = a.Backend.DeactivateUser(ctx, sessionCtx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (a *UserProvisioner) makeAcquireSemaphoreConfig(sessionCtx *Session) services.AcquireSemaphoreWithRetryConfig {
	return services.AcquireSemaphoreWithRetryConfig{
		Service: a.AuthClient,
		// The semaphore will serialize connections to the database as specific
		// user. If we fail to release the lock for some reason, it will expire
		// in a minute anyway.
		Request: types.AcquireSemaphoreRequest{
			SemaphoreKind: "db-auto-users",
			SemaphoreName: fmt.Sprintf("%v-%v", sessionCtx.Database.GetName(), sessionCtx.DatabaseUser),
			MaxLeases:     1,
			Expires:       a.Clock.Now().Add(time.Minute),
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
