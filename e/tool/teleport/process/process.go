package process

import (
	"context"
	"net/url"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/e/lib/auth"
	_ "github.com/gravitational/teleport/e/lib/backend/crdb"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/db/oracle"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/e/lib/web"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
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
	tryLoadingFeaturesFromBackend := false
	ctx := context.Background()

	license, err := configureLicense(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := configureModules(license); err != nil {
		// despite the error, we should still try to load features from the backend.
		// Features from the backend may be stale, so we should always prioritize
		// using configureModules, and only use the backend features as a fallback in case
		// some external service (like Cloud's API) is offline during the features fetching.
		cfg.Logger.WarnContext(ctx, "failed configuring cluster modules", "error", err)
		tryLoadingFeaturesFromBackend = cfg.Auth.Enabled
	}

	webPlugin, authPlugin, err := addPlugins(cfg, license)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// register enterprise specific expected services.
	registerExpectedServices(cfg)

	ossProcess, err := service.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if tryLoadingFeaturesFromBackend {
		cfg.Logger.InfoContext(ctx, "trying to read cluster features from the backend")
		f, err := feature.Load(ctx, ossProcess.GetBackend())
		if err != nil {
			return nil, trace.Wrap(err, "couldn't read or load the cluster features")
		}
		cfg.Logger.InfoContext(ctx, "successfully loaded features from backend", "features", f)
		modules.GetModules().SetFeatures(*f)
	}

	// store cloud features for future restarts
	if license != nil && license.License.GetCloud() {
		if _, err := feature.Store(ctx, modules.GetModules().Features(), ossProcess.GetBackend()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.Proxy.Enabled && !cfg.Proxy.DisableWebService {
		if err := extendProxy(ossProcess, webPlugin); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.Okta.Enabled {
		if err := services.InitOkta(ossProcess); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.Jamf.Enabled() {
		services.JamfStandaloneInit(ossProcess, nil /* httpClient */)
	}

	if cfg.Databases.Enabled {
		common.RegisterEngine(oracle.NewEngine, defaults.ProtocolOracle)
	}

	// This needs to be the last thing in NewTeleport because it needs to
	// return an enterprise specific process.
	if cfg.Auth.Enabled {
		authProcess, err := extendAuthServer(ossProcess, license, authPlugin, cfg.Auth.LicenseFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.AccessGraph.Enabled {
			if err := accessgraph.RegisterAccessGraphService(cfg, ossProcess, license); err != nil {
				return nil, trace.Wrap(err)
			}
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

		var accessGraphCfg *web.AccessGraphConfig
		if cfg.AccessGraph.Enabled {
			var caPem []byte
			if cfg.AccessGraph.CA != "" {
				caPem, err = os.ReadFile(cfg.AccessGraph.CA)
				if err != nil {
					return nil, nil, trace.Wrap(err)
				}
			}
			accessGraphCfg = &web.AccessGraphConfig{
				Addr:         cfg.AccessGraph.Addr,
				CA:           caPem,
				Insecure:     cfg.AccessGraph.Insecure,
				CipherSuites: cfg.CipherSuites,
			}
		}

		webPlugin, err = web.NewPlugin(web.Config{
			PluginShimURL: pluginShimURL,
			AccessGraph:   accessGraphCfg,
			Logger:        cfg.Logger,
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
			Logger:           cfg.Logger,
			License:          license,
			HostedPlugins:    cfg.Auth.HostedPlugins,
			AccessMonitoring: cfg.Auth.AccessMonitoring,
			AccessGraph:      cfg.AccessGraph,
			HTTPTransport:    cfg.Testing.HTTPTransport,
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

// registerExpectedServices sets up the instance role -> identity event mapping.
func registerExpectedServices(cfg *servicecfg.Config) {
	if cfg.Okta.Enabled {
		cfg.AdditionalExpectedRoles = append(cfg.AdditionalExpectedRoles,
			servicecfg.RoleAndIdentityEvent{
				Role:          types.RoleOkta,
				IdentityEvent: services.OktaIdentityEvent,
			})
		cfg.AdditionalReadyEvents = append(cfg.AdditionalReadyEvents, services.OktaReady)
	}

	services.JamfRegister(cfg)
}
