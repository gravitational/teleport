//go:build !darwin && !linux
// +build !darwin,!linux

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

package metadata

import (
	"runtime"

	log "github.com/sirupsen/logrus"
)

// fetchOSVersion returns "" if not on linux and not on darwin.
func (c *fetchConfig) fetchOSVersion() string {
	log.Warningf("fetchOSVersion is not implemented for %s", runtime.GOOS)
	return ""
}

// fetchGlibcVersion returns "" if not on linux and not on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	log.Warningf("fetchGlibcVersion is not implemented for %s", runtime.GOOS)
	return ""
}
