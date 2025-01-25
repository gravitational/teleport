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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestBotWorkloadIdentityAPI(t *testing.T) {
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
				Id: "/valid/{{ user.bot_name }}/{{ workload.unix.pid }}",
			},
		},
	}
	workloadIdentity, err = rootClient.WorkloadIdentityResourceServiceClient().
		CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: workloadIdentity,
		})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	listenAddr := url.URL{
		Scheme: "unix",
		Path:   filepath.Join(tmpDir, "workload.sock"),
	}
	onboarding, _ := makeBot(t, rootClient, "api", role.GetName())
	botConfig := defaultBotConfig(t, process, onboarding, config.ServiceConfigs{
		&config.WorkloadIdentityAPIService{
			Selector: config.WorkloadIdentitySelector{
				Name: workloadIdentity.GetMetadata().GetName(),
			},
			Listen: listenAddr.String(),
		},
	}, defaultBotConfigOpts{
		useAuthServer: true,
		insecure:      true,
	})
	botConfig.Oneshot = false
	b := New(botConfig, log)

	// Spin up goroutine for bot to run in
	botCtx, cancelBot := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(botCtx)
		assert.NoError(t, err, "bot should not exit with error")
		cancelBot()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancelBot()
		wg.Wait()
	})

	// This has a little flexibility internally in terms of waiting for the
	// socket to come up, so we don't need a manual sleep/retry here.
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(listenAddr.String()))
	require.NoError(t, err)

	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClient(client),
	)
	require.NoError(t, err)
	defer source.Close()

	// Test FetchX509SVID
	svid, err := source.GetX509SVID()
	require.NoError(t, err)

	expectedSPIFFEID := fmt.Sprintf("spiffe://root/valid/api/%d", os.Getpid())
	require.Equal(t, expectedSPIFFEID, svid.ID.String())
	require.Equal(t, expectedSPIFFEID, svid.Certificates[0].URIs[0].String())
	_, _, err = x509svid.Verify(svid.Certificates, source)
	require.NoError(t, err)

	// Test FetchX509Bundles
	set, err := client.FetchX509Bundles(ctx)
	require.NoError(t, err)
	_, _, err = x509svid.Verify(svid.Certificates, set)
	require.NoError(t, err)

	// Test FetchJWTSVID
	jwtSVID, err := client.FetchJWTSVID(ctx, jwtsvid.Params{
		Audience: "example.com",
	})
	require.NoError(t, err)

	// Check against ValidateJWTSVID
	parsed, err := client.ValidateJWTSVID(ctx, jwtSVID.Marshal(), "example.com")
	require.NoError(t, err)
	require.Equal(t, expectedSPIFFEID, parsed.ID.String())
	// Perform local validation with bundles from FetchJWTBundles
	jwtBundles, err := client.FetchJWTBundles(ctx)
	require.NoError(t, err)
	_, err = jwtsvid.ParseAndValidate(jwtSVID.Marshal(), jwtBundles, []string{"example.com"})
	require.NoError(t, err)
}
