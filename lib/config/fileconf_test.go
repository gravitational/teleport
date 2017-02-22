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

	"gopkg.in/check.v1"
)

type FileTestSuite struct {
}

var _ = check.Suite(&FileTestSuite{})
var _ = fmt.Printf

func (s *FileTestSuite) SetUpSuite(c *check.C) {
}

func (s *FileTestSuite) TearDownSuite(c *check.C) {
}

func (s *FileTestSuite) SetUpTest(c *check.C) {
}

func (s *FileTestSuite) TestAuthenticationSection(c *check.C) {
	tests := []struct {
		inConfigString          string
		outAuthenticationConfig *AuthenticationConfig
	}{
		// 0 - local with otp
		{
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
		// 3 - oidc without second factor
		{
			`
auth_service:
  authentication:
    type: oidc
    oidc:
      id: google
      redirect_url: "https://localhost:3080/v1/webapi/oidc/callback"
      client_id: id-from-google.apps.googleusercontent.com
      client_secret: secret-key-from-google
      issuer_url: "https://accounts.google.com"
      display: whaterver
      scope: [ "ssh_permissions", "roles"]
      claims_to_roles:
        - claim: role
          value: admin
          roles: ["dba", "backup", "root"]
`,
			&AuthenticationConfig{
				Type: "oidc",
				OIDC: &OIDCConnector{
					ID:           "google",
					RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
					ClientID:     "id-from-google.apps.googleusercontent.com",
					ClientSecret: "secret-key-from-google",
					IssuerURL:    "https://accounts.google.com",
					Display:      "whaterver",
					Scope: []string{
						"ssh_permissions",
						"roles",
					},
					ClaimsToRoles: []ClaimMapping{
						ClaimMapping{
							Claim: "role",
							Value: "admin",
							Roles: []string{
								"dba",
								"backup",
								"root",
							},
						},
					},
				},
			},
		},
	}

	// run tests
	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		encodedConfigString := base64.StdEncoding.EncodeToString([]byte(tt.inConfigString))

		fc, err := ReadFromString(encodedConfigString)
		c.Assert(err, check.IsNil, comment)

		c.Assert(fc.Auth.Authentication, check.DeepEquals, tt.outAuthenticationConfig, comment)
	}
}

// TestLegacySection ensures we continue to parse and correctly load deprecated
// OIDC connector and U2F authentication configuration.
func (s *FileTestSuite) TestLegacyAuthenticationSection(c *check.C) {
	encodedLegacyAuthenticationSection := base64.StdEncoding.EncodeToString([]byte(LegacyAuthenticationSection))

	// read config into struct
	fc, err := ReadFromString(encodedLegacyAuthenticationSection)
	c.Assert(err, check.IsNil)

	// validate oidc connector
	c.Assert(fc.Auth.OIDCConnectors, check.HasLen, 1)
	c.Assert(fc.Auth.OIDCConnectors[0].ID, check.Equals, "google")
	c.Assert(fc.Auth.OIDCConnectors[0].RedirectURL, check.Equals, "https://localhost:3080/v1/webapi/oidc/callback")
	c.Assert(fc.Auth.OIDCConnectors[0].ClientID, check.Equals, "id-from-google.apps.googleusercontent.com")
	c.Assert(fc.Auth.OIDCConnectors[0].ClientSecret, check.Equals, "secret-key-from-google")
	c.Assert(fc.Auth.OIDCConnectors[0].IssuerURL, check.Equals, "https://accounts.google.com")

	// validate u2f
	c.Assert(fc.Auth.U2F.EnabledFlag, check.Equals, "yes")
	c.Assert(fc.Auth.U2F.AppID, check.Equals, "https://graviton:3080")
	c.Assert(fc.Auth.U2F.Facets, check.HasLen, 1)
	c.Assert(fc.Auth.U2F.Facets[0], check.Equals, "https://graviton:3080")
}
