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
	"github.com/jackc/pgx/v5"
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
	GCPCloudSQLIAMAuth AuthMode = "gcp-cloudsql"
	// GCPAlloyDBIAMAuth fetches an access token and uses it as password when
	// connecting to GCP AlloyDB (PostgreSQL-compatible).
	GCPAlloyDBIAMAuth AuthMode = "gcp-alloydb"
)

var supportedAuthModes = []AuthMode{
	StaticAuth,
	AzureADAuth,
	GCPCloudSQLIAMAuth,
	GCPAlloyDBIAMAuth,
}

// Check returns an error if the AuthMode is invalid.
func (a AuthMode) Check() error {
	if slices.Contains(supportedAuthModes, a) {
		return nil
	}

	return trace.BadParameter("invalid authentication mode %q, should be one of \"%v\"", a, apiutils.JoinStrings(supportedAuthModes, `", "`))
}

// ApplyToPoolConfigs configures pgxpool.Config based on the authMode.
func (a AuthMode) ApplyToPoolConfigs(ctx context.Context, logger *slog.Logger, configs ...*pgxpool.Config) error {
	bc, err := a.getBeforeConnect(ctx, logger)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, config := range configs {
		if config != nil {
			config.BeforeConnect = bc
		}
	}
	return nil
}

func (a AuthMode) getBeforeConnect(ctx context.Context, logger *slog.Logger) (func(context.Context, *pgx.ConnConfig) error, error) {
	switch a {
	case AzureADAuth:
		bc, err := AzureBeforeConnect(ctx, logger)
		return bc, trace.Wrap(err)
	case GCPCloudSQLIAMAuth:
		bc, err := GCPCloudSQLBeforeConnect(ctx, logger)
		return bc, trace.Wrap(err)
	case GCPAlloyDBIAMAuth:
		bc, err := GCPAlloyDBBeforeConnect(ctx, logger)
		return bc, trace.Wrap(err)
	case StaticAuth:
		return nil, nil
	default:
		return nil, trace.BadParameter("invalid authentication mode %q", a)
	}
}
