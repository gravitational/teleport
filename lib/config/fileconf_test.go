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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthenticationSection(t *testing.T) {
	tests := []struct {
		comment                 string
		inConfigString          string
		outAuthenticationConfig *AuthenticationConfig
	}{
		// 0 - local with otp
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
		// 1 - local auth without otp
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
		// 2 - local auth with u2f
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
`,
			&AuthenticationConfig{
				Type:         "local",
				SecondFactor: "u2f",
				U2F: &UniversalSecondFactor{
					AppID: "https://graviton:3080",
					Facets: []string{
						"https://graviton:3080",
					},
				},
			},
		},
	}

	// run tests
	for _, tt := range tests {
		comment := fmt.Sprintf("Test %s", tt.comment)
		encodedConfigString := base64.StdEncoding.EncodeToString([]byte(tt.inConfigString))

		fc, err := ReadFromString(encodedConfigString)
		require.NoError(t, err, comment)
		require.Equal(t, fc.Auth.Authentication, tt.outAuthenticationConfig, comment)
	}
}

// TestLegacySection ensures we continue to parse and correctly load deprecated
// OIDC connector and U2F authentication configuration.
func TestLegacyAuthenticationSection(t *testing.T) {
	encodedLegacyAuthenticationSection := base64.StdEncoding.EncodeToString([]byte(LegacyAuthenticationSection))

	// read config into struct
	fc, err := ReadFromString(encodedLegacyAuthenticationSection)
	require.NoError(t, err)

	// validate oidc connector and u2f
	require.Equal(t, fc.Auth, Auth{
		Service: Service{
			defaultEnabled: true,
		},
		OIDCConnectors: []OIDCConnector{{
			ID:           "google",
			RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
			ClientID:     "id-from-google.apps.googleusercontent.com",
			ClientSecret: "secret-key-from-google",
			IssuerURL:    "https://accounts.google.com",
		}},
		U2F: U2F{
			AppID:  "https://graviton:3080",
			Facets: []string{"https://graviton:3080"},
		},
	})
}
