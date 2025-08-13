/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// UTC converts time to UTC timezone
func UTC(t *time.Time) {
	if t == nil {
		return
	}

	if t.IsZero() {
		// to fix issue with timezones for tests
		*t = time.Time{}
		return
	}
	*t = t.UTC()
}

// HumanTimeFormatString is a human readable date formatting
const HumanTimeFormatString = "Mon Jan _2 15:04 UTC"

// HumanTimeFormat formats time as recognized by humans
func HumanTimeFormat(d time.Time) string {
	return d.Format(HumanTimeFormatString)
}

// TimeFromProto converts a protobuf Timestamp to a Go time.Time, preserving
// the zero value across the conversion boundary (standard go/proto timestamp
// conversion doesn't preserve "zeroness").
func TimeFromProto(t *timestamppb.Timestamp) time.Time {
	// use the zero time to represent the nil timestamp. note that this is conceptually distinct
	// from using t.GetSeconds() == 0 && t.GetNanos() == 0. a timstampb that happens to be created
	// targeting the unix epoch isn't necessarily equivalent to a zero go timestamp, since the zero
	// value for the go timestamp isn't the unix epoch.
	if t == nil || (t.GetSeconds() == 0 && t.GetNanos() == 0) {
		return time.Time{}
	}

	return t.AsTime()
}

// TimeIntoProto converts a Go time.Time to a protobuf Timestamp, preserving
// the zero value across the conversion boundary (standard go/proto timestamp
// conversion doesn't preserve "zeroness").
func TimeIntoProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
