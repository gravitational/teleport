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
	"time"

	"github.com/gravitational/trace"
)

// ScheduleDict specifies a dictionary of schedules.
//
// Implements [typical.Getter]
type ScheduleDict map[string]Schedule

// Get returns the schedule with the specified key name.
func (d ScheduleDict) Get(key string) (Schedule, error) {
	return d[key], nil
}

// Schedule specifies an access monitoring rule schedule.
type Schedule struct {
	// Timezone specifies the timzone used for the schedule.
	Timezone string
	// Shifts contains a set of shifts that make up the schedule.
	Shifts []Shift
}

// Shift specifies a weekday and a time interval.
type Shift struct {
	// Weekday specifies a weekday value. (Sunday, Monday, ...)
	Weekday string
	// Start specifies the start of the shift. Formatted as a HH:MM.
	Start string
	// End specifies the end of the shift. Formatted as HH:MM.
	End string
}

// ClockTime returns a new time value overriding the hour and minute.
func ClockTime(timestamp time.Time, hourMinute string) (time.Time, error) {
	const hourMinuteFormat = "15:04" // 24-hour HH:MM format

	parsed, err := time.ParseInLocation(hourMinuteFormat, hourMinute, timestamp.Location())
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}

	return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(),
		parsed.Hour(), parsed.Minute(), 0, 0, timestamp.Location()), nil
}

// InSchedule returns true if the timestamp is within the schedule.
func InSchedule(timestamp time.Time, schedule Schedule) (bool, error) {
	if len(schedule.Shifts) == 0 {
		return false, nil
	}

	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return false, trace.Wrap(err)
	}
	timestamp = timestamp.In(loc)

	weekday := timestamp.Weekday().String()
	for _, shift := range schedule.Shifts {
		if weekday != shift.Weekday {
			continue
		}

		startTime, err := ClockTime(timestamp, shift.Start)
		if err != nil {
			return false, trace.Wrap(err, "invalid start time: %q", shift.Start)
		}

		endTime, err := ClockTime(timestamp, shift.End)
		if err != nil {
			return false, trace.Wrap(err, "invalid end time: %q", shift.End)
		}

		if !timestamp.Before(startTime) && !timestamp.After(endTime) {
			return true, nil
		}
	}
	return false, nil
}
