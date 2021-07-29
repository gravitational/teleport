package aws

import (
	"strings"
	"testing"

	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/fixtures"

	check "gopkg.in/check.v1"
)

func TestAPI(t *testing.T) { check.TestingT(t) }

type AWSSuite struct {
}

var _ = check.Suite(&AWSSuite{})

func (s *AWSSuite) TestVerifyOK(c *check.C) {
	meta, err := GetInstanceMetadata(func(uri string) ([]byte, error) {
		if strings.HasSuffix(uri, "document") {
			return []byte(fixtures.AWSDoc), nil
		}
		return []byte(fixtures.AWSSig), nil
	})
	c.Assert(err, check.IsNil)
	c.Assert(meta.AccountID, check.Equals, "126027368216")
	isProductLicensed := apiutils.SliceContainsStr(meta.MarketplaceProductCodes, "9x4pv56fe6h1gj8hejc5r6a3z")
	c.Assert(isProductLicensed, check.Equals, true)
}

func (s *AWSSuite) TestVerifyTampered(c *check.C) {
	_, err := GetInstanceMetadata(func(uri string) ([]byte, error) {
		if strings.HasSuffix(uri, "document") {
			return []byte(fixtures.AWSDoc + "tampered"), nil
		}
		return []byte(fixtures.AWSSig), nil
	})
	c.Assert(err, check.NotNil)
}
