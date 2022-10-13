package licensefile

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/lib/constants"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// RunLicenseChecker is used for running periodic checks that generate license warning alerts.
func RunLicenseChecker(ctx context.Context, alertHandler services.StatusInternal, license *LicenseFile) {
	if err := checkLicense(ctx, alertHandler, license); err != nil {
		log.WithError(err).Warn("Failed to check the license")
	}

	licenseTicker := interval.New(interval.Config{
		Jitter:   retryutils.NewSeventhJitter(),
		Duration: constants.LicenseCheckInterval,
	})
	defer licenseTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-licenseTicker.Next():
			if err := checkLicense(ctx, alertHandler, license); err != nil {
				log.WithError(err).Warn("Failed to check the license.")
			}
		}
	}
}

func checkLicense(ctx context.Context, alertHandler services.StatusInternal, license *LicenseFile) error {
	if err := clearOldLicenseAlerts(ctx, alertHandler); err != nil {
		return trace.Wrap(err)
	}
	switch {
	case license.IsExpired():
		alert, err := types.NewClusterAlert(
			constants.LicenseWarningAlertIDFormat,
			constants.LicenseExpiredMessageFormat,
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
		alert, err := types.NewClusterAlert(
			constants.LicenseWarningAlertIDFormat,
			fmt.Sprintf(constants.LicenseWarningMessageFormat, int(license.ExpiresIn().Hours()/24)),
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
