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

func TestAuthPreferenceV2_CheckAndSetDefaults(t *testing.T) {
	t.Parallel()

	validU2FPref := &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         "local",
			SecondFactor: "u2f",
			U2F: &types.U2F{
				AppID: "https://localhost:3080",
				Facets: []string{
					"https://localhost:3080",
					"https://localhost",
					"localhost:3080",
					"localhost",
				},
			},
		},
	}

	tests := []struct {
		name  string
		prefs *types.AuthPreferenceV2
		// wantErr is a substring of the returned error
		wantErr string
		// wantCmp is an optional asserting function
		wantCmp func(got *types.AuthPreferenceV2) error
	}{
		{name: "ok", prefs: &types.AuthPreferenceV2{}},
		{
			name:  "u2f_valid",
			prefs: validU2FPref,
		},
		{
			name: "u2f_invalid_missingU2F",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "u2f",
				},
			},
			wantErr: "missing required U2F configuration",
		},
		{
			name: "webauthn_derivedFromU2F",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					U2F: &types.U2F{
						AppID:                "https://example.com:1234",
						Facets:               []string{"https://example.com:1234"},
						DeviceAttestationCAs: []string{yubicoU2FCA},
					},
					Webauthn: nil,
				},
			},
			wantCmp: func(got *types.AuthPreferenceV2) error {
				want := &types.Webauthn{
					RPID:                  "example.com",
					AttestationAllowedCAs: []string{yubicoU2FCA},
				}
				if diff := cmp.Diff(want, got.Spec.Webauthn); diff != "" {
					return fmt.Errorf("webauthn mismatch (-want +got):\n%s", diff)
				}
				return nil
			},
		},
		{
			name: "webauthn_derivedFromU2F_nonURL",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					U2F: &types.U2F{
						AppID:  "teleport", // "teleport" gets parsed as a Path, not a Host.
						Facets: []string{"teleport"},
					},
					Webauthn: nil,
				},
			},
			wantCmp: func(got *types.AuthPreferenceV2) error {
				want := &types.Webauthn{
					RPID: "teleport",
				}
				if diff := cmp.Diff(want, got.Spec.Webauthn); diff != "" {
					return fmt.Errorf("webauthn mismatch (-want +got):\n%s", diff)
				}
				return nil
			},
		},
		{
			name: "webauthn_valid_minimal",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					Webauthn: &types.Webauthn{
						RPID: "example.com",
					},
				},
			},
		},
		{
			name: "webauthn_valid_attestationCAs",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					Webauthn: &types.Webauthn{
						RPID:                  "example.com",
						AttestationAllowedCAs: []string{appleWebauthnRoot},
						AttestationDeniedCAs:  []string{yubicoU2FCA},
					},
				},
			},
		},
		{
			name: "webauthn_invalid_empty",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
				},
			},
			wantErr: "missing rp_id",
		},
		{
			name: "webauthn_invalid_invalidU2F",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					U2F:          &types.U2F{},
				},
			},
			wantErr: "missing app_id",
		},
		{
			name: "webauthn_invalid_attestationAllowedCA",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					Webauthn: &types.Webauthn{
						RPID:                  "example.com",
						AttestationAllowedCAs: []string{"bad inline cert"},
					},
				},
			},
			wantErr: "webauthn allowed CAs",
		},
		{
			name: "webauthn_invalid_attestationDeniedCA",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					Webauthn: &types.Webauthn{
						RPID:                 "example.com",
						AttestationDeniedCAs: []string{"bad inline cert"},
					},
				},
			},
			wantErr: "webauthn denied CAs",
		},
		{
			name: "webauthn_invalid_disabled",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "webauthn",
					Webauthn: &types.Webauthn{
						RPID:     "example.com",
						Disabled: true, // Cannot disable when second_factor=webauthn
					},
				},
			},
			wantErr: "webauthn cannot be disabled",
		},
		{
			name: "webauthn_valid_disabledSecondFactorOn",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "on",
					U2F:          validU2FPref.Spec.U2F,
					Webauthn: &types.Webauthn{
						RPID:     "example.com",
						Disabled: true, // OK, fallback to U2F
					},
				},
			},
		},
		{
			name: "webauthn_valid_disabledSecondFactorOptional",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "optional",
					U2F:          validU2FPref.Spec.U2F,
					Webauthn: &types.Webauthn{
						RPID:     "example.com",
						Disabled: true, // OK, fallback to U2F
					},
				},
			},
		},
		{
			name: "on_valid",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "on",
					U2F:          validU2FPref.Spec.U2F,
				},
			},
		},
		{
			name: "on_invalid_requiresU2F",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "on",
					U2F:          nil,
				},
			},
			wantErr: "missing required U2F configuration",
		},
		{
			name: "optional_valid",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "optional",
					U2F:          validU2FPref.Spec.U2F,
				},
			},
		},
		{
			name: "optional_invalid_requiresU2f",
			prefs: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         "local",
					SecondFactor: "optional",
					U2F:          nil,
				},
			},
			wantErr: "missing required U2F configuration",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.prefs.CheckAndSetDefaults()
			switch {
			case err == nil && test.wantErr == "": // OK
			case err == nil && test.wantErr != "":
				t.Fatalf("CheckAndSetDefaults = nil, want %q", test.wantErr)
			case !strings.Contains(err.Error(), test.wantErr) || test.wantErr == "":
				t.Fatalf("CheckAndSetDefaults = %q, want %q", err, test.wantErr)
			}

			if test.wantCmp == nil {
				return
			}
			if err := test.wantCmp(test.prefs); err != nil {
				t.Fatal(err)
			}
		})
	}
}
