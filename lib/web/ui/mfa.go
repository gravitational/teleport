// Copyright 2021 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
)

// MFADevice describes a mfa device
type MFADevice struct {
	// ID is the device ID.
	ID string `json:"id"`
	// Name is the device name.
	Name string `json:"name"`
	// Type is the device type.
	Type string `json:"type"`
	// LastUsed is the time the user used the device last.
	LastUsed time.Time `json:"lastUsed"`
	// AddedAt is the time the user registered the device.
	AddedAt time.Time `json:"addedAt"`
}

// MakeMFADevices creates a UI list of mfa devices.
func MakeMFADevices(devices []*types.MFADevice) []MFADevice {
	uiList := make([]MFADevice, 0, len(devices))

	for _, device := range devices {
		uiDevice := MFADevice{
			ID:       device.Id,
			Name:     device.GetName(),
			Type:     device.MFAType(),
			LastUsed: device.LastUsed,
			AddedAt:  device.AddedAt,
		}
		uiList = append(uiList, uiDevice)
	}

	return uiList
}

// MakeAuthenticateChallenge converts proto to JSON format.
func MakeAuthenticateChallenge(protoChal *proto.MFAAuthenticateChallenge) *auth.MFAAuthenticateChallenge {
	chal := &auth.MFAAuthenticateChallenge{
		TOTPChallenge: protoChal.GetTOTP() != nil,
	}

	for _, u2fChal := range protoChal.GetU2F() {
		ch := u2f.AuthenticateChallenge{
			Version:   u2fChal.Version,
			Challenge: u2fChal.Challenge,
			KeyHandle: u2fChal.KeyHandle,
			AppID:     u2fChal.AppID,
		}
		if chal.AuthenticateChallenge == nil {
			chal.AuthenticateChallenge = &ch
		}
		chal.U2FChallenges = append(chal.U2FChallenges, ch)
	}

	if protoChal.GetWebauthnChallenge() != nil {
		chal.WebauthnChallenge = wanlib.CredentialAssertionFromProto(protoChal.WebauthnChallenge)
	}

	return chal
}
