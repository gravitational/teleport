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

	"github.com/gravitational/teleport/lib/utils"

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

	oc, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	c.Assert(err, check.IsNil)

	c.Assert(oc.GetName(), check.Equals, "google")
	c.Assert(oc.GetIssuerURL(), check.Equals, "https://accounts.google.com")
	c.Assert(oc.GetClientID(), check.Equals, "id-from-google.apps.googleusercontent.com")
	c.Assert(oc.GetRedirectURL(), check.Equals, "https://localhost:3080/v1/webapi/oidc/callback")
	c.Assert(oc.GetDisplay(), check.Equals, "whatever")
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

	_, err := GetOIDCConnectorMarshaler().UnmarshalOIDCConnector([]byte(input))
	c.Assert(err, check.NotNil)
}
