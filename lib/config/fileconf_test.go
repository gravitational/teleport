/*
Copyright 2015 Gravitational, Inc.

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

package config

import (
	"encoding/base64"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestAuthenticationSection(t *testing.T) {
	tests := []struct {
		comment                 string
		inConfigString          string
		outAuthenticationConfig *AuthenticationConfig
	}{
		{
			`0 - local with otp`,

			`
auth_service:
  authentication:
    type: local
    second_factor: otp
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "otp",
			},
		},
		{
			`1 - local auth without otp`,

			`
auth_service:
  authentication:
    type: local
    second_factor: off
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "off",
			},
		},
		{
			`2 - local auth with u2f`,

			`
auth_service:
   authentication:
       type: local
       second_factor: u2f
       u2f:
           app_id: https://graviton:3080
           facets:
           - https://graviton:3080
           device_attestation_cas:
           - testdata/u2f_attestation_ca.pam
           - |
             -----BEGIN CERTIFICATE-----
             fake certificate
             -----END CERTIFICATE-----
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "u2f",
				U2F: &UniversalSecondFactor{
					AppID: "https://graviton:3080",
					Facets: []string{
						"https://graviton:3080",
					},
					DeviceAttestationCAs: []string{
						"testdata/u2f_attestation_ca.pam",
						`-----BEGIN CERTIFICATE-----
fake certificate
-----END CERTIFICATE-----
`,
					},
				},
			},
		},
	}

	// run tests
	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			encodedConfigString := base64.StdEncoding.EncodeToString([]byte(tt.inConfigString))

			fc, err := ReadFromString(encodedConfigString)
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(fc.Auth.Authentication, tt.outAuthenticationConfig))
		})
	}
}
