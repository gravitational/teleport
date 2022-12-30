/*
Copyright 2022 Gravitational, Inc.

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

package users

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	libsecrets "github.com/gravitational/teleport/lib/srv/db/secrets"
)

func TestBaseUser(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	mockCloudResource := newMockCloudResource()

	secrets, err := libsecrets.NewAWSSecretsManager(libsecrets.AWSSecretsManagerConfig{
		Client: libsecrets.NewMockSecretsManagerClient(libsecrets.MockSecretsManagerClientConfig{
			Clock: clock,
		}),
	})
	require.NoError(t, err)

	user := &baseUser{
		secrets:                     secrets,
		secretKey:                   "local/testuser",
		secretTTL:                   time.Hour,
		databaseUsername:            "testuser",
		maxPasswordLength:           10,
		usePreviousPasswordForLogin: true,
		clock:                       clock,
		cloudResource:               mockCloudResource,
	}

	t.Run("CheckAndSetDefaults", func(t *testing.T) {
		require.NoError(t, user.CheckAndSetDefaults())
		require.Equal(t, "local/testuser", user.GetID())
		require.Equal(t, "local/testuser", fmt.Sprintf("%v", user))
		require.Equal(t, "testuser", user.GetDatabaseUsername())
	})

	t.Run("Setup", func(t *testing.T) {
		require.NoError(t, user.Setup(ctx))
		require.True(t, mockCloudResource.isPasswordModified())
		passwordSet := mockCloudResource.getModifiedPassword()

		// Validate password set for the cloud user is the same one fetched from secrets store.
		password, err := user.GetPassword(ctx)
		require.NoError(t, err)
		require.Equal(t, password, passwordSet)

		// Setup a second time should not fail, and nothing happens.
		require.NoError(t, user.Setup(ctx))
		require.False(t, mockCloudResource.isPasswordModified())
	})

	t.Run("RotatePassword not expired", func(t *testing.T) {
		require.NoError(t, user.RotatePassword(ctx))
		require.False(t, mockCloudResource.isPasswordModified())

		clock.Advance(user.secretTTL / 2)
		require.NoError(t, user.RotatePassword(ctx))
		require.False(t, mockCloudResource.isPasswordModified())
	})

	t.Run("RotatePassword expired", func(t *testing.T) {
		clock.Advance(user.secretTTL * 2)

		require.NoError(t, user.RotatePassword(ctx))
		require.True(t, mockCloudResource.isPasswordModified())
		passwordSet := mockCloudResource.getModifiedPassword()

		// Validate password set for the cloud user is the same one saved in secrets store.
		currentVersion, err := secrets.GetValue(ctx, "local/testuser", libsecrets.CurrentVersion)
		require.NoError(t, err)
		require.Equal(t, currentVersion.Value, passwordSet)

		// Successfully rotated once, now should use previous version for login.
		previousVersion, err := secrets.GetValue(ctx, "local/testuser", libsecrets.PreviousVersion)
		require.NoError(t, err)

		password, err := user.GetPassword(ctx)
		require.NoError(t, err)
		require.Equal(t, previousVersion.Value, password)
	})

	t.Run("RotatePassword secret not found", func(t *testing.T) {
		// Simulate a case that someone else has deleted the secret.
		require.NoError(t, secrets.Delete(ctx, "local/testuser"))

		require.NoError(t, user.RotatePassword(ctx))
		require.True(t, mockCloudResource.isPasswordModified())
		passwordSet := mockCloudResource.getModifiedPassword()

		password, err := user.GetPassword(ctx)
		require.NoError(t, err)
		require.Equal(t, password, passwordSet)
	})

	t.Run("Teardown", func(t *testing.T) {
		require.NoError(t, user.Teardown(ctx))

		_, err := secrets.GetValue(ctx, "local/testuser", libsecrets.CurrentVersion)
		require.True(t, trace.IsNotFound(err))
	})
}

// mockCloudResource is a mock implementation of cloudResource.
type mockCloudResource struct {
	lastPasswordChan chan string
}

func newMockCloudResource() *mockCloudResource {
	return &mockCloudResource{
		lastPasswordChan: make(chan string, 1),
	}
}
func (m *mockCloudResource) ModifyUserPassword(ctx context.Context, oldPassword, newPassword string) error {
	m.lastPasswordChan <- newPassword
	return nil
}
func (m *mockCloudResource) isPasswordModified() bool {
	return len(m.lastPasswordChan) != 0
}
func (m *mockCloudResource) getModifiedPassword() string {
	if m.isPasswordModified() {
		return <-m.lastPasswordChan
	}
	return ""
}
