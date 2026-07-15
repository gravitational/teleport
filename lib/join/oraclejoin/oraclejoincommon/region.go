// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package oraclejoincommon

import (
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
)

var initOCIRegionsOnce sync.Once

// StringToRegion wraps [common.StringToRegion] with a safe initialization.
func StringToRegion(rawRegion string) common.Region {
	initOCIRegionsOnce.Do(func() {
		// Hack: StringToRegion will lazily load regions from a config file if its
		// input isn't in its hard-coded list, in a non-threadsafe way. Call it once
		// on the first call so future calls are threadsafe.
		_ = common.StringToRegion("") //nolint:forbidigo // required to pre-init OCI SDK regions safely
	})
	return common.StringToRegion(rawRegion) //nolint:forbidigo // this is the wrapped callsite
}
