/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"time"

	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TimeFromProto converts a protobuf Timestamp to a Go time.Time, preserving
// the zero value across the conversion boundary (standard go/proto timestamp
// conversion doesn't preserve "zeroness").
func TimeFromProto(t *timestamppb.Timestamp) time.Time {
	// use the zero time to represent the nil timestamp. note that this is conceptually distinct
	// from using t.GetSeconds() == 0 && t.GetNanos() == 0. a timstampb that happens to be created
	// targeting the unix epoch isn't necessarily equivalent to a zero go timestamp, since the zero
	// value for the go timestamp isn't the unix epoch.
	if t == nil {
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

// MinTTL selects the smallest non-zero duration from a and b.
func MinTTL(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// ToTTL converts expiration time to TTL duration
// relative to current time as provided by clock
func ToTTL(c clockwork.Clock, tm time.Time) time.Duration {
	now := c.Now().UTC()
	if tm.IsZero() || tm.Before(now) {
		return 0
	}
	return tm.Sub(now)
}
