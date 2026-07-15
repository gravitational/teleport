// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

func TestValidateDelegationSession(t *testing.T) {
	t.Parallel()

	require.NoError(t, services.ValidateDelegationSession(validDelegationSession()))

	testCases := map[string]struct {
		modFn func(*delegationv1.DelegationSession)
		error string
	}{
		"wrong kind": {
			func(p *delegationv1.DelegationSession) { p.SetKind("") },
			"kind: must be delegation_session",
		},
		"wrong version": {
			func(p *delegationv1.DelegationSession) { p.SetVersion("") },
			"version: must be v1",
		},
		"missing name": {
			func(p *delegationv1.DelegationSession) { p.GetMetadata().SetName("") },
			"metadata.name: is required",
		},
		"no expiration": {
			func(p *delegationv1.DelegationSession) { p.GetMetadata().ClearExpires() },
			"metadata.expires: is required",
		},
		"no resources": {
			func(p *delegationv1.DelegationSession) { p.GetSpec().SetResources(nil) },
			"spec.resources: at least one resource is required",
		},
		"missing user": {
			func(p *delegationv1.DelegationSession) { p.GetSpec().SetUser("") },
			"spec.user: is required",
		},
		"invalid resource identifier": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().GetResources()[0] = delegationv1.DelegationResourceSpec_builder{Kind: "no-such-kind"}.Build()
			},
			"spec.resources[0]: invalid resource spec",
		},
		"wildcard resource kind but not name": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().SetResources([]*delegationv1.DelegationResourceSpec{
					delegationv1.DelegationResourceSpec_builder{
						Kind: types.Wildcard,
						Name: "something-specific",
					}.Build(),
				})
			},
			"name must also be '*'",
		},
		"wildcard resource name but not kind": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().SetResources([]*delegationv1.DelegationResourceSpec{
					delegationv1.DelegationResourceSpec_builder{
						Kind: types.KindApp,
						Name: types.Wildcard,
					}.Build(),
				})
			},
			"kind must also be '*'",
		},
		"mixed wildcard and explicit resources": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().SetResources([]*delegationv1.DelegationResourceSpec{
					delegationv1.DelegationResourceSpec_builder{
						Kind: types.Wildcard,
						Name: types.Wildcard,
					}.Build(),
					delegationv1.DelegationResourceSpec_builder{
						Kind: types.KindApp,
						Name: "my-app",
					}.Build(),
				})
			},
			"wildcard is mutually exclusive with explicit resources",
		},
		"no authorized users": {
			func(p *delegationv1.DelegationSession) { p.GetSpec().SetAuthorizedUsers(nil) },
			"spec.authorized_users: at least one user is required",
		},
		"invalid user kind": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().GetAuthorizedUsers()[0].SetKind("dragon")
			},
			"spec.authorized_users[0].kind: must be bot",
		},
		"no bot name": {
			func(p *delegationv1.DelegationSession) {
				p.GetSpec().GetAuthorizedUsers()[0].SetBotName("")
			},
			"spec.authorized_users[0].bot_name: is required",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			profile := validDelegationSession()

			tc.modFn(profile)

			err := services.ValidateDelegationSession(profile)
			require.ErrorContains(t, err, tc.error)
			require.True(t, trace.IsBadParameter(err))
		})
	}
}

func validDelegationSession() *delegationv1.DelegationSession {
	return delegationv1.DelegationSession_builder{
		Kind:    types.KindDelegationSession,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(1 * time.Hour)),
		}.Build(),
		Spec: delegationv1.DelegationSessionSpec_builder{
			User: "alex@example.com",
			Resources: []*delegationv1.DelegationResourceSpec{
				delegationv1.DelegationResourceSpec_builder{
					Kind: types.KindApp,
					Name: "my-app",
				}.Build(),
				delegationv1.DelegationResourceSpec_builder{
					Kind: types.KindDatabase,
					Name: "my-database",
				}.Build(),
				delegationv1.DelegationResourceSpec_builder{
					Kind: types.KindKubernetesCluster,
					Name: "my-k8s-cluster",
				}.Build(),
				delegationv1.DelegationResourceSpec_builder{
					Kind: types.KindNode,
					Name: "my-node",
				}.Build(),
			},
			AuthorizedUsers: []*delegationv1.DelegationUserSpec{
				delegationv1.DelegationUserSpec_builder{
					Kind:    types.KindBot,
					BotName: proto.String("my-bot"),
				}.Build(),
			},
		}.Build(),
	}.Build()
}
