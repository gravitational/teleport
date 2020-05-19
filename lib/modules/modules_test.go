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

package modules

import (
	"testing"

	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"
	check "gopkg.in/check.v1"
)

func TestModules(t *testing.T) { check.TestingT(t) }

type ModulesSuite struct{}

var _ = check.Suite(&ModulesSuite{})

func (s *ModulesSuite) TestDefaultModules(c *check.C) {
	err := GetModules().EmptyRolesHandler()
	c.Assert(err, check.IsNil)

	logins := GetModules().DefaultAllowedLogins()
	c.Assert(logins, check.DeepEquals, []string{teleport.TraitInternalLoginsVariable})

	kubeGroups := GetModules().DefaultKubeGroups()
	c.Assert(kubeGroups, check.DeepEquals, []string{teleport.TraitInternalKubeGroupsVariable})

	kubeUsers := GetModules().DefaultKubeUsers()
	c.Assert(kubeUsers, check.DeepEquals, []string{teleport.TraitInternalKubeUsersVariable})

	roles := GetModules().RolesFromLogins([]string{"root"})
	c.Assert(roles, check.DeepEquals, []string{teleport.AdminRoleName})

	traits := GetModules().TraitsFromLogins([]string{"root"}, []string{"system:masters"}, []string{"alice@example.com"})
	c.Assert(traits, check.DeepEquals, map[string][]string{
		teleport.TraitLogins:     []string{"root"},
		teleport.TraitKubeGroups: []string{"system:masters"},
		teleport.TraitKubeUsers:  []string{"alice@example.com"},
	})

	isBoring := GetModules().IsBoringBinary()
	c.Assert(isBoring, check.Equals, false)
}

func (s *ModulesSuite) TestTestModules(c *check.C) {
	SetModules(&testModules{})

	err := GetModules().EmptyRolesHandler()
	c.Assert(trace.IsNotFound(err), check.Equals, true)

	logins := GetModules().DefaultAllowedLogins()
	c.Assert(logins, check.DeepEquals, []string{"a", "b"})

	roles := GetModules().RolesFromLogins([]string{"root"})
	c.Assert(roles, check.DeepEquals, []string{"root"})

	traits := GetModules().TraitsFromLogins([]string{"root"}, []string{"system:masters"}, []string{"alice@example.com"})
	c.Assert(traits, check.IsNil)

	isBoring := GetModules().IsBoringBinary()
	c.Assert(isBoring, check.Equals, true)
}

type testModules struct{}

func (p *testModules) EmptyRolesHandler() error {
	return trace.NotFound("no roles specified")
}

func (p *testModules) DefaultAllowedLogins() []string {
	return []string{"a", "b"}
}

func (p *testModules) DefaultKubeUsers() []string {
	return []string{"c", "d"}
}

func (p *testModules) DefaultKubeGroups() []string {
	return []string{"kube:test"}
}

func (p *testModules) SupportsKubernetes() bool {
	return true
}

func (p *testModules) PrintVersion() {}

func (p *testModules) RolesFromLogins(logins []string) []string {
	return logins
}

func (p *testModules) TraitsFromLogins(logins []string, kubeGroups []string, kubeUsers []string) map[string][]string {
	return nil
}

func (p *testModules) IsBoringBinary() bool {
	return true
}
