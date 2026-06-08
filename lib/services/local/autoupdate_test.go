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

package local

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	autoupdatev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAutoUpdateServiceConfigCRUD verifies get/create/update/upsert/delete methods of the backend service
// for AutoUpdateConfig resource.
func TestAutoUpdateServiceConfigCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	config := autoupdatev1pb.AutoUpdateConfig_builder{
		Kind:     types.KindAutoUpdateConfig,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: types.MetaNameAutoUpdateConfig}.Build(),
		Spec: autoupdatev1pb.AutoUpdateConfigSpec_builder{
			Tools: autoupdatev1pb.AutoUpdateConfigSpecTools_builder{
				Mode: autoupdate.ToolsUpdateModeEnabled,
			}.Build(),
		}.Build(),
	}.Build()

	created, err := service.CreateAutoUpdateConfig(ctx, config)
	require.NoError(t, err)
	diff := cmp.Diff(config, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAutoUpdateConfig(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(config, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	config.GetSpec().SetTools(autoupdatev1pb.AutoUpdateConfigSpecTools_builder{
		Mode: autoupdate.ToolsUpdateModeDisabled,
	}.Build())
	updated, err := service.UpdateAutoUpdateConfig(ctx, config)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetTools(), updated.GetSpec().GetTools())

	_, err = service.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	err = service.DeleteAutoUpdateConfig(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoUpdateConfig(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	// If we try to conditionally update a missing resource, we receive
	// a CompareFailed instead of a NotFound.
	var revisionMismatchError *trace.CompareFailedError
	_, err = service.UpdateAutoUpdateConfig(ctx, config)
	require.ErrorAs(t, err, &revisionMismatchError)
}

// TestAutoUpdateServiceVersionCRUD verifies get/create/update/upsert/delete methods of the backend service
// for AutoUpdateVersion resource.
func TestAutoUpdateServiceVersionCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	version := autoupdatev1pb.AutoUpdateVersion_builder{
		Kind:     types.KindAutoUpdateVersion,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: types.MetaNameAutoUpdateVersion}.Build(),
		Spec: autoupdatev1pb.AutoUpdateVersionSpec_builder{
			Tools: autoupdatev1pb.AutoUpdateVersionSpecTools_builder{
				TargetVersion: "1.2.3",
			}.Build(),
		}.Build(),
	}.Build()

	created, err := service.CreateAutoUpdateVersion(ctx, version)
	require.NoError(t, err)
	diff := cmp.Diff(version, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAutoUpdateVersion(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(version, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	version.GetSpec().SetTools(autoupdatev1pb.AutoUpdateVersionSpecTools_builder{
		TargetVersion: "3.2.1",
	}.Build())
	updated, err := service.UpdateAutoUpdateVersion(ctx, version)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetTools().GetTargetVersion(), updated.GetSpec().GetTools().GetTargetVersion())

	_, err = service.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	err = service.DeleteAutoUpdateVersion(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoUpdateVersion(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	// If we try to conditionally update a missing resource, we receive
	// a CompareFailed instead of a NotFound.
	var revisionMismatchError *trace.CompareFailedError
	_, err = service.UpdateAutoUpdateVersion(ctx, version)
	require.ErrorAs(t, err, &revisionMismatchError)
}

// TestAutoUpdateServiceAgentReportCRUD verifies get/create/update/upsert/delete methods of the backend service
// for AutoUpdateAgentReport resource.
func TestAutoUpdateServiceAgentReportCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	authID := uuid.New()
	oldDate := time.Now()
	newDate := time.Now().Add(2 * time.Minute)

	ctx := context.Background()
	report := autoupdatev1pb.AutoUpdateAgentReport_builder{
		Kind:     types.KindAutoUpdateAgentReport,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: authID.String()}.Build(),
		Spec: autoupdatev1pb.AutoUpdateAgentReportSpec_builder{
			Timestamp: timestamppb.New(oldDate),
			Groups: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroup{
				"": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
					Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
						"1.2.3": autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 10}.Build(),
						"1.2.4": autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 2}.Build(),
					},
				}.Build(),
				"prod": autoupdatev1pb.AutoUpdateAgentReportSpecGroup_builder{
					Versions: map[string]*autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion{
						"1.2.3": autoupdatev1pb.AutoUpdateAgentReportSpecGroupVersion_builder{Count: 5}.Build(),
					},
				}.Build(),
			},
		}.Build(),
	}.Build()

	created, err := service.CreateAutoUpdateAgentReport(ctx, report)
	require.NoError(t, err)
	diff := cmp.Diff(report, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAutoUpdateAgentReport(ctx, authID.String())
	require.NoError(t, err)
	diff = cmp.Diff(report, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	report.GetSpec().SetTimestamp(timestamppb.New(newDate))

	updated, err := service.UpdateAutoUpdateAgentReport(ctx, report)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetTimestamp(), updated.GetSpec().GetTimestamp())

	_, err = service.UpsertAutoUpdateAgentReport(ctx, report)
	require.NoError(t, err)

	err = service.DeleteAutoUpdateAgentReport(ctx, authID.String())
	require.NoError(t, err)

	_, err = service.GetAutoUpdateAgentReport(ctx, authID.String())
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	// If we try to conditionally update a missing resource, we receive
	// a CompareFailed instead of a NotFound.
	var revisionMismatchError *trace.CompareFailedError
	_, err = service.UpdateAutoUpdateAgentReport(ctx, report)
	require.ErrorAs(t, err, &revisionMismatchError)
}

