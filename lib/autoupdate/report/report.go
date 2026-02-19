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

// Package report provides utilities for working with AutoUpdateAgentReport resources.
package report

import (
	"fmt"
	"slices"
	"time"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// UserFriendlyState converts an AutoUpdateAgentGroupState or AutoUpdateAgentRolloutState
// to a user-friendly string representation.
func UserFriendlyState[T autoupdatepb.AutoUpdateAgentGroupState | autoupdatepb.AutoUpdateAgentRolloutState](state T) string {
	switch state {
	case 0:
		return "Unknown"
	case 1:
		return "Unstarted"
	case 2:
		return "Active"
	case 3:
		return "Done"
	case 4:
		return "Rolledback"
	case 5:
		return "Canary"
	default:
		// If we don't know anything about this state, we display its integer
		return fmt.Sprintf("Unknown state (%d)", state)
	}
}

// ValidReports filters out stale reports and returns only valid ones.
// Reports are generated every 1 minute, so any that are older than that must be stale.
func ValidReports(reports []*autoupdatepb.AutoUpdateAgentReport, now time.Time) []*autoupdatepb.AutoUpdateAgentReport {
	isStale := func(r *autoupdatepb.AutoUpdateAgentReport) bool {
		return now.Sub(r.GetSpec().GetTimestamp().AsTime()) > time.Minute
	}
	return slices.DeleteFunc(slices.Clone(reports), isStale)
}

// AggregateVersionCounts aggregates agent reports into version counts by group
func AggregateVersionCounts(reports []*autoupdatepb.AutoUpdateAgentReport) map[string]map[string]int {
	out := make(map[string]map[string]int)

	for _, report := range reports {
		for groupName, group := range report.GetSpec().GetGroups() {
			if out[groupName] == nil {
				out[groupName] = make(map[string]int)
			}
			for version, versionCount := range group.GetVersions() {
				out[groupName][version] += int(versionCount.GetCount())
			}
		}
	}

	return out
}
