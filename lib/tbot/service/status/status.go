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
package status

// Status represents the health of a service.
type Status uint

const (
	// Initializing means the service is still setting up its dependencies etc.
	// and is not yet ready for use.
	Initializing Status = iota

	// Ready means the service is ready for use.
	Ready

	// Failed means the service has encountered an error.
	Failed
)

// String implements fmt.Stringer.
func (s Status) String() string {
	switch s {
	case Initializing:
		return "initializing"
	case Ready:
		return "ready"
	case Failed:
		return "failed"
	default:
		return "<unknown status>"
	}
}
