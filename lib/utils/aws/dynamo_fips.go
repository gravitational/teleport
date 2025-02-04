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

	"github.com/gravitational/teleport/lib/modules"
)

// EnvVarDisableDynamoDBFIPS holds the name of the environment variable that
// disables FIPS for DynamoDB access in builds where FIPS would otherwise be
// required.
const EnvVarDisableDynamoDBFIPS = "TELEPORT_UNSTABLE_DISABLE_DYNAMODB_FIPS"

// UseFIPSForDynamoDB is a DynamoDB-specific check that builds on
// [modules.Modules.IsBoringBinary].
//
// FIPS is enabled by default for boring/FIPS teleport binaries, unless the
// TELEPORT_UNSTABLE_DISABLE_DYNAMODB_FIPS env variable is set to "yes" (or an
// equivalent boolean).
func UseFIPSForDynamoDB() bool {
	// If the skip toggle is set we don't use FIPS DynamoDB.
	if val := os.Getenv(EnvVarDisableDynamoDBFIPS); val != "" {
		if val == "yes" {
			return false
		}
		if b, _ := strconv.ParseBool(val); b {
			return false
		}
	}

	return modules.GetModules().IsBoringBinary()
}
