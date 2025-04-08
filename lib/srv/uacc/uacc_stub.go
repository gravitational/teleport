//go:build !linux
// +build !linux

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

/*
Package uacc concerns itself with updating the user account database and log on nodes
that a client connects to with an interactive session.

This is a stub version that doesn't do anything and exists purely for compatibility purposes with systems we don't support.

We do not support macOS yet because they introduced ASL for user accounting with Mac OS X 10.6 (Snow Leopard)
and integrating with that takes additional effort.
*/
package uacc

import (
	"os"
)

// Open is a stub function.
func Open(utmpPath, wtmpPath string, username, hostname string, remote [4]int32, tty *os.File) error {
	return nil
}

// Close is a stub function.
func Close(utmpPath, wtmpPath string, tty *os.File) error {
	return nil
}

// UserWithPtyInDatabase is a stub function.
func UserWithPtyInDatabase(utmpPath string, username string) error {
	return nil
}

// LogFailedLogin is a stub function.
func LogFailedLogin(btmpPath, username, hostname string, remote [4]int32) error {
	return nil
}
