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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	autoupdatepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/autoupdate"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAutoUpdateServiceConfigCRUD verifies get/create/update/upsert/delete methods of the backend service
// for auto update config resource.
func TestAutoUpdateServiceConfigCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	config := &autoupdatepb.AutoUpdateConfig{
		Kind:     types.KindAutoUpdateConfig,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: types.MetaNameAutoUpdateConfig},
		Spec:     &autoupdatepb.AutoUpdateConfigSpec{ToolsAutoupdate: true},
	}

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

	config.Spec.ToolsAutoupdate = false
	updated, err := service.UpdateAutoUpdateConfig(ctx, config)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetToolsAutoupdate(), updated.GetSpec().GetToolsAutoupdate())

	_, err = service.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	err = service.DeleteAutoUpdateConfig(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoUpdateConfig(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	_, err = service.UpdateAutoUpdateConfig(ctx, config)
	require.ErrorAs(t, err, &notFoundError)
}

// TestAutoUpdateServiceVersionCRUD verifies get/create/update/upsert/delete methods of the backend service
// for auto update version resource.
func TestAutoUpdateServiceVersionCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoUpdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	version := &autoupdatepb.AutoUpdateVersion{
		Kind:     types.KindAutoUpdateVersion,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: types.MetaNameAutoUpdateVersion},
		Spec:     &autoupdatepb.AutoUpdateVersionSpec{ToolsVersion: "1.2.3"},
	}

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

	version.Spec.ToolsVersion = "3.2.1"
	updated, err := service.UpdateAutoUpdateVersion(ctx, version)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetToolsVersion(), updated.GetSpec().GetToolsVersion())

	_, err = service.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	err = service.DeleteAutoUpdateVersion(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoUpdateVersion(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	_, err = service.UpdateAutoUpdateVersion(ctx, version)
	require.ErrorAs(t, err, &notFoundError)
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
	config := &autoupdatepb.AutoUpdateConfig{
		Kind:     types.KindAutoUpdateConfig,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: "invalid-auto-update-config-name"},
		Spec:     &autoupdatepb.AutoUpdateConfigSpec{ToolsAutoupdate: true},
	}

	createdConfig, err := service.CreateAutoUpdateConfig(ctx, config)
	require.Error(t, err)
	require.Nil(t, createdConfig)

	version := &autoupdatepb.AutoUpdateVersion{
		Kind:     types.KindAutoUpdateVersion,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: "invalid-auto-update-version-name"},
		Spec:     &autoupdatepb.AutoUpdateVersionSpec{ToolsVersion: "1.2.3"},
	}

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
	config, err := autoupdate.NewAutoUpdateConfig(&autoupdatepb.AutoUpdateConfigSpec{ToolsAutoupdate: true})
	require.NoError(t, err)

	createdConfig, err := service.UpsertAutoUpdateConfig(ctx, config)
	require.NoError(t, err)

	createdConfig.GetMetadata().Name = "invalid-auto-update-config-name"

	createdConfig, err = service.UpdateAutoUpdateConfig(ctx, createdConfig)
	require.Error(t, err)
	require.Nil(t, createdConfig)

	// Validate the version update restriction.
	version, err := autoupdate.NewAutoUpdateVersion(&autoupdatepb.AutoUpdateVersionSpec{ToolsVersion: "1.2.3"})
	require.NoError(t, err)

	createdVersion, err := service.UpsertAutoUpdateVersion(ctx, version)
	require.NoError(t, err)

	createdVersion.GetMetadata().Name = "invalid-auto-update-version-name"

	createdVersion, err = service.UpdateAutoUpdateVersion(ctx, createdVersion)
	require.Error(t, err)
	require.Nil(t, createdVersion)
}
