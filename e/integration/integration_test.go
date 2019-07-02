package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/lib/backend/lite"

	"gopkg.in/check.v1"
)

var _ = check.Suite(&IntSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type IntSuite struct {
	cancel      context.CancelFunc
	ctx         context.Context
	enforcer    *pro.Enforcer
	houstonHost string
}

func (s *IntSuite) SetUpSuite(c *check.C) {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.houstonHost = os.Getenv(constants.APIHostEnvVar)
}

func (s *IntSuite) TearDownSuite(c *check.C) {
	os.Setenv(constants.APIHostEnvVar, s.houstonHost)
	s.cancel()
}

// We use a custom init function before a test because we might need to run
// custom control logic before the initialization of an "enforcer"
func (s *IntSuite) init(c *check.C) {
	backend, err := lite.NewWithConfig(s.ctx, lite.Config{
		Path:  c.MkDir(),
	})
	c.Assert(err, check.IsNil)

	s.enforcer, err = pro.NewEnforcer(s.ctx, pro.EnforcerConfig{
		Backend:        backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		Insecure:       true,
		NoStart:        true,
	})
	c.Assert(err, check.IsNil)
}

// TestReporting checks that the usage duration is getting reset if Teleport
// can successfully contact Houston.
func (s *IntSuite) TestReporting(c *check.C) {
	// Make sure we're using the correct Houston endpoint
	os.Setenv(constants.APIHostEnvVar, "localhost:10000")
	s.init(c)

	s.enforcer.RecordUsage(s.ctx, 30*time.Minute)
	s.enforcer.ReportUsage(s.ctx)

	duration, err := s.enforcer.GetUsageDuration(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(duration, check.Equals, 0*time.Second)
}

// TestFailedReporting checks that the usage duration is being retained if
// Teleport can't reach Houston.
func (s *IntSuite) TestFailedReporting(c *check.C) {
	// Set the api host to something where Houston isn't running on
	os.Setenv(constants.APIHostEnvVar, "test.localhost:5000")
	s.init(c)

	s.enforcer.RecordUsage(s.ctx, 30*time.Minute)
	s.enforcer.ReportUsage(s.ctx)

	duration, err := s.enforcer.GetUsageDuration(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(duration, check.Not(check.Equals), 0*time.Second)
}
