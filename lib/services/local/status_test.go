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

package local

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestClusterAlerts verifies basic expected behavior of cluster alert creation, querying, and expiry.
func TestClusterAlerts(t *testing.T) {
	const alertCount = 20
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := clockwork.NewFakeClock()

	backend, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	defer backend.Close()

	status := NewStatusService(backend)

	for i := 0; i < alertCount; i++ {
		// use a label to create two "groups" of alerts
		group := "odd"
		if i%2 == 0 {
			group = "even"
		}

		// create a small portion of high severity alerts
		sev := types.AlertSeverity_MEDIUM
		if i%5 == 0 {
			sev = types.AlertSeverity_HIGH
		}

		// create an alert
		alert, err := types.NewClusterAlert(
			fmt.Sprintf("alert-%d", i),
			"some message",
			types.WithAlertSeverity(sev),
			types.WithAlertLabel("num", fmt.Sprintf("%d", i)),
			types.WithAlertLabel("grp", group),
		)
		require.NoError(t, err)

		err = status.UpsertClusterAlert(ctx, alert)
		require.NoError(t, err)
	}

	// load a single alert by ID
	alerts, err := status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: "alert-2",
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	// load a single alert by label
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels: map[string]string{
			"num": "3",
		},
	})
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	// load half the alerts by label
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels: map[string]string{
			"grp": "odd",
		},
	})
	require.NoError(t, err)
	require.Len(t, alerts, alertCount/2)

	// load only alerts w/ high severity
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Severity: types.AlertSeverity_HIGH,
	})
	require.NoError(t, err)
	require.Len(t, alerts, alertCount/5)

	// query with no matching labels returns no alerts
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		Labels: map[string]string{
			"bad-key": "bad-val",
		},
	})
	require.NoError(t, err)
	require.Empty(t, alerts)

	// empty query returns all alerts
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, alertCount)

	// alerts without a specified expiry time expire within 24 hours
	clock.Advance(time.Hour * 24)
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Empty(t, alerts)
}
