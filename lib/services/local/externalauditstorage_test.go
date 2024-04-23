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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestExternalAuditStorageService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Context: ctx,
		Clock:   clock,
	})
	require.NoError(t, err)

	service := NewExternalAuditStorageService(backend.NewSanitizer(mem))

	sessRecURL1 := "s3://bucket1/ses-rec-v1"
	sessRecURL2 := "s3://bucket1/ses-rec-v2"

	spec1 := newSpecWithSessRec(t, sessRecURL1)
	draftFromSpec1, err := externalauditstorage.NewDraftExternalAuditStorage(header.Metadata{}, spec1)
	require.NoError(t, err)

	clusterFromSpec1, err := externalauditstorage.NewClusterExternalAuditStorage(header.Metadata{}, spec1)
	require.NoError(t, err)

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(header.Metadata{}, "ID", "Revision"),
	}

	t.Run("promote failed without draft", func(t *testing.T) {
		// Given no draft
		// When PromoteToClusterExternalAuditStorage
		// Then error is returned

		// When
		err := service.PromoteToClusterExternalAuditStorage(ctx)
		// Then
		require.ErrorContains(t, err, "can't promote to cluster when draft does not exist")
	})

	t.Run("create draft", func(t *testing.T) {
		// Given no draft
		// When CreateDraftExternalAuditStorage with non-existing OIDC
		// integration
		// Then an error is returned

		// When
		_, err := service.CreateDraftExternalAuditStorage(ctx, draftFromSpec1)
		// Then
		require.Error(t, err)
	})

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: spec1.IntegrationName},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "test-role",
		},
	)
	require.NoError(t, err)

	integrationsSvc, err := NewIntegrationsService(mem)
	require.NoError(t, err)
	_, err = integrationsSvc.CreateIntegration(ctx, oidcIntegration)
	require.NoError(t, err)

	t.Run("create draft", func(t *testing.T) {
		// Given no draft
		// When CreateDraftExternalAuditStorage
		// Then draft is returned on GetDraftExternalAuditStorage
		// And GetClusterExternalAuditStorage returns not found.
		// And CreateDraftExternalAuditStorage again returns AlreadyExists

		// When
		created, err := service.CreateDraftExternalAuditStorage(ctx, draftFromSpec1)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(draftFromSpec1, created, cmpOpts...))
		require.NotEmpty(t, created.GetRevision())

		// Then
		got, err := service.GetDraftExternalAuditStorage(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(created, got, cmpOpts...))
		require.Equal(t, created.GetRevision(), got.GetRevision())

		// And
		_, err = service.GetClusterExternalAuditStorage(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
		// And
		_, err = service.CreateDraftExternalAuditStorage(ctx, draftFromSpec1)
		require.True(t, trace.IsAlreadyExists(err), err)
	})

	t.Run("upsert draft", func(t *testing.T) {
		// Given an existing draft
		// When UpsertDraftExternalAuditStorage
		// Then draft is returned on GetDraftExternalAuditStorage
		// And GetClusterExternalCloutAudit returns not found.

		// When
		created, err := service.UpsertDraftExternalAuditStorage(ctx, draftFromSpec1)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(draftFromSpec1, created, cmpOpts...))
		require.NotEmpty(t, created.GetRevision())

		// Then
		got, err := service.GetDraftExternalAuditStorage(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(created, got, cmpOpts...))
		require.Equal(t, created.GetRevision(), got.GetRevision())

		// And
		_, err = service.GetClusterExternalAuditStorage(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("promote draft to cluster", func(t *testing.T) {
		// Given draft external_audit_storage resource
		// When PromoteToClusterExternalAuditStorage is executed
		// Then GetClusterExternalAudit returns copy of draft config.

		// When
		err := service.PromoteToClusterExternalAuditStorage(ctx)
		require.NoError(t, err, trace.DebugReport(err))
		// Then
		out, err := service.GetClusterExternalAuditStorage(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusterFromSpec1, out, cmpOpts...))
	})

	t.Run("updating draft does not change cluster", func(t *testing.T) {
		// Given existing cluster external_audit_storage
		// When UpsertDraftExternalAuditStorage
		// Then draft is written
		// And cluster external audit remains unchanged.

		// Given
		specWithNewSessRec := newSpecWithSessRec(t, sessRecURL2)
		draftWithNewSessRec, err := externalauditstorage.NewDraftExternalAuditStorage(header.Metadata{}, specWithNewSessRec)
		require.NoError(t, err)

		// When
		_, err = service.UpsertDraftExternalAuditStorage(ctx, draftWithNewSessRec)
		require.NoError(t, err)

		// Then
		updatedDraft, err := service.GetDraftExternalAuditStorage(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(draftWithNewSessRec, updatedDraft, cmpOpts...))
		// And
		clusterOutput, err := service.GetClusterExternalAuditStorage(ctx)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(clusterFromSpec1, clusterOutput, cmpOpts...))
	})

	t.Run("disable cluster", func(t *testing.T) {
		// Given existing cluster
		// When DisableClusterExternalAuditStorage
		// Then not found error is returner on GetCluster.

		// When
		err := service.DisableClusterExternalAuditStorage(ctx)
		require.NoError(t, err)

		// Then
		_, err = service.GetClusterExternalAuditStorage(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("delete draft", func(t *testing.T) {
		// Given existing draft
		// When DeleteDraftExternalAudit
		// Then not found error is returner on GetDraft.
		// And deleting again fails

		// When
		err := service.DeleteDraftExternalAuditStorage(ctx)
		require.NoError(t, err)

		// Then
		_, err = service.GetDraftExternalAuditStorage(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))

		// And
		err = service.DeleteDraftExternalAuditStorage(ctx)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
	})

	t.Run("generate", func(t *testing.T) {
		// Given no draft

		// When GenerateDraftExternalAuditStorage
		generateResp, err := service.GenerateDraftExternalAuditStorage(ctx, "aws-integration-1", "us-west-2")
		require.NoError(t, err)

		// Then draft is returned with generated values
		spec := generateResp.Spec
		nonce := strings.TrimPrefix(spec.PolicyName, "ExternalAuditStoragePolicy-")
		underscoreNonce := strings.ReplaceAll(nonce, "-", "_")
		expectedSpec := externalauditstorage.ExternalAuditStorageSpec{
			IntegrationName:        "aws-integration-1",
			PolicyName:             "ExternalAuditStoragePolicy-" + nonce,
			Region:                 "us-west-2",
			SessionRecordingsURI:   "s3://teleport-longterm-" + nonce + "/sessions",
			AuditEventsLongTermURI: "s3://teleport-longterm-" + nonce + "/events",
			AthenaResultsURI:       "s3://teleport-transient-" + nonce + "/query_results",
			AthenaWorkgroup:        "teleport_events_" + underscoreNonce,
			GlueDatabase:           "teleport_events_" + underscoreNonce,
			GlueTable:              "teleport_events",
		}
		assert.Equal(t, expectedSpec, spec)

		// And GetDraftExternalAuditStorage returns the same draft
		getResp, err := service.GetDraftExternalAuditStorage(ctx)
		require.NoError(t, err)
		assert.Empty(t, cmp.Diff(generateResp, getResp, cmpOpts...))

		// And can't generate when there is an existing draft
		_, err = service.GenerateDraftExternalAuditStorage(ctx, "aws-integration-1", "us-west-2")
		require.Error(t, err)
		assert.True(t, trace.IsAlreadyExists(err), "expected AlreadyExists error, got %v", err)
	})
}

func newSpecWithSessRec(t *testing.T, sessionRecordingsURI string) externalauditstorage.ExternalAuditStorageSpec {
	return externalauditstorage.ExternalAuditStorageSpec{
		IntegrationName:        "aws-integration-1",
		PolicyName:             "test-policy",
		Region:                 "us-west-2",
		SessionRecordingsURI:   sessionRecordingsURI,
		AthenaWorkgroup:        "primary",
		GlueDatabase:           "teleport_db",
		GlueTable:              "teleport_table",
		AuditEventsLongTermURI: "s3://bucket/events",
		AthenaResultsURI:       "s3://bucket/query_results",
	}
}
