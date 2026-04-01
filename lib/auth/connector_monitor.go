/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

const (
	// samlCertCheckInterval is the frequency for the SAML cert expiry check to run.
	samlCertCheckInterval = 24 * time.Hour
	// samlCertExpiryTimeframe is the duration before expiry at which a SAML cert
	// is considered to be 'expiring'. Somewhat arbitrarily set to 90 days.
	// TODO(nixpig): Make timeframe configurable in future.
	samlCertExpiryTimeframe = 90 * 24 * time.Hour
	// samlCertExpiryAlertIDPrefix is the ID prefix used for the alert.
	samlCertExpiryAlertIDPrefix = "saml-cert-expiry-warning-"
	// samlCertExpiryAlertExpires is the expiration time for the alert.
	// It's set to 2x the check cycle so any stale alerts will clear automatically without
	// affecting valid alerts.
	samlCertExpiryAlertExpires = samlCertCheckInterval * 2
	// samlCertMonitorLockTTL is the duration after which the backend lock will be released.
	samlCertMonitorLockTTL = time.Minute
	// samlCertMonitorLockRetryInterval is the interval between failing to acquire a backend
	// lock and trying again.
	samlCertMonitorLockRetryInterval = 20 * time.Second
	// samlCertMonitorLockRefreshInterval is the interval at which the backend lock will be
	// refreshed while the monitor is still running.
	samlCertMonitorLockRefreshInterval = 20 * time.Second
	// retryJitter is the jitter duration applied when restarting the monitor after failure.
	retryJitter = 5 * time.Second
	// samlCertExpiryAlertLink is the documentation linked to from the "Learn More" link on the alert
	samlCertExpiryAlertLink = "https://goteleport.com/docs/zero-trust-access/sso/saml-cert-rotation/"
	// samlCertExpiryAlertLabel is the label for the documentation link on the alert
	samlCertExpiryAlertLabel = "Learn More"
)

// SAMLCertExpiryMonitorConfig is embedded in the SAMLCertExpiryMonitor to provide access
// to the services.
type SAMLCertExpiryMonitorConfig struct {
	// Connectors provides methods for interacting with SAML connectors.
	Connectors services.Identity
	// Alerts provides methods for managing cluster alerts.
	Alerts services.Status
	// Events provides the watch mechanism to respond to SAML connector events.
	Events types.Events
	// Backend is used to specify on which backend to acquire a lock for the monitor sync.
	Backend backend.Backend
	// Clock provides the time source for the ticker and expiry calculations.
	Clock clockwork.Clock
	// Logger provides the default logger to use. Logger is optional if CheckAndSetDefaults is called.
	Logger *slog.Logger
}

// CheckAndSetDefaults checks the fields on the SAMLCertExpiryMonitorConfig.
// For any missing required fields, it returns an error.
// For any missing optional fields, it sets a default.
func (c *SAMLCertExpiryMonitorConfig) CheckAndSetDefaults() error {
	switch {
	case c.Connectors == nil:
		return trace.BadParameter("Connectors is required")
	case c.Alerts == nil:
		return trace.BadParameter("Alerts is required")
	case c.Events == nil:
		return trace.BadParameter("Events is required")
	case c.Clock == nil:
		return trace.BadParameter("Clock is required")
	case c.Backend == nil:
		return trace.BadParameter("Backend is required")
	}

	if c.Logger == nil {
		c.Logger = slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, "saml-cert-expiry-monitor"))
	}

	return nil
}

// SAMLCertExpiryMonitor watches for changes to SAML connectors and raises a cluster
// alert when any connector has a certificate that is expiring or expired.
type SAMLCertExpiryMonitor struct {
	SAMLCertExpiryMonitorConfig
}

// NewSAMLCertExpiryMonitor creates a SAMLCertExpiryMonitor with the provided config.
// If no logger is provided, then a new logger is set up.
func NewSAMLCertExpiryMonitor(cfg SAMLCertExpiryMonitorConfig) (*SAMLCertExpiryMonitor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &SAMLCertExpiryMonitor{cfg}, nil
}

// Run acquires a backend lock for the SAML cert expiry monitor and runs the
// monitor loop while the lock is held.
func (m *SAMLCertExpiryMonitor) Run(ctx context.Context) error {
	runWhileLockedConfig := backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            m.Backend,
			LockNameComponents: []string{"saml-cert-expiry-monitor"},
			TTL:                samlCertMonitorLockTTL,
			RetryInterval:      samlCertMonitorLockRetryInterval,
		},
		RefreshLockInterval: samlCertMonitorLockRefreshInterval,
	}

	for {
		err := backend.RunWhileLocked(ctx, runWhileLockedConfig, m.runSyncLoop)
		if err != nil && ctx.Err() == nil {
			m.Logger.ErrorContext(ctx, "SAML cert expiry monitor exited unexpectedly, retrying", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryutils.SeventhJitter(retryJitter)):
		}
	}
}

