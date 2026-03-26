/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreateBeam(t *testing.T) {
	t.Parallel()

	services := newBeamsTestPack(t)

	// Create the beam.
	beam := testBeam("shining-orbit")
	params := testCreateBeamParams(t, beam)

	beam, err := services.beam.CreateBeam(t.Context(), params)
	require.NoError(t, err)
	require.NotEmpty(t, beam.GetMetadata().GetRevision())

	// Reading beam by UUID should work.
	stored, err := services.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(beam, stored, protocmp.Transform()))

	// Reading beam by alias should work.
	storedByAlias, err := services.beam.GetBeamByAlias(t.Context(), beam.GetStatus().GetAlias())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(beam, storedByAlias, protocmp.Transform()))

	// Token should be stored.
	storedToken, err := services.token.GetToken(t.Context(), params.Token.GetName())
	require.NoError(t, err)
	require.Equal(t, params.Token.GetName(), storedToken.GetName())

	// Bot user should be stored.
	storedUser, err := services.user.GetUser(t.Context(), params.BotUser.GetName(), false)
	require.NoError(t, err)
	require.Equal(t, params.BotUser.GetName(), storedUser.GetName())

	// Bot role should be stored.
	storedRole, err := services.role.GetRole(t.Context(), params.BotRole.GetName())
	require.NoError(t, err)
	require.Equal(t, params.BotRole.GetName(), storedRole.GetName())

	// Workload identity should be stored.
	storedWorkloadIdentity, err := services.workloadIdentity.GetWorkloadIdentity(t.Context(), params.WorkloadIdentity.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(
		params.WorkloadIdentity,
		storedWorkloadIdentity,
		protocmp.Transform(),
		protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
	))

	// Reusing the alias should result in an error.
	_, err = services.beam.CreateBeam(
		t.Context(),
		testCreateBeamParams(t, testBeam(beam.GetStatus().GetAlias())),
	)
	require.ErrorContains(t, err, "beam alias or resource name already in-use")
	require.True(t, trace.IsAlreadyExists(err))
}

func TestUpdateBeamCreateNode(t *testing.T) {
	t.Parallel()

	services := newBeamsTestPack(t)

	beam, err := services.beam.CreateBeam(t.Context(), testCreateBeamParams(t, testBeam("steady-river")))
	require.NoError(t, err)

	node := testNode(t, uuid.NewString())
	beam.Status.NodeId = node.GetName()
	beam.Status.SshAddr = node.GetAddr()

	updated, err := services.beam.UpdateBeamCreateNode(t.Context(), beam, node)
	require.NoError(t, err)
	require.NotEqual(t, beam.GetMetadata().GetRevision(), updated.GetMetadata().GetRevision())

	// Beam should be updated.
	storedBeam, err := services.beam.GetBeam(t.Context(), updated.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(updated, storedBeam, protocmp.Transform()))

	// Node should be created.
	storedNode, err := services.presence.GetNode(t.Context(), node.GetNamespace(), node.GetName())
	require.NoError(t, err)
	require.Equal(t, node.GetName(), storedNode.GetName())
	require.Equal(t, node.GetNamespace(), storedNode.GetNamespace())
	require.Equal(t, node.GetAddr(), storedNode.GetAddr())

	// Incorrect revision should be rejected.
	storedBeam.Metadata.Revision = "incorrect"
	_, err = services.beam.UpdateBeamCreateNode(t.Context(), storedBeam, node)
	require.ErrorIs(t, err, backend.ErrConditionFailed)
}

func TestUpdateBeamApp(t *testing.T) {
	t.Parallel()

	services := newBeamsTestPack(t)

	beam, err := services.beam.CreateBeam(t.Context(), testCreateBeamParams(t, testBeam("calm-sky")))
	require.NoError(t, err)

	app := testApp(t, uuid.NewString())
	beam.Status.AppName = app.GetName()
	beam.Status.AppAddrHttp = "127.0.0.1:8080"

	beam, err = services.beam.UpdateBeamCreateApp(t.Context(), beam, app)
	require.NoError(t, err)

	storedBeam, err := services.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(beam, storedBeam, protocmp.Transform()))

	storedApp, err := services.app.GetApp(t.Context(), app.GetName())
	require.NoError(t, err)
	require.Equal(t, app.GetName(), storedApp.GetName())
	require.Equal(t, app.GetURI(), storedApp.GetURI())

	beam.Status.AppName = ""
	beam.Status.AppAddrHttp = ""

	beam, err = services.beam.UpdateBeamDeleteApp(t.Context(), beam, app.GetName())
	require.NoError(t, err)

	storedBeam, err = services.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(beam, storedBeam, protocmp.Transform()))

	_, err = services.app.GetApp(t.Context(), app.GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.beam.UpdateBeamDeleteApp(t.Context(), beam, "")
	require.True(t, trace.IsBadParameter(err))
}

