/*
Copyright 2021 Gravitational, Inc.

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

package types_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
)

const (
	appleWebauthnRoot = `-----BEGIN CERTIFICATE-----
MIICEjCCAZmgAwIBAgIQaB0BbHo84wIlpQGUKEdXcTAKBggqhkjOPQQDAzBLMR8w
HQYDVQQDDBZBcHBsZSBXZWJBdXRobiBSb290IENBMRMwEQYDVQQKDApBcHBsZSBJ
bmMuMRMwEQYDVQQIDApDYWxpZm9ybmlhMB4XDTIwMDMxODE4MjEzMloXDTQ1MDMx
NTAwMDAwMFowSzEfMB0GA1UEAwwWQXBwbGUgV2ViQXV0aG4gUm9vdCBDQTETMBEG
A1UECgwKQXBwbGUgSW5jLjETMBEGA1UECAwKQ2FsaWZvcm5pYTB2MBAGByqGSM49
AgEGBSuBBAAiA2IABCJCQ2pTVhzjl4Wo6IhHtMSAzO2cv+H9DQKev3//fG59G11k
xu9eI0/7o6V5uShBpe1u6l6mS19S1FEh6yGljnZAJ+2GNP1mi/YK2kSXIuTHjxA/
pcoRf7XkOtO4o1qlcaNCMEAwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUJtdk
2cV4wlpn0afeaxLQG2PxxtcwDgYDVR0PAQH/BAQDAgEGMAoGCCqGSM49BAMDA2cA
MGQCMFrZ+9DsJ1PW9hfNdBywZDsWDbWFp28it1d/5w2RPkRX3Bbn/UbDTNLx7Jr3
jAGGiQIwHFj+dJZYUJR786osByBelJYsVZd2GbHQu209b5RCmGQ21gpSAk9QZW4B
1bWeT0vT
-----END CERTIFICATE-----`
	yubicoU2FCA = `-----BEGIN CERTIFICATE-----
MIIDHjCCAgagAwIBAgIEG0BT9zANBgkqhkiG9w0BAQsFADAuMSwwKgYDVQQDEyNZ
dWJpY28gVTJGIFJvb3QgQ0EgU2VyaWFsIDQ1NzIwMDYzMTAgFw0xNDA4MDEwMDAw
MDBaGA8yMDUwMDkwNDAwMDAwMFowLjEsMCoGA1UEAxMjWXViaWNvIFUyRiBSb290
IENBIFNlcmlhbCA0NTcyMDA2MzEwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEK
AoIBAQC/jwYuhBVlqaiYWEMsrWFisgJ+PtM91eSrpI4TK7U53mwCIawSDHy8vUmk
5N2KAj9abvT9NP5SMS1hQi3usxoYGonXQgfO6ZXyUA9a+KAkqdFnBnlyugSeCOep
8EdZFfsaRFtMjkwz5Gcz2Py4vIYvCdMHPtwaz0bVuzneueIEz6TnQjE63Rdt2zbw
nebwTG5ZybeWSwbzy+BJ34ZHcUhPAY89yJQXuE0IzMZFcEBbPNRbWECRKgjq//qT
9nmDOFVlSRCt2wiqPSzluwn+v+suQEBsUjTGMEd25tKXXTkNW21wIWbxeSyUoTXw
LvGS6xlwQSgNpk2qXYwf8iXg7VWZAgMBAAGjQjBAMB0GA1UdDgQWBBQgIvz0bNGJ
hjgpToksyKpP9xv9oDAPBgNVHRMECDAGAQH/AgEAMA4GA1UdDwEB/wQEAwIBBjAN
BgkqhkiG9w0BAQsFAAOCAQEAjvjuOMDSa+JXFCLyBKsycXtBVZsJ4Ue3LbaEsPY4
MYN/hIQ5ZM5p7EjfcnMG4CtYkNsfNHc0AhBLdq45rnT87q/6O3vUEtNMafbhU6kt
hX7Y+9XFN9NpmYxr+ekVY5xOxi8h9JDIgoMP4VB1uS0aunL1IGqrNooL9mmFnL2k
LVVee6/VR6C5+KSTCMCWppMuJIZII2v9o4dkoZ8Y7QRjQlLfYzd3qGtKbw7xaF1U
sG/5xUb/Btwb2X2g4InpiB/yt/3CpQXpiWX/K4mBvUKiGn05ZsqeY1gx4g0xLBqc
U9psmyPzK+Vsgw2jeRQ5JlKDyqE0hebfC1tvFu0CCrJFcw==
-----END CERTIFICATE-----`
)

func TestAuthPreferenceV2_CheckAndSetDefaults_secondFactor(t *testing.T) {
	t.Parallel()

	secondFactorAll := []constants.SecondFactorType{
		constants.SecondFactorOff,
		constants.SecondFactorOTP,
		constants.SecondFactorWebauthn,
		constants.SecondFactorOn,
		constants.SecondFactorOptional,
	}
	secondFactorWebActive := []constants.SecondFactorType{
		constants.SecondFactorWebauthn,
		constants.SecondFactorOn,
		constants.SecondFactorOptional,
	}

	minimalU2F := &types.U2F{
		AppID: "https://localhost:3080",
	}
	minimalWeb := &types.Webauthn{
		RPID: "localhost",
	}

	tests := []struct {
		name string

		secondFactors []constants.SecondFactorType
		spec          types.AuthPreferenceSpecV2

		// wantErr is a substring of the returned error
		wantErr string
		// assertFn is an optional asserting function
		assertFn func(t *testing.T, got *types.AuthPreferenceV2)
	}{
		// General testing.
		{
			name: "OK empty config",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
		},
		// U2F tests.
		{
			name:          "OK U2F minimal configuration",
			secondFactors: secondFactorAll,
			spec: types.AuthPreferenceSpecV2{
				U2F: minimalU2F,
			},
		},
		{
			name: "OK U2F aliased to Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorU2F,
			},
			spec: types.AuthPreferenceSpecV2{
				U2F: minimalU2F,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.Equal(t, constants.SecondFactorWebauthn, got.Spec.SecondFactor)
			},
		},
		// Webauthn tests.
		{
			name: "OK Webauthn minimal configuration",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
				// constants.SecondFactorU2F excluded.
				constants.SecondFactorWebauthn,
				constants.SecondFactorOn,
				constants.SecondFactorOptional,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
		},
		{
			name:          "OK Webauthn derived from U2F",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				U2F: &types.U2F{
					AppID:                "https://example.com:1234",
					DeviceAttestationCAs: []string{yubicoU2FCA},
				},
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				wantWeb := &types.Webauthn{
					RPID:                  "example.com",
					AttestationAllowedCAs: []string{yubicoU2FCA},
				}
				gotWeb, err := got.GetWebauthn()
				require.NoError(t, err, "webauthn config not found")
				require.Empty(t, cmp.Diff(wantWeb, gotWeb))
			},
		},
		{
			name:          "OK Webauthn derived from non-URL",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				U2F: &types.U2F{
					AppID: "teleport", // "teleport" gets parsed as a Path, not a Host.
				},
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				wantWeb := &types.Webauthn{
					RPID: "teleport",
				}
				gotWeb, err := got.GetWebauthn()
				require.NoError(t, err, "webauthn config not found")
				require.Empty(t, cmp.Diff(wantWeb, gotWeb))
			},
		},
		{
			name:          "OK Webauthn with attestation CAs",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Webauthn: &types.Webauthn{
					RPID:                  "example.com",
					AttestationAllowedCAs: []string{appleWebauthnRoot},
					AttestationDeniedCAs:  []string{yubicoU2FCA},
				},
			},
		},
		{
			name:          "NOK Webauthn empty",
			secondFactors: secondFactorWebActive,
			wantErr:       "missing required webauthn configuration",
		},
		{
			name:          "NOK Webauthn missing RPID",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Webauthn: &types.Webauthn{},
			},
			wantErr: "missing rp_id",
		},
		{
			name:          "NOK Webauthn invalid allowed CA",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Webauthn: &types.Webauthn{
					RPID:                  "example.com",
					AttestationAllowedCAs: []string{"bad inline cert"},
				},
			},
			wantErr: "webauthn allowed CAs",
		},
		{
			name:          "NOK Webauthn invalid denied CA",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Webauthn: &types.Webauthn{
					RPID:                 "example.com",
					AttestationDeniedCAs: []string{"bad inline cert"},
				},
			},
			wantErr: "webauthn denied CAs",
		},
		// IsSecondFactorEnforced?
		{
			name: "OK second factor enforced",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOTP,
				constants.SecondFactorWebauthn,
				constants.SecondFactorOn,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.True(t, got.IsSecondFactorEnforced(), "second factor not enforced")
			},
		},
		{
			name: "OK second factor not enforced",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOptional,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.False(t, got.IsSecondFactorEnforced(), "second factor enforced")
			},
		},
		// IsSecondFactor*Allowed?
		{
			name: "OK OTP second factor allowed",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOTP,
				constants.SecondFactorOn,
				constants.SecondFactorOptional,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.True(t, got.IsSecondFactorTOTPAllowed(), "OTP not allowed")
			},
		},
		{
			name: "OK OTP second factor not allowed",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorWebauthn,
			},
			spec: types.AuthPreferenceSpecV2{
				U2F:      minimalU2F,
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.False(t, got.IsSecondFactorTOTPAllowed(), "OTP allowed")
			},
		},
		{
			name: "OK Webauthn second factor allowed",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorWebauthn,
				constants.SecondFactorOn,
				constants.SecondFactorOptional,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.True(t, got.IsSecondFactorWebauthnAllowed(), "Webauthn not allowed")
			},
		},
		{
			name: "OK Webauthn second factor not allowed",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				U2F:      minimalU2F,
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.False(t, got.IsSecondFactorWebauthnAllowed(), "Webauthn allowed")
			},
		},
		// GetPreferredLocalMFA
		{
			name: "OK preferred local MFA empty",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.Empty(t, got.GetPreferredLocalMFA(), "preferred local MFA not empty")
			},
		},
		{
			name: "OK preferred local MFA = OTP",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOTP,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.Equal(t, constants.SecondFactorOTP, got.GetPreferredLocalMFA())
			},
		},
		{
			name: "OK preferred local MFA = Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorWebauthn,
				constants.SecondFactorOn,
				constants.SecondFactorOptional,
			},
			spec: types.AuthPreferenceSpecV2{
				Webauthn: minimalWeb,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				require.Equal(t, constants.SecondFactorWebauthn, got.GetPreferredLocalMFA())
			},
		},
		// AllowLocalAuth
		{
			name: "OK AllowLocalAuth preserve explicit false for type=local",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff, // doesn't matter for this test
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:           constants.Local,
				AllowLocalAuth: types.NewBoolOption(false),
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				assert.False(t, got.GetAllowLocalAuth(), "AllowLocalAuth")
			},
		},
		// AllowLocalAuth
		{
			name: "OK AllowLocalAuth default to true for type=local",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff, // doesn't matter for this test
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type: constants.Local,
			},
			assertFn: func(t *testing.T, got *types.AuthPreferenceV2) {
				assert.True(t, got.GetAllowLocalAuth(), "AllowLocalAuth")
			},
		},
		// AllowPasswordless
		{
			name: "OK AllowPasswordless defaults to false without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				AllowPasswordless: nil, // aka unset
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowPasswordless(), "AllowPasswordless")
			},
		},
		{
			name: "OK AllowPasswordless=false without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				AllowPasswordless: types.NewBoolOption(false),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowPasswordless(), "AllowPasswordless")
			},
		},
		{
			name: "NOK AllowPasswordless=true without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				AllowPasswordless: types.NewBoolOption(true),
			},
			wantErr: "required Webauthn",
		},
		{
			name:          "OK AllowPasswordless defaults to true with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				Webauthn:          minimalWeb,
				AllowPasswordless: nil, // aka unset
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.True(t, cap.GetAllowPasswordless(), "AllowPasswordless")
			},
		},
		{
			name:          "OK AllowPasswordless=false with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				Webauthn:          minimalWeb,
				AllowPasswordless: types.NewBoolOption(false),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowPasswordless(), "AllowPasswordless")
			},
		},
		{
			name:          "OK AllowPasswordless=true with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				Webauthn:          minimalWeb,
				AllowPasswordless: types.NewBoolOption(true),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.True(t, cap.GetAllowPasswordless(), "AllowPasswordless")
			},
		},
		// AllowHeadless
		{
			name: "OK AllowHeadless defaults to false without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				AllowHeadless: nil, // aka unset
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowHeadless(), "AllowHeadless")
			},
		},
		{
			name: "OK AllowHeadless=false without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				AllowHeadless: types.NewBoolOption(false),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowHeadless(), "AllowHeadless")
			},
		},
		{
			name: "NOK AllowHeadless=true without Webauthn",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff,
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				AllowHeadless: types.NewBoolOption(true),
			},
			wantErr: "required Webauthn",
		},
		{
			name:          "OK AllowHeadless defaults to true with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				Webauthn:      minimalWeb,
				AllowHeadless: nil, // aka unset
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.True(t, cap.GetAllowHeadless(), "AllowHeadless")
			},
		},
		{
			name:          "OK AllowHeadless=false with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				Webauthn:      minimalWeb,
				AllowHeadless: types.NewBoolOption(false),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.False(t, cap.GetAllowHeadless(), "AllowHeadless")
			},
		},
		{
			name:          "OK AllowHeadless=true with Webauthn",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				Webauthn:      minimalWeb,
				AllowHeadless: types.NewBoolOption(true),
			},
			assertFn: func(t *testing.T, cap *types.AuthPreferenceV2) {
				assert.True(t, cap.GetAllowHeadless(), "AllowHeadless")
			},
		},
		// ConnectorName
		{
			name:          "OK type=local and local connector",
			secondFactors: secondFactorAll,
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				ConnectorName:     constants.LocalConnector,
				Webauthn:          minimalWeb,
				AllowPasswordless: types.NewBoolOption(false), // restriction makes no difference
			},
		},
		{
			name:          "OK type=oidc and local connector",
			secondFactors: secondFactorAll,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.OIDC,           // or SAML
				ConnectorName: constants.LocalConnector, // not validated
				Webauthn:      minimalWeb,
			},
		},
		{
			name:          "OK type=oidc and arbitrary connector",
			secondFactors: secondFactorAll,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.OIDC, // or SAML
				ConnectorName: "myconnector",
				Webauthn:      minimalWeb,
			},
		},
		{
			name:          "OK type=local and passwordless connector",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: constants.PasswordlessConnector,
				Webauthn:      minimalWeb,
			},
		},
		{
			name:          "OK type=local and headless connector",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: constants.HeadlessConnector,
				Webauthn:      minimalWeb,
			},
		},
		{
			name: "NOK type=local and passwordless connector",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff, // webauthn disabled
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: constants.PasswordlessConnector,
				Webauthn:      minimalWeb,
			},
			wantErr: "passwordless not allowed",
		},
		{
			name:          "NOK type=local, allow_passwordless=false and passwordless connector",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:              constants.Local,
				ConnectorName:     constants.PasswordlessConnector,
				Webauthn:          minimalWeb,
				AllowPasswordless: types.NewBoolOption(false),
			},
			wantErr: "passwordless not allowed",
		},

		{
			name: "NOK type=local and headless connector",
			secondFactors: []constants.SecondFactorType{
				constants.SecondFactorOff, // webauthn disabled
				constants.SecondFactorOTP,
			},
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: constants.HeadlessConnector,
				Webauthn:      minimalWeb,
			},
			wantErr: "headless not allowed",
		},
		{
			name:          "NOK type=local, allow_headless=false and headless connector",
			secondFactors: secondFactorWebActive,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: constants.HeadlessConnector,
				Webauthn:      minimalWeb,
				AllowHeadless: types.NewBoolOption(false),
			},
			wantErr: "headless not allowed",
		},
		{
			name:          "NOK type=local and unknown connector",
			secondFactors: secondFactorAll,
			spec: types.AuthPreferenceSpecV2{
				Type:          constants.Local,
				ConnectorName: "bad",
				Webauthn:      minimalWeb,
			},
			wantErr: "invalid local connector",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Sanity check setup.
			require.NotEmpty(t, test.secondFactors, "test must provide second factor values")

			cap := &types.AuthPreferenceV2{
				Spec: test.spec,
			}

			for _, sf := range test.secondFactors {
				t.Run(fmt.Sprintf("sf=%v", sf), func(t *testing.T) {
					cap.Spec.SecondFactor = sf

					switch err := cap.CheckAndSetDefaults(); {
					case err == nil && test.wantErr == "": // OK
					case err == nil && test.wantErr != "":
						t.Fatalf("CheckAndSetDefaults = nil, want %q", test.wantErr)
					case !strings.Contains(err.Error(), test.wantErr) || test.wantErr == "":
						t.Fatalf("CheckAndSetDefaults = %q, want %q", err, test.wantErr)
					}

					if test.assertFn == nil {
						return
					}
					test.assertFn(t, cap)
				})
			}
		})
	}
}

func TestAuthPreferenceV2_CheckAndSetDefaults_deviceTrust(t *testing.T) {
	tests := []struct {
		name     string
		authPref *types.AuthPreferenceV2
		wantErr  string
	}{
		{
			name: "Mode default",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: "", // "off" for OSS, "optional" for Enterprise.
					},
				},
			},
		},
		{
			name: "Mode=off",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: constants.DeviceTrustModeOff,
					},
				},
			},
		},
		{
			name: "Mode=optional",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: constants.DeviceTrustModeOptional,
					},
				},
			},
		},
		{
			name: "Mode=required",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: constants.DeviceTrustModeRequired,
					},
				},
			},
		},
		{
			name: "Mode invalid",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: "bad",
					},
				},
			},
			wantErr: "device trust mode",
		},
		{
			name: "Bad EKCertAllowedCAs",
			authPref: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					DeviceTrust: &types.DeviceTrust{
						Mode: "", // "off" for OSS, "optional" for Enterprise.
						EKCertAllowedCAs: []string{
							"this is not a pem encoded certificate for a CA",
						},
					},
				},
			},
			wantErr: "invalid EKCert allowed CAs entry",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.authPref.CheckAndSetDefaults()
			if test.wantErr == "" {
				assert.NoError(t, err, "CheckAndSetDefaults returned non-nil error")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "CheckAndSetDefaults mismatch")
				assert.True(t, trace.IsBadParameter(err), "gotErr is not a trace.BadParameter error")
			}
		})
	}
}
