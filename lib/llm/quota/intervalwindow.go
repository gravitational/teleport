// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package quota

import (
	"time"
	_ "time/tzdata"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ClockSkewTolerance is how much clock drift is tolerated when checking if a
// time belongs to a window.
const ClockSkewTolerance = time.Minute

// IntervalWindow represents a time window for a quota interval.
type IntervalWindow struct {
	starts time.Time
	ends   time.Time
}

// LifetimeIntervalWindow is the single window of lifetime intervals.
var LifetimeIntervalWindow = IntervalWindow{}

// StartsAt returns when the window starts.
func (w IntervalWindow) StartsAt() time.Time {
	return w.starts
}

// EndsAt returns when the window ends.
func (w IntervalWindow) EndsAt() time.Time {
	return w.ends
}

// Contains returns true if the time is within the window, extended on both
// ends by [ClockSkewTolerance]. The window start is inclusive and the end is
// exclusive.
func (w IntervalWindow) Contains(t time.Time) bool {
	if w.unbounded() {
		return true
	}

	return !t.Before(w.starts.Add(-ClockSkewTolerance)) && t.Before(w.ends.Add(ClockSkewTolerance))
}

// Equal returns true if both windows have the same bounds. It is used to match
// a window against the window of a specific interval, since windows of
// different intervals can contain the same time.
//
// Lifetime intervals have a single unbounded window, which is only equal to
// other unbounded windows.
func (w IntervalWindow) Equal(other IntervalWindow) bool {
	return w.starts.Equal(other.starts) && w.ends.Equal(other.ends)
}

// unbounded returns true if the window has no start and end.
func (w IntervalWindow) unbounded() bool {
	return w.starts.IsZero() && w.ends.IsZero()
}

// NewIntervalWindow creates a new [IntervalWindow].
func NewIntervalWindow(startsAt time.Time, endsAt time.Time) IntervalWindow {
	return IntervalWindow{startsAt, endsAt}
}

// NewIntervalWindowFromTime returns the [IntervalWindow] that contains the
// provided time.
func NewIntervalWindowFromTime(interval *types.LLM_Quota_Interval, t time.Time) (IntervalWindow, error) {
	var (
		tz = time.UTC
		// Defaults to reset at midnight in UTC ([time.Time] zero value).
		// Other time fields like year and day are not relevant for this.
		resetTime = time.Time{}
	)

	if interval.ResetOptions != nil {
		var err error
		if interval.ResetOptions.Timezone != "" {
			// TODO(gabrielcorado) load the timezone only once.
			tz, err = time.LoadLocation(interval.ResetOptions.Timezone)
			if err != nil {
				return IntervalWindow{}, trace.Wrap(err)
			}
		}
		if interval.ResetOptions.Time != "" {
			resetTime, err = time.Parse(types.AppLLMQuotaResetTimeFormat, interval.ResetOptions.Time)
			if err != nil {
				return IntervalWindow{}, trace.Wrap(err)
			}
		}
	}

	local := t.In(tz)
	year, month, day := local.Date()

	r := reset{location: tz, hour: resetTime.Hour(), minute: resetTime.Minute()}
	var anchor func(n int) time.Time
	switch interval.Interval {
	case types.AppLLMQuotaIntervalLifetime:
		return LifetimeIntervalWindow, nil
	case types.AppLLMQuotaIntervalYearly:
		anchor = func(n int) time.Time { return r.newTime(year+n, time.January, 1) }
	case types.AppLLMQuotaIntervalMonthly:
		anchor = func(n int) time.Time { return r.newTime(year, month+time.Month(n), 1) }
	case types.AppLLMQuotaIntervalWeekly:
		sunday := day - int(local.Weekday())
		anchor = func(n int) time.Time { return r.newTime(year, month, sunday+7*n) }
	case types.AppLLMQuotaIntervalDaily:
		anchor = func(n int) time.Time { return r.newTime(year, month, day+n) }
	default:
		return IntervalWindow{}, trace.BadParameter("invalid interval %q", interval.Interval)
	}

	// A local time of day that a UTC offset change skips normalizes backward,
	// and a date a timezone "deletes" falls into is the neighbor. So we must
	// take the closest anchor around `t` instead of assuming
	// `anchor(-1) < anchor(0) < anchor(1)`.
	//
	// Three sides are enough because the anchors are at least 23 hours (daily
	// intervals) apart and the largest UTC offset change is 24h. This means the
	// third pass won't cross `t`.
	const passes = 3
	var starts, ends time.Time
	for n := -passes; n <= passes; n++ {
		anchored := anchor(n)
		if !anchored.After(t) && (starts.IsZero() || anchored.After(starts)) {
			starts = anchored
		}
		if anchored.After(t) && (ends.IsZero() || anchored.Before(ends)) {
			ends = anchored
		}
	}
	if starts.IsZero() || ends.IsZero() {
		// This is a rare situation where we won't be able to fit the instant
		// into a window. Anyway let the callers handle it.
		return IntervalWindow{}, trace.BadParameter("no %s window contains %s", interval.Interval, t)
	}

	return IntervalWindow{starts: starts, ends: ends}, nil
}

// reset is a helper structure used to construct [time.Time] using quota
// interval reset options.
type reset struct {
	location *time.Location
	hour     int
	minute   int
}

func (r reset) newTime(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, r.hour, r.minute, 0, 0, r.location).UTC()
}
