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

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAgentUpgradeWindow(t *testing.T) {
	newTime := func(day int, hour int) time.Time {
		return time.Date(
			2000,
			time.January,
			day,
			hour,
			0, // min
			0, // sec
			0, // nsec
			time.UTC,
		)
	}

	from := newTime(1, 12)

	require.Equal(t, time.Saturday, from.Weekday()) // verify that newTime starts from expected pos

	conf := AgentUpgradeWindow{
		UTCStartHour: 2,
	}

	tts := []struct{ start, stop time.Time }{
		{newTime(1, 2), newTime(1, 3)},
		{newTime(2, 2), newTime(2, 3)},
		{newTime(3, 2), newTime(3, 3)},
		{newTime(4, 2), newTime(4, 3)},
		{newTime(5, 2), newTime(5, 3)},
		{newTime(6, 2), newTime(6, 3)},
		{newTime(7, 2), newTime(7, 3)},
		{newTime(8, 2), newTime(8, 3)},
		{newTime(9, 2), newTime(9, 3)},
	}

	gen := conf.generator(from)

	for _, tt := range tts {
		start, stop := gen()
		require.Equal(t, tt.start, start)
		require.Equal(t, tt.stop, stop)
	}

	// set weekdays fileter s.t. windows limited to m-f.
	conf.Weekdays = []string{
		"Monday",
		"tue",
		"Wed",
		"thursday",
		"Friday",
	}

	tts = []struct{ start, stop time.Time }{
		// sat {newTime(1, 2), newTime(1, 3)},
		// sun {newTime(2, 2), newTime(2, 3)},
		{newTime(3, 2), newTime(3, 3)},
		{newTime(4, 2), newTime(4, 3)},
		{newTime(5, 2), newTime(5, 3)},
		{newTime(6, 2), newTime(6, 3)},
		{newTime(7, 2), newTime(7, 3)},
		// sat {newTime(8, 2), newTime(8, 3)},
		// sun {newTime(9, 2), newTime(9, 3)},
	}

	gen = conf.generator(from)

	for _, tt := range tts {
		start, stop := gen()
		require.Equal(t, tt.start, start)
		require.Equal(t, tt.stop, stop)
	}

	// verify that invalid weekdays are omitted from filter.
	conf.Weekdays = []string{
		"Monday",
		"tues", // invalid
		"Wed",
		"Th", // invalid
		"Friday",
	}

	tts = []struct{ start, stop time.Time }{
		// sat {newTime(1, 2), newTime(1, 3)},
		// sun {newTime(2, 2), newTime(2, 3)},
		{newTime(3, 2), newTime(3, 3)},
		// tue {newTime(4, 2), newTime(4, 3)},
		{newTime(5, 2), newTime(5, 3)},
		// thu {newTime(6, 2), newTime(6, 3)},
		{newTime(7, 2), newTime(7, 3)},
		// sat {newTime(8, 2), newTime(8, 3)},
		// sun {newTime(9, 2), newTime(9, 3)},
	}

	gen = conf.generator(from)

	for _, tt := range tts {
		start, stop := gen()
		require.Equal(t, tt.start, start)
		require.Equal(t, tt.stop, stop)
	}

	// if all weekdays are invalid, revert to firing every day
	conf.Weekdays = []string{
		"Mo",
		"Tu",
		"We",
		"Th",
		"Fr",
	}

	tts = []struct{ start, stop time.Time }{
		{newTime(1, 2), newTime(1, 3)},
		{newTime(2, 2), newTime(2, 3)},
		{newTime(3, 2), newTime(3, 3)},
		{newTime(4, 2), newTime(4, 3)},
		{newTime(5, 2), newTime(5, 3)},
		{newTime(6, 2), newTime(6, 3)},
		{newTime(7, 2), newTime(7, 3)},
		{newTime(8, 2), newTime(8, 3)},
		{newTime(9, 2), newTime(9, 3)},
	}

	gen = conf.generator(from)

	for _, tt := range tts {
		start, stop := gen()
		require.Equal(t, tt.start, start)
		require.Equal(t, tt.stop, stop)
	}
}

// verify that the default (empty) maintenance window value is valid.
func TestClusterMaintenanceConfigDefault(t *testing.T) {
	t.Parallel()

	mw := NewClusterMaintenanceConfig()

	require.NoError(t, mw.CheckAndSetDefaults())
}

