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

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"

	"github.com/gravitational/trace"
)

// ScheduleDict specifies a dictionary of schedules.
//
// Implements [typical.Getter]
type ScheduleDict map[string]*accessmonitoringrulesv1.Schedule

// Get returns the schedule with the specified key name.
func (d ScheduleDict) Get(key string) (*accessmonitoringrulesv1.Schedule, error) {
	return d[key], nil
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

// inSchedule returns true if the timestamp is within the schedule.
func inSchedule(timestamp time.Time, schedule *accessmonitoringrulesv1.Schedule) (bool, error) {
	if schedule.GetTime() == nil {
		return false, nil
	}

	if len(schedule.GetTime().GetShifts()) == 0 {
		return false, nil
	}

	weekday := timestamp.Weekday().String()
	for _, shift := range schedule.GetTime().GetShifts() {
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
