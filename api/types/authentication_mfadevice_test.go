// Copyright 2021 Gravitational, Inc
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

package types_test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
)

func TestMFADevice_CheckAndSetDefaults(t *testing.T) {
	now := time.Now()

	newWebauthnDevice := func(d *types.WebauthnDevice) *types.MFADevice {
		return &types.MFADevice{
			Metadata: types.Metadata{
				Name: "webauthn",
			},
			Id:       "web-0001",
			AddedAt:  now,
			LastUsed: now,
			Device: &types.MFADevice_Webauthn{
				Webauthn: d,
			},
		}
	}

	tests := []struct {
		name    string
		dev     *types.MFADevice
		wantErr bool
	}{
		{
			name: "OK OTP device",
			dev: &types.MFADevice{
				Metadata: types.Metadata{
					Name: "otp",
				},
				Id:       "otp-0001",
				AddedAt:  now,
				LastUsed: now,
				Device:   &types.MFADevice_Totp{}, // validated elsewhere
			},
		},
		{
			name: "OK U2F device",
			dev: &types.MFADevice{
				Metadata: types.Metadata{
					Name: "u2f",
				},
				Id:       "u2f-0001",
				AddedAt:  now,
				LastUsed: now,
				Device:   &types.MFADevice_U2F{}, // validated elsewhere
			},
		},
		{
			name: "OK Webauthn device",
			dev: newWebauthnDevice(&types.WebauthnDevice{
				CredentialId:     []byte("web credential ID"),
				PublicKeyCbor:    []byte("web public key"),
				SignatureCounter: 0,
			}),
		},
		{
			name:    "NOK Webauthn device malformed",
			dev:     newWebauthnDevice(nil),
			wantErr: true,
		},
		{
			name: "NOK Webauthn missing credential ID",
			dev: newWebauthnDevice(&types.WebauthnDevice{
				PublicKeyCbor: []byte("web public key"),
			}),
			wantErr: true,
		},
		{
			name: "NOK Webauthn missing public key",
			dev: newWebauthnDevice(&types.WebauthnDevice{
				CredentialId: []byte("web credential ID"),
			}),
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.dev.CheckAndSetDefaults()
			if hasErr := err != nil; hasErr != test.wantErr {
				t.Errorf("CheckAndSetDefaults returned err = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
