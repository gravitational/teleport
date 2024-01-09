package prehog

import (
	"context"
	"os"

	"github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/lib/events/usageevents"
	"github.com/gravitational/teleport/lib/service"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	// envVarPreHogEndpoint is the URL where prehog events should be submitted.
	// If unset, prehog event submission is not enabled.
	envVarPreHogEndpoint = "PREHOG_ENDPOINT"

	// envVarPreHogCAPath is a path to the CA certificate for Prehog. May be
	// omitted if Prehog's certificate is in the system trust store.
	envVarPreHogCAPath = "PREHOG_CA_PATH"

	// prehogComponent is the logrus component for the prehog module
	prehogComponent = "prehog"
)

var log = logrus.WithField(trace.Component, prehogComponent)

// InitStreamingUsageReporting adds the prehog usage reporter to the given
// Teleport process.
func InitStreamingUsageReporting(
	ctx context.Context,
	licenseFile *licensefile.LicenseFile,
	process *service.TeleportProcess,
) error {
	var endpoint string
	if e := os.Getenv(envVarPreHogEndpoint); e != "" {
		endpoint = e
	} else {
		log.Warnf("%q not set and no default available, PreHog usage reporting will not be enabled.", envVarPreHogEndpoint)
		return nil
	}

	var caCert []byte
	if caPath, exists := os.LookupEnv(envVarPreHogCAPath); exists {
		var err error
		caCert, err = os.ReadFile(caPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	clusterName, err := process.GetAuthServer().GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := license.MakeTLSCert(*licenseFile.KeyPair)
	if err != nil {
		return trace.Wrap(err)
	}

	submitter, err := usagereporter.NewPrehogSubmitter(ctx, endpoint, cert, caCert)
	if err != nil {
		return trace.Wrap(err)
	}

	anonymizationKey, err := process.GetAuthServer().GetAnonymizationKey(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Replace the discard usage reporter with the real implementation.
	reporter, err := usagereporter.NewStreamingUsageReporter(log, clusterName, anonymizationKey, submitter)
	if err != nil {
		return trace.Wrap(err)
	}
	process.GetAuthServer().SetUsageReporter(reporter)
	go reporter.Run(ctx)

	// Wrap the audit log in the usage reporting impl.
	wrappedLog, err := usageevents.New(
		reporter,
		log,
		process.GetAuthServer().GetEmitter(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	process.GetAuthServer().SetEmitter(wrappedLog)

	log.Infof("PreHog usage reporter has been enabled, endpoint: %s", endpoint)

	return nil
}
