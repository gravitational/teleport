package process

import (
	"path/filepath"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/cloud"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/e/lib/web"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
)

// NewTeleport initializes a new Teleport Enterprise process
func NewTeleport(cfg *service.Config) (service.Process, error) {
	// Only the auth service requires a license. Load it first so it can
	// be used by auth plugins.
	var licenseFile *licensefile.LicenseFile
	if cfg.Auth.Enabled {
		var err error
		licenseFile, err = licensefile.NewLicenseFile(cfg.Auth.LicenseFile)
		if err != nil {
			cfg.Log.Debug(trace.DebugReport(err))
			return nil, trace.AccessDenied("auth server requires a valid license file to start, "+
				"please set the correct license_file path under auth_service section "+
				"in your teleport config or put the license into the default search "+
				"location at %v", filepath.Join(cfg.DataDir, defaults.LicenseFile))
		}

		emodules.SetModules(licenseFile.License)
		cfg.Log.Infof("Using license from %v %v.", cfg.Auth.LicenseFile, licenseFile.License)
	}

	pluginRegistry := plugin.NewRegistry()

	// Init plugins
	webPlugin, err := web.NewPlugin(web.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Capture process in a variable so plugins can access its backend after it
	// is initialized.
	var enterpriseProcess interface{ GetBackend() backend.Backend }

	authPlugin, err := auth.NewPlugin(auth.Config{
		License: licenseFile,
		GetBackend: func() backend.Backend {
			if enterpriseProcess == nil {
				panic("Failed to acquire backend, Teleport process is nil")
			}
			return enterpriseProcess.GetBackend()
		},
	})
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

	ossProcess, err := service.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Initialize the IdP.
	if cfg.Proxy.IdP.SAMLIdP.Enabled && cfg.Proxy.Enabled && !cfg.Proxy.DisableWebService {
		// We have to wait for the proxy to be finished before we can initiate the SAML IdP.
		ossProcess.RegisterFunc("saml-idp", func() error {
			_, err := ossProcess.WaitForEvent(ossProcess.ExitContext(), service.ProxyWebServerReady)
			if err != nil {
				return trace.Wrap(err)
			}
			return trace.Wrap(initSAMLIdP(ossProcess.ExitContext(), cfg, webPlugin))
		})
	}

	// Only the auth service has extensions, so we're done now if not running an auth server
	if !cfg.Auth.Enabled {
		return ossProcess, nil
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

		enterpriseProcess = cloudProcess

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

	enterpriseProcess = proProcess

	return proProcess, nil
}
