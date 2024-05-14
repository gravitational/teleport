package process

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/licensefile"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

const DeprecatedLicenseWarning = `Error: Outdated License File

This Teleport Enterprise cluster is currently using an outdated license file. To resolve this issue, please follow these steps:

1. Navigate to https://teleport.sh/ to download your updated license file.
2. Refer to our documentation at https://goteleport.com/docs/choose-an-edition/teleport-enterprise/license/ for detailed instructions on updating your license.
3. If you have any questions or need assistance, please reach out to our support team at support@goteleport.com.

Thank you for using Teleport.`

// configureLicense reads in the license if the services configured for the process
// require it.
func configureLicense(cfg *servicecfg.Config) (*licensefile.LicenseFile, error) {
	if !servicesNeedLicense(cfg) {
		return nil, nil
	}

	ctx := context.Background()
	licenseFile, err := licensefile.NewLicenseFile(cfg.Auth.LicenseFile)
	if err != nil {
		cfg.Logger.DebugContext(ctx, "Failed to load license.", "error", err, "license_file", cfg.Auth.LicenseFile)
		return nil, trace.AccessDenied("auth server requires a valid license file to start, "+
			"please set the correct license_file path under auth_service section "+
			"in your teleport config or put the license into the default search "+
			"location at %v", filepath.Join(cfg.DataDir, defaults.LicenseFile))
	}

	if isLicenseDeprecated(licenseFile) {
		cfg.Logger.DebugContext(ctx, "tried to start auth server with a deprecated license", "license_file", cfg.Auth.LicenseFile)
		return nil, trace.AccessDenied(DeprecatedLicenseWarning)
	}

	cfg.Logger.InfoContext(ctx, "Successfully loaded license.", "license_file", cfg.Auth.LicenseFile, "license", licenseFile.License)
	return licenseFile, nil
}

// configureModules configures the modules based on the license
func configureModules(licenseFile *licensefile.LicenseFile) error {
	err := emodules.SetModules(licenseFile)
	if err != nil {
		return trace.Wrap(err, "error setting enterprise modules")
	}
	return nil
}

// servicesNeedLicense returns true if the configured services require a license.
func servicesNeedLicense(cfg *servicecfg.Config) bool {
	return cfg.Auth.Enabled
}

func isLicenseDeprecated(license *licensefile.LicenseFile) bool {
	// Cloud licenses are not deprecated
	if license.License != nil && license.License.GetCloud() {
		return false
	}

	// During a period, self-hosted enterprise lisences were generated as being valid for
	// long periods, sometimes for 100 years.
	// Since now all licenses are valid for up to 3 years, we consider
	// any license valid for 4 years or more to be deprecated.
	validityDuration := license.KeyPair.Cert.NotAfter.Sub(license.KeyPair.Cert.NotBefore)
	fourYears := time.Hour * ((24 * 365) + 1) * 4
	if validityDuration >= fourYears {
		return true
	}

	// Any license issued before Jan 1 2024 is deprecated
	cutoffDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return license.KeyPair.Cert.NotBefore.Before(cutoffDate)
}
