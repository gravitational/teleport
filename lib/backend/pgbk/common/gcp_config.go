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
	"net/url"
	"slices"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"

	apiutils "github.com/gravitational/teleport/api/utils"
	gcputils "github.com/gravitational/teleport/lib/utils/gcp"
)

// gcpConfig defines GCP specific configs.
type gcpConfig struct {
	// connectionName is the GCP connection name in format of
	// project:region:instance. The connection name is required by the
	// connector libraries as the connection target.
	connectionName string
	// ipType specifies the type of IP used for GCP connection.
	ipType gcpIPType
	// serviceAccount is the target service account that will login the
	// database.
	serviceAccount string
}

func (c *gcpConfig) check() error {
	if c.connectionName == "" {
		return trace.NotFound("missing #%s (hint: project:region:instance)", gcpConnectionNameParam)
	}
	if err := c.ipType.check(); err != nil {
		return trace.Wrap(err)
	}
	if err := gcputils.ValidateGCPServiceAccountName(c.serviceAccount); err != nil {
		return trace.Wrap(err, "IAM database user for service account should have usernames in format of <service_account_name>@<project_id>.iam")
	}
	return nil
}

const (
	gcpConnectionNameParam = "gcp_connection_name"
	gcpIPTypeParam         = "gcp_ip_type"
)

func gcpConfigFromConnConfig(connConfig *pgx.ConnConfig) (*gcpConfig, error) {
	// TODO(greedy52) maybe support DSN format?
	connStringURL, err := url.Parse(connConfig.ConnString())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	params, err := url.ParseQuery(connStringURL.EscapedFragment())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &gcpConfig{
		connectionName: params.Get(gcpConnectionNameParam),
		ipType:         gcpIPType(strings.ToLower(params.Get(gcpIPTypeParam))),
		// IAM auth users have the PostgreSQL username of their emails minus
		// the ".gserviceaccount.com" part. Now add the suffix back for the
		// full service account email.
		serviceAccount: connConfig.User + ".gserviceaccount.com",
	}
	if err := config.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// gcpIPType specifies the type of IP used for GCP connection.
//
// Values are sourced from:
// https://github.com/GoogleCloudPlatform/cloud-sql-go-connector/blob/main/internal/cloudsql/refresh.go
// https://github.com/GoogleCloudPlatform/alloydb-go-connector/blob/main/internal/alloydb/refresh.go
//
// Note that AutoIP is not recommended for Cloud SQL and not present for
// AlloyDB. So we are not supporting AutoIP. Values are also lower-cased for
// simplicity. If not specified, the library defaults to public.
type gcpIPType string

const (
	gcpIPTypeUnspecified           gcpIPType = ""
	gcpIPTypePublicIP              gcpIPType = "public"
	gcpIPTypePrivateIP             gcpIPType = "private"
	gcpIPTypePrivateServiceConnect gcpIPType = "psc"
)

func (g gcpIPType) check() error {
	supportedModes := []gcpIPType{
		gcpIPTypeUnspecified,
		gcpIPTypePublicIP,
		gcpIPTypePrivateIP,
		gcpIPTypePrivateServiceConnect,
	}

	if slices.Contains(supportedModes, g) {
		return nil
	}
	return trace.BadParameter("invalid %s %q, should be one of \"%v\"", gcpIPTypeParam, g, apiutils.JoinStrings(supportedModes, `", "`))
}

func (g gcpIPType) cloudsqlconnOption() cloudsqlconn.DialOption {
	switch g {
	case gcpIPTypePublicIP:
		return cloudsqlconn.WithPublicIP()
	case gcpIPTypePrivateIP:
		return cloudsqlconn.WithPrivateIP()
	case gcpIPTypePrivateServiceConnect:
		return cloudsqlconn.WithPSC()
	default:
		return nil
	}
}