// runSyncLoop creates a watcher for SAML connector events and reconciles the expiry alerts on each
// put or delete event, and periodically at samlCertCheckInterval. An error is returned if the watcher
// fails to create or unexpectedly closes.
func (m *SAMLCertExpiryMonitor) runSyncLoop(ctx context.Context) error {
	ticker := m.Clock.NewTicker(samlCertCheckInterval)
	defer ticker.Stop()

	watch, err := m.Events.NewWatcher(ctx, types.Watch{
		Name:  "saml_cert_expiry_watcher",
		Kinds: []types.WatchKind{{Kind: types.KindSAML}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watch.Close()

	if err := waitForWatcherInit(ctx, watch); err != nil {
		return trace.Wrap(err)
	}

	m.reconcileAlerts(ctx)

	for {
		select {
		case ev := <-watch.Events():
			switch ev.Type {
			case types.OpPut, types.OpDelete:
				m.reconcileAlerts(ctx)
				ticker.Reset(samlCertCheckInterval)
			default:
				continue
			}
		case <-ticker.Chan():
			m.reconcileAlerts(ctx)
		case <-watch.Done():
			if err := watch.Error(); err != nil {
				return trace.Wrap(err)
			}
			return trace.Errorf("watcher closed unexpectedly")
		case <-ctx.Done():
			return nil
		}
	}
}

// reconcileAlerts checks all SAML connectors for any that have certs expiring or expired
// and creates or updates alerts. If none are expiring, then any existing alerts are deleted.
func (m *SAMLCertExpiryMonitor) reconcileAlerts(ctx context.Context) {
	alerts := map[string]string{}

	// For each of the SAML connectors, we check the certs stored in the various locations:
	//  - SAML entity descriptor XML on the connector.
	//  - Cert field on the connector.
	//  - TODO(nixpig): Signing key pair field on the connector.
	for connector, err := range m.Connectors.RangeSAMLConnectorsWithOptions(ctx, "", "", false, types.SAMLConnectorValidationFollowURLs(false)) {
		if err != nil {
			m.Logger.ErrorContext(ctx, "Failed to get connector", "error", err)
			continue
		}

		// Get and validate the certs from the SAML entity descriptor.
		certs, err := services.CheckSAMLEntityDescriptor(connector.GetEntityDescriptor())
		if err != nil {
			m.Logger.ErrorContext(ctx, "Failed to check SAML entity descriptor", "connector", connector.GetName(), "error", err)
		}

		// Get and validate the cert from the SAML connector Cert field.
		if connector.GetCert() != "" {
			cert, err := tlsca.ParseCertificatePEM([]byte(connector.GetCert()))
			if err != nil {
				m.Logger.ErrorContext(ctx, "Failed to parse certificate defined in cert", "connector", connector.GetName(), "error", err)
			} else {
				certs = append(certs, cert)
			}
		}

		for _, cert := range certs {
			if cert.NotAfter.Sub(m.Clock.Now()) <= samlCertExpiryTimeframe {
				alertID := buildAlertID(connector.GetName(), cert)
				alertMessage := m.buildAlertMessage(connector.GetName(), cert)
				alerts[alertID] = alertMessage
			}
		}
	}

	for id, message := range alerts {
		if err := m.upsertAlert(ctx, id, message); err != nil {
			m.Logger.ErrorContext(ctx, "Failed to upsert connector expiry alert", "alert_id", id, "error", err)
		}
	}

	clusterAlerts, err := m.Alerts.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{})
	if err != nil {
		m.Logger.ErrorContext(ctx, "Failed to get cluster alerts", "error", err)
		return
	}

	for _, clusterAlert := range clusterAlerts {
		id := clusterAlert.GetName()
		if !strings.HasPrefix(id, samlCertExpiryAlertIDPrefix) {
			continue
		}

		if _, ok := alerts[id]; ok {
			continue
		}

		if err := m.Alerts.DeleteClusterAlert(ctx, id); err != nil && !trace.IsNotFound(err) {
			m.Logger.ErrorContext(ctx, "Failed to delete cluster alert", "alert_id", clusterAlert.GetName(), "error", err)
		}
	}
}

// upsertAlert creates or updates a SAML cert expiry cluster alert by id with the provided message.
func (m *SAMLCertExpiryMonitor) upsertAlert(ctx context.Context, id, message string) error {
	alert, err := types.NewClusterAlert(
		id,
		message,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead)),
		types.WithAlertExpires(m.Clock.Now().Add(samlCertExpiryAlertExpires)),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		types.WithAlertLabel(types.AlertLink, samlCertExpiryAlertLink),
		types.WithAlertLabel(types.AlertLinkText, samlCertExpiryAlertLabel),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(m.Alerts.UpsertClusterAlert(ctx, alert))
}

// buildAlertMessage returns a SAML cert expiry alert message for the given connector name and cert.
func (m *SAMLCertExpiryMonitor) buildAlertMessage(connectorName string, cert *x509.Certificate) string {
	expiredTemplate := "SAML SSO users are no longer able to authenticate to Teleport. Connector '%s' references a certificate that expired at %s. Please rotate the expired certificate. %s"
	expiringTemplate := "SAML SSO users will no longer be able to authenticate to Teleport in %d days. Connector '%s' references a certificate that expires at %s. Please rotate the expiring certificate. %s"
	learnMore := "Click 'Learn More' for more details about rotating certificates."

	remaining := cert.NotAfter.Sub(m.Clock.Now())
	expiry := cert.NotAfter.UTC().Format("2006-01-02 15:04:05")

	if remaining <= 0 {
		return fmt.Sprintf(expiredTemplate, connectorName, expiry, learnMore)
	}

	return fmt.Sprintf(expiringTemplate, int(remaining.Hours()/24), connectorName, expiry, learnMore)
}

// waitForWatcherInit waits to receive the OpInit event from the watcher.
func waitForWatcherInit(ctx context.Context, watch types.Watcher) error {
	select {
	case <-watch.Done():
		if err := watch.Error(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Errorf("watcher closed unexpectedly")
	case evt := <-watch.Events():
		if evt.Type != types.OpInit {
			return trace.BadParameter("expected init event, got %v", evt.Type)
		}
		return nil
	case <-ctx.Done():
		return nil
	}
}

// buildAlertID creates an ID to use for a SAML cert expiry alert.
func buildAlertID(connectorName string, cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return samlCertExpiryAlertIDPrefix + connectorName + "-" + hex.EncodeToString(sum[:12])
}
