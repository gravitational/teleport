package process

import (
	"context"
	"net/url"
	"os"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/accessgraph"
	"github.com/gravitational/teleport/e/lib/auth"
	_ "github.com/gravitational/teleport/e/lib/backend/crdb"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/db/oracle"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/prehog"
	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/e/lib/services"
	"github.com/gravitational/teleport/e/lib/web"
	emodules "github.com/gravitational/teleport/e/tool/modules"
	ossauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
	"github.com/gravitational/teleport/lib/utils"
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

	features, err := pro.LoadFeatures(ctx, license, cfg.InsecureMode)
	if err != nil {
		// despite the error, we should still try to load features from the backend.
		// Features from the backend may be stale, so we should always prioritize
		// using configureModules, and only use the backend features as a fallback in case
		// some external service (like Cloud's API) is offline during the features fetching.
		cfg.Logger.WarnContext(ctx, "failed configuring cluster modules", "error", err)
		tryLoadingFeaturesFromBackend = cfg.Auth.Enabled
	}

	mod := emodules.NewEnterpriseModules(emodules.EnterpriseModulesConfig{
		License:                  license,
		Features:                 features,
		HostedPluginsEnabled:     cfg.Auth.HostedPlugins.Enabled,
		Cloud:                    cloud.IsCloudEnv(),
		AutomaticUpgradesEnabled: automaticupgrades.IsEnabled(),
	})
	cfg.Modules = mod

	// TODO(tross): delete once modules are injected everywhere.
	modules.SetModules(mod)

	pluginRegistry := plugin.NewRegistry()

	if cfg.Auth.Enabled {
		// Register the usage reporting init callback so it runs after the OSS auth
		// server is ready, ensuring enterprise auth extensions see the correct
		// UsageReporter service.
		pluginRegistry.SetUsageReportingInitFunc(func(processI any) error {
			process, ok := processI.(*service.TeleportProcess)
			if !ok {
				return trace.BadParameter("invalid process type, expected %T, got %T (this is a bug)", process, processI)
			}

			return trace.Wrap(initUsageReporting(process, cfg, license))
		})
	}

	webPlugin, authPlugin, err := addPlugins(cfg, license, pluginRegistry)
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
			// At this point, we couldn't get features from the Cloud API and the backend.
			// To avoid failing to start the auth server, we continue with just the license features.
			// This will only happen for Cloud clusters during the first time the auth service is started
			// since subsequent starts will have backend features.
			mod.UpdateModules(license, emodules.GetSelfHostedLicenseFeatures(license.License))
			cfg.Logger.InfoContext(ctx, "feature loading from backend failed, starting with license features", "features", mod.Features())
		} else {
			cfg.Logger.InfoContext(ctx, "successfully loaded features from backend", "features", f)
			mod.UpdateModules(license, *f)
		}
	}

	// store cloud features for future restarts
	if license != nil && license.License.GetCloud() {
		if _, err := feature.Store(ctx, cfg.Modules.Features(), ossProcess.GetBackend()); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	if cfg.Proxy.Enabled && !cfg.Proxy.DisableWebService {
		if err := extendProxy(ossProcess, webPlugin); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// TODO(kopiczko) v19: remove the whole servicecfg.OktaConfig type and its references.
	if cfg.Okta.Enabled {
		return nil, trace.BadParameter("okta_service configuration is not supported anymore. Please migrate to Okta plugin https://goteleport.com/docs/admin-guides/access-controls/okta/. This message will be removed in v19.")
	}

	if cfg.Jamf.Enabled() {
		services.JamfStandaloneInit(ossProcess, nil /* httpClient */)
	}

	if cfg.Databases.Enabled {
		common.RegisterEngine(oracle.NewEngine, defaults.ProtocolOracle)
		healthchecks.RegisterHealthChecker(oracle.NewHealthChecker, defaults.ProtocolOracle)
	}

	// This needs to be the last thing in NewTeleport because it needs to
	// return an enterprise specific process.
	if cfg.Auth.Enabled {
		authProcess, err := extendAuthServer(ossProcess, license, authPlugin, cfg.Auth.LicenseFile, mod)
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

func addPlugins(cfg *servicecfg.Config, license *licensefile.LicenseFile, pluginRegistry plugin.Registry) (webPlugin *web.Plugin, authPlugin *auth.Plugin, err error) {
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
			Modules:          cfg.Modules,
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
	services.JamfRegister(cfg)
}

// initUsageReporting starts all usage-reporting pipelines for the given process.
// It is invoked via the plugin registry callback after the OSS auth server is fully initialized,
// ensuring enterprise auth extensions receive the correct UsageReporter service.
//
// For cloud deployments it starts:
//  1. Streaming usage reporting — sends events to the Cloud in near real-time.
//  2. Aggregating usage reporting — batches and forwards events periodically.
//
// For self-hosted deployments it starts:
//  1. Aggregating usage reporting — only when the license has sales-center reporting enabled.
func initUsageReporting(process *service.TeleportProcess, cfg *servicecfg.Config, license *licensefile.LicenseFile) error {
	anonymizer, err := newAnonimizer(process.GetAuthServer(), license)
	if err != nil {
		return trace.Wrap(err, "failed to initialize HMAC anonymizer")
	}

	isCloud := cfg.Modules.Features().Cloud
	if isCloud {
		if err := prehog.InitStreamingUsageReporting(
			process.ExitContext(),
			license,
			process,
			anonymizer,
		); err != nil {
			return trace.Wrap(err)
		}
	}
	if license.License.GetSalesCenterReporting().Value() || isCloud {
		if err := prehog.InitAggregatingUsageReporting(
			process,
			license,
			isCloud,
			anonymizer,
		); err != nil {
			return trace.Wrap(err)
		}
	} else {
		prehog.ClearAggregatingUsageReportingAlert(process)
	}
	return nil
}

func newAnonimizer(authServer *ossauth.Server, license *licensefile.LicenseFile) (utils.Anonymizer, error) {
	// set the license before initializing the anonymization key,
	// since the license may contain the anonymization key for self-hosted clusters.
	if license != nil {
		authServer.SetLicense(license.GetKeyPair())
	}

	if err := authServer.InitializeAnonymizationKey(); err != nil {
		return nil, trace.Wrap(err, "failed to initialize anonymization key on auth server")
	}

	anonymizer, err := utils.NewHMACAnonymizer(authServer)
	if err != nil {
		return nil, trace.Wrap(err, "failed to initialize HMAC anonymizer")
	}
	return anonymizer, nil
}
