package aws

import (
	"slices"
	"strings"
	"testing"

	check "gopkg.in/check.v1"

	"github.com/gravitational/teleport/e/lib/fixtures"
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
	isProductLicensed := slices.Contains(meta.MarketplaceProductCodes, "9x4pv56fe6h1gj8hejc5r6a3z")
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
