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

package types

import (
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
)

func newTOTPDev(t *testing.T, name, id string) *MFADevice {
	t.Helper()
	dev, err := NewMFADevice(name, id, time.Now(), &MFADevice_Totp{
		Totp: &TOTPDevice{Key: "totp-key-" + id},
	})
	require.NoError(t, err)
	return dev
}

func newU2FDev(t *testing.T, name, id string) *MFADevice {
	t.Helper()
	dev, err := NewMFADevice(name, id, time.Now(), &MFADevice_U2F{
		U2F: &U2FDevice{
			KeyHandle: []byte("handle-" + id),
			PubKey:    []byte("pubkey-" + id),
			Counter:   42,
		},
	})
	require.NoError(t, err)
	return dev
}

func newWebauthnDev(t *testing.T, name, id string) *MFADevice {
	t.Helper()
	dev, err := NewMFADevice(name, id, time.Now(), &MFADevice_Webauthn{
		Webauthn: &WebauthnDevice{
			CredentialId:             []byte("cred-" + id),
			PublicKeyCbor:            []byte("cbor-" + id),
			AttestationType:          "none",
			Aaguid:                   []byte("aaguid"),
			SignatureCounter:         10,
			AttestationObject:        []byte("att-obj"),
			ResidentKey:              true,
			CredentialRpId:           "example.com",
			CredentialBackupEligible: &gogotypes.BoolValue{Value: true},
			CredentialBackedUp:       &gogotypes.BoolValue{Value: false},
		},
	})
	require.NoError(t, err)
	return dev
}

func newSSODev(t *testing.T, name, id string) *MFADevice {
	t.Helper()
	dev, err := NewMFADevice(name, id, time.Now(), &MFADevice_Sso{
		Sso: &SSOMFADevice{
			ConnectorId:   "connector-" + id,
			ConnectorType: "saml",
			DisplayName:   "SSO " + id,
		},
	})
	require.NoError(t, err)
	return dev
}

