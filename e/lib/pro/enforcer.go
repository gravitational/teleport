package pro

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils"

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
	// Presence is a service which can be used to retrieve information about
	// Cluster components
	services.Presence
	// ClusterID is the ID of the cluster that this process is running on
	ClusterID string
	// Anonymizer is used for anonymizing sent data
	Anonymizer utils.Anonymizer
}

// EnforcerConfig is enforcer configuration
type EnforcerConfig struct {
	// Anonymizer is used for anonymizing sent data
	Anonymizer utils.Anonymizer
	// Backend is the configured backend
	Backend backend.Backend
	// LicenseKeyPair is the license key pair
	LicenseKeyPair *liblicense.License
	// Insecure is whether to skip cert verification
	// when talking to the control plane
	Insecure bool
	// NoStart is used in tests to skip starting goroutines
	NoStart bool
	// ClusterID is the ID of the cluster that this process is running on
	ClusterID string
}

// Check makes sure that enforcer config is valid
func (c *EnforcerConfig) Check() error {
	if c.Anonymizer == nil {
		return trace.BadParameter("enforcer config is missing anonymizer")
	}
	if c.Backend == nil {
		return trace.BadParameter("enforcer config is missing backend")
	}
	if c.LicenseKeyPair == nil {
		return trace.BadParameter("enforcer config is missing license")
	}
	if c.ClusterID == "" {
		return trace.BadParameter("enforcer config is missing cluster ID")
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
		Anonymizer: config.Anonymizer,
		ClusterID:  config.ClusterID,
		WebClient:  client,
		Backend:    config.Backend,
		Presence:   local.NewPresenceService(config.Backend),
		Entry: log.WithFields(log.Fields{
			trace.Component: "enforcer",
		}),
	}
	if !config.NoStart {
		enforcer.Debug("Starting enforcer.")
		go enforcer.startRecordingUsage(ctx)
		go enforcer.startReportingUsage(ctx)
	}
	return enforcer, nil
}

func (e *Enforcer) startRecordingUsage(ctx context.Context) {
	ticker := e.Clock().NewTicker(constants.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.Chan():
			err := e.processLicenseCheckResult(ctx)
			if err != nil {
				log.Error(trace.DebugReport(err))
			}

			e.RecordUsage(ctx, constants.HeartbeatInterval)
		case <-ctx.Done():
			e.Debug("Enforce loop is exiting.")
			return
		}
	}
}

func (e *Enforcer) startReportingUsage(ctx context.Context) {
	ticker := e.Clock().NewTicker(constants.ReportingInterval)
	defer ticker.Stop()

	for {
		// We will send a heartbeat to Houston when starting the
		// application to verify that the license is valid.
		err := e.ReportUsage(ctx)
		if err != nil {
			log.Debug(trace.DebugReport(err))
		}

		select {
		case <-ticker.Chan():
			continue
		case <-ctx.Done():
			e.Debug("Heartbeat loop is exiting.")
			return
		}
	}
}

// RecordUsage records additional usage in the datastore
func (e *Enforcer) RecordUsage(ctx context.Context, newUsage time.Duration) {
	record, err := e.GetUsageRecord(ctx)
	if err != nil {
		log.WithError(err).Error("Failed to retrieve existing usage record.")
		return
	}

	backup := make(map[string]time.Duration)
	for k, v := range record {
		backup[k] = v
	}

	namespaces, err := e.Presence.GetNamespaces()
	if err != nil {
		log.WithError(err).Error("Failed to retrieve namespaces.")
		return
	}

	for _, namespace := range namespaces {
		nodes, err := e.Presence.GetNodes(namespace.GetName())
		if err != nil {
			log.WithError(err).Errorf("Failed to get nodes for namespace %q.", namespace)
			continue
		}

		for _, node := range nodes {
			record[node.String()] += newUsage
		}
	}

	err = e.SetUsageRecord(ctx, backup, record)
	if err != nil {
		log.WithError(err).Debug("Failed to update usage record.")
	}
}

