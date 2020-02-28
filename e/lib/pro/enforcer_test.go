package pro

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/reporting/types"
	check "gopkg.in/check.v1"
)

func TestPro(t *testing.T) { check.TestingT(t) }

type EnforcerSuite struct {
	enforcer *Enforcer
}

var _ = check.Suite(&EnforcerSuite{})

func (s *EnforcerSuite) SetUpSuite(c *check.C) {
	directory := c.MkDir()

	backend, err := lite.NewWithConfig(context.TODO(), lite.Config{Path: directory})
	c.Assert(err, check.IsNil)

	clusterID := "test"
	anonymizer, err := utils.NewHMACAnonymizer(clusterID)
	c.Assert(err, check.IsNil)

	s.enforcer, err = NewEnforcer(context.Background(), EnforcerConfig{
		Backend:        backend,
		LicenseKeyPair: fixtures.TestLicenseKeyPair(c),
		NoStart:        true,
		Anonymizer:     anonymizer,
		ClusterID:      clusterID,
	})
	c.Assert(err, check.IsNil)
}

func (s *EnforcerSuite) TestEnforcer(c *check.C) {
	h := types.NewHeartbeat(
		types.Notification{
			Type:     types.NotificationUsage,
			Severity: types.SeverityWarning,
			Text:     "Usage limit exceeded",
			HTML:     "<div>Usage limit exceeded</div>",
		},
		types.Notification{
			Type:     types.NotificationTerms,
			Severity: types.SeverityError,
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
	h := types.NewHeartbeat()
	h.Metadata.Created = time.Now().Add(-constants.MaxControlPlaneUnreachableDuration - time.Hour)

	err := s.enforcer.SetLicenseCheckHeartbeat(*h)
	c.Assert(err, check.IsNil)

	res, err := s.enforcer.GetLicenseCheckResult(context.TODO())
	c.Assert(err, check.IsNil)
	c.Assert(len(res.Spec.Notifications), check.Equals, 1)
}
