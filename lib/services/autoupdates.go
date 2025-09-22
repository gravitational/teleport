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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
)

// AutoUpdateServiceGetter defines only read-only service methods.
type AutoUpdateServiceGetter interface {
	// GetAutoUpdateConfig gets the AutoUpdateConfig singleton resource.
	GetAutoUpdateConfig(ctx context.Context) (*autoupdate.AutoUpdateConfig, error)

	// GetAutoUpdateVersion gets the AutoUpdateVersion singleton resource.
	GetAutoUpdateVersion(ctx context.Context) (*autoupdate.AutoUpdateVersion, error)

	// GetAutoUpdateAgentRollout gets the AutoUpdateAgentRollout singleton resource.
	GetAutoUpdateAgentRollout(ctx context.Context) (*autoupdate.AutoUpdateAgentRollout, error)

	// GetAutoUpdateAgentReport gets an AutoUpdateAgentReport.
	GetAutoUpdateAgentReport(ctx context.Context, name string) (*autoupdate.AutoUpdateAgentReport, error)

	// ListAutoUpdateAgentReports returns an AutoUpdateAgentReports page.
	ListAutoUpdateAgentReports(ctx context.Context, pageSize int, pageToken string) ([]*autoupdate.AutoUpdateAgentReport, string, error)
}

// AutoUpdateService stores the autoupdate service.
type AutoUpdateService interface {
	AutoUpdateServiceGetter

	// CreateAutoUpdateConfig creates the AutoUpdateConfig singleton resource.
	CreateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// UpdateAutoUpdateConfig updates the AutoUpdateConfig singleton resource.
	UpdateAutoUpdateConfig(ctx context.Context, config *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// UpsertAutoUpdateConfig sets the AutoUpdateConfig singleton resource.
	UpsertAutoUpdateConfig(ctx context.Context, c *autoupdate.AutoUpdateConfig) (*autoupdate.AutoUpdateConfig, error)

	// DeleteAutoUpdateConfig deletes the AutoUpdateConfig singleton resource.
	DeleteAutoUpdateConfig(ctx context.Context) error

	// CreateAutoUpdateVersion creates the AutoUpdateVersion singleton resource.
	CreateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// UpdateAutoUpdateVersion updates the AutoUpdateVersion singleton resource.
	UpdateAutoUpdateVersion(ctx context.Context, config *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// UpsertAutoUpdateVersion sets the AutoUpdateVersion singleton resource.
	UpsertAutoUpdateVersion(ctx context.Context, c *autoupdate.AutoUpdateVersion) (*autoupdate.AutoUpdateVersion, error)

	// DeleteAutoUpdateVersion deletes the AutoUpdateVersion singleton resource.
	DeleteAutoUpdateVersion(ctx context.Context) error

	// CreateAutoUpdateAgentRollout creates the AutoUpdateAgentRollout singleton resource.
	CreateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error)

	// UpdateAutoUpdateAgentRollout updates the AutoUpdateAgentRollout singleton resource.
	UpdateAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error)

	// UpsertAutoUpdateAgentRollout sets the AutoUpdateAgentRollout singleton resource.
	UpsertAutoUpdateAgentRollout(ctx context.Context, rollout *autoupdate.AutoUpdateAgentRollout) (*autoupdate.AutoUpdateAgentRollout, error)

	// DeleteAutoUpdateAgentRollout deletes the AutoUpdateAgentRollout singleton resource.
	DeleteAutoUpdateAgentRollout(ctx context.Context) error

	// CreateAutoUpdateAgentReport creates the AutoUpdateAgentReport singleton resource.
	CreateAutoUpdateAgentReport(ctx context.Context, report *autoupdate.AutoUpdateAgentReport) (*autoupdate.AutoUpdateAgentReport, error)

	// UpdateAutoUpdateAgentReport updates the AutoUpdateAgentReport singleton resource.
	UpdateAutoUpdateAgentReport(ctx context.Context, report *autoupdate.AutoUpdateAgentReport) (*autoupdate.AutoUpdateAgentReport, error)

	// UpsertAutoUpdateAgentReport sets the AutoUpdateAgentReport singleton resource.
	UpsertAutoUpdateAgentReport(ctx context.Context, report *autoupdate.AutoUpdateAgentReport) (*autoupdate.AutoUpdateAgentReport, error)

	// DeleteAutoUpdateAgentReport deletes the AutoUpdateAgentReport singleton resource.
	DeleteAutoUpdateAgentReport(ctx context.Context, name string) error

	// DeleteAllAutoUpdateAgentReports deletes all AutoUpdateAgentReport resources.
	DeleteAllAutoUpdateAgentReports(ctx context.Context) error
}
