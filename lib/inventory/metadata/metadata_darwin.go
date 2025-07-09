//go:build darwin
// +build darwin

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
	"fmt"
)

// fetchOSVersion returns something equivalent to the output of
// '$(sw_vers -productName) $(sw_vers -productVersion)'.
func (c *fetchConfig) fetchOSVersion() string {
	productName, err := c.exec("sw_vers", "-productName")
	if err != nil {
		return ""
	}

	productVersion, err := c.exec("sw_vers", "-productVersion")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s %s", productName, productVersion)
}

// fetchGlibcVersion returns "" on darwin.
func (c *fetchConfig) fetchGlibcVersion() string {
	return ""
}
