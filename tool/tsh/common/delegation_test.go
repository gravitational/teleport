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

package common

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	delegationv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
)

func TestBuildCreateDelegationSessionRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		conf               *CLIConf
		wantUser           string
		wantTTL            time.Duration
		wantResources      []*delegationv1pb.DelegationResourceSpec
		wantAuthorizedUser []*delegationv1pb.DelegationUserSpec
		wantErr            string
	}{
		{
			name: "explicit resources",
			conf: &CLIConf{
				Username:                       "bob",
				SessionTTL:                     10 * time.Minute,
				DelegationAllowNodes:           []string{"node1"},
				DelegationAllowDatabases:       []string{"database1"},
				DelegationAllowApps:            []string{"app1"},
				DelegationAllowKubeClusters:    []string{"cluster1"},
				DelegationAllowWindowsDesktops: []string{"desktop1"},
				DelegationAllowGitServers:      []string{"git1"},
				DelegationBots:                 []string{"bot1", "bot2"},
			},
			wantUser: "bob",
			wantTTL:  10 * time.Minute,
			wantResources: []*delegationv1pb.DelegationResourceSpec{
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindNode, Name: "node1"}.Build(),
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindDatabase, Name: "database1"}.Build(),
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindApp, Name: "app1"}.Build(),
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindKubernetesCluster, Name: "cluster1"}.Build(),
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindWindowsDesktop, Name: "desktop1"}.Build(),
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.KindGitServer, Name: "git1"}.Build(),
			},
			wantAuthorizedUser: []*delegationv1pb.DelegationUserSpec{
				delegationv1pb.DelegationUserSpec_builder{
					Kind:    types.KindBot,
					BotName: proto.String("bot1"),
				}.Build(),
				delegationv1pb.DelegationUserSpec_builder{
					Kind:    types.KindBot,
					BotName: proto.String("bot2"),
				}.Build(),
			},
		},
		{
			name: "wildcard resources",
			conf: &CLIConf{
				Username:           "bob",
				SessionTTL:         5 * time.Minute,
				DelegationAllowAll: true,
				DelegationBots:     []string{"bot1"},
			},
			wantUser: "bob",
			wantTTL:  5 * time.Minute,
			wantResources: []*delegationv1pb.DelegationResourceSpec{
				delegationv1pb.DelegationResourceSpec_builder{Kind: types.Wildcard, Name: types.Wildcard}.Build(),
			},
			wantAuthorizedUser: []*delegationv1pb.DelegationUserSpec{
				delegationv1pb.DelegationUserSpec_builder{
					Kind:    types.KindBot,
					BotName: proto.String("bot1"),
				}.Build(),
			},
		},
		{
			name: "allow all is mutually exclusive",
			conf: &CLIConf{
				Username:             "bob",
				SessionTTL:           5 * time.Minute,
				DelegationAllowAll:   true,
				DelegationAllowNodes: []string{"node1"},
				DelegationBots:       []string{"bot1"},
			},
			wantErr: "--allow-all is mutually exclusive",
		},
		{
			name: "requires resource selection",
			conf: &CLIConf{
				Username:       "bob",
				SessionTTL:     5 * time.Minute,
				DelegationBots: []string{"bot1"},
			},
			wantErr: "at least one resource must be provided",
		},
		{
			name: "requires bot",
			conf: &CLIConf{
				Username:             "bob",
				SessionTTL:           5 * time.Minute,
				DelegationAllowNodes: []string{"node1"},
			},
			wantErr: "at least one --bot must be provided",
		},
		{
			name: "requires ttl",
			conf: &CLIConf{
				Username:             "bob",
				DelegationAllowNodes: []string{"node1"},
				DelegationBots:       []string{"bot1"},
			},
			wantErr: "--session-ttl must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := buildCreateDelegationSessionRequest(tt.conf)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantUser, req.GetSpec().GetUser())
			require.Equal(t, tt.wantTTL, req.GetTtl().AsDuration())
			require.Empty(t, cmp.Diff(tt.wantResources, req.GetSpec().GetResources(), protocmp.Transform()))
			require.Empty(t, cmp.Diff(tt.wantAuthorizedUser, req.GetSpec().GetAuthorizedUsers(), protocmp.Transform()))
		})
	}
}
