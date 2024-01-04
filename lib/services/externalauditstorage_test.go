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

package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/utils"
)

// TestDraftExternalAuditStorage verifies if marshaling/unmarshaling a draft External Audit Storage works.
func TestDraftExternalAuditStorage(t *testing.T) {
	expected, err := externalauditstorage.NewDraftExternalAuditStorage(
		header.Metadata{},
		externalauditstorage.ExternalAuditStorageSpec{
			IntegrationName:        "aws-integration-1",
			PolicyName:             "test-policy-1",
			Region:                 "us-west-2",
			SessionRecordingsURI:   "s3://bucket/sess_rec",
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
		actual, err := UnmarshalExternalAuditStorage(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
	t.Run("marshal and unmarshal back", func(t *testing.T) {
		require.NoError(t, err)
		data, err := MarshalExternalAuditStorage(expected)
		require.NoError(t, err)
		actual, err := UnmarshalExternalAuditStorage(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

// TestClusterExternalAuditStorage verifies if marshaling/unmarshaling a cluster External Audit Storage works.
func TestClusterExternalAuditStorage(t *testing.T) {
	expected, err := externalauditstorage.NewClusterExternalAuditStorage(
		header.Metadata{},
		externalauditstorage.ExternalAuditStorageSpec{
			IntegrationName:        "aws-integration-1",
			PolicyName:             "test-policy-1",
			Region:                 "us-west-2",
			SessionRecordingsURI:   "s3://bucket/sess_rec",
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
		actual, err := UnmarshalExternalAuditStorage(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
	t.Run("marshal and unmarshal back", func(t *testing.T) {
		require.NoError(t, err)
		data, err := MarshalExternalAuditStorage(expected)
		require.NoError(t, err)
		actual, err := UnmarshalExternalAuditStorage(data)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	})
}

var draftExternalAuditYAML = `---
kind: external_audit_storage
version: v1
metadata:
  name: draft
spec:
  integration_name: "aws-integration-1"
  policy_name: "test-policy-1"
  region: "us-west-2"
  session_recordings_uri: "s3://bucket/sess_rec"
  athena_workgroup: "primary"
  glue_database: "teleport_db"
  glue_table: "teleport_table"
  audit_events_long_term_uri: "s3://bucket/events"
  athena_results_uri: "s3://bucket/results"
`

var clusterExternalAuditYAML = `---
kind: external_audit_storage
version: v1
metadata:
  name: cluster
spec:
  integration_name: "aws-integration-1"
  policy_name: "test-policy-1"
  region: "us-west-2"
  session_recordings_uri: "s3://bucket/sess_rec"
  athena_workgroup: "primary"
  glue_database: "teleport_db"
  glue_table: "teleport_table"
  audit_events_long_term_uri: "s3://bucket/events"
  athena_results_uri: "s3://bucket/results"
`
