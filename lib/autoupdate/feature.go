/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package autoupdate

const (
	// FlagEnt represents enterprise version.
	FlagEnt = 1 << iota
	// FlagFips represents enterprise version with fips feature enabled.
	FlagFips
)

var (
	// featureFlag stores information about enabled feature to define which package needs
	// to be downloaded for auto update (e.g. Enterprise, or package with FIPS).
	featureFlag int
)

// FeatureFlag returns information about enabled feature to identify which package needs
// to be downloaded for auto update (e.g. Enterprise, or package with FIPS).
func FeatureFlag() int {
	return featureFlag
}
