/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package rollout

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	"github.com/gravitational/teleport/api/types"
)

// Client is the subset of the Teleport client RPCs the controller needs.
type Client interface {
	// GetAutoUpdateConfig gets the AutoUpdateConfig singleton resource.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdatepb.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the AutoUpdateVersion singleton resource.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdatepb.AutoUpdateVersion, error)

	// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout singleton resource.
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdatepb.AutoUpdateAgentRollout, error)

	// CreateAutoUpdateAgentRollout creates the AutoUpdateAgentRollout singleton resource.
	CreateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdatepb.AutoUpdateAgentRollout) (*autoupdatepb.AutoUpdateAgentRollout, error)

	// UpdateAutoUpdateAgentRollout updates the AutoUpdateAgentRollout singleton resource.
	UpdateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdatepb.AutoUpdateAgentRollout) (*autoupdatepb.AutoUpdateAgentRollout, error)

	// DeleteAutoUpdateAgentRollout deletes the AutoUpdateAgentRollout singleton resource.
	DeleteAutoUpdateAgentRollout(ctx context.Context) error

	// GetClusterMaintenanceConfig loads the current maintenance config singleton.
	GetClusterMaintenanceConfig(ctx context.Context) (types.ClusterMaintenanceConfig, error)

	// ListAutoUpdateAgentReports lists the autoupdate_agent_report resources
	// so the controller can measure the rollout progress.
	ListAutoUpdateAgentReports(ctx context.Context, pageSize int, nextKey string) ([]*autoupdatepb.AutoUpdateAgentReport, string, error)
}

func getAllReports(ctx context.Context, clt Client) ([]*autoupdatepb.AutoUpdateAgentReport, error) {
	var reports []*autoupdatepb.AutoUpdateAgentReport

	// this is an in-memory client, we go for the max page size
	const pageSize = 0
	var pageToken string
	for {
		page, nextToken, err := clt.ListAutoUpdateAgentReports(ctx, pageSize, pageToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		reports = append(reports, page...)
		if nextToken == "" {
			return reports, nil
		}
		pageToken = nextToken
	}
}

func getAllValidReports(ctx context.Context, clt Client, now time.Time) ([]*autoupdatepb.AutoUpdateAgentReport, error) {
	allReports, err := getAllReports(ctx, clt)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err, "getting all reports")
	}

	validReports := make([]*autoupdatepb.AutoUpdateAgentReport, len(allReports))
	for _, report := range allReports {
		if now.Sub(report.GetSpec().GetTimestamp().AsTime()) <= constants.AutoUpdateAgentReportPeriod {
			validReports = append(validReports, report)
		}
	}
	return validReports, nil
}
