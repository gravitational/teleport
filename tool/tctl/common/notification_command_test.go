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

package common

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestNotificationCommandCRUD tests creating, listing, and deleting notifications via the `tctl notifications` commands.
func TestNotificationCommmandCRUD(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fileConfig := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	process := makeAndRunTestAuthServer(t, withFileConfig(fileConfig), withFileDescriptors(dynAddr.Descriptors))

	clt := testenv.MakeDefaultAuthClient(t, process)

	auditorUsername := "auditor-user"
	managerUsername := "manager-user"

	// Test creating a user-specific notification for auditor user.
	buf, err := runNotificationsCommand(t, clt, []string{"create", "--user", auditorUsername, "--title", "auditor notification", "--content", "This is a test notification."})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "for user auditor-user")
	auditorUserNotificationId := strings.Split(buf.String(), " ")[2]

	// Test creating a user-specific notification for manager user.
	buf, err = runNotificationsCommand(t, clt, []string{"create", "--user", managerUsername, "--title", "manager notification", "--content", "This is a test notification."})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "for user manager-user")

	// Test creating a global notification for users with the test-1 role.
	buf, err = runNotificationsCommand(t, clt, []string{
		"create", "--roles", "test-1", "--title", "test-1 notification",
		"--labels", "forrole=test-1",
		"--content", "This is a test notification.",
	})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "for users with one or more of the following roles: [test-1]")
	globalNotificationId := strings.Split(buf.String(), " ")[2]

	// We periodically check with a timeout since it can take some time for the item to be replicated in the cache and be available for listing.
	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// List notifications for auditor and verify that auditor notification exists.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", auditorUsername})
		assert.NoError(collectT, err)
		assert.Contains(collectT, buf.String(), "auditor notification")
		assert.NotContains(collectT, buf.String(), "manager notification")

		// List notifications for manager and verify output.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", managerUsername})
		assert.NoError(collectT, err)
		assert.Contains(collectT, buf.String(), "manager notification")
		assert.NotContains(collectT, buf.String(), "auditor notification")

		// List global notifications and verify that test-1 notification exists.
		buf, err = runNotificationsCommand(t, clt, []string{"ls"})
		assert.NoError(collectT, err)
		assert.Contains(collectT, buf.String(), "test-1 notification")
		assert.NotContains(collectT, buf.String(), "auditor notification")
		assert.NotContains(collectT, buf.String(), "manager notification")

		// Filter out notifications with a non-existent label and make sure nothing comes back.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--labels=thislabel=doesnotexist"})
		assert.NotContains(collectT, buf.String(), "test-1 notification")
		assert.NotContains(collectT, buf.String(), "auditor notification")
		assert.NotContains(collectT, buf.String(), "manager notification")

		// Filter out global notifications with a valid label.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--labels=forrole=test-1"})
		assert.Contains(collectT, buf.String(), "test-1 notification")
		assert.NotContains(collectT, buf.String(), "auditor notification")
		assert.NotContains(collectT, buf.String(), "manager notification")

	}, 3*time.Second, 100*time.Millisecond)

	// Delete the auditor's user-specific notification.
	_, err = runNotificationsCommand(t, clt, []string{"rm", auditorUserNotificationId, "--user", auditorUsername})
	require.NoError(t, err)
	// Delete the global notification.
	_, err = runNotificationsCommand(t, clt, []string{"rm", globalNotificationId})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(collectT *assert.CollectT) {
		// Verify that the global notification is no longer listed.
		buf, err = runNotificationsCommand(t, clt, []string{"ls"})
		assert.NoError(collectT, err)
		assert.NotContains(collectT, buf.String(), "test-1 notification")

		// Verify that the auditor notification is no longer listed.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", auditorUsername})
		assert.NoError(collectT, err)
		assert.NotContains(collectT, buf.String(), "auditor notification")
	}, 3*time.Second, 100*time.Millisecond)
}
