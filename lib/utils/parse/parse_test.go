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

package parse

import (
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type ParseSuite struct{}

var _ = check.Suite(&ParseSuite{})
var _ = fmt.Printf

func (s *ParseSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}
func (s *ParseSuite) TearDownSuite(c *check.C) {}
func (s *ParseSuite) SetUpTest(c *check.C)     {}
func (s *ParseSuite) TearDownTest(c *check.C)  {}

func (s *ParseSuite) TestIsRoleVariable(c *check.C) {
	var tests = []struct {
		inVariable        string
		outIsNotFound     bool
		outVariablePrefix string
		outVariableName   string
	}{
		// 0 - no curly bracket prefix
		{
			"external.foo}}",
			true,
			"",
			"",
		},
		// 1 - invalid syntax
		{
			`{{external.foo("bar")`,
			true,
			"",
			"",
		},
		// 2 - invalid sytnax
		{
			"{{internal.}}",
			true,
			"",
			"",
		},
		// 3 - invalid syntax
		{
			"{{external..foo}}",
			true,
			"",
			"",
		},
		// 4 - invalid syntax
		{
			"{{}}",
			true,
			"",
			"",
		},
		// 5 - invalid syntax
		{
			"{{internal.foo",
			true,
			"",
			"",
		},
		// 6 - invalid syntax
		{
			"{{internal.foo.bar}}",
			true,
			"",
			"",
		},
		// 7 - valid brackets
		{
			`{{internal["foo"]}}`,
			false,
			"internal",
			"foo",
		},
		// 8 - valid
		{
			"{{external.foo}}",
			false,
			"external",
			"foo",
		},
		// 9 - valid
		{
			"{{internal.bar}}",
			false,
			"internal",
			"bar",
		},
	}

	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		variablePrefix, variableName, err := IsRoleVariable(tt.inVariable)
		if tt.outIsNotFound {
			c.Assert(trace.IsNotFound(err), check.Equals, true, comment)
			continue
		}
		c.Assert(variablePrefix, check.Equals, tt.outVariablePrefix, comment)
		c.Assert(variableName, check.Equals, tt.outVariableName, comment)
	}
}
