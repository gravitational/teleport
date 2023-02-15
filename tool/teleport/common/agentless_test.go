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

	"github.com/google/uuid"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

type agentlessCtx struct {
	server    *auth.TestTLSServer
	clock     clockwork.Clock
	agentless *agentless
}

func setupAgentlessTest(t *testing.T) *agentlessCtx {
	t.Helper()

	return nil
}

func TestAgentless(t *testing.T) {
	clock := clockwork.NewFakeClock()
	tt, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: "cluster",
		Dir:         t.TempDir(),
		Clock:       clock,
	})
	require.NoError(t, err)
	defer tt.Close()

	server, err := tt.NewTestTLSServer()
	require.NoError(t, err)
	defer server.Close()

	testdir := t.TempDir()
	addr := utils.FromAddr(server.Addr())
	uuid := uuid.NewString()

	keysDir := filepath.Join(testdir, "keys")
	require.NoError(t, os.MkdirAll(keysDir, 0700))
	backupKeysDir := filepath.Join(testdir, "keys_backup")
	require.NoError(t, os.MkdirAll(backupKeysDir, 0700))

	var sshdRestarted bool
	ag := agentless{
		uuid:                 uuid,
		principals:           []string{uuid},
		hostname:             "hostname",
		addr:                 &addr,
		imds:                 nil,
		defaultKeysDir:       keysDir,
		defaultBackupKeysDir: backupKeysDir,
		clock:                clock,
		restartSSHD: func() error {
			sshdRestarted = true
			return nil
		},
	}

	ctx := context.Background()

	token, err := types.NewProvisionToken("join-token", types.SystemRoles{
		types.RoleNode,
	}, clock.Now().Add(10*time.Minute))
	require.NoError(t, err)

	require.NoError(t, server.Auth().CreateToken(ctx, token))

	configPath := filepath.Join(testdir, "config")
	cfgFile, err := os.Create(configPath)
	require.NoError(t, err)
	require.NoError(t, cfgFile.Close())

	clf := config.CommandLineFlags{
		OpenSSHKeysPath:       keysDir,
		OpenSSHKeysBackupPath: backupKeysDir,
		OpenSSHConfigPath:     configPath,
		NodeName:              uuid,
		FIPS:                  false,
		JoinMethod:            "token",
		AuthToken:             "join-token",
		InsecureMode:          true,
	}

	err = ag.openSSHInitialJoin(ctx, clf)
	require.NoError(t, err)
	checkKeysExist(t, keysDir)
	require.True(t, sshdRestarted)
	checkConfigFile(t, keysDir, configPath)

	rotate(ctx, t, server.Auth())

	err = ag.openSSHRotateStageUpdate(ctx, clf)

	require.NoError(t, err)
	checkKeysExist(t, backupKeysDir)

	err = ag.openSSHRotateStageRollback(clf)
	require.NoError(t, err)
	checkKeysExist(t, keysDir)
	_, err = os.Stat(backupKeysDir)
	require.True(t, os.IsNotExist(err))
}

func checkConfigFile(t *testing.T, keyDir, configPath string) {
	t.Helper()
	configContents, err := os.ReadFile(configPath)
	require.NoError(t, err)

	expected := fmt.Sprintf(`
%s
TrustedUserCaKeys %s
HostKey %s
HostCertificate %s
### Section end
`,
		sshdConfigSectionModificationHeader,
		filepath.Join(keyDir, "teleport_user_ca.pub"),
		filepath.Join(keyDir, "teleport"),
		filepath.Join(keyDir, "teleport-cert.pub"),
	)

	require.Equal(t, string(expected), string(configContents))
}

func checkKeysExist(t *testing.T, keysDir string) {
	t.Helper()
	for _, keyfile := range []string{teleportKey, teleportCert, teleportOpenSSHCA} {
		_, err := os.Stat(filepath.Join(keysDir, keyfile))
		require.NoError(t, err)
	}
}

func rotate(ctx context.Context, t *testing.T, server *auth.Server) {
	t.Helper()

	err := server.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.OpenSSHCA,
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	err = server.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.OpenSSHCA,
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	err = server.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.OpenSSHCA,
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	err = server.RotateCertAuthority(ctx, auth.RotateRequest{
		Type:        types.OpenSSHCA,
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)
}
