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
	require.Len(t, alerts, 0)

	// empty query returns all alerts
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, alertCount)

	// alerts without a specified expiry time expire within 24 hours
	clock.Advance(time.Hour * 24)
	alerts, err = status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	require.NoError(t, err)
	require.Len(t, alerts, 0)
}
