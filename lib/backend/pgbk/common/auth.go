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
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	// GCPSQLIAMAuth fetches an access token and uses it as password when
	// connecting to GCP SQL PostgreSQL.
	GCPSQLIAMAuth AuthMode = "gcp-sql"
	// GCPAlloyDBIAMAuth fetches an access token and uses it as password when
	// connecting to GCP AlloyDB (PostgreSQL-compatible).
	GCPAlloyDBIAMAuth AuthMode = "gcp-alloydb"
)

var supportedAuthModes = []AuthMode{
	StaticAuth,
	AzureADAuth,
	GCPSQLIAMAuth,
	GCPAlloyDBIAMAuth,
}

// Check returns an error if the AuthMode is invalid.
func (a AuthMode) Check() error {
	if slices.Contains(supportedAuthModes, a) {
		return nil
	}

	quotedModes := make([]string, 0, len(supportedAuthModes))
	for _, mode := range supportedAuthModes {
		quotedModes = append(quotedModes, fmt.Sprintf("%q", mode))
	}

	return trace.BadParameter("invalid authentication mode %q, should be one of %s", a, strings.Join(quotedModes, ", "))
}

// ConfigurePoolConfigs configures pgxpool.Config based on the authMode.
func (a AuthMode) ConfigurePoolConfigs(ctx context.Context, logger *slog.Logger, configs ...*pgxpool.Config) error {
	if bc, err := a.getBeforeConnect(ctx, logger); err != nil {
		return trace.Wrap(err)
	} else if bc != nil {
		for _, config := range configs {
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
	case GCPSQLIAMAuth:
		bc, err := GCPSQLBeforeConnect(ctx, logger)
		return bc, trace.Wrap(err)
	case GCPAlloyDBIAMAuth:
		bc, err := GCPAlloyDBBeforeConnect(ctx, logger)
		return bc, trace.Wrap(err)
	}
	return nil, nil
}
