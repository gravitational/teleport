/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/services"
)

const (
	// samlCertCheckCycle is the frequency for the SAML cert expiry check to run.
	samlCertCheckCycle = 24 * time.Hour
	// samlCertExpiryTimeframe is the duration before expiry at which a SAML cert
	// is considered to be 'expiring'. Somewhat arbitrarily set to 90 days.
	// TODO(nixpig): Make timeframe configurable in future.
	samlCertExpiryTimeframe = 90 * 24 * time.Hour
	// samlCertExpiryAlertID is the ID used for the alert.
	samlCertExpiryAlertID = "saml-cert-expiry-warning"
	// samlCertExpiryAlertExpires is the expiration time for the alert.
	// It's set to 2x the check cycle so any stale alerts will clear automatically without
	// affecting valid alerts.
	samlCertExpiryAlertExpires = samlCertCheckCycle * 2
)

type SAMLCertExpiryMonitorConfig struct {
	Connectors services.Identity
	Alerts     services.Status
	Events     types.Events
	Clock      clockwork.Clock
	Logger     *slog.Logger
}

// SAMLCertExpiryMonitor watches for changes to SAML connectors and raises a cluster
// alert when any connector has a certificate that is expiring or expired.
type SAMLCertExpiryMonitor struct {
	connectors services.Identity
	alerts     services.Status
	events     types.Events
	clock      clockwork.Clock
	logger     *slog.Logger
}

// NewSAMLCertExpiryMonitor creates a SAMLCertExpiryMonitor with the provided config.
// If no logger is provided, then a new logger is set up.
func NewSAMLCertExpiryMonitor(cfg SAMLCertExpiryMonitorConfig) (*SAMLCertExpiryMonitor, error) {
	switch {
	case cfg.Connectors == nil:
		return nil, trace.BadParameter("Connectors is required")
	case cfg.Alerts == nil:
		return nil, trace.BadParameter("Alerts is required")
	case cfg.Events == nil:
		return nil, trace.BadParameter("Events is required")
	case cfg.Clock == nil:
		return nil, trace.BadParameter("Clock is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.Component(teleport.ComponentAuth, "saml-cert-expiry-monitor"))
	}

	return &SAMLCertExpiryMonitor{
		connectors: cfg.Connectors,
		alerts:     cfg.Alerts,
		events:     cfg.Events,
		clock:      cfg.Clock,
		logger:     cfg.Logger,
	}, nil
}

// Run performs an initial SAML cert expiry alert reconciliation, then starts the watch loop that
// reconciles the SAML cert expiry alert periodically, and on every put or delete of SAML connector.
func (m *SAMLCertExpiryMonitor) Run(ctx context.Context) error {
	shouldRetryAfterJitterFn := func() bool {
		select {
		case <-time.After(retryutils.SeventhJitter(5 * time.Second)):
			return true
		case <-ctx.Done():
			return false
		}
	}

	if err := m.reconcileAlert(ctx); err != nil {
		m.logger.ErrorContext(ctx, "Failed initial reconciliation of SAML cert expiry alert", "error", err)
	}

	ticker := time.NewTicker(samlCertCheckCycle)
	defer ticker.Stop()

	for {
		if err := m.runWatchLoop(ctx, ticker); err != nil {
			m.logger.ErrorContext(ctx, "SAML connector watcher exited unexpectedly, retrying", "error", err)
			if !shouldRetryAfterJitterFn() {
				return nil
			}
			continue
		}
		return nil
	}
}

// runWatchLoop creates a watcher for SAML connector events and reconciles the expiry alert on each
// put or delete event, and on each tick of the provided ticker. An error is returned if the watcher
// fails to create or unexpectedly closes.
func (m *SAMLCertExpiryMonitor) runWatchLoop(ctx context.Context, ticker *time.Ticker) error {
	watch, err := m.events.NewWatcher(ctx, types.Watch{
		Name:  "saml_cert_expiry_watcher",
		Kinds: []types.WatchKind{{Kind: types.KindSAMLConnector}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watch.Close()

	for {
		select {
		case ev := <-watch.Events():
			if ev.Type != types.OpPut && ev.Type != types.OpDelete {
				continue
			}
			if err := m.reconcileAlert(ctx); err != nil {
				m.logger.ErrorContext(ctx, "Failed to reconcile SAML cert expiry alert", "error", err)
			}
		case <-ticker.C:
			if err := m.reconcileAlert(ctx); err != nil {
				m.logger.ErrorContext(ctx, "Failed to reconcile SAML cert expiry alert", "error", err)
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
	connectors, err := m.connectors.GetSAMLConnectors(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	var expiringConnectors []string
	for _, connector := range connectors {
		expiring, err := services.CheckSAMLCertExpiry(connector, samlCertExpiryTimeframe)
		if err != nil {
			return trace.Wrap(err)
		}

		if expiring {
			expiringConnectors = append(expiringConnectors, connector.GetName())
		}
	}

	if len(expiringConnectors) > 0 {
		message := m.buildAlertMessage(expiringConnectors)
		return trace.Wrap(m.upsertAlert(ctx, message))
	}

	if err := m.alerts.DeleteClusterAlert(ctx, samlCertExpiryAlertID); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
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
		types.WithAlertExpires(m.clock.Now().Add(samlCertExpiryAlertExpires)),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(m.alerts.UpsertClusterAlert(ctx, alert))
}

// buildAlertMessage returns a SAML cert expiry alert message for the given connector names.
func (m *SAMLCertExpiryMonitor) buildAlertMessage(connectorNames []string) string {
	return fmt.Sprintf(
		"The following connectors have one or more certificates that have expired or will expire in the next %d days: %s.",
		int(samlCertExpiryTimeframe/(24*time.Hour)),
		strings.Join(connectorNames, ", "),
	)
}
