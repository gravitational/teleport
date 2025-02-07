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

package dynamodbutils

import (
	"github.com/gravitational/teleport/lib/modules"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

// IsFIPSEnabled returns true if FIPS should be enabled for DynamoDB.
// FIPS is enabled is the binary is boring ([modules.Modules.IsBoringBinary])
// and if FIPS is not disabled by the environment
// ([awsutils.IsFIPSDisabledByEnv]).
func IsFIPSEnabled() bool {
	return !awsutils.IsFIPSDisabledByEnv() && modules.GetModules().IsBoringBinary()
}