// ReportUsage gets the current usage duration from the datastore and sends it
// to Houston
func (e *Enforcer) ReportUsage(ctx context.Context) error {
	// Record is the JSON record of a heartbeat.
	type Record struct {
		// EndTime is the end of a usage period
		EndTime time.Time `json:"end_time"`
		// StartTime is the start of a usage period
		StartTime time.Time `json:"start_time"`
		// ClusterID is the ID of the cluster
		ClusterID string `json:"cluster_id"`
		// HostID is the UUID of this specific host / node
		HostID string `json:"host_id"`
	}

	record, err := e.GetUsageRecord(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	now := e.Clock().Now().UTC()
	body := make([]Record, 0, len(record))

	for hostID, duration := range record {
		body = append(body, Record{
			EndTime:   now,
			StartTime: now.Add(-duration),
			HostID:    e.Anonymizer.Anonymize([]byte(hostID)),
			ClusterID: e.Anonymizer.Anonymize([]byte(e.ClusterID)),
		})
	}

	// It is important that we attempt to update the usage record before
	// we post the usage to houston. This is because multiple auth servers
	// might attempt to report usage at the same time - only
	// `SetUsageRecord` is concurrency safe. If this call succeeds, we can
	// go ahead and contact Houston.
	err = e.SetUsageRecord(ctx, record, make(map[string]time.Duration))
	if err != nil {
		return trace.Wrap(err)
	}

	out, err := e.WebClient.PostJSON(ctx, e.Endpoint("heartbeat"), body)
	if err != nil {
		item, mkErr := makeItemFromUsageRecord(record)
		if mkErr != nil {
			return trace.NewAggregate(err, mkErr)
		}

		// Instead of using `SetUsageRecord`, we write directly to the
		// datastore to force restore the old value
		if _, putErr := e.Put(ctx, item); putErr != nil {
			return trace.NewAggregate(err, putErr)
		}

		return trace.Wrap(err)
	}

	heartbeat, err := types.UnmarshalHeartbeat(out.Bytes())
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(e.SetLicenseCheckHeartbeat(*heartbeat))
}

const (
	heartbeatPrefix = "heartbeat"
	valPrefix       = "val"
	usagePrefix     = "usage"
)

// SetUsageRecord sets the unreported usage record to a new value
func (e *Enforcer) SetUsageRecord(ctx context.Context, old, new map[string]time.Duration) error {
	newItem, err := makeItemFromUsageRecord(new)
	if err != nil {
		return trace.Wrap(err)
	}

	oldItem, err := makeItemFromUsageRecord(old)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = e.CompareAndSwap(ctx, oldItem, newItem)
	if trace.IsCompareFailed(err) {
		_, getErr := e.Backend.Get(ctx, backend.Key(heartbeatPrefix, usagePrefix))
		if !trace.IsNotFound(getErr) {
			return trace.Wrap(err)
		}

		// This is the case when putting usage data into the backend for
		// the first time
		_, err = e.Backend.Put(ctx, newItem)
	}

	return trace.Wrap(err)
}

// GetUsageRecord returns the usage duration record for unreported usage
func (e *Enforcer) GetUsageRecord(ctx context.Context) (map[string]time.Duration, error) {
	var record map[string]time.Duration

	item, err := e.Backend.Get(ctx, backend.Key(heartbeatPrefix, usagePrefix))
	if err != nil {
		if trace.IsNotFound(err) {
			return make(map[string]time.Duration), nil
		}

		return nil, trace.Wrap(err)
	}

	err = json.Unmarshal(item.Value, &record)
	if err != nil {
		// This special case happens in case someone ran a previous
		// version of Teleport Enterprise with usage-based billing
		// (this was the case for v4.1.0-alpha.1 to v4.1.0-alpha5)
		// Since those are not production builds, we just reset the
		// usage.
		_, parseErr := time.ParseDuration(string(item.Value))
		if parseErr == nil {
			return make(map[string]time.Duration), nil
		}

		return nil, trace.Wrap(err)
	}

	return record, nil
}

// SetLicenseCheckHeartbeat saves the license check heartbeat into the database
func (e *Enforcer) SetLicenseCheckHeartbeat(heartbeat types.Heartbeat) error {
	value, err := types.MarshalHeartbeat(heartbeat)
	if err != nil {
		return trace.Wrap(err)
	}
	item := backend.Item{
		Key:   backend.Key(heartbeatPrefix, valPrefix),
		Value: value,
	}
	_, err = e.Put(context.TODO(), item)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getLicenseCheckHeartbeat returns the latest license check heartbeat
func (e *Enforcer) getLicenseCheckHeartbeat(ctx context.Context) (*types.Heartbeat, error) {
	item, err := e.Backend.Get(ctx, backend.Key(heartbeatPrefix, valPrefix))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	heartbeat, err := types.UnmarshalHeartbeat(item.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return heartbeat, nil
}

// GetLicenseCheckResult returns the last license check result
func (e *Enforcer) GetLicenseCheckResult(ctx context.Context) (*types.Heartbeat, error) {
	heartbeat, err := e.getLicenseCheckHeartbeat(ctx)
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
func (e *Enforcer) processLicenseCheckResult(ctx context.Context) error {
	heartbeat, err := e.GetLicenseCheckResult(ctx)
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

func makeItemFromUsageRecord(m map[string]time.Duration) (backend.Item, error) {
	value, err := json.Marshal(m)
	if err != nil {
		return backend.Item{}, trace.Wrap(err)
	}

	return backend.Item{
		Key:   backend.Key(heartbeatPrefix, usagePrefix),
		Value: value,
	}, nil
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
