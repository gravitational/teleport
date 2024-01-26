/*
Copyright 2024 Gravitational, Inc.

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

package config

// IntegrationConfExternalAuditStorage contains the arguments of the
// `teleport integration configure externalauditstorage` command
type IntegrationConfExternalAuditStorage struct {
	// Bootstrap is whether to bootstrap infrastructure (default: false).
	Bootstrap bool
	// Region is the AWS Region used.
	Region string
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
}
