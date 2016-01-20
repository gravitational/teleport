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
package native

import (
	"testing"

	"github.com/gravitational/teleport/lib/auth/test"

	. "gopkg.in/check.v1"
)

func TestNative(t *testing.T) { TestingT(t) }

type NativeSuite struct {
	suite *test.AuthSuite
}

var _ = Suite(&NativeSuite{})

func (s *NativeSuite) SetUpSuite(c *C) {
	s.suite = &test.AuthSuite{A: New()}
}

func (s *NativeSuite) TestGenerateKeypairEmptyPass(c *C) {
	s.suite.GenerateKeypairEmptyPass(c)
}

func (s *NativeSuite) TestGenerateKeypairPass(c *C) {
	s.suite.GenerateKeypairPass(c)
}

func (s *NativeSuite) TestGenerateHostCert(c *C) {
	s.suite.GenerateHostCert(c)
}

func (s *NativeSuite) TestGenerateUserCert(c *C) {
	s.suite.GenerateUserCert(c)
}
