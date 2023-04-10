package process

import (
	"net/url"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/db/oracle"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// PluginShimURLEnvVar is the environment variable name
// for the Cloud plugin shim URL.
// See lib/web/Plugin.*Config for more info.
const pluginShimURLEnvVar = "TELEPORT_PLUGIN_SHIM_URL"

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

	if cfg.Okta.Enabled {
		ossProcess.SetExpectedInstanceRole(types.RoleOkta, services.OktaIdentityEvent)
		services.InitOkta(ossProcess)
	}

	if cfg.Databases.Enabled {
		common.RegisterEngine(oracle.NewEngine, defaults.ProtocolOracle)
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
		var pluginShimURL *url.URL
		if urlVal := os.Getenv(pluginShimURLEnvVar); urlVal != "" {
			var err error
			pluginShimURL, err = url.Parse(urlVal)
			if err != nil {
				return nil, nil, trace.WrapWithMessage(err, "error parsing env var %v", pluginShimURLEnvVar)
			}
			if pluginShimURL.Scheme == "" {
				return nil, nil, trace.BadParameter("scheme must be present, but was missing in %q", pluginShimURL.String())
			}
		}

		webPlugin, err = web.NewPlugin(web.Config{
			PluginShimURL: pluginShimURL,
		})
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
