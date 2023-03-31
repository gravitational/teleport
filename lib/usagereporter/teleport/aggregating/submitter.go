// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregating

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	prehogv1 "github.com/gravitational/teleport/gen/proto/go/prehog/v1alpha"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// UsageReportsSubmitter is almost SubmitUsageReports from
// prehog.v1alpha.TeleportReportingService, but instead of returning a response
// (and requiring wrapping the request in a [connect.Request]) it should do its
// own error checking on the response.
type UsageReportsSubmitter func(context.Context, *prehogv1.SubmitUsageReportsRequest) error

const (
	submitInterval     = 5 * time.Minute
	submitLockDuration = time.Minute
	submitBatch        = 10

	alertGraceHours    = 24
	alertGraceDuration = alertGraceHours * time.Hour
	alertName          = "reporting-failed"
)

const DefaultEndpoint = "https://reporting-teleport.teleportinfra.sh"

var alertMessage = fmt.Sprintf("Teleport has failed to contact the usage reporting server for more than %v hours. "+
	"Please make sure that the Teleport Auth Server can connect to (%v). "+
	"Otherwise, contact Teleport Support at (https://support.goteleport.com).",
	alertGraceHours, DefaultEndpoint)

type SubmitterConfig struct {
	Backend   backend.Backend
	Log       logrus.FieldLogger
	Status    services.StatusInternal
	Submitter UsageReportsSubmitter
}

func (cfg *SubmitterConfig) CheckAndSetDefaults() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Log == nil {
		return trace.BadParameter("missing Log")
	}
	if cfg.Status == nil {
		return trace.BadParameter("missing Status")
	}
	if cfg.Submitter == nil {
		return trace.BadParameter("missing Submitter")
	}
	return nil
}

// RunSubmitter periodically fetches aggregated reports from the backend and
// sends them in small batches with the provided submitter, before deleting
// them. Must only be called after validating the config object with
// CheckAndSetDefaults, and should probably be called in a goroutine.
func RunSubmitter(ctx context.Context, cfg SubmitterConfig) {
	iv := interval.New(interval.Config{
		FirstDuration: utils.HalfJitter(2 * submitInterval),
		Duration:      submitInterval,
		Jitter:        retryutils.NewSeventhJitter(),
	})
	defer iv.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-iv.Next():
		}

		submitOnce(ctx, cfg)
	}
}

func submitOnce(ctx context.Context, c SubmitterConfig) {
	rangeStart := backend.ExactKey(userActivityReportsPrefix)
	getResult, err := c.Backend.GetRange(ctx, rangeStart, backend.RangeEnd(rangeStart), submitBatch)
	if err != nil {
		c.Log.WithError(err).Errorf("Failed to load usage reports for submission.")
		return
	}

	if len(getResult.Items) < 1 {
		if _, err := c.Status.GetClusterAlerts(ctx, types.GetClusterAlertsRequest{
			AlertID: alertName,
		}); err != nil && trace.IsNotFound(err) {
			// if we can confirm that there's no cluster alert we go ahead and
			// exit here without attempting the delete (reads are cheaper than
			// writes)
			return
		}
		err := c.Status.DeleteClusterAlert(ctx, alertName)
		if err == nil {
			c.Log.Infof("Deleted cluster alert %v after successfully clearing usage report backlog.", alertName)
		} else if !trace.IsNotFound(err) {
			c.Log.WithError(err).Errorf("Failed to delete cluster alert %v.", alertName)
		}
		return
	}

	reports := make([]*prehogv1.UserActivityReport, 0, len(getResult.Items))
	for _, item := range getResult.Items {
		report := &prehogv1.UserActivityReport{}
		if err := proto.Unmarshal(item.Value, report); err != nil {
			c.Log.WithError(err).WithField("key", item.Key).Error("Failed to parse usage report from the backend.")
			return
		}
		reports = append(reports, report)
	}

	if _, err := c.Backend.Create(ctx, backend.Item{
		Key:     backend.Key(userActivityReportsLock),
		Value:   []byte("null"),
		Expires: c.Backend.Clock().Now().UTC().Add(submitLockDuration),
	}); err != nil {
		if trace.IsAlreadyExists(err) {
			c.Log.Debugf("Failed to acquire lock %v, already held.", userActivityReportsLock)
		} else {
			c.Log.WithError(err).Errorf("Failed to acquire lock %v.", userActivityReportsLock)
		}
		return
	}

	lockCtx, cancel := context.WithTimeout(ctx, submitLockDuration*9/10)
	defer cancel()

	if err := c.Submitter(lockCtx, &prehogv1.SubmitUsageReportsRequest{
		UserActivity: reports,
	}); err != nil {
		c.Log.WithError(err).WithFields(logrus.Fields{
			"reports":       len(reports),
			"oldest_report": reports[0].GetStartTime().AsTime(),
			"newest_report": reports[len(reports)-1].GetStartTime().AsTime(),
		}).Errorf("Failed to send usage reports.")

		if time.Since(reports[0].StartTime.AsTime()) <= alertGraceDuration {
			return
		}
		alert, err := types.NewClusterAlert(
			alertName,
			alertMessage,
			types.WithAlertLabel(types.AlertOnLogin, "yes"),
			types.WithAlertLabel(types.AlertPermitAll, "yes"),
		)
		if err != nil {
			c.Log.WithError(err).Errorf("Failed to create cluster alert %v.", alertName)
			return
		}
		if err := c.Status.UpsertClusterAlert(ctx, alert); err != nil {
			c.Log.WithError(err).Errorf("Failed to upsert cluster alert %v.", alertName)
		}
		return
	}

	c.Log.WithFields(logrus.Fields{
		"reports":       len(reports),
		"oldest_report": reports[0].GetStartTime().AsTime(),
	}).Info("Successfully sent usage reports.")

	var lastErr error
	for _, item := range getResult.Items {
		if err := c.Backend.Delete(ctx, item.Key); err != nil {
			lastErr = err
		}
	}
	if lastErr != nil {
		c.Log.WithField("last_error", lastErr).Warn("Failed to delete some usage reports after successful send.")
	}
}
