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
package utils

import (
	"strings"
	"testing"

	"gopkg.in/check.v1"
)

func TestValidators(t *testing.T) { check.TestingT(t) }

type ValidatorsSuite struct {
}

var _ = check.Suite(&ValidatorsSuite{})

func (s *ValidatorsSuite) TestUnixUsers(c *check.C) {
	// good users
	c.Assert(IsValidUnixUser("user"), check.Equals, true)

	// bad users:
	c.Assert(IsValidUnixUser("User"), check.Equals, false)
	c.Assert(IsValidUnixUser(""), check.Equals, false)
	c.Assert(IsValidUnixUser("*()"), check.Equals, false)
	longName := strings.Repeat("a", 33)
	c.Assert(IsValidUnixUser(longName), check.Equals, false)

}

func (s *ValidatorsSuite) TestDomains(c *check.C) {
	badDomains := []string{
		"",
		strings.Repeat("a", 256),
		"((*23*",
		"one..two",
	}
	for _, d := range badDomains {
		c.Assert(IsValidDomainName(d), check.Equals, false)
	}
	goodDomains := []string{
		"one",
		"one.two.three",
		strings.Repeat("a", 60) + ".com",
	}
	for _, d := range goodDomains {
		c.Assert(IsValidDomainName(d), check.Equals, true)
	}
}