func TestDeleteBeam(t *testing.T) {
	t.Parallel()

	services := newBeamsTestPack(t)

	beam := testBeam("deep-forest")
	params := testCreateBeamParams(t, beam)

	beam, err := services.beam.CreateBeam(t.Context(), params)
	require.NoError(t, err)

	node := testNode(t, uuid.NewString())
	beam.Status.NodeId = node.GetName()
	beam.Status.SshAddr = node.GetAddr()
	beam, err = services.beam.UpdateBeamCreateNode(t.Context(), beam, node)
	require.NoError(t, err)

	app := testApp(t, uuid.NewString())
	beam.Status.AppName = app.GetName()
	beam.Status.AppAddrHttp = "127.0.0.1:8080"
	beam, err = services.beam.UpdateBeamCreateApp(t.Context(), beam, app)
	require.NoError(t, err)

	err = services.beam.DeleteBeam(t.Context(), beam.GetMetadata().GetName())
	require.NoError(t, err)

	_, err = services.beam.GetBeam(t.Context(), beam.GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.beam.GetBeamByAlias(t.Context(), beam.GetStatus().GetAlias())
	require.True(t, trace.IsNotFound(err))

	_, err = services.token.GetToken(t.Context(), params.Token.GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.user.GetUser(t.Context(), params.BotUser.GetName(), false)
	require.True(t, trace.IsNotFound(err))

	_, err = services.role.GetRole(t.Context(), params.BotRole.GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.workloadIdentity.GetWorkloadIdentity(t.Context(), params.WorkloadIdentity.GetMetadata().GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.presence.GetNode(t.Context(), node.GetNamespace(), node.GetName())
	require.True(t, trace.IsNotFound(err))

	_, err = services.app.GetApp(t.Context(), app.GetName())
	require.True(t, trace.IsNotFound(err))
}

func TestBeamServiceGetBeamByAlias(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	services := newBeamsTestPack(t)

	beam, err := services.beam.CreateBeam(ctx,
		testCreateBeamParams(t, testBeam("brisk-otter")),
	)
	require.NoError(t, err)

	got, err := services.beam.GetBeamByAlias(ctx, "brisk-otter")
	require.NoError(t, err)
	require.Equal(t,
		beam.GetMetadata().GetName(),
		got.GetMetadata().GetName(),
	)
}

type beamsTestPack struct {
	beam             *BeamService
	token            *ProvisioningService
	user             *IdentityService
	role             *AccessService
	workloadIdentity *WorkloadIdentityService
	presence         *PresenceService
	app              *AppService
}

func newBeamsTestPack(t *testing.T) beamsTestPack {
	t.Helper()

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	beamService, err := NewBeamService(backend)
	require.NoError(t, err)

	userService, err := NewIdentityService(backend)
	require.NoError(t, err)

	workloadIdentityService, err := NewWorkloadIdentityService(backend)
	require.NoError(t, err)

	return beamsTestPack{
		beam:             beamService,
		token:            NewProvisioningService(backend),
		user:             userService,
		role:             NewAccessService(backend),
		workloadIdentity: workloadIdentityService,
		presence:         NewPresenceService(backend),
		app:              NewAppService(backend),
	}
}

func testBeam(alias string) *beamsv1.Beam {
	return &beamsv1.Beam{
		Kind:    types.KindBeam,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Spec: &beamsv1.BeamSpec{
			Egress:         beamsv1.EgressMode_EGRESS_MODE_RESTRICTED,
			AllowedDomains: []string{"example.com."},
			Publish: &beamsv1.PublishSpec{
				Port:     8080,
				Protocol: beamsv1.Protocol_PROTOCOL_HTTP,
			},
			Expires: timestamppb.New(time.Now().Add(time.Hour)),
		},
		Status: &beamsv1.BeamStatus{
			User:                 "alice",
			Alias:                alias,
			BotName:              uuid.NewString(),
			JoinTokenName:        uuid.NewString(),
			WorkloadIdentityName: uuid.NewString(),
			ComputeStatus:        beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_PENDING,
		},
	}
}

func testWorkloadIdentity(name string, expires *timestamppb.Timestamp) *workloadidentityv1pb.WorkloadIdentity {
	return &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    name,
			Expires: expires,
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/test/" + name,
			},
		},
	}
}

func testCreateBeamParams(t *testing.T, beam *beamsv1.Beam) services.CreateBeamParams {
	t.Helper()

	token, err := types.NewProvisionTokenFromSpec(
		beam.GetStatus().GetJoinTokenName(),
		beam.GetMetadata().GetExpires().AsTime(),
		types.ProvisionTokenSpecV2{
			Roles:   []types.SystemRole{types.RoleBot},
			BotName: beam.GetStatus().GetBotName(),
		},
	)
	require.NoError(t, err)

	botUser, err := types.NewUser(services.BotResourceName(beam.GetStatus().GetBotName()))
	require.NoError(t, err)

	botRole, err := types.NewRole(services.BotResourceName(beam.GetStatus().GetBotName()), types.RoleSpecV6{})
	require.NoError(t, err)

	return services.CreateBeamParams{
		Beam:             beam,
		Token:            token,
		BotUser:          botUser,
		BotRole:          botRole,
		WorkloadIdentity: testWorkloadIdentity(beam.GetStatus().GetWorkloadIdentityName(), beam.GetMetadata().GetExpires()),
	}
}

func testNode(t *testing.T, name string) types.Server {
	t.Helper()

	node, err := types.NewServer(name, types.KindNode, types.ServerSpecV2{
		Addr:     "127.0.0.1:3022",
		Hostname: name,
	})
	require.NoError(t, err)
	return node
}

func testApp(t *testing.T, name string) types.Application {
	t.Helper()

	app, err := types.NewAppV3(types.Metadata{Name: name}, types.AppSpecV3{
		URI: "http://localhost:8080",
	})
	require.NoError(t, err)
	return app
}
