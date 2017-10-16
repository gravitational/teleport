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

package plugins

import (
	"testing"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	check "gopkg.in/check.v1"
)

func TestPlugins(t *testing.T) { check.TestingT(t) }

type PluginsSuite struct{}

var _ = check.Suite(&PluginsSuite{})

func (s *PluginsSuite) TestDefaultPlugins(c *check.C) {
	err := GetPlugins().EmptyRolesHandler()
	c.Assert(err, check.IsNil)

	logins := GetPlugins().DefaultAllowedLogins()
	c.Assert(logins, check.DeepEquals, []string{teleport.TraitInternalRoleVariable})
}

func (s *PluginsSuite) TestTestPlugins(c *check.C) {
	SetPlugins(&testPlugins{})

	err := GetPlugins().EmptyRolesHandler()
	c.Assert(trace.IsNotFound(err), check.Equals, true)

	logins := GetPlugins().DefaultAllowedLogins()
	c.Assert(logins, check.DeepEquals, []string{"a", "b"})
}

type testPlugins struct{}

func (p *testPlugins) EmptyRolesHandler() error {
	return trace.NotFound("no roles specified")
}

func (p *testPlugins) DefaultAllowedLogins() []string {
	return []string{"a", "b"}
}

func (p *testPlugins) PrintVersion() {}
