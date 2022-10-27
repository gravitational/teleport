package licensefile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/fixtures"
)

func TestLicenseLegacyEnterprise(t *testing.T) {
	licenseFile, err := FromPEM([]byte(fixtures.TestLegacyEntepriseLicensePEM))
	require.Nil(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, license.GetName(), constants.EnterprisePlan)
	require.Equal(t, license.Expiry(), time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC))
	require.Equal(t, license.Expiry(), time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC))
	require.Equal(t, license.GetSupportsKubernetes(), types.NewBool(false))
	require.Equal(t, license.GetReportsUsage(), types.NewBool(false))
	require.Equal(t, license.GetAWSAccountID(), "")
}
func TestLicenseLegacyPro(t *testing.T) {
	licenseFile, err := FromPEM([]byte(fixtures.TestLegacyProLicensePEM))
	require.Nil(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, license.GetName(), constants.ProPlan)
	require.Equal(t, license.Expiry(), time.Date(2117, 12, 17, 1, 5, 54, 796964194, time.UTC))
	require.Equal(t, license.GetSupportsKubernetes(), types.NewBool(false))
	require.Equal(t, license.GetReportsUsage(), types.NewBool(true))
	require.Equal(t, license.GetAWSAccountID(), "")
}

func TestLicenseLegacyAWS(t *testing.T) {
	licenseFile, err := FromPEM([]byte(fixtures.TestLegacyAWSLicensePEM))
	require.Nil(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.Nil(t, err)
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, license.GetName(), constants.EnterpriseAWSPlan)
	require.Equal(t, license.Expiry(), time.Date(2118, 3, 9, 20, 33, 13, 165754527, time.UTC))
	require.Equal(t, license.GetSupportsKubernetes(), types.NewBool(false))
	require.Equal(t, license.GetReportsUsage(), types.NewBool(false))
	require.Equal(t, license.GetAWSAccountID(), "126027368216")
}
