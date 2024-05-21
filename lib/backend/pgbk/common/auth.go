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

package pgcommon

import (
	"context"
	"log/slog"
	"slices"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5/pgxpool"

	apiutils "github.com/gravitational/teleport/api/utils"
)

// AuthMode determines if we should use some environment-specific authentication
// mechanism or credentials.
type AuthMode string

const (
	// StaticAuth uses the static credentials as defined in the connection
	// string.
	StaticAuth AuthMode = ""
	// AzureADAuth gets a connection token from Azure and uses it as the
	// password when connecting.
	AzureADAuth AuthMode = "azure"
	// GCPCloudSQLIAMAuth fetches an access token and uses it as password when
	// connecting to GCP Cloud SQL PostgreSQL.
	// TODO(greedy52) gcp-alloydb
	GCPCloudSQLIAMAuth AuthMode = "gcp-cloudsql"
)

// Check returns an error if the AuthMode is invalid.
func (a AuthMode) Check() error {
	supportedModes := []AuthMode{
		StaticAuth,
		AzureADAuth,
		GCPCloudSQLIAMAuth,
	}

	if slices.Contains(supportedModes, a) {
		return nil
	}
	return trace.BadParameter("invalid authentication mode %q, should be one of \"%v\"", a, apiutils.JoinStrings(supportedModes, `", "`))
}

// ApplyToPoolConfigs configures pgxpool.Config based on the authMode.
func (a AuthMode) ApplyToPoolConfigs(ctx context.Context, logger *slog.Logger, configs ...*pgxpool.Config) error {
	switch a {
	case StaticAuth:
		// Nothing to do
		return nil

	case AzureADAuth:
		bc, err := AzureBeforeConnect(ctx, logger)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, config := range configs {
			config.BeforeConnect = bc
		}
		return nil

	case GCPCloudSQLIAMAuth:
		for _, config := range configs {
			if err := ConfigureConnectionForGCPCloudSQL(ctx, logger, config.ConnConfig); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil

	default:
		return trace.BadParameter("invalid authentication mode %q", a)
	}
}
