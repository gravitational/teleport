/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
	"google.golang.org/api/googleapi"
)

// GetGCPRequireSSL requests settings for the project/instance in session from GCP
// and returns true when the instance requires SSL. An error is returned when the
// API call fails or the returned settings are incomplete.
func GetGCPRequireSSL(ctx context.Context, sessionCtx *common.Session, gcpClient common.GCPSQLAdminClient) (requireSSL bool, err error) {
	dbi, err := gcpClient.GetDatabaseInstance(ctx, sessionCtx)
	if err != nil {
		// Fallback: don't require SSL when not authorized.
		if e, ok := trace.Unwrap(err).(*googleapi.Error); ok && e.Code == http.StatusForbidden {
			return false, nil
		}
		return false, trace.Wrap(err, "Failed to get Cloud SQL instance information for %q.", sessionCtx.GCPServerName())
	} else if dbi.Settings == nil || dbi.Settings.IpConfiguration == nil {
		return false, trace.BadParameter("Failed to find Cloud SQL settings for %q. GCP returned %+v.", sessionCtx.GCPServerName(), dbi)
	}
	return dbi.Settings.IpConfiguration.RequireSsl, nil
}

// AppendGCPClientCert calls the GCP API to generate an ephemeral certificate
// and adds it to the TLS config.
func AppendGCPClientCert(ctx context.Context, sessionCtx *common.Session, gcpClient common.GCPSQLAdminClient, tlsConfig *tls.Config) error {
	cert, err := gcpClient.GenerateEphemeralCert(ctx, sessionCtx)
	if err == nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
		return nil
	}
	return trace.AccessDenied(`Could not generate GCP ephemeral client certificate:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role,
or "cloudsql.sslCerts.createEphemeral" IAM permission.
`, err)
}
