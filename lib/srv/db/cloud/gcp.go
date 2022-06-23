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

	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/trace"
)

// GetGCPRequireSSL requests settings for the project/instance in session from GCP
// and returns true when the instance requires SSL. An access denied error is
// returned when an unauthorized error is returned from GCP.
func GetGCPRequireSSL(ctx context.Context, sessionCtx *common.Session, gcpClient common.GCPSQLAdminClient) (requireSSL bool, err error) {
	dbi, err := gcpClient.GetDatabaseInstance(ctx, sessionCtx)
	if err != nil {
		err = common.ConvertError(err)
		if trace.IsAccessDenied(err) {
			return false, trace.Wrap(err, `Could not get GCP database instance settings:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role,
or "cloudsql.instances.get" IAM permission.`, err)
		}
		return false, trace.Wrap(err, "Failed to get Cloud SQL instance information for %q.", common.GCPServerName(sessionCtx))
	} else if dbi.Settings == nil || dbi.Settings.IpConfiguration == nil {
		return false, trace.BadParameter("Failed to find Cloud SQL settings for %q. GCP returned %+v.", common.GCPServerName(sessionCtx), dbi)
	}
	return dbi.Settings.IpConfiguration.RequireSsl, nil
}

// AppendGCPClientCert calls the GCP API to generate an ephemeral certificate
// and adds it to the TLS config. An access denied error is returned when the
// generate call fails.
func AppendGCPClientCert(ctx context.Context, sessionCtx *common.Session, gcpClient common.GCPSQLAdminClient, tlsConfig *tls.Config) error {
	cert, err := gcpClient.GenerateEphemeralCert(ctx, sessionCtx)
	if err != nil {
		err = common.ConvertError(err)
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, `Cloud not generate GCP ephemeral client certificate:

  %v

Make sure Teleport db service has "Cloud SQL Admin" GCP IAM role,
or "cloudsql.sslCerts.createEphemeral" IAM permission.`, err)
		}
		return trace.Wrap(err, "Failed to generate GCP ephemeral client certificate for %q.", common.GCPServerName(sessionCtx))
	}
	tlsConfig.Certificates = []tls.Certificate{*cert}
	return nil
}
