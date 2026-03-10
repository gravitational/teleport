// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package plugin

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

const (
	// OktaDefaultTimeBetweenImports is the default amount of time between Okta to Teleport
	// periodic syncs. This setting can be configured with the
	// okta.sync_settings.time_between_imports Okta plugin value.
	OktaDefaultTimeBetweenImports = 30 * time.Minute

	// OktaDefaultTimeBetweenAssignmentProcessLoops is the default amount of time that has to
	// pass between running the assignments process loop. It also determines how often the
	// cached Okta assignments client is invalidated. This setting can be configured with the
	// okta.sync_settings.time_between_assignment_process_loops Okta plugin value.
	OktaDefaultTimeBetweenAssignmentProcessLoops = 10 * time.Minute
)

// OktaGetTimeBetweenImports parses syncSettings.TimeBetweenImports to duration. If it is not set
// it returns a default value of 30m.
func OktaGetTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	d, err := oktaParseTimeBetweenImports(syncSettings)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if d == 0 {
		return OktaDefaultTimeBetweenImports, nil
	}
	return d, nil
}

// Deprecated: Use [OktaGetTimeBetweenImports] instead.
// TODO(kopiczko): remove once https://github.com/gravitational/teleport.e/pull/8217 is merged.
func OktaParseTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	return oktaParseTimeBetweenImports(syncSettings)
}

func oktaParseTimeBetweenImports(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return 0, nil
	}
	raw := syncSettings.TimeBetweenImports
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_imports is not valid: %s", err)
	}
	if parsed < 0 {
		return 0, trace.BadParameter("time_between_imports %q cannot be a negative value", raw)
	}
	return parsed, nil
}

// OktaGetTimeBetweenAssignmentProcessLoops parses syncSettings.TimeBetweenAssignmentProcessLoops
// to duration. If it is not set it returns a default value of 10m.
func OktaGetTimeBetweenAssignmentProcessLoops(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	d, err := oktaParseTimeBetweenAssignmentProcessLoops(syncSettings)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if d == 0 {
		return OktaDefaultTimeBetweenAssignmentProcessLoops, nil
	}
	return d, nil
}

// Deprecated: Use [OktaGetTimeBetweenAssignmentProcessLoops] instead.
// TODO(kopiczko): remove once https://github.com/gravitational/teleport.e/pull/8217 is merged.
func OktaParseTimeBetweenAssignmentProcessLoops(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	return oktaParseTimeBetweenAssignmentProcessLoops(syncSettings)
}

func oktaParseTimeBetweenAssignmentProcessLoops(syncSettings *types.PluginOktaSyncSettings) (time.Duration, error) {
	if syncSettings == nil {
		return 0, nil
	}
	raw := syncSettings.TimeBetweenAssignmentProcessLoops
	if raw == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, trace.BadParameter("time_between_assignment_process_loops is not valid: %s", err)
	}
	if parsed < 0 {
		return 0, trace.BadParameter("time_between_assignment_process_loops %q cannot be a negative value", raw)
	}
	return parsed, nil
}
