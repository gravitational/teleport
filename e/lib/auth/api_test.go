package auth

import (
	"context"
	"testing"

	reporting "github.com/gravitational/reporting/types"
	check "gopkg.in/check.v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/pro/enforcer"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAPI(t *testing.T) { check.TestingT(t) }

type APISuite struct {
	enforcer *enforcer.Enforcer
	server   *auth.TestTLSServer
}

var _ = check.Suite(&APISuite{})

func (s *APISuite) TearDownSuite(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *APISuite) SetUpSuite(c *check.C) {
	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: c.MkDir(),
	})
	c.Assert(err, check.IsNil)

	authPlugin, err := NewPlugin(Config{
		GetBackend: func() backend.Backend { return authServer.Backend },
	})
	c.Assert(err, check.IsNil)

	registry := plugin.NewRegistry()
	registry.Add(authPlugin)

	s.server, err = auth.NewTestTLSServer(auth.TestTLSServerConfig{
		APIConfig: &auth.APIConfig{
			PluginRegistry: registry,
			AuthServer:     authServer.AuthServer,
			Authorizer:     authServer.Authorizer,
			AuditLog:       authServer.AuditLog,
			Emitter:        authServer.AuditLog,
		},
		AuthServer:    authServer,
		AcceptedUsage: authServer.AcceptedUsage,
	})
	c.Assert(err, check.IsNil)

	clusterID := "test"
	anonymizer, err := utils.NewHMACAnonymizer(clusterID)
	c.Assert(err, check.IsNil)

	s.enforcer, err = enforcer.New(context.Background(), enforcer.Config{
		Backend:        s.server.AuthServer.Backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		Anonymizer:     anonymizer,
		NoStart:        true,
		ClusterID:      clusterID,
	})
	c.Assert(err, check.IsNil)

	authPlugin.EnableEnforcer(s.enforcer)
}

func (s *APISuite) TestHeartbeat(c *check.C) {
	authClient, err := s.server.NewClient(auth.TestBuiltin(types.RoleProxy))
	c.Assert(err, check.IsNil)

	client, err := NewProClient(authClient)
	c.Assert(err, check.IsNil)

	needed := reporting.NewHeartbeat()
	err = s.enforcer.SetLicenseCheckHeartbeat(*needed)
	c.Assert(err, check.IsNil)

	retrieved, err := client.GetLicenseCheckResult(context.Background())
	c.Assert(err, check.IsNil)
	c.Assert(retrieved, check.DeepEquals, needed)
}
