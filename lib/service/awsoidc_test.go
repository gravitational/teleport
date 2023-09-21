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

package service

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestWithinUpgradeWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc          string
		upgradeWindow types.AgentUpgradeWindow
		date          string
		withinWindow  bool
	}{
		{
			desc: "within upgrade window",
			upgradeWindow: types.AgentUpgradeWindow{
				UTCStartHour: 8,
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: true,
		},
		{
			desc: "not within upgrade window",
			upgradeWindow: types.AgentUpgradeWindow{
				UTCStartHour: 8,
			},
			date:         "Mon, 02 Jan 2006 09:04:05 UTC",
			withinWindow: false,
		},
		{
			desc: "within upgrade window weekday",
			upgradeWindow: types.AgentUpgradeWindow{
				UTCStartHour: 8,
				Weekdays:     []string{"Monday"},
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: true,
		},
		{
			desc: "not within upgrade window weekday",
			upgradeWindow: types.AgentUpgradeWindow{
				UTCStartHour: 8,
				Weekdays:     []string{"Tuesday"},
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cmc := types.NewClusterMaintenanceConfig()
			cmc.SetAgentUpgradeWindow(tt.upgradeWindow)

			date, err := time.Parse(time.RFC1123, tt.date)
			require.NoError(t, err)
			require.Equal(t, tt.withinWindow, withinUpgradeWindow(cmc, clockwork.NewFakeClockAt(date)))
		})
	}
}
