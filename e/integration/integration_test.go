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

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&IntSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type IntSuite struct {
	clock       clockwork.FakeClock
	cancel      context.CancelFunc
	ctx         context.Context
	enforcer    *pro.Enforcer
	houstonHost string
}

func (s *IntSuite) SetUpSuite(c *check.C) {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.clock = clockwork.NewFakeClock()
	s.houstonHost = os.Getenv(constants.ApiHostEnvVar)
}

func (s *IntSuite) TearDownSuite(c *check.C) {
	os.Setenv(constants.ApiHostEnvVar, s.houstonHost)
	s.cancel()
}

// We use a custom init function before a test because we might need to run
// custom control logic before the initialization of an "enforcer"
func (s *IntSuite) init(c *check.C) {
	backend, err := lite.NewWithConfig(s.ctx, lite.Config{
		Clock: s.clock,
		Path:  c.MkDir(),
	})
	c.Assert(err, check.IsNil)

	s.enforcer, err = pro.NewEnforcer(s.ctx, pro.EnforcerConfig{
		Backend:        backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		Insecure:       true,
	})
	c.Assert(err, check.IsNil)

	s.clock.BlockUntil(2)
}

// advance advances the fake clock until the "reporting interval" is reached.
// We have to do this in increments of "heartbeats" to imitate the real world
// usage.
func (s *IntSuite) advance() {
	intervals := int(constants.ReportingInterval / constants.HeartbeatInterval)
	for ; intervals != -1; intervals-- {
		s.clock.Advance(constants.HeartbeatInterval)

		// The next statement is a bit unfortunate. Ideally, we would
		// use clock.BlockUntil to block until the go routines are
		// waiting again, but this somehow doesn't work.
		// We need to wait until the usage is recorded in the backend
		// before we can query it.
		// The sleep duration of 30 milliseconds was found using trial
		// and error. This might behave differently on the CI system
		// though.
		time.Sleep(30 * time.Millisecond)
	}
}

// TestReporting checks that the usage duration is getting reset if Teleport
// can successfully contact Houston.
func (s *IntSuite) TestReporting(c *check.C) {
	// Make sure we're using the correct Houston endpoint
	os.Setenv(constants.ApiHostEnvVar, "localhost:10000")

	s.init(c)
	s.advance()

	duration, err := s.enforcer.GetUsageDuration(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(duration, check.Equals, 0*time.Second)
}

// TestFailedReporting checks that the usage duration is being retained if
// Teleport can't reach Houston.
func (s *IntSuite) TestFailedReporting(c *check.C) {
	// Set the api host so something where Houston isn't running on
	os.Setenv(constants.ApiHostEnvVar, "test.localhost:5000")

	s.init(c)
	s.advance()

	duration, err := s.enforcer.GetUsageDuration(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(duration, check.Not(check.Equals), 0*time.Second)
}
