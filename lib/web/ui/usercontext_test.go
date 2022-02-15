// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ui

import (
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type UserContextSuite struct{}

var _ = check.Suite(&UserContextSuite{})

func TestUserContext(t *testing.T) { check.TestingT(t) }

func (s *UserContextSuite) TestNewUserContext(c *check.C) {
	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
	}

	// set some rules
	role1 := &types.RoleV5{}
	role1.SetNamespaces(types.Allow, []string{apidefaults.Namespace})
	role1.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{types.KindAuthConnector},
			Verbs:     services.RW(),
		},
		{
			Resources: []string{types.KindWindowsDesktop},
			Verbs:     services.RW(),
		},
	})

	// not setting the rule, or explicitly denying, both denies access
	role1.SetRules(types.Deny, []types.Rule{
		{
			Resources: []string{types.KindEvent},
			Verbs:     services.RW(),
		},
	})

	role2 := &types.RoleV5{}
	role2.SetNamespaces(types.Allow, []string{apidefaults.Namespace})
	role2.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{types.KindTrustedCluster},
			Verbs:     services.RW(),
		},
		{
			Resources: []string{types.KindBilling},
			Verbs:     services.RO(),
		},
	})

	// set some logins
	role1.SetLogins(types.Allow, []string{"a", "b"})
	role1.SetLogins(types.Deny, []string{"c"})
	role2.SetLogins(types.Allow, []string{"d"})

	// set some windows desktop logins
	role1.SetWindowsLogins(types.Allow, []string{"a", "b"})
	role1.SetWindowsLogins(types.Deny, []string{"c"})
	role2.SetWindowsLogins(types.Allow, []string{"d"})

	roleSet := []types.Role{role1, role2}
	userContext, err := NewUserContext(user, roleSet, proto.Features{})
	c.Assert(err, check.IsNil)

	allowed := access{true, true, true, true, true}
	denied := access{false, false, false, false, false}

	// test user name and acl
	c.Assert(userContext.Name, check.Equals, "root")
	c.Assert(userContext.ACL.AuthConnectors, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.TrustedClusters, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.AppServers, check.DeepEquals, denied)
	c.Assert(userContext.ACL.DBServers, check.DeepEquals, denied)
	c.Assert(userContext.ACL.KubeServers, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Events, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Sessions, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Roles, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Users, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Tokens, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Nodes, check.DeepEquals, denied)
	c.Assert(userContext.ACL.AccessRequests, check.DeepEquals, denied)
	c.Assert(userContext.ACL.Desktops, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.SSHLogins, check.DeepEquals, []string{"a", "b", "d"})
	c.Assert(userContext.ACL.WindowsLogins, check.DeepEquals, []string{"a", "b", "d"})
	c.Assert(userContext.AccessStrategy, check.DeepEquals, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	})
	c.Assert(userContext.ACL.Billing, check.DeepEquals, denied)

	// test local auth type
	c.Assert(userContext.AuthType, check.Equals, authLocal)

	// test sso auth type
	user.Spec.GithubIdentities = []types.ExternalIdentity{{ConnectorID: "foo", Username: "bar"}}
	userContext, err = NewUserContext(user, roleSet, proto.Features{})
	c.Assert(err, check.IsNil)
	c.Assert(userContext.AuthType, check.Equals, authSSO)

	userContext, err = NewUserContext(user, roleSet, proto.Features{Cloud: true})
	c.Assert(err, check.IsNil)
	c.Assert(userContext.ACL.Billing, check.DeepEquals, access{true, true, false, false, false})
}

func (s *UserContextSuite) TestNewUserContextCloud(c *check.C) {
	user := &types.UserV2{
		Metadata: types.Metadata{
			Name: "root",
		},
	}

	role := &types.RoleV5{}
	role.SetNamespaces(types.Allow, []string{"*"})
	role.SetRules(types.Allow, []types.Rule{
		{
			Resources: []string{"*"},
			Verbs:     services.RW(),
		},
	})

	role.SetLogins(types.Allow, []string{"a", "b"})
	role.SetLogins(types.Deny, []string{"c"})
	role.SetWindowsLogins(types.Allow, []string{"a", "b"})
	role.SetWindowsLogins(types.Deny, []string{"c"})

	roleSet := []types.Role{role}

	allowed := access{true, true, true, true, true}
	denied := access{false, false, false, false, false}

	userContext, err := NewUserContext(user, roleSet, proto.Features{Cloud: true})
	c.Assert(err, check.IsNil)

	c.Assert(userContext.Name, check.Equals, "root")
	c.Assert(userContext.ACL.AuthConnectors, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.TrustedClusters, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.AppServers, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.DBServers, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.KubeServers, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Events, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Sessions, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Roles, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Users, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Tokens, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Nodes, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.AccessRequests, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.SSHLogins, check.DeepEquals, []string{"a", "b"})
	c.Assert(userContext.ACL.WindowsLogins, check.DeepEquals, []string{"a", "b"})
	c.Assert(userContext.AccessStrategy, check.DeepEquals, accessStrategy{
		Type:   types.RequestStrategyOptional,
		Prompt: "",
	})

	// cloud-specific asserts
	c.Assert(userContext.ACL.Billing, check.DeepEquals, allowed)
	c.Assert(userContext.ACL.Desktops, check.DeepEquals, denied)
}
