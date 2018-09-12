package pro

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/reporting/types"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// Enforcer is responsible for making sure that cluster doesn't lose
// connection to the control plane for too long and that the license
// is valid
type Enforcer struct {
	// WebClient is used to make requests to the control plane API
	*client.WebClient
	// Backend is the configured backend
	backend.Backend
	// Entry is used for logging
	*log.Entry
}

// EnforcerConfig is enforcer configuration
type EnforcerConfig struct {
	// Backend is the configured backend
	Backend backend.Backend
	// LicenseKeyPair is the license key pair
	LicenseKeyPair *liblicense.License
	// Insecure is whether to skip cert verification
	// when talking to the control plane
	Insecure bool
	// NoStart is used in tests to skip starting goroutines
	NoStart bool
}

// Check makes sure that enforcer config is valid
func (c *EnforcerConfig) Check() error {
	if c.Backend == nil {
		return trace.BadParameter("enforcer config is missing backend")
	}
	if c.LicenseKeyPair == nil {
		return trace.BadParameter("enforcer config is missing license")
	}
	return nil
}

// NewEnforcer initializes enforcer and starts its services
func NewEnforcer(ctx context.Context, config EnforcerConfig) (*Enforcer, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := liblicense.MakeTLSConfig(*config.LicenseKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.ServerName = constants.GetControlPlaneAPIHost()
	if config.Insecure {
		tlsConfig.InsecureSkipVerify = config.Insecure
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	client, err := client.NewWebClient(
		constants.GetControlPlaneAPIURL(), roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	enforcer := &Enforcer{
		WebClient: client,
		Backend:   config.Backend,
		Entry: log.WithFields(log.Fields{
			trace.Component: "enforcer",
		}),
	}
	if !config.NoStart {
		enforcer.Debug("Starting enforcer.")
		go enforcer.periodicHeartbeat(ctx)
		go enforcer.enforcer(ctx)
	}
	return enforcer, nil
}

func (e *Enforcer) enforcer(ctx context.Context) {
	ticker := time.NewTicker(constants.EnforcementInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := e.processLicenseCheckResult()
			if err != nil {
				log.Error(trace.DebugReport(err))
			}
		case <-ctx.Done():
			e.Debug("Enforce loop is exiting.")
			return
		}
	}
}

func (e *Enforcer) periodicHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(constants.LicenseCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := e.heartbeat()
			if err != nil {
				log.Debug(trace.DebugReport(err))
			}
		case <-ctx.Done():
			e.Debug("Heartbeat loop is exiting.")
			return
		}
	}
}

func (e *Enforcer) heartbeat() error {
	out, err := e.Get(e.Endpoint("heartbeat"), url.Values{})
	if err != nil {
		return trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(out.Bytes())
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.SetLicenseCheckHeartbeat(*heartbeat)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetLicenseCheckHeartbeat saves the license check heartbeat into the database
func (e *Enforcer) SetLicenseCheckHeartbeat(heartbeat types.Heartbeat) error {
	bytes, err := types.MarshalHeartbeat(heartbeat)
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.UpsertVal([]string{"heartbeat"}, "val", bytes, backend.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getLicenseCheckHeartbeat returns the latest license check heartbeat
func (e *Enforcer) getLicenseCheckHeartbeat() (*types.Heartbeat, error) {
	out, err := e.GetVal([]string{"heartbeat"}, "val")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return heartbeat, nil
}

// GetLicenseCheckResult returns the last license check result
func (e *Enforcer) GetLicenseCheckResult() (*types.Heartbeat, error) {
	heartbeat, err := e.getLicenseCheckHeartbeat()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// there may be no heartbeats yet, for example upon the very first start,
	// or in the enterprise mode, so make an empty one in this case
	if heartbeat == nil {
		heartbeat = types.NewHeartbeat()
	}
	// if the last successful heartbeat was more than 48 hours ago, add a
	// connection problem notification to the list of messages returned to
	// the user
	if isExpired(heartbeat) {
		heartbeat.Spec.Notifications = append(heartbeat.Spec.Notifications,
			types.Notification{
				Severity: types.SeverityError,
				Text:     licenseCheckConnectionProblemText,
				HTML:     licenseCheckConnectionProblemHTML,
			})
	}
	return heartbeat, nil
}

// processLicenseCheckResult implements "enforcement" policies, right now it
// only logs all messages received from the control plane into Teleport logs
func (e *Enforcer) processLicenseCheckResult() error {
	heartbeat, err := e.GetLicenseCheckResult()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, notification := range heartbeat.Spec.Notifications {
		switch notification.Severity {
		case types.SeverityWarning:
			log.Warn(notification.Text)
		case types.SeverityError:
			log.Error(notification.Text)
		default:
			log.Info(notification.Text)
		}
	}
	return nil
}

func isExpired(heartbeat *types.Heartbeat) bool {
	return time.Since(heartbeat.GetMetadata().Created) >
		constants.MaxControlPlaneUnreachableDuration
}

// licenseCheckConnectionProblemText is a warning message that gets displayed
// when teleport has failed to contact control plane for 48 hours
var licenseCheckConnectionProblemText = fmt.Sprintf(
	"Teleport has failed to contact the license server for more than %v "+
		"consecutive hours. Please make sure the Teleport auth server machine "+
		"is capable of connecting to %v. Otherwise, contact Gravitational "+
		"support (%v).",
	constants.MaxControlPlaneUnreachableHours,
	constants.GravitationalDownloadPortalURL,
	constants.GravitationalSupportURL)

// licenseCheckConnectionProblemHTML is a warning message in HTML format that
// gets displayed when teleport has failed to contact control plane for 48 hours
var licenseCheckConnectionProblemHTML = fmt.Sprintf(
	"Teleport has failed to contact the license server for more than %v "+
		"consecutive hours. Please make sure the Teleport auth server machine "+
		`is capable of connecting to (%v). Otherwise, contact <a href="%v">`+
		"Gravitational Support</a>.",
	constants.MaxControlPlaneUnreachableHours,
	constants.GravitationalDownloadPortalURL,
	constants.GravitationalSupportURL)
