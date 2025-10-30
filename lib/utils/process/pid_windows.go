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

package process

import "errors"

// CreateLockedPIDFile creates a PID file in the path specified by pidFile
// containing the current PID, atomically swapping it in the final place and
// leaving it with an exclusive advisory lock that will get released when the
// process ends, for the benefit of "pkill -L".
func CreateLockedPIDFile(pidFile string) error {
	return errors.New("PID files are not supported on Windows")
}
