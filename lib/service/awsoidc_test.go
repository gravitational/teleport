package service

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
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
