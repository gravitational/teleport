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

package discovery

import (
	"time"
)

type discoverySummary []configSummary

type configSummary struct {
	Name           string          `json:"name" yaml:"name"`
	DiscoveryGroup string          `json:"discovery_group" yaml:"discovery_group"`
	State          string          `json:"state" yaml:"state"`
	ErrorMessage   string          `json:"error_message,omitempty" yaml:"error_message,omitempty"`
	LastSyncTime   *time.Time      `json:"last_sync_time,omitempty" yaml:"last_sync_time,omitempty"`
	Servers        []serverSummary `json:"servers,omitempty" yaml:"servers,omitempty"`
}

type serverSummary struct {
	ServerID     string               `json:"server_id" yaml:"server_id"`
	PollInterval string               `json:"poll_interval,omitempty" yaml:"poll_interval,omitempty"`
	LastUpdate   *time.Time           `json:"last_update,omitempty" yaml:"last_update,omitempty"`
	Integrations []integrationSummary `json:"integrations,omitempty" yaml:"integrations,omitempty"`
}

type integrationSummary struct {
	Integration string           `json:"integration,omitempty" yaml:"integration,omitempty"`
	Resources   []resourceResult `json:"resources,omitempty" yaml:"resources,omitempty"`
}

type resourceResult struct {
	Kind      string     `json:"kind" yaml:"kind"`
	Found     uint64     `json:"found" yaml:"found"`
	Enrolled  uint64     `json:"enrolled" yaml:"enrolled"`
	Failed    uint64     `json:"failed" yaml:"failed"`
	SyncStart *time.Time `json:"sync_start,omitempty" yaml:"sync_start,omitempty"`
	SyncEnd   *time.Time `json:"sync_end,omitempty" yaml:"sync_end,omitempty"`
}
