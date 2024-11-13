package licensefile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// RunLicenseChecker is used for running periodic checks that generate license warning alerts.
func RunLicenseChecker(ctx context.Context, alertHandler services.StatusInternal, license *LicenseFile) {
	if err := checkLicense(ctx, alertHandler, license); err != nil {
		slog.WarnContext(ctx, "Failed to check the license", "error", err)
	}

	licenseTicker := interval.New(interval.Config{
		Jitter:   retryutils.SeventhJitter,
		Duration: constants.LicenseCheckInterval,
	})
	// Use an anonymous func as licenseTicker is re-bound later
	defer func() { licenseTicker.Stop() }()

	for {
		select {
		case <-ctx.Done():
			return
		case <-licenseTicker.Next():
			if err := checkLicense(ctx, alertHandler, license); err != nil {
				slog.WarnContext(ctx, "Failed to check the license", "error", err)
			}
			// Ensure there is a license check exactly when the license expires
			// and when the grace period expires so that alerts are emitted in
			// a timely manner. We do this by restarting the ticker with a
			// FirstDuration equal to the next expiry event.
			exp := license.ExpiresIn()
			if exp <= 0 {
				exp = license.DisabledIn()
			}
			if 0 < exp && exp < constants.LicenseCheckInterval {
				licenseTicker.Stop()
				licenseTicker = interval.New(interval.Config{
					Jitter:        retryutils.SeventhJitter,
					Duration:      constants.LicenseCheckInterval,
					FirstDuration: exp,
				})
			}
		}
	}
}

func checkLicense(ctx context.Context, alertHandler services.StatusInternal, license *LicenseFile) error {
	if err := clearOldLicenseAlerts(ctx, alertHandler); err != nil {
		return trace.Wrap(err)
	}
	switch {
	case license.IsDisabled():
		alert, err := types.NewClusterAlert(
			constants.LicenseWarningAlertIDFormat,
			constants.LicenseDisabledMessageFormat,
			types.WithAlertSeverity(types.AlertSeverity_HIGH),
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertPermitAll, "yes"),
			types.WithAlertLabel(types.AlertLicenseExpired, "yes"),
			types.WithAlertExpires(time.Now().Add(constants.LicenseCheckInterval)),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := alertHandler.UpsertClusterAlert(ctx, alert); err != nil {
			return trace.Wrap(err, "failed to create cluster alert: %v", alert.GetName())
		}
		return nil

	case license.IsExpired():
		dmsg := durationMessage(license.DisabledIn())
		alert, err := types.NewClusterAlert(
			constants.LicenseWarningAlertIDFormat,
			fmt.Sprintf(constants.LicenseExpiredMessageFormat, dmsg),
			types.WithAlertSeverity(types.AlertSeverity_HIGH),
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertPermitAll, "yes"),
			types.WithAlertLabel(types.AlertLicenseExpired, "yes"),
			types.WithAlertExpires(time.Now().Add(constants.LicenseCheckInterval)),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := alertHandler.UpsertClusterAlert(ctx, alert); err != nil {
			return trace.Wrap(err, "failed to create cluster alert: %v", alert.GetName())
		}
		return nil

	case license.ExpiresIn() < constants.LicenseWarningInterval:
		dmsg := durationMessage(license.ExpiresIn())
		alert, err := types.NewClusterAlert(
			constants.LicenseWarningAlertIDFormat,
			fmt.Sprintf(constants.LicenseWarningMessageFormat, dmsg),
			types.WithAlertSeverity(types.AlertSeverity_MEDIUM),
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertPermitAll, "yes"),
			types.WithAlertExpires(time.Now().Add(constants.LicenseCheckInterval)),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := alertHandler.UpsertClusterAlert(ctx, alert); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// durationMessage formats a duration as a number of days until that duration,
// either as "today", "in 1 day" or "in N days".
func durationMessage(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days == 0 {
		return "today"
	}
	if days == 1 {
		return "in 1 day"
	}
	return fmt.Sprintf("in %d days", days)
}

func clearOldLicenseAlerts(ctx context.Context, alertHandler services.StatusInternal) error {
	query := types.GetClusterAlertsRequest{
		Labels: map[string]string{
			types.AlertOnLogin:   "yes",
			types.AlertPermitAll: "yes",
		},
	}
	alerts, err := alertHandler.GetClusterAlerts(ctx, query)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, alert := range alerts {
		if alert.GetName() == constants.LicenseWarningAlertIDFormat {
			if err := alertHandler.DeleteClusterAlert(ctx, alert.GetName()); err != nil {
				return trace.Wrap(err)
			}
			break
		}
	}
	return nil
}
