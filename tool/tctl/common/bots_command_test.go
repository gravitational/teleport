/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestBots(t *testing.T) {
	dynAddr := helpers.NewDynamicServiceAddr(t)
	fc := &config.FileConfig{
		Global: config.Global{
			DataDir: t.TempDir(),
		},
		Proxy: config.Proxy{
			Service: config.Service{
				EnabledFlag: "true",
			},
			WebAddr: dynAddr.WebAddr,
			TunAddr: dynAddr.TunnelAddr,
		},
		Auth: config.Auth{
			Service: config.Service{
				EnabledFlag:   "true",
				ListenAddress: dynAddr.AuthAddr,
			},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	authProc := makeAndRunTestAuthServer(t, withFileConfig(fc), withFileDescriptors(dynAddr.Descriptors))

	// rotate HostCA so we get multiple CA pins.
	err := authProc.GetAuthServer().RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.HostCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	resp, err := authProc.GetAuthServer().GetClusterCACert(ctx)
	require.NoError(t, err)
	caPins, err := tlsca.CalculatePins(resp.TLSCA)
	require.NoError(t, err)
	require.Len(t, caPins, 2)

	roleYAMLPath := filepath.Join(t.TempDir(), "role.yaml")
	require.NoError(t, os.WriteFile(roleYAMLPath, []byte(botRoleYAML), 0644))
	_, err = runResourceCommand(t, fc, []string{"create", roleYAMLPath})
	require.NoError(t, err)

	buf, err := runBotsCommand(t, fc, []string{"add", "hal9000", "--roles", "robot"})
	require.NoError(t, err)

	wantOut := fmt.Sprintf(`--ca-pin="%s" \
   --ca-pin="%s"`, caPins[0], caPins[1])
	require.Contains(t, buf.String(), wantOut)
}

const botRoleYAML = `kind: role
version: v7
metadata:
  name: robot
spec:
  allow:
    db_labels:
      '*': '*'
    db_names: ["*"]
    db_users: ["*"]`
