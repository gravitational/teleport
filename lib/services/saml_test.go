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
	"strings"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"

	"gopkg.in/check.v1"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

type SAMLSuite struct{}

var _ = check.Suite(&SAMLSuite{})

func (s *SAMLSuite) TestParseFromMetadata(c *check.C) {
	input := fixtures.SAMLOktaConnectorV2

	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw UnknownResource
	err := decoder.Decode(&raw)
	c.Assert(err, check.IsNil)

	oc, err := UnmarshalSAMLConnector(raw.Raw)
	c.Assert(err, check.IsNil)
	err = ValidateSAMLConnector(oc)
	c.Assert(err, check.IsNil)
	c.Assert(oc.GetIssuer(), check.Equals, "http://www.okta.com/exkafftca6RqPVgyZ0h7")
	c.Assert(oc.GetSSO(), check.Equals, "https://dev-813354.oktapreview.com/app/gravitationaldev813354_teleportsaml_1/exkafftca6RqPVgyZ0h7/sso/saml")
	c.Assert(oc.GetAssertionConsumerService(), check.Equals, "https://localhost:3080/v1/webapi/saml/acs")
	c.Assert(oc.GetAudience(), check.Equals, "https://localhost:3080/v1/webapi/saml/acs")
	c.Assert(oc.GetSigningKeyPair(), check.NotNil)
	c.Assert(oc.GetAttributes(), check.DeepEquals, []string{"groups"})
}