func TestMFADevicesEqual(t *testing.T) {
	totp1 := newTOTPDev(t, "totp-1", "id-totp-1")
	totp2 := newTOTPDev(t, "totp-2", "id-totp-2")
	u2f1 := newU2FDev(t, "u2f-1", "id-u2f-1")
	webauthn1 := newWebauthnDev(t, "webauthn-1", "id-webauthn-1")
	sso1 := newSSODev(t, "sso-1", "id-sso-1")
	fixedNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	dupNameA, err := NewMFADevice("dup-name", "id-dup-a", fixedNow, &MFADevice_Totp{
		Totp: &TOTPDevice{Key: "dup-key-a"},
	})
	require.NoError(t, err)
	dupNameB, err := NewMFADevice("dup-name", "id-dup-b", fixedNow, &MFADevice_Totp{
		Totp: &TOTPDevice{Key: "dup-key-b"},
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		a, b []*MFADevice
		want bool
	}{
		{
			name: "both nil",
			a:    nil,
			b:    nil,
			want: true,
		},
		{
			name: "both empty",
			a:    []*MFADevice{},
			b:    []*MFADevice{},
			want: true,
		},
		{
			name: "nil vs empty",
			a:    nil,
			b:    []*MFADevice{},
			want: true,
		},
		{
			name: "different lengths",
			a:    []*MFADevice{totp1},
			b:    []*MFADevice{totp1, totp2},
			want: false,
		},
		{
			name: "single TOTP, same order",
			a:    []*MFADevice{totp1},
			b:    []*MFADevice{totp1},
			want: true,
		},
		{
			name: "single U2F, same order",
			a:    []*MFADevice{u2f1},
			b:    []*MFADevice{u2f1},
			want: true,
		},
		{
			name: "single Webauthn, same order",
			a:    []*MFADevice{webauthn1},
			b:    []*MFADevice{webauthn1},
			want: true,
		},
		{
			name: "single SSO, same order",
			a:    []*MFADevice{sso1},
			b:    []*MFADevice{sso1},
			want: true,
		},
		{
			name: "all types, same order",
			a:    []*MFADevice{totp1, u2f1, webauthn1, sso1},
			b:    []*MFADevice{totp1, u2f1, webauthn1, sso1},
			want: true,
		},
		{
			name: "all types, reversed order",
			a:    []*MFADevice{totp1, u2f1, webauthn1, sso1},
			b:    []*MFADevice{sso1, webauthn1, u2f1, totp1},
			want: true,
		},
		{
			name: "all types, shuffled order",
			a:    []*MFADevice{totp1, u2f1, webauthn1, sso1},
			b:    []*MFADevice{webauthn1, totp1, sso1, u2f1},
			want: true,
		},
		{
			name: "two TOTPs, same order",
			a:    []*MFADevice{totp1, totp2},
			b:    []*MFADevice{totp1, totp2},
			want: true,
		},
		{
			name: "two TOTPs, swapped order",
			a:    []*MFADevice{totp1, totp2},
			b:    []*MFADevice{totp2, totp1},
			want: true,
		},
		{
			name: "nil entries reordered",
			a:    []*MFADevice{nil, totp1},
			b:    []*MFADevice{totp1, nil},
			want: true,
		},
		{
			name: "duplicate names do not collapse multiplicity",
			a:    []*MFADevice{dupNameA, dupNameB},
			b:    []*MFADevice{dupNameB, dupNameB},
			want: false,
		},
		{
			name: "TOTP vs U2F",
			a:    []*MFADevice{totp1},
			b:    []*MFADevice{u2f1},
			want: false,
		},
		{
			name: "TOTP vs Webauthn",
			a:    []*MFADevice{totp1},
			b:    []*MFADevice{webauthn1},
			want: false,
		},
		{
			name: "TOTP vs SSO",
			a:    []*MFADevice{totp1},
			b:    []*MFADevice{sso1},
			want: false,
		},
		{
			name: "U2F vs Webauthn",
			a:    []*MFADevice{u2f1},
			b:    []*MFADevice{webauthn1},
			want: false,
		},
		{
			name: "U2F vs SSO",
			a:    []*MFADevice{u2f1},
			b:    []*MFADevice{sso1},
			want: false,
		},
		{
			name: "Webauthn vs SSO",
			a:    []*MFADevice{webauthn1},
			b:    []*MFADevice{sso1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, mfaDevicesEqual(tt.a, tt.b))
		})
	}
}

