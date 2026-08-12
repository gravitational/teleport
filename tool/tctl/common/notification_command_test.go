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
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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
}

// TestOutputCreatedNotification covers the structured output of `tctl
// notifications create`. The format rendering is exercised directly with a
// synthetic notification rather than through a full auth server, mirroring the
// other mutation-command structured-output unit tests.
func TestOutputCreatedNotification(t *testing.T) {
	t.Parallel()

	created := notificationspb.Notification_builder{
		Kind: types.KindNotification,
		Metadata: headerv1.Metadata_builder{
			Name: "notif-123",
			Labels: map[string]string{
				types.NotificationTitleLabel: "json notification",
			},
		}.Build(),
		Spec: notificationspb.NotificationSpec_builder{
			Username: "auditor-user",
		}.Build(),
	}.Build()

	t.Run("text", func(t *testing.T) {
		var buf bytes.Buffer
		n := &NotificationCommand{stdout: &buf, format: teleport.Text}
		require.NoError(t, n.outputCreatedNotification(created, "Created notification notif-123 for user auditor-user\n"))
		require.Equal(t, "Created notification notif-123 for user auditor-user\n", buf.String())
	})

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		n := &NotificationCommand{stdout: &buf, format: teleport.JSON}
		require.NoError(t, n.outputCreatedNotification(created, "ignored"))
		require.NotContains(t, buf.String(), "Created notification")

		got := mustDecodeJSON[*notificationspb.Notification](t, &buf)
		require.Equal(t, "notif-123", got.GetMetadata().GetName())
		require.Equal(t, "json notification", got.GetMetadata().GetLabels()[types.NotificationTitleLabel])
	})

	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		n := &NotificationCommand{stdout: &buf, format: teleport.YAML}
		require.NoError(t, n.outputCreatedNotification(created, "ignored"))
		require.NotContains(t, buf.String(), "Created notification")

		var got notificationspb.Notification
		require.NoError(t, yaml.Unmarshal(buf.Bytes(), &got))
		require.Equal(t, "json notification", got.GetMetadata().GetLabels()[types.NotificationTitleLabel])
	})

	t.Run("invalid format", func(t *testing.T) {
		n := &NotificationCommand{stdout: &bytes.Buffer{}, format: "bogus"}
		err := n.outputCreatedNotification(created, "ignored")
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	})
}
