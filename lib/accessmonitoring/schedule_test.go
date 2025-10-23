/*
Copyright 2025 Gravitational, Inc.

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

package accessmonitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
)

func TestClockTime(t *testing.T) {
	timestamp := time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC)
	tests := []struct {
		description string
		clockTime   string
		assertErr   require.ErrorAssertionFunc
		assertTime  require.ValueAssertionFunc
	}{
		{
			description: "min clock time",
			clockTime:   "00:00",
			assertErr:   require.NoError,
			assertTime: func(t require.TestingT, ts any, _ ...any) {
				require.Equal(t, time.Date(2025, time.August, 11, 0, 0, 0, 0, time.UTC), ts)
			},
		},
		{
			description: "max clock time",
			clockTime:   "23:59",
			assertErr:   require.NoError,
			assertTime: func(t require.TestingT, ts any, _ ...any) {
				require.Equal(t, time.Date(2025, time.August, 11, 23, 59, 0, 0, time.UTC), ts)
			},
		},
		{
			description: "24 hour out of range",
			clockTime:   "24:00",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "hour out of range")
			},
		},
		{
			description: "60 minute out of range",
			clockTime:   "00:60",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "minute out of range")
			},
		},
		{
			description: "seconds specified",
			clockTime:   "12:34:56",
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "extra text")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()
			ts, err := ClockTime(timestamp, tt.clockTime)
			tt.assertErr(t, err)
			if tt.assertTime != nil {
				tt.assertTime(t, ts)
			}
		})
	}
}

func TestInSchedule(t *testing.T) {
	timestamp := time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC) // Monday 14:30
	tests := []struct {
		description      string
		schedules        map[string]*accessmonitoringrulesv1.Schedule
		assertErr        require.ErrorAssertionFunc
		assertInSchedule require.BoolAssertionFunc
	}{
		{
			description: "in schedule",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:00",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.True,
		},
		{
			description: "schedule does not contain any shifts",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.False,
		},
		{
			description: "invalid timezone",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Timezone: "invalid",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:00",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "unknown time zone")
			},
			assertInSchedule: require.False,
		},
		{
			description: "different timezone",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Timezone: "America/Los_Angeles",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:00",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.False,
		},
		{
			description: "different weekday",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Tuesday.String(),
								Start:   "14:00",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.False,
		},
		{
			description: "before schedule",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:31",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.False,
		},
		{
			description: "exact start time",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:30",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.True,
		},
		{
			description: "multiple schedules",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"schedule1": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Monday.String(),
								Start:   "14:30",
								End:     "15:00",
							},
						},
					},
				},
				"schedule2": {
					Time: &accessmonitoringrulesv1.TimeSchedule{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							{
								Weekday: time.Tuesday.String(),
								Start:   "14:30",
								End:     "15:00",
							},
						},
					},
				},
			},
			assertErr:        require.NoError,
			assertInSchedule: require.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()
			ts, err := InSchedules(tt.schedules, timestamp)
			tt.assertErr(t, err)
			if tt.assertInSchedule != nil {
				tt.assertInSchedule(t, ts)
			}
		})
	}
}
