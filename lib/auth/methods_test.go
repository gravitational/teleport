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

package auth

import (
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/suite"
	"github.com/gravitational/teleport/lib/utils"

	check "gopkg.in/check.v1"
)

type MethodsSuite struct{}

var _ = check.Suite(&MethodsSuite{})

func (s *MethodsSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

// TestCheckPublicKeys tests checking of SSH and TLS certificates
func (s *MethodsSuite) TestCheckPublicKeys(c *check.C) {

	// same public keys match
	ca1 := suite.NewTestCA(services.HostCA, "localhost")
	err := CheckPublicKeysEqual(ca1.GetCheckingKeys()[0], ca1.GetTLSKeyPairs()[0].Cert)
	c.Assert(err, check.IsNil)

	ca2 := suite.NewTestCA(services.HostCA, "other")
	err = CheckPublicKeysEqual(ca1.GetCheckingKeys()[0], ca2.GetTLSKeyPairs()[0].Cert)
	c.Assert(err, check.IsNil)

	// different public keys don't match
	ca3 := suite.NewTestCA(services.HostCA, "localhost", fixtures.PEMBytes["rsa2"])
	err = CheckPublicKeysEqual(ca1.GetCheckingKeys()[0], ca3.GetTLSKeyPairs()[0].Cert)
	fixtures.ExpectCompareFailed(c, err)
}