// TestAutoUpdateServiceInvalidNameCreate verifies that configuration and version
// with not constant name is rejected to be created.
func TestAutoUpdateServiceInvalidNameCreate(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	config := autoupdatev1pb.AutoUpdateConfig_builder{
		Kind:     types.KindAutoUpdateConfig,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: "invalid-auto-update-config-name"}.Build(),
		Spec: autoupdatev1pb.AutoUpdateConfigSpec_builder{
			Tools: autoupdatev1pb.AutoUpdateConfigSpecTools_builder{
				Mode: autoupdate.ToolsUpdateModeEnabled,
			}.Build(),
		}.Build(),
	}.Build()

	createdConfig, err := service.CreateAutoUpdateConfig(ctx, config)
	require.Error(t, err)
	require.Nil(t, createdConfig)

	version := autoupdatev1pb.AutoUpdateVersion_builder{
		Kind:     types.KindAutoUpdateVersion,
		Version:  types.V1,
		Metadata: headerv1.Metadata_builder{Name: "invalid-auto-update-version-name"}.Build(),
		Spec: autoupdatev1pb.AutoUpdateVersionSpec_builder{
			Tools: autoupdatev1pb.AutoUpdateVersionSpecTools_builder{
				TargetVersion: "1.2.3",
			}.Build(),
		}.Build(),
	}.Build()

	createdVersion, err := service.CreateAutoUpdateVersion(ctx, version)
	require.Error(t, err)
	require.Nil(t, createdVersion)
}

// TestAutoUpdateServiceInvalidNameUpdate verifies that configuration and version
// with not constant name is rejected to be updated.
func TestAutoUpdateServiceInvalidNameUpdate(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()

	// Validate the config update restriction.
	config, err := autoupdate.NewAutoUpdateConfig(autoupdatev1pb.AutoUpdateConfigSpec_builder{
		Tools: autoupdatev1pb.AutoUpdateConfigSpecTools_builder{
			Mode: autoupdate.ToolsUpdateModeEnabled,
		}.Build(),
	}.Build())
	require.NoError(t, err)

	createdConfig, err := service.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	createdConfig.GetMetadata().SetName("invalid-auto-update-config-name")

	createdConfig, err = service.UpdateAutoUpdateConfig(ctx, createdConfig)
	require.Error(t, err)
	require.Nil(t, createdConfig)

	// Validate the version update restriction.
	version, err := autoupdate.NewAutoUpdateVersion(autoupdatev1pb.AutoUpdateVersionSpec_builder{
		Tools: autoupdatev1pb.AutoUpdateVersionSpecTools_builder{
			TargetVersion: "1.2.3",
		}.Build(),
	}.Build())
	require.NoError(t, err)

	createdVersion, err := service.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	createdVersion.GetMetadata().SetName("invalid-auto-update-version-name")

	createdVersion, err = service.UpdateAutoUpdateVersion(ctx, createdVersion)
	require.Error(t, err)
	require.Nil(t, createdVersion)
}
