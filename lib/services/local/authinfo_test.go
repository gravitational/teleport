/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

	authinfo1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/authinfo/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
)

// TestAutoInfoServiceCRUD verifies create/read/update/delete methods of the backend service
// for AuthInfo resource.
func TestAutoInfoServiceCRUD(t *testing.T) {
	t.Parallel()

	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)

	service, err := NewAuthInfoService(bk)
	require.NoError(t, err)

	ctx := context.Background()
	info := &authinfo1pb.AuthInfo{
		Kind:     types.KindAuthInfo,
		Version:  types.V1,
		Metadata: &headerv1.Metadata{Name: types.MetaNameAuthInfo},
		Spec: &authinfo1pb.AuthInfoSpec{
			TeleportVersion: "1.2.3",
		},
	}

	created, err := service.CreateAuthInfo(ctx, info)
	require.NoError(t, err)
	diff := cmp.Diff(info, created,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.NotEmpty(t, created.GetMetadata().GetRevision())

	got, err := service.GetAuthInfo(ctx)
	require.NoError(t, err)
	diff = cmp.Diff(info, got,
		cmpopts.IgnoreFields(headerv1.Metadata{}, "Revision"),
		protocmp.Transform(),
	)
	require.Empty(t, diff)
	require.Equal(t, created.GetMetadata().GetRevision(), got.GetMetadata().GetRevision())

	info.Spec.TeleportVersion = "3.2.1"
	updated, err := service.UpdateAuthInfo(ctx, info)
	require.NoError(t, err)
	require.NotEqual(t, got.GetSpec().GetTeleportVersion(), updated.GetSpec().GetTeleportVersion())

	err = service.DeleteAuthInfo(ctx)
	require.NoError(t, err)

	_, err = service.GetAuthInfo(ctx)
	var notFoundError *trace.NotFoundError
	require.ErrorAs(t, err, &notFoundError)

	// If we try to conditionally update a missing resource, we receive
	// a CompareFailed instead of a NotFound.
	var revisionMismatchError *trace.CompareFailedError
	_, err = service.UpdateAuthInfo(ctx, info)
	require.ErrorAs(t, err, &revisionMismatchError)
}
