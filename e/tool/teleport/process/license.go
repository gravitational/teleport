package process

import (
	"context"
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/licensefile"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

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
