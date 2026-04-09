/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mfa

import (
	"github.com/gravitational/teleport/lib/auth/touchid"
)

// MFADeviceType is a type of MFA device for device registration purposes.
type MFADeviceType string

const (
	MFADeviceTypeTOTP     MFADeviceType = "TOTP"
	MFADeviceTypeWebauthn MFADeviceType = "WEBAUTHN"
	MFADeviceTypeTouchID  MFADeviceType = "TOUCHID"
)

var (
	// totpDeviceTypes are device types available when the second factor option
	// is set to [constants.SecondFactorOff].
	totpDeviceTypes = []MFADeviceType{MFADeviceTypeTOTP}

	// webDeviceTypes are device types available when the second factor option is
	// set to [constants.SecondFactorWebauthn].
	webDeviceTypes = initWebDevs()

	// DefaultDeviceTypes lists the supported device types for `tsh mfa add`.
	DefaultDeviceTypes = append(totpDeviceTypes, webDeviceTypes...)
)

func initWebDevs() []MFADeviceType {
	if touchid.IsAvailable() {
		return []MFADeviceType{MFADeviceTypeWebauthn, MFADeviceTypeTouchID}
	}
	return []MFADeviceType{MFADeviceTypeWebauthn}
}
