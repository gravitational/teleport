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

This file contains a single global variable controlling which edition
of Teleport is running

This flag contains various global booleans that are set during
Teleport initialization.

These are NOT for configuring Teleport: use regular Config facilities for that,
preferably tailored to specific services, i.e proxy config, auth config, etc

These are for being set once, at the beginning of the process, and for
being visible to any code under 'lib'

*/

package lib

import (
	"sync"
)

var (
	// insecureDevMode is set to 'true' when teleport is started with a hidden
	// --insecure flag. This mode is only useful for learning Teleport and following
	// quick starts: it disables HTTPS certificate validation
	insecureDevMode bool

	// flagLock protects access to all globals declared in this file
	flagLock sync.Mutex
)

// SetInsecureDevMode turns the 'insecure' mode on. In this mode Teleport accepts
// self-signed HTTPS certificates (for development only!)
func SetInsecureDevMode(m bool) {
	flagLock.Lock()
	defer flagLock.Unlock()
	insecureDevMode = m
}

// IsInsecureDevMode returns 'true' if Teleport daemon was started with the
// --insecure flag
func IsInsecureDevMode() bool {
	flagLock.Lock()
	defer flagLock.Unlock()
	return insecureDevMode
}
