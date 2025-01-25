// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package tbot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestBotWorkloadIdentityJWT(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	role, err := types.NewRole("issue-foo", types.RoleSpecV6{
		Allow: types.RoleConditions{
			WorkloadIdentityLabels: map[string]apiutils.Strings{
				"foo": []string{"bar"},
			},
			Rules: []types.Rule{
				{
					Resources: []string{types.KindWorkloadIdentity},
					Verbs:     []string{types.VerbRead, types.VerbList},
				},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	workloadIdentity := &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "foo-bar-bizz",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/valid/{{ user.bot_name }}",
			},
		},
	}
	workloadIdentity, err = rootClient.WorkloadIdentityResourceServiceClient().
		CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: workloadIdentity,
		})
	require.NoError(t, err)

	t.Run("By Name", func(t *testing.T) {
		tmpDir := t.TempDir()
		onboarding, _ := makeBot(t, rootClient, "by-name", role.GetName())
		botConfig := defaultBotConfig(t, process, onboarding, config.ServiceConfigs{
			&config.WorkloadIdentityJWTService{
				Selector: config.WorkloadIdentitySelector{
					Name: workloadIdentity.GetMetadata().GetName(),
				},
				Destination: &config.DestinationDirectory{
					Path: tmpDir,
				},
				Audiences: []string{"example", "foo"},
			},
		}, defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		})
		botConfig.Oneshot = true
		b := New(botConfig, log)
		// Run Bot with 10 second timeout to catch hangs.
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		require.NoError(t, b.Run(ctx))

		jwtBytes, err := os.ReadFile(filepath.Join(tmpDir, config.JWTSVIDPath))
		require.NoError(t, err)
		jwt, err := jwtsvid.ParseInsecure(string(jwtBytes), []string{"example"})
		require.NoError(t, err)
		require.Equal(t, "spiffe://root/valid/by-name", jwt.ID.String())
		require.Equal(t, []string{"example", "foo"}, jwt.Audience)
	})
	t.Run("By Labels", func(t *testing.T) {
		tmpDir := t.TempDir()
		onboarding, _ := makeBot(t, rootClient, "by-labels", role.GetName())
		botConfig := defaultBotConfig(t, process, onboarding, config.ServiceConfigs{
			&config.WorkloadIdentityJWTService{
				Selector: config.WorkloadIdentitySelector{
					Labels: map[string][]string{
						"foo": {"bar"},
					},
				},
				Destination: &config.DestinationDirectory{
					Path: tmpDir,
				},
				Audiences: []string{"example"},
			},
		}, defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		})
		botConfig.Oneshot = true
		b := New(botConfig, log)
		// Run Bot with 10 second timeout to catch hangs.
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		require.NoError(t, b.Run(ctx))

		jwtBytes, err := os.ReadFile(filepath.Join(tmpDir, config.JWTSVIDPath))
		require.NoError(t, err)
		jwt, err := jwtsvid.ParseInsecure(string(jwtBytes), []string{"example"})
		require.NoError(t, err)
		require.Equal(t, "spiffe://root/valid/by-labels", jwt.ID.String())
		require.Equal(t, []string{"example"}, jwt.Audience)
	})
}
