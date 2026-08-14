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
	require.NoError(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, constants.EnterprisePlan, license.GetName())
	require.Equal(t, time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC), license.Expiry())
	require.Equal(t, time.Date(2117, 11, 5, 20, 10, 6, 805865795, time.UTC), license.Expiry())
	require.Equal(t, types.NewBool(false), license.GetSupportsKubernetes())
	require.Equal(t, types.NewBool(false), license.GetReportsUsage())
	require.Empty(t, license.GetAWSAccountID())
}
func TestLicenseLegacyPro(t *testing.T) {
	licenseFile, err := FromPEM([]byte(fixtures.TestLegacyProLicensePEM))
	require.NoError(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, constants.ProPlan, license.GetName())
	require.Equal(t, time.Date(2117, 12, 17, 1, 5, 54, 796964194, time.UTC), license.Expiry())
	require.Equal(t, types.NewBool(false), license.GetSupportsKubernetes())
	require.Equal(t, types.NewBool(true), license.GetReportsUsage())
	require.Empty(t, license.GetAWSAccountID())
}

func TestLicenseLegacyAWS(t *testing.T) {
	licenseFile, err := FromPEM([]byte(fixtures.TestLegacyAWSLicensePEM))
	require.NoError(t, err)

	license := licenseFile.License
	licenseKeyPair := licenseFile.KeyPair
	require.NoError(t, err)
	require.NotNil(t, licenseKeyPair)
	require.NotNil(t, license)
	require.Equal(t, constants.EnterpriseAWSPlan, license.GetName())
	require.Equal(t, time.Date(2118, 3, 9, 20, 33, 13, 165754527, time.UTC), license.Expiry())
	require.Equal(t, types.NewBool(false), license.GetSupportsKubernetes())
	require.Equal(t, types.NewBool(false), license.GetReportsUsage())
	require.Equal(t, "126027368216", license.GetAWSAccountID())
}
