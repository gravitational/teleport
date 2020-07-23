/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/golang/protobuf/proto"
	. "gopkg.in/check.v1"
)

var _ = fmt.Printf

type AuthoritySuite struct {
}

var _ = Suite(&AuthoritySuite{})

func (s *AuthoritySuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

// TestAuthorityEquals verifies expected behavior of the
// CertAuthority.Equals method.
func (s *AuthoritySuite) TestAuthorityEquals(c *C) {
	var ca1 CertAuthority
	ca1 = NewTestCA(UserCA, "example.com")
	c.Assert(ca1.CheckAndSetDefaults(), IsNil)

	// verify Clone produces an equal copy
	ca2 := ca1.Clone()
	c.Assert(ca1.Equals(ca2), Equals, true)

	// verify changing resource spec member changes equality
	ca2.AddRole("some-role")
	c.Assert(ca1.Equals(ca2), Equals, false)

	// verify json marshaling does not break equality
	ser1, err := GetCertAuthorityMarshaler().MarshalCertAuthority(ca1)
	c.Assert(err, IsNil)
	ca3, err := GetCertAuthorityMarshaler().UnmarshalCertAuthority(ser1)
	c.Assert(err, IsNil)
	c.Assert(ca1.Equals(ca3), Equals, true)

	// verify name change breaks equality
	ca3.SetName("other.example.com")
	c.Assert(ca1.Equals(ca3), Equals, false)

	// verify protobuf marshaling does not break equality
	ser2, err := proto.Marshal(ca1.(*CertAuthorityV2))
	c.Assert(err, IsNil)
	var ca4 CertAuthorityV2
	c.Assert(proto.Unmarshal(ser2, &ca4), IsNil)
	c.Assert(ca1.Equals(&ca4), Equals, true)

	// verify that public variants don't equate to their private variants, but that
	// two equal private variants produce equal public variants.
	ca1p, ca4p := ca1.WithoutSecrets().(CertAuthority), ca4.WithoutSecrets().(CertAuthority)
	c.Assert(ca1p.Equals(ca1), Equals, false)
	c.Assert(ca1p.Equals(ca4p), Equals, true)
}
