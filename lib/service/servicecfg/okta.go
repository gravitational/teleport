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

package servicecfg

import "time"

// OktaConfig specifies configuration for the Okta service.
type OktaConfig struct {
	// Enabled turns the Okta service on or off for this process
	Enabled bool

	// APIEndpoint is the Okta API endpoint to use.
	APIEndpoint string

	// APITokenPath is the path to the Okta API token.
	APITokenPath string

	// SyncPeriod is the duration between synchronization calls.
	// TODO(mdwn): Remove this once enterprise changes have been made.
	SyncPeriod time.Duration

	// SyncSettings is the settings for synchronizing access lists from Okta.
	SyncSettings OktaSyncSettings
}

// OktaSyncSettings specifies the configuration for synchronizing permissions from Okta.
type OktaSyncSettings struct {
	// AppGroupSyncPeriod is the duration between synchronization calls for synchronizing Okta applications and groups.
	AppGroupSyncPeriod time.Duration

	// SyncAccessLists turns the Okta access list synchronization functionality.
	SyncAccessLists bool

	// DefaultOwners are the default owners for all imported access lists.
	DefaultOwners []string

	// GroupFilters are filters for which Okta groups to synchronize as access lists.
	// These are globs/regexes.
	GroupFilters []string

	// AppFilters are filters for which Okta applications to synchronize as access lists.
	// These are globs/regexes.
	AppFilters []string
}
