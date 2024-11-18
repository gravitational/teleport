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

	log := cfg.Logger.With("license_file", cfg.Auth.LicenseFile)

	ctx := context.Background()
	licenseFile, err := licensefile.NewLicenseFile(cfg.Auth.LicenseFile)
	if err != nil {
		log.DebugContext(ctx, "Failed to load license.", "error", err)
		return nil, trace.AccessDenied("Failed to load license file from %v: %v. "+
			"Please set the correct license_file path under the auth_service section in your config, "+
			"or put the license into the default search location at %v.",
			cfg.Auth.LicenseFile, err, filepath.Join(cfg.DataDir, defaults.LicenseFile))
	}

	if isLicenseDeprecated(licenseFile) {
		log.DebugContext(ctx, "tried to start auth server with a deprecated license")
		return nil, trace.AccessDenied(DeprecatedLicenseWarning)
	}

	log.InfoContext(ctx, "Successfully loaded license.", "license", licenseFile.License)
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

// isLicenseDeprecated checks if the given license is considered deprecated.
// It returns false for cloud licenses, which are not deprecated.
// For self-hosted enterprise licenses, it considers licenses with a validity
// of 4 years or more, issued before January 1, 2024, as deprecated.
func isLicenseDeprecated(license *licensefile.LicenseFile) bool {
	// Cloud licenses are not deprecated
	if license.License != nil && license.License.GetCloud() {
		return false
	}

	validityDuration := license.KeyPair.Cert.NotAfter.Sub(license.KeyPair.Cert.NotBefore)
	fourYears := time.Hour * ((24 * 365) + 1) * 4

	cutoffDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	isOldLicense := license.KeyPair.Cert.NotBefore.Before(cutoffDate)

	if validityDuration >= fourYears && isOldLicense {
		return true
	}
	return false
}
