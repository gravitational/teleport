package pro

import (
	"context"
	"io/ioutil"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/license"
	reporting "github.com/gravitational/reporting/client"
	"github.com/gravitational/trace"
)

// TeleportProcess augments struct from open-source version
type TeleportProcess struct {
	// TeleportProcess is the OSS process
	*service.TeleportProcess
	// Enforcer is the teleport pro enforcer
	Enforcer *Enforcer
	// License is the teleport license
	License *license.License
}

// NewTeleport instantiates a new pro/enterprise teleport process
func NewTeleport(config *service.Config) (*TeleportProcess, error) {
	teleport, err := service.NewTeleport(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process := &TeleportProcess{
		TeleportProcess: teleport,
	}
	// only check the license and run extra services on auth server
	if !config.Auth.Enabled {
		return process, nil
	}
	process.License, err = checkLicense(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// in "pro" mode teleport runs some additional services that phone
	// home once in a while to report usage metrics and verify license
	if process.IsPro() {
		process.Enforcer, err = initServices(context.Background(), &proConfig{
			Teleport: process,
			Insecure: lib.IsInsecureDevMode(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return process, nil
}

// IsPro returns true if the process is running in "pro" mode
func (p *TeleportProcess) IsPro() bool {
	return utils.SliceContainsStr(constants.ProPlans, p.License.Payload.ProductName)
}

// proConfig combines pro mode configuration parameters
type proConfig struct {
	// Teleport is the Teleport process
	Teleport *TeleportProcess
	// Insecure is whether the server runs in insecure mode
	Insecure bool
}

// initServices initializes services for teleport pro mode
func initServices(ctx context.Context, config *proConfig) (*Enforcer, error) {
	certificate, err := license.MakeTLSCert(*config.Teleport.License)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recorder, err := reporting.NewClient(ctx,
		reporting.ClientConfig{
			ServerAddr:  constants.ControlPlaneAPIAddr,
			ServerName:  constants.ControlPlaneAPIHost,
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
	auditLog, err := NewAuditLog(AuditLogConfig{
		Inner:        config.Teleport.GetAuditLog(),
		Recorder:     recorder,
		AnonymizeKey: clusterConfig.GetClusterID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.Teleport.GetAuthServer().SetAuditLog(auditLog)
	enforcer, err := NewEnforcer(ctx, EnforcerConfig{
		Backend:  config.Teleport.GetBackend(),
		License:  config.Teleport.License,
		Insecure: config.Insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return enforcer, nil
}

// checkLicense verified the presence of license and runs basic checks on it
func checkLicense(config *service.Config) (*license.License, error) {
	if config.Auth.LicenseFile == "" {
		return nil, trace.AccessDenied("please provide a valid license file")
	}
	bytes, err := ioutil.ReadFile(config.Auth.LicenseFile)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read the license file: %v",
			config.Auth.LicenseFile)
	}
	parsed, err := license.ParseString(string(bytes))
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse the license")
	}
	name := parsed.Payload.ProductName
	switch name {
	case constants.ProPlan, constants.BusinessPlan, constants.EnterprisePlan:
	default:
		return nil, trace.BadParameter("invalid license product name: %q", name)
	}
	return parsed, nil
}
