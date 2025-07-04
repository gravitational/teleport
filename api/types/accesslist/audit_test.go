/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package accesslist

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestAudit_setInitialReviewDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		frequency         ReviewFrequency
		dayOfMonth        ReviewDayOfMonth
		currentReviewDate time.Time
		expected          time.Time
	}{
		{
			name:              "one month, first day",
			frequency:         OneMonth,
			dayOfMonth:        FirstDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "one month, fifteenth day",
			frequency:         OneMonth,
			dayOfMonth:        FifteenthDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "one month, last day",
			frequency:         OneMonth,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 2, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:              "six months, last day",
			frequency:         SixMonths,
			dayOfMonth:        LastDayOfMonth,
			currentReviewDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected:          time.Date(2023, 7, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			audit := Audit{
				Recurrence: Recurrence{
					Frequency:  test.frequency,
					DayOfMonth: test.dayOfMonth,
				},
			}
			audit.setInitialAuditDate(clockwork.NewFakeClockAt(test.currentReviewDate))
			require.Equal(t, test.expected, audit.NextAuditDate)
		})
	}
}
