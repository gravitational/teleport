package pro

import (
	"context"

	"github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/e/lib/pro/enforcer"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	liblicense "github.com/gravitational/license"
	reporting "github.com/gravitational/reporting/client"
	"github.com/gravitational/trace"
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

	// when reporting usage, teleport runs some additional services that phone
	// home once in a while to report usage metrics and verify license
	if cfg.LicenseFile.License.GetReportsUsage() {
		enforcer, err := initServices(process.ExitContext(), &proConfig{
			Teleport: process,
			Insecure: lib.IsInsecureDevMode(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cfg.AuthPlugin.EnableEnforcer(enforcer)
	}

	return process, nil
}

// proConfig combines pro mode configuration parameters
type proConfig struct {
	// Teleport is the Teleport process
	Teleport *Process
	// Insecure is whether the server runs in insecure mode
	Insecure bool
}

// initServices initializes services for teleport pro mode
func initServices(ctx context.Context, config *proConfig) (*enforcer.Enforcer, error) {
	certificate, err := liblicense.MakeTLSCert(*config.Teleport.LicenseFile.KeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recorder, err := reporting.NewClient(ctx,
		reporting.ClientConfig{
			ServerAddr:  constants.GetControlPlaneAPIAddr(),
			ServerName:  constants.GetControlPlaneAPIHost(),
			Certificate: *certificate,
			Insecure:    config.Insecure,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterConfig, err := config.Teleport.GetAuthServer().GetClusterConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	anonymizer, err := utils.NewHMACAnonymizer(clusterConfig.GetClusterID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auditLog, err := NewAuditLog(AuditLogConfig{
		Inner:      config.Teleport.GetAuditLog(),
		Recorder:   recorder,
		Anonymizer: anonymizer,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.Teleport.GetAuthServer().SetAuditLog(auditLog)
	enforcer, err := enforcer.New(ctx, enforcer.Config{
		Anonymizer:     anonymizer,
		Backend:        config.Teleport.GetBackend(),
		LicenseKeyPair: config.Teleport.LicenseFile.KeyPair,
		Insecure:       config.Insecure,
		ClusterID:      clusterConfig.GetClusterID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return enforcer, nil
}
