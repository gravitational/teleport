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

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/autoupdate/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAutoupdateServiceConfigCRUD verifies get/create/update/upsert/delete methods of the backend service
// for autoupdate config resource.
func TestAutoupdateServiceConfigCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoupdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	config := &autoupdate.AutoupdateConfig{
		Kind:     types.KindAutoupdateConfig,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: types.MetaNameAutoupdateConfig},
		Spec:     &autoupdate.AutoupdateConfigSpec{ToolsAutoupdate: true},
	}

	created, err := service.CreateAutoupdateConfig(ctx, config)
	require.NoError(t, err)
	diff := cmp.Diff(config, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(autoupdate.AutoupdateConfig{}, autoupdate.AutoupdateConfigSpec{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAutoupdateConfig(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(config, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(autoupdate.AutoupdateConfig{}, autoupdate.AutoupdateConfigSpec{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	config.Spec.ToolsAutoupdate = false
	updated, err := service.UpdateAutoupdateConfig(ctx, config)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetToolsAutoupdate(), updated.GetSpec().GetToolsAutoupdate())

	_, err = service.UpsertAutoupdateConfig(ctx, config)
	require.NoError(t, err)

	err = service.DeleteAutoupdateConfig(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoupdateConfig(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	_, err = service.UpdateAutoupdateConfig(ctx, config)
	require.ErrorAs(t, err, &notFoundError)
}

// TestAutoupdateServiceVersionCRUD verifies get/create/update/upsert/delete methods of the backend service
// for autoupdate version resource.
func TestAutoupdateServiceVersionCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAutoupdateService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	version := &autoupdate.AutoupdateVersion{
		Kind:     types.KindAutoupdateVersion,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: types.MetaNameAutoupdateVersion},
		Spec:     &autoupdate.AutoupdateVersionSpec{ToolsVersion: "1.2.3"},
	}

	created, err := service.CreateAutoupdateVersion(ctx, version)
	require.NoError(t, err)
	diff := cmp.Diff(version, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(autoupdate.AutoupdateVersion{}, autoupdate.AutoupdateVersionSpec{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAutoupdateVersion(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(version, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(autoupdate.AutoupdateVersion{}, autoupdate.AutoupdateVersionSpec{}, headerv1.Metadata{}),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	version.Spec.ToolsVersion = "3.2.1"
	updated, err := service.UpdateAutoupdateVersion(ctx, version)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetToolsVersion(), updated.GetSpec().GetToolsVersion())

	_, err = service.UpsertAutoupdateVersion(ctx, version)
	require.NoError(t, err)

	err = service.DeleteAutoupdateVersion(ctx)
	require.NoError(t, err)

	_, err = service.GetAutoupdateVersion(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	_, err = service.UpdateAutoupdateVersion(ctx, version)
	require.ErrorAs(t, err, &notFoundError)
}
