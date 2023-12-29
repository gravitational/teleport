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

package version

import "fmt"

// NoNewVersionError indicates that no new version was found and the controller did not reconcile.
type NoNewVersionError struct {
	Message        string `json:"message"`
	CurrentVersion string `json:"currentVersion"`
	NextVersion    string `json:"nextVersion"`
}

// Error returns log friendly description of an error
func (e *NoNewVersionError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("no new version (current: %q, next: %q)", e.CurrentVersion, e.NextVersion)
}
