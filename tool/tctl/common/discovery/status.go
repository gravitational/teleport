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
	"slices"
	"time"
)

// discoverySummary is directly serializable because configSummary already
// carries the stable JSON/YAML output contract.
type discoverySummary []configSummary

type configSummary struct {
	Name           string            `json:"name" yaml:"name"`
	DiscoveryGroup string            `json:"discovery_group" yaml:"discovery_group"`
	Status         configStatus      `json:"status" yaml:"status"`
	Resources      []resourceSummary `json:"resources" yaml:"resources"`
}

type configStatus struct {
	Reported     bool       `json:"-" yaml:"-"`
	State        string     `json:"state" yaml:"state"`
	LastRun      *time.Time `json:"last_run,omitempty" yaml:"last_run,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty" yaml:"error_message,omitempty"`
}

// resourceSummary is a single rendered resource section: one
// cloud/resource-type/integration grouping for a discovery config.
type resourceSummary struct {
	Cloud        string          `json:"cloud" yaml:"cloud"`
	ResourceType string          `json:"resource_type" yaml:"resource_type"`
	Source       string          `json:"source" yaml:"source"`
	Integration  string          `json:"integration,omitempty" yaml:"integration,omitempty"`
	Scopes       []resourceScope `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	LastSync     *time.Time      `json:"last_resource_sync,omitempty" yaml:"last_resource_sync,omitempty"`
	Result       resultSummary   `json:"result" yaml:"result"`
}

type resourceScope struct {
	Regions        []string `json:"regions,omitempty" yaml:"regions,omitempty"`
	Subscriptions  []string `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	ResourceGroups []string `json:"resource_groups,omitempty" yaml:"resource_groups,omitempty"`
	MatchTags      []string `json:"match_tags,omitempty" yaml:"match_tags,omitempty"`
}

type resultSummary struct {
	Kind    string        `json:"kind" yaml:"kind"`
	Counts  *resultCounts `json:"counts,omitempty" yaml:"counts,omitempty"`
	Message string        `json:"message,omitempty" yaml:"message,omitempty"`
}

type resultCounts struct {
	Found    uint64 `json:"found" yaml:"found"`
	Enrolled uint64 `json:"enrolled" yaml:"enrolled"`
	Failed   uint64 `json:"failed" yaml:"failed"`
}

func appendUnique(dst []string, values ...string) []string {
	for _, value := range values {
		if value == "" || slices.Contains(dst, value) {
			continue
		}
		dst = append(dst, value)
	}
	return dst
}
