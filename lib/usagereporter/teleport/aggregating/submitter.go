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

package aggregating

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// UsageReportsSubmitter is almost SubmitUsageReports from
// prehog.v1alpha.TeleportReportingService, but instead of returning the raw
// response (and requiring wrapping the request in a [connect.Request]) it
// should parse the response and return the batch UUID.
type UsageReportsSubmitter func(context.Context, *prehogv1.SubmitUsageReportsRequest) (uuid.UUID, error)

const (
	submitInterval     = 5 * time.Minute
	submitLockDuration = time.Minute
	submitBatchSize    = 10

	alertGraceHours    = 24
	alertGraceDuration = alertGraceHours * time.Hour
	alertName          = "reporting-failed"
	alertLink          = "https://goteleport.com/support/"
	alertLinkText      = "Contact Support"
)

const (
	defaultEndpointHostname = "reporting-teleport.teleportinfra.sh"
	DefaultEndpoint         = "https://" + defaultEndpointHostname
)

var alertMessage = fmt.Sprintf("Teleport has failed to contact the usage reporting server for more than %v hours. "+
	"Please make sure that the Teleport Auth Server can connect to (%v). "+
	"Otherwise, contact Teleport Support at (%v).",
	alertGraceHours, defaultEndpointHostname, alertLink)

// SubmitterConfig contains the configuration for a [Submitter].
type SubmitterConfig struct {
	// Backend is the backend to use to read reports and apply locks. Required.
	Backend backend.Backend
	// Logger is the used for emitting log messages.
	Logger *slog.Logger
	// Status is used to create or clear cluster alerts on a failure. Required.
	Status services.StatusInternal
	// Submitter is used to submit usage reports. Required.
	Submitter UsageReportsSubmitter

	// HostID is the host ID of the current Teleport instance, used in the lock
	// payload and cluster alert description to help debugging. Optional.
	HostID string
}

// CheckAndSetDefaults checks the [SubmitterConfig] for validity, returning nil
// if there's no error. Idempotent but not concurrent safe, as it might need to
// write to the config to apply defaults.
func (cfg *SubmitterConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Status == nil {
		return trace.BadParameter("missing Status")
	}
	if cfg.Submitter == nil {
		return trace.BadParameter("missing Submitter")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return nil
}

// RunSubmitter periodically fetches aggregated reports from the backend and
// sends them in small batches with the provided submitter, before deleting
// them. Must only be called after validating the config object with
// CheckAndSetDefaults, and should probably be called in a goroutine.
func RunSubmitter(ctx context.Context, cfg SubmitterConfig) {
	iv := interval.New(interval.Config{
		FirstDuration: retryutils.HalfJitter(2 * submitInterval),
		Duration:      submitInterval,
		Jitter:        retryutils.SeventhJitter,
	})
	defer iv.Stop()

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-iv.Next():
		}

		submitOnce(ctx, cfg)
	}
}

