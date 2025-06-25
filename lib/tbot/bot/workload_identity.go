/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package bot

import "github.com/gravitational/trace"

// WorkloadIdentitySelector allows the user to select which WorkloadIdentity
// resource should be used.
//
// Only one of Name or Labels can be set.
type WorkloadIdentitySelector struct {
	// Name is the name of a specific WorkloadIdentity resource.
	Name string `yaml:"name"`
	// Labels is a set of labels that the WorkloadIdentity resource must have.
	Labels map[string][]string `yaml:"labels,omitempty"`
}

// CheckAndSetDefaults checks the WorkloadIdentitySelector values and sets any
// defaults.
func (s *WorkloadIdentitySelector) CheckAndSetDefaults() error {
	switch {
	case s.Name == "" && len(s.Labels) == 0:
		return trace.BadParameter("one of ['name', 'labels'] must be set")
	case s.Name != "" && len(s.Labels) > 0:
		return trace.BadParameter("at most one of ['name', 'labels'] can be set")
	}
	for k, v := range s.Labels {
		if len(v) == 0 {
			return trace.BadParameter("labels[%s]: must have at least one value", k)
		}
	}
	return nil
}
