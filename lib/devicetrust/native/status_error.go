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

package native

import (
	"fmt"

	"github.com/gravitational/teleport/lib/devicetrust"
)

const (
	// https://www.osstatus.com/search/results?framework=Security&search=-25300
	errSecItemNotFound = -25300
	// https://www.osstatus.com/search/results?framework=Security&search=-34018
	errSecMissingEntitlement = -34018
)

// statusError represents a native error that contains a status code, typically
// an OSStatus value.
type statusError struct {
	status int32
}

func (e *statusError) Error() string {
	switch e.status {
	case errSecItemNotFound:
		// errSecItemNotFound can also occur because of an unsigned binary - it
		// cannot read the correct key, so it appears the same as not finding any.
		// TODO(codingllama): Consider adding a signature check to client-side
		//  Device Trust, like lib/auth/touchid.
		return "device key not found, was the device enrolled?"
	case errSecMissingEntitlement:
		return "binary missing signature or entitlements, download the client binaries from https://goteleport.com/download/"
	default:
		return fmt.Sprintf("status %d", e.status)
	}
}

func (e *statusError) Is(target error) bool {
	if target == devicetrust.ErrDeviceKeyNotFound && e.status == errSecItemNotFound {
		return true
	}

	other, ok := target.(*statusError)
	return ok && other.status == e.status
}
