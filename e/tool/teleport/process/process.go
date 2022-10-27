package process

import (
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/cloud"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
)

// NewTeleport initializes a new Teleport Enterprise process
func NewTeleport(cfg *service.Config) (service.Process, error) {
	pluginRegistry := plugin.NewRegistry()

	// Init plugins
	webPlugin, err := web.NewPlugin(web.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authPlugin, err := auth.NewPlugin(auth.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := pluginRegistry.Add(webPlugin); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := pluginRegistry.Add(authPlugin); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.PluginRegistry = pluginRegistry

	// Only auth service requires a license and has extensions
	if !cfg.Auth.Enabled {
		ossProcess, err := service.NewTeleport(cfg)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return ossProcess, nil
	}

	logger := cfg.Log
	licenseFile, err := licensefile.ReadAndActivate(cfg.Auth.LicenseFile)
	if err != nil {
		logger.Debug(trace.DebugReport(err))
		return nil, trace.AccessDenied("auth server requires a valid license file to start, "+
			"please set the correct license_file path under auth_service section "+
			"in your teleport config or put the license into the default search "+
			"location at %v", filepath.Join(cfg.DataDir, defaults.LicenseFile))
	}

	logger.Infof("Using license from %v %v.", cfg.Auth.LicenseFile, licenseFile.License)

	// Now when license is activated, initialize the OSS process
	ossProcess, err := service.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Initialize teleport cloud
	if modules.GetModules().Features().Cloud {
		cloudProcess, err := cloud.NewTeleport(cloud.Config{
			AuthPlugin:  authPlugin,
			OSSProcess:  ossProcess,
			LicenseFile: licenseFile,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return cloudProcess, nil
	}

	// Initialize teleport pro
	proProcess, err := pro.NewTeleport(pro.Config{
		AuthPlugin:  authPlugin,
		OSSProcess:  ossProcess,
		LicenseFile: licenseFile,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proProcess, nil
}
