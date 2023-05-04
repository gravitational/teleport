/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package process

import (
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

	licenseFile, err := licensefile.NewLicenseFile(cfg.Auth.LicenseFile)
	if err != nil {
		cfg.Log.Debug(trace.DebugReport(err))
		return nil, trace.AccessDenied("auth server requires a valid license file to start, "+
			"please set the correct license_file path under auth_service section "+
			"in your teleport config or put the license into the default search "+
			"location at %v", filepath.Join(cfg.DataDir, defaults.LicenseFile))
	}

	cfg.Log.Infof("Using license from %v %v.", cfg.Auth.LicenseFile, licenseFile.License)
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
