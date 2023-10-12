/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDraftExternalCloudAudit verifies if marshaling/unmarshaling on draft external cloud audit works.
func TestDraftExternalCloudAudit(t *testing.T) {
	expected, err := externalcloudaudit.NewDraftExternalCloudAudit(
		header.Metadata{},
		externalcloudaudit.ExternalCloudAuditSpec{
			IntegrationName:        "aws-integration-1",
			SessionsRecordingsURI:  "s3://bucket/sess_rec",
			AthenaWorkgroup:        "primary",
			GlueDatabase:           "teleport_db",
			GlueTable:              "teleport_table",
			AuditEventsLongTermURI: "s3://bucket/events",
			AthenaResultsURI:       "s3://bucket/results",
		},
	)
	require.NoError(t, err)
	t.Run("Unmarshal from testdata and compare", func(t *testing.T) {
		data, err := utils.ToJSON([]byte(draftExternalAuditYAML))
		require.NoError(t, err)
		actual, err := UnmarshalExternalCloudAudit(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
	t.Run("marshal and unmarshal back", func(t *testing.T) {
		require.NoError(t, err)
		data, err := MarshalExternalCloudAudit(expected)
		require.NoError(t, err)
		actual, err := UnmarshalExternalCloudAudit(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

// TestClusterExternalCloudAudit verifies if marshaling/unmarshaling on cluster external cloud audit works.
func TestClusterExternalCloudAudit(t *testing.T) {
	expected, err := externalcloudaudit.NewClusterExternalCloudAudit(
		header.Metadata{},
		externalcloudaudit.ExternalCloudAuditSpec{
			IntegrationName:        "aws-integration-1",
			SessionsRecordingsURI:  "s3://bucket/sess_rec",
			AthenaWorkgroup:        "primary",
			GlueDatabase:           "teleport_db",
			GlueTable:              "teleport_table",
			AuditEventsLongTermURI: "s3://bucket/events",
			AthenaResultsURI:       "s3://bucket/results",
		},
	)
	require.NoError(t, err)
	t.Run("Unmarshal from testdata and compare", func(t *testing.T) {
		data, err := utils.ToJSON([]byte(clusterExternalAuditYAML))
		require.NoError(t, err)
		actual, err := UnmarshalExternalCloudAudit(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
	t.Run("marshal and unmarshal back", func(t *testing.T) {
		require.NoError(t, err)
		data, err := MarshalExternalCloudAudit(expected)
		require.NoError(t, err)
		actual, err := UnmarshalExternalCloudAudit(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

var draftExternalAuditYAML = `---
kind: external_cloud_audit
version: v1
metadata:
  name: external-cloud-audit-draft
spec:
  integration_name: "aws-integration-1"
  sessions_recordings_uri: "s3://bucket/sess_rec"
  athena_workgroup: "primary"
  glue_database: "teleport_db"
  glue_table: "teleport_table"
  audit_events_long_term_uri: "s3://bucket/events"
  athena_results_uri: "s3://bucket/results"
`

var clusterExternalAuditYAML = `---
kind: external_cloud_audit
version: v1
metadata:
  name: external-cloud-audit-cluster
spec:
  integration_name: "aws-integration-1"
  sessions_recordings_uri: "s3://bucket/sess_rec"
  athena_workgroup: "primary"
  glue_database: "teleport_db"
  glue_table: "teleport_table"
  audit_events_long_term_uri: "s3://bucket/events"
  athena_results_uri: "s3://bucket/results"
`
