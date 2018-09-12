package pro

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/teleport/e/lib/aws"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	liblicense "github.com/gravitational/license"
	reporting "github.com/gravitational/reporting/client"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// TeleportProcess augments struct from open-source version
type TeleportProcess struct {
	// TeleportProcess is the OSS process
	*service.TeleportProcess
	// Entry is used for logging
	*log.Entry
	// Enforcer is the teleport pro enforcer
	Enforcer *Enforcer
	// LicenseKeyPair is the license key pair
	LicenseKeyPair *liblicense.License
	// License is the teleport license
	License services.License
}

// NewTeleport instantiates a new pro/enterprise teleport process
func NewTeleport(config *service.Config) (*TeleportProcess, error) {
	teleport, err := service.NewTeleport(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process := &TeleportProcess{
		TeleportProcess: teleport,
		Entry: log.WithFields(log.Fields{
			trace.Component: "process",
		}),
	}
	// only check the license and run extra services on auth server
	if !config.Auth.Enabled {
		return process, nil
	}
	process.LicenseKeyPair, process.License, err = checkLicense(process, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// when reporting usage, teleport runs some additional services that phone
	// home once in a while to report usage metrics and verify license
	if process.ReportsUsage() {
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

// ReportsUsage returns true if the process has to report usage
func (p *TeleportProcess) ReportsUsage() bool {
	return p.License.GetReportsUsage().Value()
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
	certificate, err := liblicense.MakeTLSCert(*config.Teleport.LicenseKeyPair)
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
	enforcer, err := NewEnforcer(ctx, EnforcerConfig{
		Backend:        config.Teleport.GetBackend(),
		LicenseKeyPair: config.Teleport.LicenseKeyPair,
		Insecure:       config.Insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return enforcer, nil
}

// parseLicense returns license key pair and payload
func parseLicense(licenseBytes []byte) (*liblicense.License, services.License, error) {
	licenseKeyPair, err := liblicense.ParseLicensePEM(licenseBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// check if it's a legacy license
	var legacyLicense LegacyLicense
	if err := json.Unmarshal(licenseKeyPair.RawPayload, &legacyLicense); err == nil {
		teleportLicense, err := legacyLicense.ToV3()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return licenseKeyPair, teleportLicense, nil
	}

	teleportLicense, err := services.UnmarshalLicense(licenseKeyPair.RawPayload)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return licenseKeyPair, teleportLicense, nil
}

// checkLicense verified the presence of license and runs basic checks on it
func checkLicense(process *TeleportProcess, config *service.Config) (*liblicense.License, services.License, error) {
	if config.Auth.LicenseFile == "" {
		return nil, nil, trace.AccessDenied(
			fmt.Sprintf(errLicensePath, filepath.Join(config.DataDir, defaults.LicenseFile)))
	}
	bytes, err := ioutil.ReadFile(config.Auth.LicenseFile)
	if err != nil {
		process.Debug(trace.DebugReport(err))
		return nil, nil, trace.AccessDenied(
			fmt.Sprintf(errLicensePath, filepath.Join(config.DataDir, defaults.LicenseFile)))
	}
	licenseKeyPair, license, err := parseLicense(bytes)
	if err != nil {
		process.Debug(trace.DebugReport(err))
		// unrecognized plan, return error
		return nil, nil, trace.AccessDenied(
			fmt.Sprintf(errLicenseParse, config.Auth.LicenseFile))
	}
	if license.GetAWSProductID() != "" || license.GetAWSAccountID() != "" {
		if err := aws.Verify(*licenseKeyPair, license); err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	process.Infof("Using license from %v %v.", config.Auth.LicenseFile, license)
	return licenseKeyPair, license, nil
}

const (
	// errLicensePath is displayed when auth server is started w/o valid license
	errLicensePath = "auth server requires a valid license file to start, " +
		"please set the correct license_file path under auth_server section " +
		"in your teleport config or put the license into the default search " +
		"location at %v"
	// errLicenseParse is displayed on license parsing error
	errLicenseParse = "the provided license file %v could not be parsed, " +
		"please contact support@gravitational.com for assistance"
	// errLicenseProduct is displayed when license product name is invalid
	errLicenseProduct = "the provided license file %v has invalid product " +
		"name %q, please contact support@gravitational.com for assistance"
)
