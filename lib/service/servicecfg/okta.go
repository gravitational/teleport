// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
}
