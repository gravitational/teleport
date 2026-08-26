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
			func(p *delegationv1.DelegationSession) { p.Kind = "" },
			"kind: must be delegation_session",
		},
		"wrong version": {
			func(p *delegationv1.DelegationSession) { p.Version = "" },
			"version: must be v1",
		},
		"missing name": {
			func(p *delegationv1.DelegationSession) { p.Metadata.Name = "" },
			"metadata.name: is required",
		},
		"no expiration": {
			func(p *delegationv1.DelegationSession) { p.Metadata.Expires = nil },
			"metadata.expires: is required",
		},
		"no resources": {
			func(p *delegationv1.DelegationSession) { p.Spec.Resources = nil },
			"spec.resources: at least one resource is required",
		},
		"missing user": {
			func(p *delegationv1.DelegationSession) { p.Spec.User = "" },
			"spec.user: is required",
		},
		"invalid resource identifier": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.Resources[0] = &delegationv1.DelegationResourceSpec{Kind: "no-such-kind"}
			},
			"spec.resources[0]: invalid resource spec",
		},
		"wildcard resource kind but not name": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.Resources = []*delegationv1.DelegationResourceSpec{
					{
						Kind: types.Wildcard,
						Name: "something-specific",
					},
				}
			},
			"name must also be '*'",
		},
		"wildcard resource name but not kind": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.Resources = []*delegationv1.DelegationResourceSpec{
					{
						Kind: types.KindApp,
						Name: types.Wildcard,
					},
				}
			},
			"kind must also be '*'",
		},
		"mixed wildcard and explicit resources": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.Resources = []*delegationv1.DelegationResourceSpec{
					{
						Kind: types.Wildcard,
						Name: types.Wildcard,
					},
					{
						Kind: types.KindApp,
						Name: "my-app",
					},
				}
			},
			"wildcard is mutually exclusive with explicit resources",
		},
		"no authorized users": {
			func(p *delegationv1.DelegationSession) { p.Spec.AuthorizedUsers = nil },
			"spec.authorized_users: at least one user is required",
		},
		"invalid user kind": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.AuthorizedUsers[0].Kind = "dragon"
			},
			"spec.authorized_users[0].kind: must be bot",
		},
		"no bot name": {
			func(p *delegationv1.DelegationSession) {
				p.Spec.AuthorizedUsers[0].Matcher = &delegationv1.DelegationUserSpec_BotName{}
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
	return &delegationv1.DelegationSession{
		Kind:    types.KindDelegationSession,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name:    uuid.NewString(),
			Expires: timestamppb.New(time.Now().Add(1 * time.Hour)),
		},
		Spec: &delegationv1.DelegationSessionSpec{
			User: "alex@example.com",
			Resources: []*delegationv1.DelegationResourceSpec{
				{
					Kind: types.KindApp,
					Name: "my-app",
				},
				{
					Kind: types.KindDatabase,
					Name: "my-database",
				},
				{
					Kind: types.KindKubernetesCluster,
					Name: "my-k8s-cluster",
				},
				{
					Kind: types.KindNode,
					Name: "my-node",
				},
			},
			AuthorizedUsers: []*delegationv1.DelegationUserSpec{
				{
					Kind:    types.KindBot,
					Matcher: &delegationv1.DelegationUserSpec_BotName{BotName: "my-bot"},
				},
			},
		},
	}
}
