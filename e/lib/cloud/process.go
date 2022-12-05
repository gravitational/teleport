package cloud

import (
	"os"
	"time"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/cloud/usagereporter"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/prehog"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// defaultAPIServerAddr is default cloud API server address
	defaultAPIServerAddr = "api.teleport.sh"
	// defaultReportingInterval is how often Teleport Cloud reports its usage
	defaultReportingInterval = 5 * time.Minute
	// defaultAPIServerPort is the default SalesCenter API port
	defaultAPIServerPort = 443
	// envVarHostPort is used to override the default cloud api server address
	envVarHostPort = "TELEPORT_CLOUD_HOSTPORT"
	// envVarInterval is used to override the default reporting interval
	envVarInterval = "TELEPORT_CLOUD_INTERVAL"
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
	// CloudAPIServerAddr is the address of the Cloud API Server
	CloudAPIServerAddr string
	// ReportingInterval is how often Teleport Cloud reports usage
	ReportingInterval time.Duration
	// Log is the logger
	Log *logrus.Entry
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

	apiServerAddr, err := getServerAddr(cfg.CloudAPIServerAddr)
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

	presence := process.Config.Presence
	if presence == nil {
		presence = local.NewPresenceService(process.GetBackend())
	}

	identity := process.Config.Identity
	if identity == nil {
		identity = local.NewIdentityService(process.GetBackend())
	}

	access := process.Config.Access
	if access == nil {
		access = local.NewAccessService(process.GetBackend())
	}

	resourceGetter := struct {
		services.Presence
		services.Identity
		services.Access
		services.StatusInternal
		events.IAuditLog
	}{
		Identity:       identity,
		Presence:       presence,
		Access:         access,
		StatusInternal: local.NewStatusService(process.GetBackend()),
		IAuditLog:      process.GetAuditLog(),
	}
	usageReporter, err := usagereporter.New(usagereporter.Config{
		Log:            cfg.Log,
		Interval:       cfg.ReportingInterval,
		Clock:          process.Clock,
		BackendGetter:  process.GetBackend(),
		CloudClient:    cloudClient,
		ResourceGetter: resourceGetter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Activate cloud API methods
	cfg.AuthPlugin.EnableCloud(cloudClient)

	process.OnExit("cloud.shutdown", func(payload interface{}) {
		cloudClient.Close()
	})

	// Start usage reporting
	go usageReporter.Run(process.ExitContext())

	if err := prehog.InitPreHogUsageReporting(
		process.ExitContext(),
		cfg.LicenseFile,
		process.TeleportProcess,
	); err != nil {
		return nil, trace.Wrap(nil)
	}

	return process, nil
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, cloudComponent)
	}

	if c.AuthPlugin == nil {
		return trace.BadParameter("missing AuthPlugin")
	}

	if c.OSSProcess == nil {
		return trace.BadParameter("missing OSSProcess")
	}

	if c.LicenseFile == nil {
		return trace.BadParameter("missing LicenseFile ")
	}

	if c.LicenseFile.KeyPair == nil {
		return trace.BadParameter("missing License KeyPair value")
	}

	err = c.checkAndSetInterval()
	if err != nil {
		return trace.Wrap(err, "invalid reporting interval value")
	}

	// TODO(alexeyk): add server address field to the license
	if c.CloudAPIServerAddr == "" {
		c.CloudAPIServerAddr = os.Getenv(envVarHostPort)
	}

	if c.CloudAPIServerAddr == "" {
		c.CloudAPIServerAddr = defaultAPIServerAddr
	}

	return nil
}

func (c *Config) checkAndSetInterval() (err error) {
	if c.ReportingInterval > 0 {
		return nil
	}

	// Look up the value from environment variable.
	// This makes debugging easier in the Teleport cloud.
	envValue := os.Getenv(envVarInterval)
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

func getServerAddr(hostport string) (*utils.NetAddr, error) {
	addr, err := utils.ParseHostPortAddr(hostport, defaultAPIServerPort)
	if err != nil {
		return nil, trace.BadParameter("invalid cloud API server address")
	}

	return addr, nil
}
