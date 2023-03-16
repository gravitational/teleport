package process

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

// NewTeleport initializes a new Teleport Enterprise process
func NewTeleport(cfg *servicecfg.Config) (service.Process, error) {
	license, err := configureLicense(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webPlugin, authPlugin, err := addPlugins(cfg, license)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ossProcess, err := service.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Proxy.Enabled && !cfg.Proxy.DisableWebService {
		if err := extendProxy(ossProcess, webPlugin); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// This needs to be the last thing in NewTeleport because it needs to
	// return an enterprise specific process.
	if cfg.Auth.Enabled {
		authProcess, err := extendAuthServer(ossProcess, license, authPlugin)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return authProcess, nil
	}

	return ossProcess, nil
}

func addPlugins(cfg *servicecfg.Config, license *licensefile.LicenseFile) (webPlugin *web.Plugin, authPlugin *auth.Plugin, err error) {
	pluginRegistry := plugin.NewRegistry()

	if cfg.Proxy.Enabled {
		webPlugin, err = web.NewPlugin(web.Config{})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if err := pluginRegistry.Add(webPlugin); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	if cfg.Auth.Enabled {
		authPlugin, err = auth.NewPlugin(auth.Config{
			License:       license,
			HostedPlugins: cfg.Auth.HostedPlugins,
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if err := pluginRegistry.Add(authPlugin); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	cfg.PluginRegistry = pluginRegistry

	return webPlugin, authPlugin, nil
}
