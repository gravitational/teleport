package auth

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/reporting/types"
	check "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { check.TestingT(t) }

type APISuite struct {
	apiServer *httptest.Server
	enforcer  *pro.Enforcer
}

var _ = check.Suite(&APISuite{})

func (s *APISuite) SetUpSuite(c *check.C) {
	directory := c.MkDir()

	backend, err := dir.New(backend.Params{"path": directory})
	c.Assert(err, check.IsNil)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "localhost",
	})
	c.Assert(err, check.IsNil)

	authServer, err := auth.NewAuthServer(&auth.InitConfig{
		Backend:     backend,
		ClusterName: clusterName,
	})
	c.Assert(err, check.IsNil)

	// set cluster config
	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: services.RecordAtNode,
	})
	c.Assert(err, check.IsNil)

	err = authServer.SetClusterConfig(clusterConfig)
	c.Assert(err, check.IsNil)

	err = authServer.SetClusterName(clusterName)
	c.Assert(err, check.IsNil)

	authorizer, err := auth.NewRoleAuthorizer(clusterName.GetName(), clusterConfig, teleport.RoleAdmin)
	c.Assert(err, check.IsNil)

	s.enforcer, err = pro.NewEnforcer(context.Background(), pro.EnforcerConfig{
		Backend: backend,
		License: fixtures.TestLicense(c),
		NoStart: true,
	})
	c.Assert(err, check.IsNil)

	InitPlugin()
	SetEnforcer(s.enforcer)

	apiServer := auth.NewAPIServer(&auth.APIConfig{
		AuthServer: authServer,
		Authorizer: authorizer,
	})
	s.apiServer = httptest.NewServer(apiServer)
}

func (s *APISuite) TestHeartbeat(c *check.C) {
	httpClient, err := auth.NewClient(s.apiServer.URL, nil)
	c.Assert(err, check.IsNil)

	client, err := NewClient(httpClient)
	c.Assert(err, check.IsNil)

	heartbeat := types.NewHeartbeat()

	err = s.enforcer.SetLicenseCheckHeartbeat(*heartbeat)
	c.Assert(err, check.IsNil)

	retrieved, err := client.GetLicenseCheckResult()
	c.Assert(err, check.IsNil)
	c.Assert(retrieved, check.DeepEquals, heartbeat)
}
