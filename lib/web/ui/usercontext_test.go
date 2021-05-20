package ui

import (
	"testing"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
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
	role1 := &types.RoleV3{}
	role1.SetNamespaces(services.Allow, []string{defaults.Namespace})
	role1.SetRules(services.Allow, []types.Rule{
		{
			Resources: []string{services.KindAuthConnector},
			Verbs:     services.RW(),
		},
	})

	// not setting the rule, or explicitly denying, both denies access
	role1.SetRules(services.Deny, []types.Rule{
		{
			Resources: []string{services.KindEvent},
			Verbs:     services.RW(),
		},
	})

	role2 := &types.RoleV3{}
	role2.SetNamespaces(services.Allow, []string{defaults.Namespace})
	role2.SetRules(services.Allow, []types.Rule{
		{
			Resources: []string{services.KindTrustedCluster},
			Verbs:     services.RW(),
		},
		{
			Resources: []string{services.KindBilling},
			Verbs:     services.RO(),
		},
	})

	// set some logins
	role1.SetLogins(services.Allow, []string{"a", "b"})
	role1.SetLogins(services.Deny, []string{"c"})
	role2.SetLogins(services.Allow, []string{"d"})

	roleSet := []services.Role{role1, role2}
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
	c.Assert(userContext.ACL.SSHLogins, check.DeepEquals, []string{"a", "b", "d"})
	c.Assert(userContext.AccessStrategy, check.DeepEquals, accessStrategy{
		Type:   services.RequestStrategyOptional,
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
