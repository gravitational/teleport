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

package cloud

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// GetGCPRequireSSL requests settings for the project/instance in session from GCP
// and returns true when the instance requires SSL. An access denied error is
// returned when an unauthorized error is returned from GCP.
func GetGCPRequireSSL(ctx context.Context, database types.Database, gcpClient gcp.SQLAdminClient) (requireSSL bool, err error) {
	dbi, err := gcpClient.GetDatabaseInstance(ctx, database)
	if err != nil {
		err = common.ConvertError(err)
		if trace.IsAccessDenied(err) {
			return false, trace.Wrap(err, `Could not get GCP database instance settings:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role,
or "cloudsql.instances.get" IAM permission.`, err)
		}
		return false, trace.Wrap(err, "Failed to get Cloud SQL instance information for %q.", database.GetGCP().GetServerName())
	} else if dbi.Settings == nil || dbi.Settings.IpConfiguration == nil {
		return false, trace.BadParameter("Failed to find Cloud SQL settings for %q. GCP returned %+v.", database.GetGCP().GetServerName(), dbi)
	}
	return dbi.Settings.IpConfiguration.RequireSsl, nil
}

// AppendGPCClientCertRequest is a request to update [TLSConfig] with an
// ephemeral GCP client certificate.
type AppendGCPClientCertRequest struct {
	GCPClient   gcp.SQLAdminClient
	GenerateKey func(context.Context) (*keys.PrivateKey, error)
	Expiry      time.Time
	Database    types.Database
	TLSConfig   *tls.Config
}

// AppendGCPClientCert calls the GCP API to generate an ephemeral certificate
// and adds it to the TLS config. An access denied error is returned when the
// generate call fails.
func AppendGCPClientCert(ctx context.Context, req *AppendGCPClientCertRequest) error {
	privateKey, err := req.GenerateKey(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	certPEM, err := req.GCPClient.GenerateEphemeralCert(ctx, req.Database, req.Expiry, privateKey.Public())
	if err != nil {
		err = common.ConvertError(err)
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, `Cloud not generate GCP ephemeral client certificate:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role,
or "cloudsql.sslCerts.createEphemeral" IAM permission.`, err)
		}
		return trace.Wrap(err, "Failed to generate GCP ephemeral client certificate for %q.", req.Database.GetGCP().GetServerName())
	}
	tlsCert, err := privateKey.TLSCertificate([]byte(certPEM))
	if err != nil {
		return trace.Wrap(err)
	}
	req.TLSConfig.Certificates = []tls.Certificate{tlsCert}
	return nil
}
