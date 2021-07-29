package integration

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/e/lib/pro/enforcer"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

const ClusterID = "foobar"

var _ = check.Suite(&IntSuite{})

func Test(t *testing.T) {
	check.TestingT(t)
}

type IntSuite struct {
	cancel      context.CancelFunc
	ctx         context.Context
	enforcer    *enforcer.Enforcer
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
	ctx := context.TODO()

	anonymizer, err := utils.NewHMACAnonymizer(ClusterID)
	c.Assert(err, check.IsNil)

	backend, err := lite.NewWithConfig(s.ctx, lite.Config{
		Path: c.MkDir(),
	})
	c.Assert(err, check.IsNil)

	server := &types.ServerV2{
		Metadata: types.Metadata{
			Name: "server",
		},
		Kind:    types.KindNode,
		Version: types.V2,
	}
	server.SetNamespace(ClusterID)

	namespace := &types.Namespace{}
	namespace.SetName(ClusterID)

	presence := local.NewPresenceService(backend)
	err = presence.UpsertNamespace(*namespace)
	c.Assert(err, check.IsNil)
	_, err = presence.UpsertNode(ctx, server)
	c.Assert(err, check.IsNil)

	s.enforcer, err = enforcer.New(s.ctx, enforcer.Config{
		Anonymizer:     anonymizer,
		Backend:        backend,
		ClusterID:      ClusterID,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		Insecure:       true,
		NoStart:        true,
	})
	c.Assert(err, check.IsNil)
}

// TestReporting checks that the usage duration is getting reset if Teleport
// can successfully contact Houston.
func (s *IntSuite) TestReporting(c *check.C) {
	targetHost := "localhost:10000"
	_, err := net.Dial("tcp", targetHost)
	if err != nil {
		c.Skip("Warning: %v is not up, skipping test")
	}
	// Make sure we're using the correct Houston endpoint
	os.Setenv(constants.APIHostEnvVar, targetHost)
	s.init(c)
	s.enforcer.RecordUsage(s.ctx, 30*time.Minute)

	err = s.enforcer.ReportUsage(s.ctx)
	c.Assert(err, check.IsNil)

	record, err := s.enforcer.GetUsageRecord(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(record), check.Equals, 0)
}

// TestFailedReporting checks that the usage duration is being retained if
// Teleport can't reach Houston.
func (s *IntSuite) TestFailedReporting(c *check.C) {
	// Set the api host to something where Houston isn't running on
	os.Setenv(constants.APIHostEnvVar, "test.localhost:5000")
	s.init(c)
	s.enforcer.RecordUsage(s.ctx, 30*time.Minute)

	err := s.enforcer.ReportUsage(s.ctx)
	c.Assert(err, check.NotNil)

	record, err := s.enforcer.GetUsageRecord(s.ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(record), check.Not(check.Equals), 0)
}
