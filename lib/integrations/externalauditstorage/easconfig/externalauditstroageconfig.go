/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package easconfig

// ExternalAuditStorageConfiguration contains the arguments to configure the
// External Audit Storage.
type ExternalAuditStorageConfiguration struct {
	// Bootstrap is whether to bootstrap infrastructure (default: false).
	Bootstrap bool
	// Region is the AWS Region used.
	Region string
	// ClusterName is the Teleport cluster name.
	// Used for resource tagging.
	ClusterName string
	// IntegrationName is the Teleport AWS OIDC Integration name.
	// Used for resource tagging.
	IntegrationName string
	// Role is the AWS IAM Role associated with the OIDC integration.
	Role string
	// Policy is the name to use for the IAM policy.
	Policy string
	// SessionRecordingsURI is the S3 URI where session recordings are stored.
	SessionRecordingsURI string
	// AuditEventsURI is the S3 URI where audit events are stored.
	AuditEventsURI string
	// AthenaResultsURI is the S3 URI where temporary Athena results are stored.
	AthenaResultsURI string
	// AthenaWorkgroup is the name of the Athena workgroup used.
	AthenaWorkgroup string
	// GlueDatabase is the name of the Glue database used.
	GlueDatabase string
	// GlueTable is the name of the Glue table used.
	GlueTable string
	// Partition is the AWS partition to use (default: aws).
	Partition string
	// AccountID is the AWS account ID.
	AccountID string
}
