// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package aws

import (
	"os"
	"strconv"
)

// IsFIPSDisabled returns true if the TELEPORT_UNSTABLE_DISABLE_AWS_FIPS
// environment variable is set.
//
// Either "yes" or a "truthy" value (as defined by [strconv.ParseBool]) are
// considered true.
//
// Prefer using specific functions, such as those in the
// lib/utils/aws/stsutils or lib/utils/aws/dynamodbutils packages.
func IsFIPSDisabledByEnv() bool {
	const envVar = "TELEPORT_UNSTABLE_DISABLE_AWS_FIPS"

	// Disable FIPS endpoint?
	if val := os.Getenv(envVar); val != "" {
		b, _ := strconv.ParseBool(val)
		return b || val == "yes"
	}

	return false
}
