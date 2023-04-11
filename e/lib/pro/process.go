package pro

import (
	"os"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/auth"
	cloudlib "github.com/gravitational/teleport/e/lib/cloud"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/prehog"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
)

// Config is the Teleport Pro (Enterprise) config
type Config struct {
	// OSSProcess is the open source teleport process
	OSSProcess *service.TeleportProcess
	// LicenseFile is an instance of the LicenseFile
	LicenseFile *licensefile.LicenseFile
	// AuthPlugin is the AuthPlugin
	AuthPlugin *auth.Plugin
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.OSSProcess == nil {
		return trace.BadParameter("missing OSSProcess")
	}

	if c.LicenseFile == nil {
		return trace.BadParameter("missing LicenseFile")
	}

	if c.AuthPlugin == nil {
		return trace.BadParameter("missing AuthPlugin")
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
	if cloudHostPort := os.Getenv(cloudlib.EnvVarHostPort); cloudHostPort != "" {
		apiServerAddr, err := cloudlib.GetServerAddr(cloudHostPort)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tlsConfig, err := liblicense.MakeTLSConfig(*cfg.LicenseFile.KeyPair)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConfig.ServerName = apiServerAddr.Host()
		tlsConfig.InsecureSkipVerify = lib.IsInsecureDevMode()

		cloudClient, err := cloud.NewClient(cloud.ClientConfig{
			Hostname:  apiServerAddr.Addr,
			TLSConfig: tlsConfig,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.AuthPlugin.EnableCloud(cloudClient)
		modules.GetModules().EnableRecoveryCodes()

		process.OnExit("cloudClient.shutdown", func(payload interface{}) {
			cloudClient.Close()
		})

		return process, nil
	}

	go licensefile.RunLicenseChecker(process.ExitContext(), process.GetAuthServer(), process.LicenseFile)

	if cfg.LicenseFile.License.GetSalesCenterReporting() {
		// forcibly stops when ExitContext closes or is gracefully stopped in
		// auth.shutdown
		if err := prehog.InitAggregatingUsageReporting(
			process.TeleportProcess,
			cfg.LicenseFile,
		); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return process, nil
}
