// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package local

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestExternalCloudAuditService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewExternalCloudAuditService(backend.NewSanitizer(mem))

	sessRecURL1 := "s3://bucket1/ses-rec-v1"
	sessRecURL2 := "s3://bucket1/ses-rec-v2"

	spec1 := newSpecWithSessRec(t, sessRecURL1)
	draftFromSpec1, err := externalcloudaudit.NewDraftExternalCloudAudit(header.Metadata{}, spec1)
	require.NoError(t, err)

	clusterFromSpec1, err := externalcloudaudit.NewClusterExternalCloudAudit(header.Metadata{}, spec1)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	t.Run("no draft, can't promote anything", func(t *testing.T) {
		// Given no draft
		// When PromoteToClusterExternalCloudAudit
		// Then error is returned

		// When
		err := service.PromoteToClusterExternalCloudAudit(ctx)
		// Then
		require.ErrorContains(t, err, "can't promote to cluster when draft does not exist")
	})

	t.Run("create draft", func(t *testing.T) {
		// Given no draft
		// When UpsertDraftExternalCloudAudit
		// Then draft is returned on GetDraftExternalCloudAudit
		// And GetClusterExternalCloutAudit returns not found.

		// When
		_, err := service.UpsertDraftExternalCloudAudit(ctx, draftFromSpec1)
		require.NoError(t, err)

		// Then
		out, err := service.GetDraftExternalCloudAudit(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(draftFromSpec1, out, cmpOpts...))
		// And
		_, err = service.GetClusterExternalCloudAudit(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("promote draft audit to cluster external cloud audit", func(t *testing.T) {
		// Given draft external_cloud_audit resource
		// When PromoteToClusterExternalCloudAudit is executed
		// Then GetClusterExternalAudit returns copy of draft config.

		// When
		err := service.PromoteToClusterExternalCloudAudit(ctx)
		require.NoError(t, err)
		// Then
		out, err := service.GetClusterExternalCloudAudit(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusterFromSpec1, out, cmpOpts...))
	})

	t.Run("updating draft does not change to cluster", func(t *testing.T) {
		// Given existing cluster external_cloud_audit
		// When UpsertDraftExternalCloudAudit
		// Then draft is written
		// And cluster external audit remains unchanged.

		// Given
		specWithNewSessRec := newSpecWithSessRec(t, sessRecURL2)
		draftWithNewSessRec, err := externalcloudaudit.NewDraftExternalCloudAudit(header.Metadata{}, specWithNewSessRec)
		require.NoError(t, err)

		// When
		_, err = service.UpsertDraftExternalCloudAudit(ctx, draftWithNewSessRec)
		require.NoError(t, err)

		// Then
		updatedDraft, err := service.GetDraftExternalCloudAudit(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(draftWithNewSessRec, updatedDraft, cmpOpts...))
		// And
		clusterOutput, err := service.GetClusterExternalCloudAudit(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusterFromSpec1, clusterOutput, cmpOpts...))
	})

	t.Run("disable cluster", func(t *testing.T) {
		// Given existing cluster
		// When DisableClusterExternalCloudAudit
		// Then not found error is returner on GetCluster.

		// When
		err := service.DisableClusterExternalCloudAudit(ctx)
		require.NoError(t, err)

		// Then
		_, err = service.GetClusterExternalCloudAudit(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("delete draft", func(t *testing.T) {
		// Given existing draft
		// When DeleteDraftExternalAudit
		// Then not found error is returner on GetDraft.
		// And deleting again fails

		// When
		err := service.DeleteDraftExternalCloudAudit(ctx)
		require.NoError(t, err)

		// Then
		_, err = service.GetDraftExternalCloudAudit(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))

		// And
		err = service.DeleteDraftExternalCloudAudit(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	})

	t.Run("generate", func(t *testing.T) {
		// Given no draft

		// When GenerateDraftExternalCloudAudit
		generateResp, err := service.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
		require.NoError(t, err)

		// Then draft is returned with generated values
		spec := generateResp.Spec
		nonce := strings.TrimPrefix(spec.PolicyName, "ExternalCloudAuditPolicy-")
		underscoreNonce := strings.ReplaceAll(nonce, "-", "_")
		expectedSpec := externalcloudaudit.ExternalCloudAuditSpec{
			IntegrationName:        "test-integration",
			PolicyName:             "ExternalCloudAuditPolicy-" + nonce,
			Region:                 "us-west-2",
			SessionsRecordingsURI:  "s3://teleport-longterm-" + nonce + "/sessions",
			AuditEventsLongTermURI: "s3://teleport-longterm-" + nonce + "/events",
			AthenaResultsURI:       "s3://teleport-transient-" + nonce + "/query_results",
			AthenaWorkgroup:        "teleport_events_" + underscoreNonce,
			GlueDatabase:           "teleport_events_" + underscoreNonce,
			GlueTable:              "teleport_events",
		}
		assert.Equal(t, expectedSpec, spec)

		// And GetDraftExternalCloudAudit returns the same draft
		getResp, err := service.GetDraftExternalCloudAudit(ctx)
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(generateResp, getResp, cmpOpts...))

		// And can't generate when there is an existing draft
		_, err = service.GenerateDraftExternalCloudAudit(ctx, "test-integration", "us-west-2")
		require.Error(t, err)
		assert.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got %v", err)
	})
}

func newSpecWithSessRec(t *testing.T, sessionsRecordingsURI string) externalcloudaudit.ExternalCloudAuditSpec {
	return externalcloudaudit.ExternalCloudAuditSpec{
		IntegrationName:        "aws-integration-1",
		PolicyName:             "test-policy",
		Region:                 "us-west-2",
		SessionsRecordingsURI:  sessionsRecordingsURI,
		AthenaWorkgroup:        "primary",
		GlueDatabase:           "teleport_db",
		GlueTable:              "teleport_table",
		AuditEventsLongTermURI: "s3://bucket/events",
		AthenaResultsURI:       "s3://bucket/query_results",
	}
}
