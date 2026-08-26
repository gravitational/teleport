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
	GCPCloudSQLIAMAuth AuthMode = "gcp-cloudsql"
)

var authModes = []AuthMode{
	StaticAuth,
	AzureADAuth,
	GCPCloudSQLIAMAuth,
}

// Check returns an error if the AuthMode is invalid.
func (a AuthMode) Check() error {
	if slices.Contains(authModes, a) {
		return nil
	}
	return trace.BadParameter("invalid authentication mode %q, should be one of \"%v\"", a, apiutils.JoinStrings(authModes, `", "`))
}

// AuthConfig contains common auth configs.
type AuthConfig struct {
	// AuthMode is the authentication mode.
	AuthMode AuthMode `json:"auth_mode"`
	// GCPConnectionName is the GCP connection name in format of
	// project:region:instance. The connection name is required by the
	// connector libraries as the connection target.
	GCPConnectionName string `json:"gcp_connection_name"`
	// GCPIPType specifies the type of IP used for GCP connection.
	GCPIPType GCPIPType `json:"gcp_ip_type"`
}

// Check returns an error if the AuthMode is invalid.
func (a AuthConfig) Check() error {
	if err := a.AuthMode.Check(); err != nil {
		return trace.Wrap(err)
	}

	if a.AuthMode == GCPCloudSQLIAMAuth {
		if a.GCPConnectionName == "" {
			return trace.NotFound("empty GCP connection name (hint: project:region:instance)")
		}
		if err := a.GCPIPType.check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ApplyToPoolConfigs configures pgxpool.Config based on the authMode.
func (a AuthConfig) ApplyToPoolConfigs(ctx context.Context, logger *slog.Logger, configs ...*pgxpool.Config) error {
	switch a.AuthMode {
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
			dialFunc, err := GCPCloudSQLDialFunc(ctx, a, config.ConnConfig.User, logger)
			if err != nil {
				return trace.Wrap(err)
			}
			config.ConnConfig.DialFunc = dialFunc
		}
		return nil

	default:
		return trace.BadParameter("invalid authentication mode %q", a)
	}
}
