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
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Audit describes the audit configuration for an access list.
type Audit struct {
	// NextAuditDate is the date that the next audit should be performed.
	NextAuditDate time.Time `json:"next_audit_date" yaml:"next_audit_date"`

	// Recurrence is the recurrence definition for auditing. Valid values are
	// 1, first, 15, and last.
	Recurrence Recurrence `json:"recurrence" yaml:"recurrence"`

	// Notifications is the configuration for notifying users.
	Notifications Notifications `json:"notifications" yaml:"notifications"`
}

// Validate performs client-side resource validation which should be performed before sending
// create/update request.
func (a *Audit) SetDefaults() error {
	if a.Recurrence.Frequency == 0 {
		a.Recurrence.Frequency = SixMonths
	}

	if a.Recurrence.DayOfMonth == 0 {
		a.Recurrence.DayOfMonth = FirstDayOfMonth
	}

	if a.NextAuditDate.IsZero() {
		a.setInitialAuditDate(clockwork.NewRealClock())
	}

	if a.Notifications.Start == 0 {
		a.Notifications.Start = twoWeeks
	}

	return nil
}

// setInitialAuditDate sets the NextAuditDate for a newly created AccessList.
// The function is extracted from CheckAndSetDefaults for the sake of testing
// (we need to pass a fake clock).
func (a *Audit) setInitialAuditDate(clock clockwork.Clock) {
	// We act as if the AccessList just got reviewed (we just created it, so
	// we're pretty sure of what it does) and pick the next review date.
	a.NextAuditDate = clock.Now()
	a.NextAuditDate = selectNextReviewDate(*a)
}

// Validate performs client-side resource validation which should be performed before sending
// create/update request.
func (a *Audit) Validate() error {
	switch a.Recurrence.Frequency {
	case OneMonth, ThreeMonths, SixMonths, OneYear:
	default:
		return trace.BadParameter("recurrence frequency is an invalid value")
	}

	switch a.Recurrence.DayOfMonth {
	case FirstDayOfMonth, FifteenthDayOfMonth, LastDayOfMonth:
	default:
		return trace.BadParameter("recurrence day of month is an invalid value")
	}

	return nil
}

// selectNextReviewDate will select the next review date for the access list.
func selectNextReviewDate(a Audit) time.Time {
	numMonths := int(a.Recurrence.Frequency)
	dayOfMonth := int(a.Recurrence.DayOfMonth)

	// If the last day of the month has been specified, use the 0 day of the
	// next month, which will result in the last day of the target month.
	if dayOfMonth == int(LastDayOfMonth) {
		numMonths += 1
		dayOfMonth = 0
	}

	currentReviewDate := a.NextAuditDate
	nextDate := time.Date(currentReviewDate.Year(), currentReviewDate.Month()+time.Month(numMonths), dayOfMonth,
		0, 0, 0, 0, time.UTC)

	return nextDate
}
