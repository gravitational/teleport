package constants

import (
	"os"
	"testing"

	check "gopkg.in/check.v1"
)

func TestConstants(t *testing.T) { check.TestingT(t) }

type ConstantSuite struct {
	houstonHost string
}

var _ = check.Suite(&ConstantSuite{})

func (s *ConstantSuite) SetUpTest(c *check.C) {
	s.houstonHost = os.Getenv(APIHostEnvVar)
}

func (s *ConstantSuite) TearDownTest(c *check.C) {
	if s.houstonHost != "" {
		os.Setenv(APIHostEnvVar, s.houstonHost)
	}
}

func (s *ConstantSuite) TestFunctions(c *check.C) {
	// test standard behavior
	c.Assert(GetControlPlaneAPIHost(), check.Equals, "dashboard-api.gravitational.com")
	c.Assert(GetControlPlaneAPIAddr(), check.Equals, "dashboard-api.gravitational.com:443")
	c.Assert(GetControlPlaneAPIURL(), check.Equals, "https://dashboard-api.gravitational.com:443/api")

	// test custom host+port behavior
	os.Setenv(APIHostEnvVar, "test.localhost:5000")

	c.Assert(GetControlPlaneAPIHost(), check.Equals, "test.localhost")
	c.Assert(GetControlPlaneAPIAddr(), check.Equals, "test.localhost:5000")
	c.Assert(GetControlPlaneAPIURL(), check.Equals, "https://test.localhost:5000/api")

	// test custom host (without port) behavior
	os.Setenv(APIHostEnvVar, "dev.localhost")

	c.Assert(GetControlPlaneAPIHost(), check.Equals, "dev.localhost")
	c.Assert(GetControlPlaneAPIAddr(), check.Equals, "dev.localhost:443")
	c.Assert(GetControlPlaneAPIURL(), check.Equals, "https://dev.localhost:443/api")
}