func TestWeekdayParser(t *testing.T) {
	t.Parallel()

	tts := []struct {
		input  string
		expect time.Weekday
		fail   bool
	}{
		{
			input:  "Tue",
			expect: time.Tuesday,
		},
		{
			input:  "tue",
			expect: time.Tuesday,
		},
		{
			input: "tues",
			fail:  true, // only 3-letter shorthand is accepted
		},
		{
			input:  "Saturday",
			expect: time.Saturday,
		},
		{
			input:  "saturday",
			expect: time.Saturday,
		},
		{
			input:  "sun",
			expect: time.Sunday,
		},
		{
			input: "sundae", // containing a valid prefix is insufficient
			fail:  true,
		},
		{
			input: "",
			fail:  true,
		},
	}

	for _, tt := range tts {
		day, ok := ParseWeekday(tt.input)
		if tt.fail {
			require.False(t, ok)
			continue
		}

		require.Equal(t, tt.expect, day)
	}
}

func TestWithinUpgradeWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc          string
		upgradeWindow AgentUpgradeWindow
		date          string
		withinWindow  bool
	}{
		{
			desc: "within upgrade window",
			upgradeWindow: AgentUpgradeWindow{
				UTCStartHour: 8,
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: true,
		},
		{
			desc: "not within upgrade window",
			upgradeWindow: AgentUpgradeWindow{
				UTCStartHour: 8,
			},
			date:         "Mon, 02 Jan 2006 09:04:05 UTC",
			withinWindow: false,
		},
		{
			desc: "within upgrade window weekday",
			upgradeWindow: AgentUpgradeWindow{
				UTCStartHour: 8,
				Weekdays:     []string{"Monday"},
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: true,
		},
		{
			desc: "not within upgrade window weekday",
			upgradeWindow: AgentUpgradeWindow{
				UTCStartHour: 8,
				Weekdays:     []string{"Tuesday"},
			},
			date:         "Mon, 02 Jan 2006 08:04:05 UTC",
			withinWindow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cmc := NewClusterMaintenanceConfig()
			cmc.SetAgentUpgradeWindow(tt.upgradeWindow)

			date, err := time.Parse(time.RFC1123, tt.date)
			require.NoError(t, err)
			require.Equal(t, tt.withinWindow, cmc.WithinUpgradeWindow(date))
		})
	}
}

func TestParseWeekdays(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		input       []string
		expect      map[time.Weekday]struct{}
		expectError require.ErrorAssertionFunc
	}{
		{
			name:        "Nil slice",
			input:       nil,
			expect:      nil,
			expectError: require.Error,
		},
		{
			name:        "Empty slice",
			input:       []string{},
			expect:      nil,
			expectError: require.Error,
		},
		{
			name:  "Few valid days",
			input: []string{"Mon", "Tuesday", "WEDNESDAY"},
			expect: map[time.Weekday]struct{}{
				time.Monday:    {},
				time.Tuesday:   {},
				time.Wednesday: {},
			},
			expectError: require.NoError,
		},
		{
			name:  "Every day",
			input: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
			expect: map[time.Weekday]struct{}{
				time.Monday:    {},
				time.Tuesday:   {},
				time.Wednesday: {},
				time.Thursday:  {},
				time.Friday:    {},
				time.Saturday:  {},
				time.Sunday:    {},
			},
			expectError: require.NoError,
		},
		{
			name:  "Wildcard",
			input: []string{"*"},
			expect: map[time.Weekday]struct{}{
				time.Monday:    {},
				time.Tuesday:   {},
				time.Wednesday: {},
				time.Thursday:  {},
				time.Friday:    {},
				time.Saturday:  {},
				time.Sunday:    {},
			},
			expectError: require.NoError,
		},
		{
			name:        "Duplicated day",
			input:       []string{"Mon", "Monday"},
			expect:      nil,
			expectError: require.Error,
		},
		{
			name:        "Invalid days",
			input:       []string{"Mon", "Tuesday", "frurfday"},
			expect:      nil,
			expectError: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseWeekdays(tt.input)
			tt.expectError(t, err)
			require.Equal(t, tt.expect, result)
		})
	}
}
