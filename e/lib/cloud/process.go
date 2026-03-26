package cloud

import (
	"log/slog"
	"os"
	"time"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/cloud/feature"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/plugins"
	"github.com/gravitational/teleport/lib/service"
)

var (
	// defaultReportingInterval is how often Teleport Cloud reports its usage
	defaultReportingInterval = 5 * time.Minute
	// defaultFeatureQueryInterval is how often Teleport will query Cloud for the feature set
	defaultFeatureQueryInterval = 2 * time.Minute
	//  defaultRequestTimeout is the timeout of requests to fetch Cloud features
	defaultFeatureQueryTimeout = time.Second * 30
	// EnvVarInterval is used to override the default reporting interval
	EnvVarInterval = "TELEPORT_CLOUD_INTERVAL"
	// cloudComponent is the logging name of the Teleport cloud component
	cloudComponent = "cloud"
)

// Config is the Teleport Cloud config
type Config struct {
	// AuthPlugin is the auth plugin
	AuthPlugin *auth.Plugin
	// OSSProcess is the open source teleport process
	OSSProcess *service.TeleportProcess
	// LicenseFile is a license file
	LicenseFile *licensefile.LicenseFile
	// ReportingInterval is how often Teleport Cloud reports usage
	ReportingInterval time.Duration
	// Logger emits log messages
	Logger *slog.Logger
}

// Process augments struct from open-source version
type Process struct {
	*service.TeleportProcess
}

// NewTeleport instantiates a new teleport cloud process
func NewTeleport(cfg Config) (*Process, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	process := &Process{
		TeleportProcess: cfg.OSSProcess,
	}

	tlsConfig, err := liblicense.MakeTLSConfig(*cfg.LicenseFile.KeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.InsecureSkipVerify = cfg.OSSProcess.Config.InsecureMode

	cloudClient, err := cloud.NewClientFromTLSConfig(tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Activate cloud API methods
	cfg.AuthPlugin.EnableCloud(cloudClient)

	process.OnExit("cloud.shutdown", func(payload any) {
		cloudClient.Close()
	})

	// Start feature service
	if cfg.LicenseFile.License.GetCloud() {
		// Use a jittered interval between (defaultFeatureQueryInterval, defaultFeatureQueryInterval * 2)
		// so cloud server doesn't get too crowded when all auth servers are restarted on upgrades
		jitteredQueryInterval := retryutils.HalfJitter(defaultFeatureQueryInterval * 2)
		featureService, err := feature.NewService(feature.Config{
			Backend:                  process.GetBackend(),
			CloudClient:              cloudClient,
			Interval:                 jitteredQueryInterval,
			RequestTimeout:           defaultFeatureQueryTimeout,
			OnAnonymizationKeyUpdate: process.GetAuthServer().SetAnonymizationKey,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		go featureService.Run(process.ExitContext())
	}

	if cfg.AuthPlugin.HostedPlugins.Enabled {
		if err := plugins.RegisterPluginManager(cfg.AuthPlugin.HostedPlugins.OAuthProviders, process.TeleportProcess); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return process, nil
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, cloudComponent)
	}

	if c.AuthPlugin == nil {
		return trace.BadParameter("missing AuthPlugin")
	}

	if c.OSSProcess == nil {
		return trace.BadParameter("missing OSSProcess")
	}

	if c.LicenseFile == nil {
		return trace.BadParameter("missing LicenseFile")
	}

	if c.LicenseFile.KeyPair == nil {
		return trace.BadParameter("missing License KeyPair value")
	}

	err = c.checkAndSetInterval()
	if err != nil {
		return trace.Wrap(err, "invalid reporting interval value")
	}

	return nil
}

func (c *Config) checkAndSetInterval() (err error) {
	if c.ReportingInterval > 0 {
		return nil
	}

	// Look up the value from environment variable.
	// This makes debugging easier in the Teleport cloud.
	envValue := os.Getenv(EnvVarInterval)
	if envValue == "" {
		c.ReportingInterval = defaultReportingInterval
		return nil
	}

	c.ReportingInterval, err = time.ParseDuration(envValue)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
