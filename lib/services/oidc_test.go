/*
Copyright 2017 Gravitational, Inc.

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
package services

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/coreos/go-oidc/jose"
	"gopkg.in/check.v1"
)

type OIDCSuite struct{}

var _ = check.Suite(&OIDCSuite{})
var _ = fmt.Printf

func (s *OIDCSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (s *OIDCSuite) TestUnmarshal(c *check.C) {
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
          "claims_to_roles": [{
            "claim": "roles",
            "value": "teleport-user",
            "role_template": {
              "kind": "role",
              "version": "v2",
              "metadata": {
                "name": "{{index . \"email\"}}",
                "namespace": "default"
              },
              "spec": {
                "namespaces": ["default"],
                "max_session_ttl": "90h0m0s",
                "logins": ["{{index . \"nickname\"}}", "root"],
                "node_labels": {
                  "*": "*"
                }
              }
            }
          }]
        }
      }
	`

	output := OIDCConnectorV2{
		Kind:    KindOIDCConnector,
		Version: V2,
		Metadata: Metadata{
			Name:      "google",
			Namespace: defaults.Namespace,
		},
		Spec: OIDCConnectorSpecV2{
			IssuerURL:    "https://accounts.google.com",
			ClientID:     "id-from-google.apps.googleusercontent.com",
			ClientSecret: "secret-key-from-google",
			RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
			Display:      "whatever",
			Scope:        []string{"roles"},
			ClaimsToRoles: []ClaimMapping{
				ClaimMapping{
					Claim: "roles",
					Value: "teleport-user",
					RoleTemplate: &RoleV2{
						Kind:    KindRole,
						Version: V2,
						Metadata: Metadata{
							Name:      `{{index . "email"}}`,
							Namespace: defaults.Namespace,
						},
						Spec: RoleSpecV2{
							Namespaces:    []string{"default"},
							MaxSessionTTL: NewDuration(90 * 60 * time.Minute),
							Logins:        []string{`{{index . "nickname"}}`, `root`},
							NodeLabels:    map[string]string{"*": "*"},
						},
					},
				},
			},
		},
	}

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	c.Assert(err, check.IsNil)
	c.Assert(oc, check.DeepEquals, &output)
}

func (s *OIDCSuite) TestUnmarshalInvalid(c *check.C) {
	input := `
      {
        "kind": "oidc",
        "version": "v2",
        "metadata": {
          "name": "google"
        },
        "spec": {
          "issuer_url": "https://accounts.google.com",
          "client_id": "id-from-google.apps.googleusercontent.com",
          "client_secret": "secret-key-from-google",
          "redirect_url": "https://localhost:3080/v1/webapi/oidc/callback",
          "display": "whatever",
          "scope": ["roles"],
          "claims_to_roles": [{
            "claim": "roles",
            "value": "teleport-user",
            "roles": [ "foo", "bar", "baz" ],
            "role_template": {
              "kind": "role",
              "version": "v2",
              "metadata": {
                "name": "{{index . \"email\"}}",
                "namespace": "default"
              },
              "spec": {
                "namespaces": ["default"],
                "max_session_ttl": "90h0m0s",
                "logins": ["{{index . \"nickname\"}}", "root"],
                "node_labels": {
                  "*": "*"
                }
              }
            }
          }]
        }
      }
	`

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	c.Assert(err, check.IsNil)
	err = oc.Check()
	c.Assert(err, check.NotNil)
}

// TestRoleFromTemplate checks that we can create a valid role from a template. Also
// makes sure missing fields are filled in.
func (s *OIDCSuite) TestRoleFromTemplate(c *check.C) {
	oidcConnector := OIDCConnectorV2{
		Kind:    KindOIDCConnector,
		Version: V2,
		Metadata: Metadata{
			Name:      "google",
			Namespace: defaults.Namespace,
		},
		Spec: OIDCConnectorSpecV2{
			IssuerURL:    "https://accounts.google.com",
			ClientID:     "id-from-google.apps.googleusercontent.com",
			ClientSecret: "secret-key-from-google",
			RedirectURL:  "https://localhost:3080/v1/webapi/oidc/callback",
			Display:      "whatever",
			Scope:        []string{"roles"},
			ClaimsToRoles: []ClaimMapping{
				ClaimMapping{
					Claim: "roles",
					Value: "teleport-user",
					RoleTemplate: &RoleV2{
						Kind:    KindRole,
						Version: V2,
						Metadata: Metadata{
							Name:      `{{index . "email"}}`,
							Namespace: defaults.Namespace,
						},
						Spec: RoleSpecV2{
							MaxSessionTTL: NewDuration(30 * 60 * time.Minute),
							Logins:        []string{`{{index . "nickname"}}`, `root`},
							NodeLabels:    map[string]string{"*": "*"},
						},
					},
				},
			},
		},
	}

	// create some claims
	var claims = make(jose.Claims)
	claims.Add("roles", "teleport-user")
	claims.Add("email", "foo@example.com")
	claims.Add("nickname", "foo")
	claims.Add("full_name", "foo bar")

	role, err := oidcConnector.RoleFromTemplate(claims)
	c.Assert(err, check.IsNil)

	outRole, err := NewRole("foo@example.com", RoleSpecV2{
		Logins:        []string{"foo", "root"},
		MaxSessionTTL: NewDuration(30 * time.Hour),
		NodeLabels:    map[string]string{"*": "*"},
		Namespaces:    []string{"default"},
		Resources:     nil,
		ForwardAgent:  false,
	})
	c.Assert(err, check.IsNil)
	c.Assert(role, check.DeepEquals, outRole)
}
