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

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	notificationspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
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

	clt, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clt.Close() })

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
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// List notifications for auditor and verify that auditor notification exists.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", auditorUsername})
		require.NoError(t, err)
		require.Contains(t, buf.String(), "auditor notification")
		require.NotContains(t, buf.String(), "manager notification")

		// List notifications for manager and verify output.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", managerUsername})
		require.NoError(t, err)
		require.Contains(t, buf.String(), "manager notification")
		require.NotContains(t, buf.String(), "auditor notification")

		// List global notifications and verify that test-1 notification exists.
		buf, err = runNotificationsCommand(t, clt, []string{"ls"})
		require.NoError(t, err)
		require.Contains(t, buf.String(), "test-1 notification")
		require.NotContains(t, buf.String(), "auditor notification")
		require.NotContains(t, buf.String(), "manager notification")

		// Filter out notifications with a non-existent label and make sure nothing comes back.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--labels=thislabel=doesnotexist"})
		require.NotContains(t, buf.String(), "test-1 notification")
		require.NotContains(t, buf.String(), "auditor notification")
		require.NotContains(t, buf.String(), "manager notification")

		// Filter out global notifications with a valid label.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--labels=forrole=test-1"})
		require.Contains(t, buf.String(), "test-1 notification")
		require.NotContains(t, buf.String(), "auditor notification")
		require.NotContains(t, buf.String(), "manager notification")

	}, 3*time.Second, 100*time.Millisecond)

	// Delete the auditor's user-specific notification.
	_, err = runNotificationsCommand(t, clt, []string{"rm", auditorUserNotificationId, "--user", auditorUsername})
	require.NoError(t, err)
	// Delete the global notification.
	_, err = runNotificationsCommand(t, clt, []string{"rm", globalNotificationId})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		// Verify that the global notification is no longer listed.
		buf, err = runNotificationsCommand(t, clt, []string{"ls"})
		require.NoError(t, err)
		require.NotContains(t, buf.String(), "test-1 notification")

		// Verify that the auditor notification is no longer listed.
		buf, err = runNotificationsCommand(t, clt, []string{"ls", "--user", auditorUsername})
		require.NoError(t, err)
		require.NotContains(t, buf.String(), "auditor notification")
	}, 3*time.Second, 100*time.Millisecond)

	// Structured output: JSON/YAML must serialize the created notification
	// object instead of the human-readable "Created notification" prose. We
	// decode the user-targeted notification (which has a flat, oneof-free shape)
	// for both formats.
	t.Run("create json returns notification", func(t *testing.T) {
		buf, err := runNotificationsCommand(t, clt, []string{
			"create", "--user", auditorUsername,
			"--title", "json notification", "--content", "structured output",
			"--format", "json",
		})
		require.NoError(t, err)
		require.NotContains(t, buf.String(), "Created notification")

		got := mustDecodeJSON[*notificationspb.Notification](t, buf)
		require.NotEmpty(t, got.GetMetadata().GetName())
		require.Equal(t, "json notification", got.GetMetadata().GetLabels()[types.NotificationTitleLabel])
	})

	t.Run("create yaml returns notification", func(t *testing.T) {
		buf, err := runNotificationsCommand(t, clt, []string{
			"create", "--user", managerUsername,
			"--title", "yaml notification", "--content", "structured output",
			"--format", "yaml",
		})
		require.NoError(t, err)
		require.NotContains(t, buf.String(), "Created notification")

		var got notificationspb.Notification
		require.NoError(t, yaml.Unmarshal(buf.Bytes(), &got))
		require.Equal(t, "yaml notification", got.GetMetadata().GetLabels()[types.NotificationTitleLabel])
	})

	// A role-targeted create returns the created global notification. Its
	// protobuf oneof matcher does not round-trip through encoding/json, so we
	// assert the output is structured (not prose) and carries the title.
	t.Run("create json returns global notification", func(t *testing.T) {
		buf, err := runNotificationsCommand(t, clt, []string{
			"create", "--roles", "test-1",
			"--title", "global json notification", "--content", "structured output",
			"--format", "json",
		})
		require.NoError(t, err)
		require.NotContains(t, buf.String(), "Created notification")
		require.True(t, strings.HasPrefix(strings.TrimSpace(buf.String()), "{"), "expected JSON object output")
		require.Contains(t, buf.String(), "global json notification")
	})
}
