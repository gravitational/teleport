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
)

const (
	// samlCertCheckInterval is the frequency for the SAML cert expiry check to run.
	samlCertCheckInterval = 24 * time.Hour
	// samlCertExpiryTimeframe is the duration before expiry at which a SAML cert
	// is considered to be 'expiring'. Somewhat arbitrarily set to 90 days.
	// TODO(nixpig): Make timeframe configurable in future.
	samlCertExpiryTimeframe = 90 * 24 * time.Hour
	// samlCertExpiryAlertID is the ID used for the alert.
	samlCertExpiryAlertID = "saml-cert-expiry-warning"
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
)

// SAMLCertExpiryMonitorConfig is embedded in the SAMLCertExpiryMonitor to provide access
// to the services.
type SAMLCertExpiryMonitorConfig struct {
	Connectors services.Identity
	Alerts     services.Status
	Events     types.Events
	Backend    backend.Backend
	Clock      clockwork.Clock
	Logger     *slog.Logger
}

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
		err := backend.RunWhileLocked(ctx, runWhileLockedConfig, m.run)
		if err != nil && ctx.Err() == nil {
			m.Logger.ErrorContext(ctx, "SAML cert expiry monitor exited unexpectedly, retrying", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryutils.SeventhJitter(5 * time.Second)):
		}
	}
}

// run creates the ticker used for periodic reconcilliation and starts the watch loop
// that reconciles the SAML cert expiry alert.
func (m *SAMLCertExpiryMonitor) run(ctx context.Context) error {
	// Start ticker upfront so none are missed due to sparseness of interval.
	ticker := m.Clock.NewTicker(samlCertCheckInterval)
	defer ticker.Stop()

	return trace.Wrap(m.runSyncLoop(ctx, ticker))
}

// runSyncLoop creates a watcher for SAML connector events and reconciles the expiry alert on each
// put or delete event, and on each tick of the provided ticker. An error is returned if the watcher
// fails to create or unexpectedly closes.
func (m *SAMLCertExpiryMonitor) runSyncLoop(ctx context.Context, ticker clockwork.Ticker) error {
	watch, err := m.Events.NewWatcher(ctx, types.Watch{
		Name:  "saml_cert_expiry_watcher",
		Kinds: []types.WatchKind{{Kind: types.KindSAML}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watch.Close()

	select {
	case <-watch.Done():
		if err := watch.Error(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	case evt := <-watch.Events():
		if evt.Type == types.OpInit {
			break
		}
		return trace.BadParameter("expected init event, got %v", evt.Type)
	case <-ctx.Done():
		return nil
	}

	if err := m.reconcileAlert(ctx); err != nil {
		m.Logger.ErrorContext(ctx, "Failed initial reconciliation of SAML cert expiry alert", "error", err)
	}

	for {
		select {
		case ev := <-watch.Events():
			switch ev.Type {
			case types.OpPut, types.OpDelete:
				if err := m.reconcileAlert(ctx); err != nil {
					m.Logger.ErrorContext(ctx, "Failed to reconcile SAML cert expiry alert", "error", err)
				} else {
					ticker.Reset(samlCertCheckInterval)
				}
			default:
				continue
			}
		case <-ticker.Chan():
			if err := m.reconcileAlert(ctx); err != nil {
				m.Logger.ErrorContext(ctx, "Failed to reconcile SAML cert expiry alert", "error", err)
			}
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

// reconcileAlert checks all SAML connectors for any that have certs expiring or expired
// and creates or updates an alert. If none are expiring, then any existing alert is deleted.
func (m *SAMLCertExpiryMonitor) reconcileAlert(ctx context.Context) error {
	var expiringConnectors []string
	var expiredConnectors []string
	for connector, err := range m.Connectors.RangeSAMLConnectorsWithOptions(ctx, "", "", false, types.SAMLConnectorValidationFollowURLs(false)) {
		if err != nil {
			return trace.Wrap(err)
		}

		certs, err := services.GetExpiringSAMLCertsAt(connector, m.Clock.Now(), samlCertExpiryTimeframe)
		if err != nil {
			return trace.Wrap(err)
		}

		if len(certs) == 0 {
			continue
		}

		var isExpired bool
		for _, cert := range certs {
			if cert.TTL <= 0 {
				isExpired = true
				break
			}

		}

		if isExpired {
			expiredConnectors = append(expiredConnectors, connector.GetName())
		} else {
			expiringConnectors = append(expiringConnectors, connector.GetName())
		}
	}

	if len(expiringConnectors) > 0 || len(expiredConnectors) > 0 {
		return trace.Wrap(m.upsertAlert(ctx, buildAlertMessage(expiringConnectors, expiredConnectors)))
	}

	alerts, err := m.Alerts.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: samlCertExpiryAlertID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(alerts) > 0 {
		return trace.Wrap(m.Alerts.DeleteClusterAlert(ctx, samlCertExpiryAlertID))
	}

	return nil
}

// upsertAlert creates or updates the SAML cert expiry cluster alert with the provided message.
func (m *SAMLCertExpiryMonitor) upsertAlert(ctx context.Context, message string) error {
	alert, err := types.NewClusterAlert(
		samlCertExpiryAlertID,
		message,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead)),
		types.WithAlertExpires(m.Clock.Now().Add(samlCertExpiryAlertExpires)),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
		// TODO: Link to better documentation page when it's created.
		types.WithAlertLabel(types.AlertLink, "https://goteleport.com/docs/reference/infrastructure-as-code/teleport-resources/saml-connector-v2/#saml-connector-spec-v2"),
		types.WithAlertLabel(types.AlertLinkText, "Learn More"),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(m.Alerts.UpsertClusterAlert(ctx, alert))
}

// buildAlertMessage returns a SAML cert expiry alert message for the given connector names.
func buildAlertMessage(expiringConnectors, expiredConnectors []string) string {
	messageParts := []string{fmt.Sprintf(
		"The following connectors have one or more signing certificates that have expired or will expire in the next %d days. If a signing certificate expires, users will no longer be able to authenticate to Teleport with the affected connector.",
		int(samlCertExpiryTimeframe/(24*time.Hour)),
	)}

	if len(expiredConnectors) > 0 {
		messageParts = append(messageParts, fmt.Sprintf("Expired: %s.", strings.Join(expiredConnectors, ", ")))
	}

	if len(expiringConnectors) > 0 {
		messageParts = append(messageParts, fmt.Sprintf("Expiring: %s.", strings.Join(expiringConnectors, ", ")))
	}

	return strings.Join(messageParts, " ")
}