func submitOnce(ctx context.Context, c SubmitterConfig) {
	svc := reportService{c.Backend}

	userActivityReports, err := svc.listUserActivityReports(ctx, submitBatchSize)
	if err != nil {
		c.Logger.ErrorContext(ctx, "Failed to load usage reports for submission.", "error", err)
		return
	}

	freeBatchSize := submitBatchSize - len(userActivityReports)
	var resourcePresenceReports []*prehogv1.ResourcePresenceReport
	if freeBatchSize > 0 {
		resourcePresenceReports, err = svc.listResourcePresenceReports(ctx, freeBatchSize)
		if err != nil {
			c.Logger.ErrorContext(ctx, "Failed to load resource counts reports for submission.", "error", err)
			return
		}
	}

	freeBatchSize = submitBatchSize - len(userActivityReports) - len(resourcePresenceReports)
	var botInstanceActivityReports []*prehogv1.BotInstanceActivityReport
	if freeBatchSize > 0 {
		botInstanceActivityReports, err = svc.listBotInstanceActivityReports(ctx, freeBatchSize)
		if err != nil {
			c.Logger.ErrorContext(ctx, "Failed to load bot instance activity reports for submission.", "error", err)
			return
		}
	}

	totalReportCount := len(userActivityReports) + len(resourcePresenceReports) + len(botInstanceActivityReports)

	if totalReportCount < 1 {
		err := ClearAlert(ctx, c.Status)
		if err == nil {
			c.Logger.InfoContext(ctx, "Deleted cluster alert after successfully clearing usage report backlog.", "alert", alertName)
		} else if !trace.IsNotFound(err) {
			c.Logger.ErrorContext(ctx, "Failed to delete cluster alert", "alert", alertName, "error", err)
		}
		return
	}

	oldest := time.Now()
	newest := time.Time{}
	if len(userActivityReports) > 0 {
		if t := userActivityReports[0].GetStartTime().AsTime(); t.Before(oldest) {
			oldest = t
		}
		if t := userActivityReports[len(userActivityReports)-1].GetStartTime().AsTime(); t.After(newest) {
			newest = t
		}
	}
	if len(resourcePresenceReports) > 0 {
		if t := resourcePresenceReports[0].GetStartTime().AsTime(); t.Before(oldest) {
			oldest = t
		}
		if t := resourcePresenceReports[len(resourcePresenceReports)-1].GetStartTime().AsTime(); t.After(newest) {
			newest = t
		}
	}
	if len(botInstanceActivityReports) > 0 {
		if t := botInstanceActivityReports[0].GetStartTime().AsTime(); t.Before(oldest) {
			oldest = t
		}
		if t := botInstanceActivityReports[len(botInstanceActivityReports)-1].GetStartTime().AsTime(); t.After(newest) {
			newest = t
		}
	}

	debugPayload := fmt.Sprintf("%v %q", time.Now().Round(0), c.HostID)
	if err := svc.createUsageReportingLock(ctx, submitLockDuration, []byte(debugPayload)); err != nil {
		if trace.IsAlreadyExists(err) {
			c.Logger.DebugContext(ctx, "Failed to acquire lock, already held.", "lock", usageReportingLock)
		} else {
			c.Logger.ErrorContext(ctx, "Failed to acquire lock.", "lock", usageReportingLock, "error", err)
		}
		return
	}

	lockCtx, cancel := context.WithTimeout(ctx, submitLockDuration*9/10)
	defer cancel()

	batchUUID, err := c.Submitter(lockCtx, &prehogv1.SubmitUsageReportsRequest{
		UserActivity:        userActivityReports,
		ResourcePresence:    resourcePresenceReports,
		BotInstanceActivity: botInstanceActivityReports,
	})
	if err != nil {
		c.Logger.ErrorContext(ctx, "Failed to send usage reports.",
			"reports", totalReportCount,
			"oldest_report", oldest,
			"newest_report", newest,
			"error", err,
		)

		if time.Since(oldest) <= alertGraceDuration {
			return
		}
		alert, err := types.NewClusterAlert(
			alertName,
			alertMessage,
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertPermitAll, "yes"),
			types.WithAlertLabel(types.AlertLink, alertLink),
			types.WithAlertLabel(types.AlertLinkText, alertLinkText),
		)
		if err != nil {
			c.Logger.ErrorContext(ctx, "Failed to create cluster alert", "alert", alertName, "error", err)
			return
		}
		alert.Metadata.Description = debugPayload
		if err := c.Status.UpsertClusterAlert(ctx, alert); err != nil {
			c.Logger.ErrorContext(ctx, "Failed to upsert cluster alert", "alert", alertName, "error", err)
		}
		return
	}

	c.Logger.InfoContext(ctx, "Successfully sent usage reports.",
		"batch_uuid", batchUUID,
		"reports", totalReportCount,
		"oldest_report", oldest,
		"newest_report", newest,
	)

	var lastErr error
	for _, report := range userActivityReports {
		if err := svc.deleteUserActivityReport(ctx, report); err != nil {
			lastErr = err
		}
	}
	for _, report := range resourcePresenceReports {
		if err := svc.deleteResourcePresenceReport(ctx, report); err != nil {
			lastErr = err
		}
	}
	for _, report := range botInstanceActivityReports {
		if err := svc.deleteBotInstanceActivityReport(ctx, report); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		c.Logger.WarnContext(ctx, "Failed to delete some usage reports after successful send.", "last_error", lastErr)
	}
}

// ClearAlert attempts to delete the reporting-failed alert; it's expected to
// return nil if it successfully deletes the alert, and a trace.NotFound error
// if there's no alert.
func ClearAlert(ctx context.Context, status services.StatusInternal) error {
	if _, err := status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
		AlertID: alertName,
	}); err != nil && trace.IsNotFound(err) {
		// if we can confirm that there's no cluster alert we go ahead and
		// return the NotFound immediately without attempting the delete (reads
		// are cheaper than writes)
		return trace.Wrap(err)
	}
	return trace.Wrap(status.DeleteClusterAlert(ctx, alertName))
}
