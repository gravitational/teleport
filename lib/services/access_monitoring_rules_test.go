/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestValidateAccessMonitoringRule(t *testing.T) {
	tests := []struct {
		description string
		modifyAMR   func(amr *accessmonitoringrulesv1.AccessMonitoringRule)
		assertErr   require.ErrorAssertionFunc
	}{
		{
			description: "valid AMR",
			assertErr:   require.NoError,
		},
		{
			description: "notification name required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().GetNotification().SetName("")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "notification plugin name is missing")
			},
		},
		{
			description: "automatic_review integration required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().GetAutomaticReview().SetIntegration("")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "automatic_review integration is missing")
			},
		},
		{
			description: "automatic_review decision required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().GetAutomaticReview().SetDecision("")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "automatic_review decision is missing")
			},
		},
		{
			description: "notification or automatic_review required",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().ClearNotification()
				amr.GetSpec().ClearAutomaticReview()
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "notification or automatic_review must be configured")
			},
		},
		{
			description: "allow automatic_review to be nil",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().ClearAutomaticReview()
			},
			assertErr: require.NoError,
		},
		{
			description: "allow notifications to be nil",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().ClearNotification()
			},
			assertErr: require.NoError,
		},
		{
			description: "invalid automatic_review decision",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().GetAutomaticReview().SetDecision("invalid-decision")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `accessMonitoringRule automatic_review decision "invalid-decision" is not supported`)
			},
		},
		{
			description: "invalid desired_state",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().SetDesiredState("invalid-desired-state")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, `accessMonitoringRule desired_state "invalid-desired-state" is not supported`)
			},
		},
		{
			description: "invalid condition",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().SetCondition("invalid-condition")
			},
			assertErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "accessMonitoringRule condition is invalid")
			},
		},
		{
			description: "allow desired_state to be empty",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().SetDesiredState("")
			},
			assertErr: require.NoError,
		},
		{
			description: "valid time schedule",
			modifyAMR: func(amr *accessmonitoringrulesv1.AccessMonitoringRule) {
				amr.GetSpec().SetSchedules(map[string]*accessmonitoringrulesv1.Schedule{
					"default": accessmonitoringrulesv1.Schedule_builder{
						Time: accessmonitoringrulesv1.TimeSchedule_builder{
							Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
								accessmonitoringrulesv1.TimeSchedule_Shift_builder{
									Weekday: time.Monday.String(),
									Start:   "00:00",
									End:     "23:59",
								}.Build(),
							},
						}.Build(),
					}.Build(),
				})
			},
			assertErr: require.NoError,
		},
	}

	validAMR := accessmonitoringrulesv1.AccessMonitoringRule_builder{
		Kind:     types.KindAccessMonitoringRule,
		Metadata: &headerv1.Metadata{},
		Version:  types.V1,
		Spec: accessmonitoringrulesv1.AccessMonitoringRuleSpec_builder{
			Subjects:     []string{types.KindAccessRequest},
			Condition:    "true",
			DesiredState: types.AccessMonitoringRuleStateReviewed,
			Notification: accessmonitoringrulesv1.Notification_builder{
				Name: "fakePlugin",
			}.Build(),
			AutomaticReview: accessmonitoringrulesv1.AutomaticReview_builder{
				Integration: "fakePlugin",
				Decision:    types.RequestState_APPROVED.String(),
			}.Build(),
		}.Build(),
	}.Build()

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()
			amr, ok := proto.Clone(validAMR).(*accessmonitoringrulesv1.AccessMonitoringRule)
			require.True(t, ok)

			if test.modifyAMR != nil {
				test.modifyAMR(amr)
			}
			test.assertErr(t, ValidateAccessMonitoringRule(amr))
		})
	}
}

func TestValidateSchedules(t *testing.T) {
	tests := []struct {
		description string
		schedules   map[string]*accessmonitoringrulesv1.Schedule
		assertErr   require.ErrorAssertionFunc
	}{
		{
			description: "valid schedules",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "multiple schedules",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"on-call-1": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Saturday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
				"on-call-2": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Sunday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "schedule time not specified",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": {},
			},
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "time is required")
			},
		},
		{
			description: "does not contain any shifts",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: &accessmonitoringrulesv1.TimeSchedule{},
				}.Build(),
			},
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "at least one shift is require")
			},
		},
		{
			description: "valid timezone (UTC)",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Timezone: "UTC",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "valid timezone (America/Los_Angeles)",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Timezone: "America/Los_Angeles",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "valid timezone (Europe/Lisbon)",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Timezone: "Europe/Lisbon",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "valid timezone (Asia/Singapore)",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Timezone: "Asia/Singapore",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: require.NoError,
		},
		{
			description: "invalid timezone",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Timezone: "invalid",
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "00:00",
								End:     "23:59",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "invalid timezone")
			},
		},
		{
			description: "start time is not before end time",
			schedules: map[string]*accessmonitoringrulesv1.Schedule{
				"default": accessmonitoringrulesv1.Schedule_builder{
					Time: accessmonitoringrulesv1.TimeSchedule_builder{
						Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
							accessmonitoringrulesv1.TimeSchedule_Shift_builder{
								Weekday: time.Monday.String(),
								Start:   "23:59",
								End:     "00:00",
							}.Build(),
						},
					}.Build(),
				}.Build(),
			},
			assertErr: func(t require.TestingT, err error, _ ...interface{}) {
				require.ErrorContains(t, err, "start time must be before end time")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()
			test.assertErr(t, validateSchedules(test.schedules))
		})
	}
}
