package enforcer

import (
	"context"
	"testing"
	"time"

	reporting "github.com/gravitational/reporting/types"
	"github.com/jonboulle/clockwork"
	check "gopkg.in/check.v1"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

func TestPro(t *testing.T) { check.TestingT(t) }

type EnforcerSuite struct {
	enforcer *Enforcer
	clock    clockwork.FakeClock
}

var _ = check.Suite(&EnforcerSuite{})

func (s *EnforcerSuite) SetUpSuite(c *check.C) {
	ctx := context.TODO()

	directory := c.MkDir()
	s.clock = clockwork.NewFakeClock()

	backend, err := lite.NewWithConfig(context.TODO(), lite.Config{
		Clock:            s.clock,
		Path:             directory,
		PollStreamPeriod: 50 * time.Millisecond,
	})
	c.Assert(err, check.IsNil)

	clusterID := "test"
	anonymizer, err := utils.NewHMACAnonymizer(clusterID)
	c.Assert(err, check.IsNil)

	presence := local.NewPresenceService(backend)

	namespace := &types.Namespace{}
	namespace.SetName(apidefaults.Namespace)
	err = presence.UpsertNamespace(*namespace)
	c.Assert(err, check.IsNil)

	server := &types.ServerV2{
		Metadata: types.Metadata{Name: "foo"},
		Kind:     types.KindNode,
		Version:  types.V2,
	}
	_, err = presence.UpsertNode(ctx, server)
	c.Assert(err, check.IsNil)

	s.enforcer, err = New(context.Background(), Config{
		Backend:        backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		NoStart:        true,
		Anonymizer:     anonymizer,
		ClusterID:      clusterID,
	})
	c.Assert(err, check.IsNil)
}

func (s *EnforcerSuite) TestEnforcer(c *check.C) {
	h := reporting.NewHeartbeat(
		reporting.Notification{
			Type:     reporting.NotificationUsage,
			Severity: reporting.SeverityWarning,
			Text:     "Usage limit exceeded",
			HTML:     "<div>Usage limit exceeded</div>",
		},
		reporting.Notification{
			Type:     reporting.NotificationTerms,
			Severity: reporting.SeverityError,
			Text:     "Terms of service violation",
			HTML:     "<div>Terms of service violation</div>",
		})

	err := s.enforcer.SetLicenseCheckHeartbeat(*h)
	c.Assert(err, check.IsNil)

	res, err := s.enforcer.GetLicenseCheckResult(context.TODO())
	c.Assert(err, check.IsNil)
	c.Assert(res, check.DeepEquals, h)
}

func (s *EnforcerSuite) TestEnforcerUnreachable(c *check.C) {
	h := reporting.NewHeartbeat()
	h.Metadata.Created = time.Now().Add(-constants.MaxControlPlaneUnreachableDuration - time.Hour)

	err := s.enforcer.SetLicenseCheckHeartbeat(*h)
	c.Assert(err, check.IsNil)

	res, err := s.enforcer.GetLicenseCheckResult(context.TODO())
	c.Assert(err, check.IsNil)
	c.Assert(len(res.Spec.Notifications), check.Equals, 1)
}

// TestSequentialRecording checks that the basic usage record (using one auth instance) works as expected
func (s *EnforcerSuite) TestSequentialRecording(c *check.C) {
	// The SQLite backend has an asynchronous job that cleans expired records. Unfortunately, we cannot use a fake
	// ticker for that job, so we wait a bit until we can be somewhat sure that the job has run.
	advance := func() {
		s.clock.Advance(30 * time.Minute)
		time.Sleep(200 * time.Millisecond)
	}

	ctx := context.Background()
	s.enforcer.RecordUsage(ctx, 30*time.Minute)
	advance()
	s.enforcer.RecordUsage(ctx, 30*time.Minute)
	advance()
	s.enforcer.RecordUsage(ctx, 30*time.Minute)

	record, err := s.enforcer.GetUsageRecord(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(record), check.Equals, 1)

	for _, v := range record {
		c.Assert(v, check.Equals, 90*time.Minute)
	}
}

// TestConcurrentRecording checks that multiple teleport instances don't end up recording the same usage multiple times
// We emulate parallelism by not advancing the clock in-between the calls to `RecordUsage`
func (s *EnforcerSuite) TestConcurrentRecording(c *check.C) {
	ctx := context.Background()
	s.enforcer.RecordUsage(ctx, 30*time.Minute)
	s.enforcer.RecordUsage(ctx, 30*time.Minute)
	s.enforcer.RecordUsage(ctx, 30*time.Minute)

	record, err := s.enforcer.GetUsageRecord(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(len(record), check.Equals, 1)

	for _, v := range record {
		c.Assert(v, check.Equals, 30*time.Minute)
	}
}
