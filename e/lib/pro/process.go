package pro

import (
	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/e/lib/prehog"
	"github.com/gravitational/teleport/e/tool/modules"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

// Config is the Teleport Pro (Enterprise) config
type Config struct {
	// OSSProcess is the open source teleport process
	OSSProcess *service.TeleportProcess
	// LicenseFile is an instance of the LicenseFile
	LicenseFile *licensefile.LicenseFile
	// AuthPlugin is the AuthPlugin
	AuthPlugin *auth.Plugin
	// LicensePath is the path of the license on disk
	LicensePath string
	// Modules defines build time constraints and licensed features.
	Modules *modules.EnterpriseModules
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.OSSProcess == nil {
		return trace.BadParameter("missing OSSProcess")
	}

	if c.LicenseFile == nil {
		return trace.BadParameter("missing LicenseFile")
	}

	if c.LicensePath == "" {
		return trace.BadParameter("missing LicensePath")
	}

	if c.AuthPlugin == nil {
		return trace.BadParameter("missing AuthPlugin")
	}

	if c.Modules == nil {
		return trace.BadParameter("missing Modules")
	}

	return nil
}

// Process augments struct from open-source version
type Process struct {
	// TeleportProcess is the OSS process
	*service.TeleportProcess
	// LicenseFile is an instance of the LicenseFile
	LicenseFile *licensefile.LicenseFile
}

// NewTeleport instantiates a new pro/enterprise teleport process
func NewTeleport(cfg Config) (*Process, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	process := &Process{
		TeleportProcess: cfg.OSSProcess,
		LicenseFile:     cfg.LicenseFile,
	}

	// if the cloud hostport is set when we don't have a cloud license, we're in
	// "tenant dashboard mode"
	if cloud.IsCloudEnv() {
		tlsConfig, err := liblicense.MakeTLSConfig(*cfg.LicenseFile.KeyPair)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cloudClient, err := cloud.NewClientFromTLSConfig(tlsConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.AuthPlugin.EnableCloud(cloudClient)

		process.OnExit("cloudClient.shutdown", func(payload any) {
			cloudClient.Close()
		})

		return process, nil
	}

	go licensefile.RunLicenseChecker(process.ExitContext(), process.GetAuthServer(), process.LicenseFile)

	// run the license auto update service if the entitlement is enabled
	if info, ok := cfg.Modules.Features().Entitlements[entitlements.LicenseAutoUpdate]; ok && info.Enabled {
		updateService, err := newLicenseUpdateService(licenseUpdateServiceConfig{
			ServerID:    process.GetAuthServer().ServerID,
			LicenseFile: process.LicenseFile,
			LicensePath: cfg.LicensePath,
			Modules:     cfg.Modules,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		go updateService.Run(process.ExitContext())
	}

	// todo (michellescripts) set this in getSelfHostedLicenseFeatures and treat Features as source of truth
	if cfg.LicenseFile.License.GetSalesCenterReporting() {
		const isCloudFalse = false

		anonymizationKey, err := process.GetAuthServer().GetAnonymizationKey(process.ExitContext())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		anonymizer, err := utils.NewHMACAnonymizer(anonymizationKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// forcibly stops when ExitContext closes or is gracefully stopped in
		// auth.shutdown
		if err := prehog.InitAggregatingUsageReporting(
			process.TeleportProcess,
			cfg.LicenseFile,
			isCloudFalse,
			anonymizer,
		); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		prehog.ClearAggregatingUsageReportingAlert(process.TeleportProcess)
	}

	if cfg.AuthPlugin.HostedPlugins.Enabled {
		if err := plugins.RegisterPluginManager(cfg.AuthPlugin.HostedPlugins.OAuthProviders, process.TeleportProcess); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return process, nil
}
