// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
