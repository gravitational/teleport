package pro

import (
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"

	check "gopkg.in/check.v1"
)

type LicenseLegacySuite struct {
}

var _ = check.Suite(&LicenseLegacySuite{})

func (s *LicenseLegacySuite) TestLicenseLegacyEnterprise(c *check.C) {
	licenseKeyPair, license, err := parseLicense([]byte(fixtures.TestLegacyEntepriseLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(licenseKeyPair, check.NotNil)
	c.Assert(license, check.NotNil)
	c.Assert(license.GetName(), check.Equals, constants.EnterprisePlan)
	c.Assert(license.Expiry(), check.Equals, time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC))
	c.Assert(license.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(license.GetReportsUsage(), check.Equals, services.NewBool(false))
	c.Assert(license.GetAWSAccountID(), check.Equals, "")
}
func (s *LicenseLegacySuite) TestLicenseLegacyPro(c *check.C) {
	licenseKeyPair, license, err := parseLicense([]byte(fixtures.TestLegacyProLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(licenseKeyPair, check.NotNil)
	c.Assert(license, check.NotNil)
	c.Assert(license.GetName(), check.Equals, constants.ProPlan)
	c.Assert(license.Expiry(), check.Equals, time.Date(2117, 12, 17, 1, 5, 54, 796964194, time.UTC))
	c.Assert(license.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(license.GetReportsUsage(), check.Equals, services.NewBool(true))
	c.Assert(license.GetAWSAccountID(), check.Equals, "")
}

func (s *LicenseLegacySuite) TestLicenseLegacyAWS(c *check.C) {
	licenseKeyPair, license, err := parseLicense([]byte(fixtures.TestLegacyAWSLicensePEM))
	c.Assert(err, check.IsNil)
	c.Assert(licenseKeyPair, check.NotNil)
	c.Assert(license, check.NotNil)
	c.Assert(license.GetName(), check.Equals, constants.EnterpriseAWSPlan)
	c.Assert(license.Expiry(), check.Equals, time.Date(2118, 3, 9, 20, 33, 13, 165754527, time.UTC))
	c.Assert(license.GetSupportsKubernetes(), check.Equals, services.NewBool(false))
	c.Assert(license.GetReportsUsage(), check.Equals, services.NewBool(false))
	c.Assert(license.GetAWSAccountID(), check.Equals, "126027368216")
}
