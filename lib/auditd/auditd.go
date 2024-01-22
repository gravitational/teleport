//go:build !linux

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

// Package auditd implements Linux Audit client that allows sending events
// and checking system configuration.
package auditd

// SendEvent is a stub function that is called on macOS. Auditd is implemented in Linux kernel and doesn't
// work on system different from Linux.
func SendEvent(_ EventType, _ ResultType, _ Message) error {
	return nil
}

// IsLoginUIDSet returns always false on non Linux systems.
func IsLoginUIDSet() bool {
	return false
}
