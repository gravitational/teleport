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

	"github.com/gravitational/license"
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
	// License is the parsed license
	License *license.License
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
	if c.License == nil {
		return trace.BadParameter("enforcer config is missing license")
	}
	return nil
}

// NewEnforcer initializes enforcer and starts its services
func NewEnforcer(ctx context.Context, config EnforcerConfig) (*Enforcer, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := license.MakeTLSConfig(*config.License)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig.ServerName = constants.ControlPlaneAPIHost
	if config.Insecure {
		tlsConfig.InsecureSkipVerify = config.Insecure
	}
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	client, err := client.NewWebClient(
		constants.ControlPlaneAPIURL, roundtrip.HTTPClient(httpClient))
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
		enforcer.Debug("starting enforcer")
		go enforcer.periodicHeartbeat(ctx)
		go enforcer.enforcer(ctx)
	}
	return enforcer, nil
}

func (e *Enforcer) enforcer(ctx context.Context) {
	ticker := time.NewTicker(constants.EnforcerEnforcePeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := e.processHeartbeatResult()
			if err != nil {
				log.Error(trace.DebugReport(err))
			}
		case <-ctx.Done():
			e.Debug("enforce loop is exiting")
			return
		}
	}
}

func (e *Enforcer) periodicHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(constants.EnforcerHeartbeatPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := e.heartbeat()
			if err != nil {
				log.Debug(trace.DebugReport(err))
			}
		case <-ctx.Done():
			e.Debug("heartbeat loop is exiting")
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
	err = e.SetHeartbeat(*heartbeat)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SetHeartbeat saves the heartbeat into the database
func (e *Enforcer) SetHeartbeat(heartbeat types.Heartbeat) error {
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

// GetHeartbeat returns the latest heartbeat
func (e *Enforcer) GetHeartbeat() (*types.Heartbeat, error) {
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

// GetHeartbeatResult returns the heartbeat result
func (e *Enforcer) GetHeartbeatResult() (*types.Heartbeat, error) {
	heartbeat, err := e.GetHeartbeat()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if heartbeat == nil {
		heartbeat = types.NewHeartbeat()
	}
	if isExpired(heartbeat) {
		heartbeat.Spec.Notifications = append(heartbeat.Spec.Notifications,
			types.Notification{
				Severity: types.SeverityError,
				Text:     tosViolationMessageText,
				HTML:     tosViolationMessageHTML,
			})
	}
	return heartbeat, nil
}

func (e *Enforcer) processHeartbeatResult() error {
	heartbeat, err := e.GetHeartbeatResult()
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

// tosViolationMessageText is a warning message that gets displayed when teleport
// has failed to contact control plane for 48 hours
var tosViolationMessageText = fmt.Sprintf(`You have exceeded the usage restrictions on your Teleport license. `+
	`Please create a support ticket at our support center (%v) `+
	`so that we can resolve the issue. Failure to resolve this is a violation of our Terms of Service for Teleport (%v).`,
	constants.HoustonSupportURL, constants.GravitationalTOS)

// tosViolationMessageHTML is a warning message that gets displayed when teleport
// has failed to contact control plane for 48 hours
var tosViolationMessageHTML = fmt.Sprintf(`You have exceeded the usage restrictions on your Teleport license. `+
	`Please create a support ticket at our <a href="%v"> Support Center </a> `+
	`so that we can resolve the issue. Failure to resolve this is a violation of our <a href="%v"> Terms of Service </a> for Teleport.`,
	constants.HoustonSupportURL, constants.GravitationalTOS)
