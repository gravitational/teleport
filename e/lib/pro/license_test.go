package pro

import (
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	check "gopkg.in/check.v1"
)

type LicenseSuite struct {
	enforcer *Enforcer
}

var _ = check.Suite(&LicenseSuite{})

func (s *LicenseSuite) TestLicenseLegacyEnterprise(c *check.C) {
	license, flags, err := parseLicense([]byte(fixtures.TestLegacyEntepriseLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(license, check.NotNil)
	c.Assert(flags, check.NotNil)
	c.Assert(flags.GetName(), check.Equals, constants.EnterprisePlan)
	c.Assert(flags.Expiry(), check.Equals, time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC))
	c.Assert(flags.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(flags.GetReportsUsage(), check.Equals, services.NewBool(false))
	c.Assert(flags.GetAWSAccountID(), check.Equals, "")
}

func (s *LicenseSuite) TestLicenseLegacyPro(c *check.C) {
	license, flags, err := parseLicense([]byte(fixtures.TestLegacyProLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(license, check.NotNil)
	c.Assert(flags, check.NotNil)
	c.Assert(flags.GetName(), check.Equals, constants.ProPlan)
	c.Assert(flags.Expiry(), check.Equals, time.Date(2117, 12, 17, 1, 5, 54, 796964194, time.UTC))
	c.Assert(flags.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(flags.GetReportsUsage(), check.Equals, services.NewBool(true))
	c.Assert(flags.GetAWSAccountID(), check.Equals, "")
}

func (s *LicenseSuite) TestLicenseLegacyAWS(c *check.C) {
	license, flags, err := parseLicense([]byte(fixtures.TestLegacyAWSLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(license, check.NotNil)
	c.Assert(flags, check.NotNil)
	c.Assert(flags.GetName(), check.Equals, constants.EnterpriseAWSPlan)
	c.Assert(flags.Expiry(), check.Equals, time.Date(2118, 3, 9, 20, 33, 13, 165754527, time.UTC))
	c.Assert(flags.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(flags.GetReportsUsage(), check.Equals, services.NewBool(false))
	c.Assert(flags.GetAWSAccountID(), check.Equals, "126027368216")
}
