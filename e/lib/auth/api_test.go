package auth

import (
	"context"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/reporting/types"
	check "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { check.TestingT(t) }

type APISuite struct {
	enforcer *pro.Enforcer
	server   *auth.TestTLSServer
	dataDir  string
}

var _ = check.Suite(&APISuite{})

func (s *APISuite) TearDownSuite(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *APISuite) SetUpSuite(c *check.C) {
	s.dataDir = c.MkDir()
	InitPlugin()

	testAuthServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir: s.dataDir,
	})
	c.Assert(err, check.IsNil)
	s.server, err = testAuthServer.NewTestTLSServer()
	c.Assert(err, check.IsNil)

	clusterID := "test"
	anonymizer, err := utils.NewHMACAnonymizer(clusterID)
	c.Assert(err, check.IsNil)

	s.enforcer, err = pro.NewEnforcer(context.Background(), pro.EnforcerConfig{
		Backend:        s.server.AuthServer.Backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		Anonymizer:     anonymizer,
		NoStart:        true,
		ClusterID:      clusterID,
	})
	c.Assert(err, check.IsNil)

	SetEnforcer(s.enforcer)
}

func (s *APISuite) TestHeartbeat(c *check.C) {
	authClient, err := s.server.NewClient(auth.TestBuiltin(teleport.RoleProxy))
	c.Assert(err, check.IsNil)

	client, err := NewClient(authClient)
	c.Assert(err, check.IsNil)

	heartbeat := types.NewHeartbeat()

	err = s.enforcer.SetLicenseCheckHeartbeat(*heartbeat)
	c.Assert(err, check.IsNil)

	retrieved, err := client.GetLicenseCheckResult()
	c.Assert(err, check.IsNil)
	c.Assert(retrieved, check.DeepEquals, heartbeat)
}
