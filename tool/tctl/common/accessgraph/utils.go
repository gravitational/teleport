/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"strings"
	"time"

	types "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

type requestStateValue struct {
	target *types.RequestState
}

func (v requestStateValue) Set(s string) error {
	value, ok := types.RequestState_value[s]
	if !ok {
		return trace.BadParameter("invalid request state %q", s)
	}

	*v.target = types.RequestState(value)
	return nil
}

func (v requestStateValue) String() string {
	if v.target == nil {
		return ""
	}

	return v.target.String()
}

type timeValue struct {
	target *time.Time
}

func (v timeValue) Set(s string) error {
	parsed, err := parseTimeFilterValue(s, time.Now())
	if err != nil {
		return trace.Wrap(err)
	}

	*v.target = parsed
	return nil
}

func (v timeValue) String() string {
	if v.target == nil || v.target.IsZero() {
		return ""
	}

	return v.target.Format(time.RFC3339)
}

func parseTimeFilterValue(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts, nil
	}

	duration, err := parseRelativeDuration(s)
	if err != nil {
		return time.Time{}, trace.BadParameter("invalid time %q, expected RFC3339 or relative duration like 24h or 7d", s)
	}

	return now.Add(-duration), nil
}

func parseRelativeDuration(s string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(s, "d"); ok {
		days, err := time.ParseDuration(before + "h")
		if err != nil {
			return 0, trace.Wrap(err)
		}

		return days * 24, nil
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return duration, nil
}
