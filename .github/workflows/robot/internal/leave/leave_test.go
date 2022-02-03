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

package leave

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShouldOmitRangeOverNonBusinessDays(t *testing.T) {
	// Leave is 3 days long over a weekend.
	req := employeeLeaveRequest{
		StartDate: Time{time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC)},
		EndDate:   Time{time.Date(2021, time.Month(1), 5, 0, 0, 0, 0, time.UTC)},
	}

	tests := []struct {
		currentDay time.Time
		expected   bool
		desc       string
	}{
		{
			currentDay: time.Date(2020, time.Month(12), 29, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "three-business-days-before-leave--don't-omit",
		},
		{
			currentDay: time.Date(2020, time.Month(12), 30, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "two-business-days-before-leave--omit",
		},
		{
			currentDay: time.Date(2020, time.Month(12), 31, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "one-business-day before-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "first-day-of-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 4, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "during-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 5, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "last-day-of-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 6, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "one-business-day-after-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 7, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "two-business-days-after-leave--don't-omit",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			res := req.shouldOmitEmployee(test.currentDay)
			require.Equal(t, test.expected, res)

		})
	}
}

func TestShouldOmitRangeOverOnlyBusinessDays(t *testing.T) {
	// Leave is a business week.
	req := employeeLeaveRequest{
		StartDate: Time{time.Date(2021, time.Month(1), 18, 0, 0, 0, 0, time.UTC)},
		EndDate:   Time{time.Date(2021, time.Month(1), 22, 0, 0, 0, 0, time.UTC)},
	}

	tests := []struct {
		currentDay time.Time
		expected   bool
		desc       string
	}{
		{
			currentDay: time.Date(2021, time.Month(1), 13, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "three-business-days-before-leave--don't-omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 14, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "two-business-days-before-leave--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 15, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "one-business-day-before-leave-before-weekend--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 25, 0, 0, 0, 0, time.UTC),
			expected:   true,
			desc:       "one-business-day-after-leave-after-weekend--omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 26, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "two-business-days-after-leave--don't-omit",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			res := req.shouldOmitEmployee(test.currentDay)
			require.Equal(t, test.expected, res)
		})
	}
}

func TestShouldOmitTwoDaysOfLeave(t *testing.T) {
	// Leave is not more than two business days.
	req := employeeLeaveRequest{
		StartDate: Time{time.Date(2021, time.Month(1), 18, 0, 0, 0, 0, time.UTC)},
		EndDate:   Time{time.Date(2021, time.Month(1), 19, 0, 0, 0, 0, time.UTC)},
	}

	tests := []struct {
		currentDay time.Time
		expected   bool
		desc       string
	}{
		{
			currentDay: time.Date(2021, time.Month(1), 15, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "one-business-day-before-two-day-leave--don't-omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 18, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "first-day-of-two-day-leave--don't-omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 19, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "last-day-of-two-day-leave--don't-omit",
		},
		{
			currentDay: time.Date(2021, time.Month(1), 20, 0, 0, 0, 0, time.UTC),
			expected:   false,
			desc:       "day-after-two-day-leave--don't-omit",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			res := req.shouldOmitEmployee(test.currentDay)
			require.Equal(t, test.expected, res)

		})
	}
}

func TestBusinessDaysCount(t *testing.T) {
	tests := []struct {
		leave    employeeLeaveRequest
		expected int
		desc     string
	}{
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(1), 5, 0, 0, 0, 0, time.UTC)},
			},
			expected: 3,
			desc:     "5-day-range-over-a-weekend",
		},
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 15, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(1), 28, 0, 0, 0, 0, time.UTC)},
			},
			expected: 10,
			desc:     "14-day-range-over-two-weekends",
		},
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC)},
			},
			expected: 1,
			desc:     "1-day-range",
		},
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 1, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(1), 4, 0, 0, 0, 0, time.UTC)},
			},
			expected: 2,
			desc:     "4-day-range-over-a-weekend",
		},
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 18, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(1), 22, 0, 0, 0, 0, time.UTC)},
			},
			expected: 5,
			desc:     "5-day-range",
		},
		{
			leave: employeeLeaveRequest{
				StartDate: Time{time.Date(2021, time.Month(1), 27, 0, 0, 0, 0, time.UTC)},
				EndDate:   Time{time.Date(2021, time.Month(2), 17, 0, 0, 0, 0, time.UTC)},
			},
			expected: 16,
			desc:     "22-day-range-over-three-weekends",
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := test.leave.businessDayCount()
			require.Equal(t, test.expected, result)
		})
	}
}

func TestShouldOmit(t *testing.T) {
	employeeUsernames := map[string]string{"username-a": "1", "username-b": "2"}
	omit := map[string]bool{
		"1": true,
	}
	l := leave{usernames: employeeUsernames, onLeave: omit}

	result := l.ShouldOmit("username-a")
	require.True(t, result)

	result = l.ShouldOmit("username-b")
	require.False(t, result)
}