func TestMFADevicesEqual_DeviceFieldDifferences(t *testing.T) {
	now := time.Now()

	baseTOTP := func() *MFADevice {
		d, err := NewMFADevice("totp", "id-totp", now, &MFADevice_Totp{
			Totp: &TOTPDevice{Key: "secret"},
		})
		require.NoError(t, err)
		return d
	}

	baseU2F := func() *MFADevice {
		d, err := NewMFADevice("u2f", "id-u2f", now, &MFADevice_U2F{
			U2F: &U2FDevice{
				KeyHandle: []byte("handle"),
				PubKey:    []byte("pubkey"),
				Counter:   1,
			},
		})
		require.NoError(t, err)
		return d
	}

	baseWebauthn := func() *MFADevice {
		d, err := NewMFADevice("webauthn", "id-webauthn", now, &MFADevice_Webauthn{
			Webauthn: &WebauthnDevice{
				CredentialId:             []byte("cred"),
				PublicKeyCbor:            []byte("cbor"),
				AttestationType:          "none",
				Aaguid:                   []byte("aaguid"),
				SignatureCounter:         5,
				AttestationObject:        []byte("att"),
				ResidentKey:              true,
				CredentialRpId:           "example.com",
				CredentialBackupEligible: &gogotypes.BoolValue{Value: true},
				CredentialBackedUp:       &gogotypes.BoolValue{Value: false},
			},
		})
		require.NoError(t, err)
		return d
	}

	baseSSO := func() *MFADevice {
		d, err := NewMFADevice("sso", "id-sso", now, &MFADevice_Sso{
			Sso: &SSOMFADevice{
				ConnectorId:   "conn",
				ConnectorType: "saml",
				DisplayName:   "My SSO",
			},
		})
		require.NoError(t, err)
		return d
	}

	tests := []struct {
		name    string
		a, b    func() *MFADevice
		mutateB func(d *MFADevice)
		want    bool
	}{
		{
			name: "TOTP identical",
			a:    baseTOTP,
			b:    baseTOTP,
			want: true,
		},
		{
			name: "TOTP different key",
			a:    baseTOTP,
			b:    baseTOTP,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Totp).Totp.Key = "other"
			},
			want: false,
		},
		{
			name: "U2F identical",
			a:    baseU2F,
			b:    baseU2F,
			want: true,
		},
		{
			name: "U2F different KeyHandle",
			a:    baseU2F,
			b:    baseU2F,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_U2F).U2F.KeyHandle = []byte("other")
			},
			want: false,
		},
		{
			name: "U2F different PubKey",
			a:    baseU2F,
			b:    baseU2F,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_U2F).U2F.PubKey = []byte("other")
			},
			want: false,
		},
		{
			name: "U2F different Counter",
			a:    baseU2F,
			b:    baseU2F,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_U2F).U2F.Counter = 999
			},
			want: false,
		},
		{
			name: "Webauthn identical",
			a:    baseWebauthn,
			b:    baseWebauthn,
			want: true,
		},
		{
			name: "Webauthn different CredentialId",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialId = []byte("other")
			},
			want: false,
		},
		{
			name: "Webauthn different PublicKeyCbor",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.PublicKeyCbor = []byte("other")
			},
			want: false,
		},
		{
			name: "Webauthn different AttestationType",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.AttestationType = "packed"
			},
			want: false,
		},
		{
			name: "Webauthn different Aaguid",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.Aaguid = []byte("other")
			},
			want: false,
		},
		{
			name: "Webauthn different SignatureCounter",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.SignatureCounter = 999
			},
			want: false,
		},
		{
			name: "Webauthn different AttestationObject",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.AttestationObject = []byte("other")
			},
			want: false,
		},
		{
			name: "Webauthn different ResidentKey",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.ResidentKey = false
			},
			want: false,
		},
		{
			name: "Webauthn different CredentialRpId",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialRpId = "other.com"
			},
			want: false,
		},
		{
			name: "Webauthn different CredentialBackupEligible value",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialBackupEligible = &gogotypes.BoolValue{Value: false}
			},
			want: false,
		},
		{
			name: "Webauthn CredentialBackupEligible nil vs set",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialBackupEligible = nil
			},
			want: false,
		},
		{
			name: "Webauthn different CredentialBackedUp value",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialBackedUp = &gogotypes.BoolValue{Value: true}
			},
			want: false,
		},
		{
			name: "Webauthn CredentialBackedUp nil vs set",
			a:    baseWebauthn,
			b:    baseWebauthn,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Webauthn).Webauthn.CredentialBackedUp = nil
			},
			want: false,
		},
		{
			name: "SSO identical",
			a:    baseSSO,
			b:    baseSSO,
			want: true,
		},
		{
			name: "SSO different ConnectorId",
			a:    baseSSO,
			b:    baseSSO,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Sso).Sso.ConnectorId = "other"
			},
			want: false,
		},
		{
			name: "SSO different ConnectorType",
			a:    baseSSO,
			b:    baseSSO,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Sso).Sso.ConnectorType = "oidc"
			},
			want: false,
		},
		{
			name: "SSO different DisplayName",
			a:    baseSSO,
			b:    baseSSO,
			mutateB: func(d *MFADevice) {
				d.Device.(*MFADevice_Sso).Sso.DisplayName = "Other"
			},
			want: false,
		},
		{
			name: "different Id",
			a:    baseTOTP,
			b:    baseTOTP,
			mutateB: func(d *MFADevice) {
				d.Id = "different-id"
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			da := tt.a()
			db := tt.b()
			if tt.mutateB != nil {
				tt.mutateB(db)
			}
			require.Equal(t, tt.want, mfaDevicesEqual([]*MFADevice{da}, []*MFADevice{db}))
		})
	}
}
