package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
)

const (
	// samlCertCheckCycle is the frequency for the SAML cert expiry check to run.
	samlCertCheckCycle = 24 * time.Hour
	// samlCertExpiryTimeframe is the duration before expiry at which a SAML cert
	// is considered to be 'expiring'. Somewhat arbitrarily set to 90 days.
	// TODO(nixpig): Make timeframe configurable in future.
	samlCertExpiryTimeframe = 90 * 24 * time.Hour
	// samlCertExpiryAlertName is the ID used for the alert.
	samlCertExpiryAlertName = "saml-cert-expiry-warning"
	// samlCertExpiryAlertExpires is the expiration time for the alert.
	// It's set to 2x the check cycle so any stale alerts will clear automatically without
	// affecting valid alerts.
	samlCertExpiryAlertExpires = samlCertCheckCycle * 2
)

// MonitorSAMLCertExpiry periodically runs the SAML certificate expiry check.
func (a *Server) MonitorSAMLCertExpiry(ctx context.Context) error {
	checkInterval := interval.New(interval.Config{
		FirstDuration: 10 * time.Second,
		Duration:      samlCertCheckCycle,
		Clock:         a.GetClock(),
	})
	defer checkInterval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-checkInterval.Next():
			if err := a.checkSAMLCertExpiry(ctx); err != nil {
				a.logger.ErrorContext(ctx, "Failed to check SAML cert expiry", "error", err)
			}
		}
	}
}

// CheckSAMLCertExpiry checks all SAML connectors for any that have certs expiring or expired
// and creates or updates an alert. If none are expiring, then any existing alert is deleted.
func (a *Server) checkSAMLCertExpiry(ctx context.Context) error {
	connectors, err := a.GetSAMLConnectors(ctx, false)
	if err != nil {
		return trace.Wrap(err)
	}

	var expiringConnectors []string
	for _, connector := range connectors {
		if expiring, err := services.CheckSAMLCertExpiry(connector, samlCertExpiryTimeframe); err != nil {
			a.logger.WarnContext(ctx, "Failed to check SAML connector cert expiry", "connector", connector.GetName(), "error", err)
		} else if expiring {
			expiringConnectors = append(expiringConnectors, connector.GetName())
		}
	}

	if len(expiringConnectors) > 0 {
		message := buildSAMLCertExpiryAlertMessage(expiringConnectors)
		return trace.Wrap(a.upsertSAMLCertExpiryAlert(ctx, message))
	}

	if err := a.Services.DeleteClusterAlert(ctx, samlCertExpiryAlertName); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

func (a *Server) upsertSAMLCertExpiryAlert(ctx context.Context, message string) error {
	alert, err := types.NewClusterAlert(
		samlCertExpiryAlertName,
		message,
		types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
		types.WithAlertLabel(types.AlertVerbPermit, fmt.Sprintf("%s:%s", types.KindSAML, types.VerbRead)),
		types.WithAlertExpires(a.GetClock().Now().Add(samlCertExpiryAlertExpires)),
		types.WithAlertLabel(types.AlertOnLogin, "yes"),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.UpsertClusterAlert(ctx, alert))
}

// buildSAMLCertExpiryAlertMessage uses the list of connector names to create the SAML cert expiry alert message.
func buildSAMLCertExpiryAlertMessage(connectorNames []string) string {
	return fmt.Sprintf(
		"The following connectors have one or more certificates that have expired or will expire in the next %d days: %s.",
		int(samlCertExpiryTimeframe/(24*time.Hour)),
		strings.Join(connectorNames, ", "),
	)
}
