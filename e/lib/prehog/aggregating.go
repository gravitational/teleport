package prehog

import (
	"cmp"
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"os"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/e/lib/licensefile"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	prehogv1c "github.com/gravitational/teleport/gen/proto/go/prehog/v1/prehogv1connect"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events/usageevents"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/usagereporter/teleport/aggregating"
	"github.com/gravitational/teleport/lib/utils"
)

// NewUsageReportsSubmitter returns an [aggregating.UsageReportsSubmitter] that
// sends usage reports to our ingest service via
// prehog.v1alpha.TeleportReportingService/SubmitUsageReports .
func NewUsageReportsSubmitter(clientCert *tls.Certificate, cipherSuites []uint16, endpoint string) (aggregating.UsageReportsSubmitter, error) {
	ht, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// logic extended from [httpproxy.FromEnvironment]
	proxyFunc := (&httpproxy.Config{
		HTTPProxy: cmp.Or(
			os.Getenv("TELEPORT_REPORTING_HTTP_PROXY"),
			os.Getenv("HTTP_PROXY"),
			os.Getenv("http_proxy"),
		),
		HTTPSProxy: cmp.Or(
			os.Getenv("TELEPORT_REPORTING_HTTPS_PROXY"),
			os.Getenv("HTTPS_PROXY"),
			os.Getenv("https_proxy"),
		),
		NoProxy: cmp.Or(
			os.Getenv("NO_PROXY"),
			os.Getenv("no_proxy"),
		),
		CGI: os.Getenv("REQUEST_METHOD") != "",
	}).ProxyFunc()
	ht.Proxy = func(req *http.Request) (*url.URL, error) {
		return proxyFunc(req.URL)
	}
	ht.TLSClientConfig = utils.TLSConfig(cipherSuites)
	ht.TLSClientConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return clientCert, nil
	}

	hc := &http.Client{
		Transport: ht,
		// we expect no redirects
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	clt := prehogv1c.NewTeleportReportingServiceClient(hc, endpoint, connect.WithGRPC())

	return func(ctx context.Context, req *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error) {
		resp, err := clt.SubmitUsageReports(ctx, connect.NewRequest(req))
		if err != nil {
			// TODO(espadolini): convert connectrpc.com/connect errors similarly to [trail.FromGRPC]
			return uuid.Nil, trace.Wrap(err)
		}
		batchUUID, err := uuid.FromBytes(resp.Msg.GetBatchUuid())
		if err != nil {
			return uuid.Nil, trace.Wrap(err)
		}
		return batchUUID, nil
	}, nil
}

// InitAggregatingUsageReporting adds the aggregating usage reporter to the
// given Teleport process.
func InitAggregatingUsageReporting(
	process *service.TeleportProcess,
	licenseFile *licensefile.LicenseFile,
	isCloud bool,
) error {
	endpoint := aggregating.DefaultEndpoint
	if isCloud {
		if e := os.Getenv(envVarPreHogAggregatingEndpoint); e != "" {
			endpoint = e
		} else if e := os.Getenv(envVarPreHogEndpoint); e != "" {
			endpoint = e
		} else {
			log.Warnf("%q not set and no default available, PreHog aggregated usage reporting will not be enabled.", envVarPreHogAggregatingEndpoint)
			return nil
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

	log := usageReportingLog(process)

	anonymizationKey, err := process.GetAuthServer().GetAnonymizationKey(process.ExitContext())
	if err != nil {
		return trace.Wrap(err)
	}

	reporter, err := aggregating.NewReporter(process.ExitContext(),
		aggregating.ReporterConfig{
			Backend:          process.GetBackend(),
			Log:              log,
			ClusterName:      clusterName,
			HostID:           process.GetAuthServer().ServerID,
			AnonymizationKey: anonymizationKey,
		})
	if err != nil {
		return trace.Wrap(err)
	}
	AddReporter(process.GetAuthServer(), reporter)

	emitter, err := usageevents.New(
		reporter, log, process.GetAuthServer().GetEmitter(),
	)
	if err != nil {
		return trace.Wrap(err)
	}
	process.GetAuthServer().SetEmitter(emitter)

	submitter, err := NewUsageReportsSubmitter(cert, process.Config.CipherSuites, endpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	submitterCfg := aggregating.SubmitterConfig{
		Backend:   process.GetBackend(),
		Log:       log,
		Status:    process.GetAuthServer(),
		Submitter: submitter,
		HostID:    process.GetAuthServer().ServerID,
	}
	if err := submitterCfg.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	go aggregating.RunSubmitter(process.GracefulExitContext(), submitterCfg)

	log.Info("Successfully started.")

	return nil
}

// ClearAggregatingUsageReportingAlert deletes the reporting-failed cluster
// alert, if present.
func ClearAggregatingUsageReportingAlert(process *service.TeleportProcess) {
	log := usageReportingLog(process)
	err := aggregating.ClearAlert(process.GracefulExitContext(), process.GetAuthServer())
	if err == nil {
		log.Infof("Deleted cluster alert.")
	} else if !trace.IsNotFound(err) {
		log.WithError(err).Errorf("Failed to delete cluster alert.")
	}
}

func usageReportingLog(process *service.TeleportProcess) *logrus.Entry {
	return process.Config.Log.WithField(
		teleport.ComponentKey,
		teleport.Component(
			teleport.ComponentUsageReporting,
			process.GetID(),
		),
	)
}
