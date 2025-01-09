/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/openssh"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestJoinOpenSSH(t *testing.T) {
	testDir := t.TempDir()

	cfg := helpers.InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Loopback,
		Logger:      utils.NewSlogLoggerForTests(),
	}
	cfg.Listeners = helpers.StandardListenerSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)
	var err error
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "cluster")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
	rcConf.SSH.Enabled = false
	rcConf.Proxy.DisableWebInterface = true
	rcConf.Version = "v3"
	rcConf.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{
			{
				Roles: []types.SystemRole{types.RoleNode},
				Token: "token",
			},
		},
	})
	require.NoError(t, err)

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)
	err = rc.Start()
	require.NoError(t, err)
	defer rc.StopAll()

	ctx := context.Background()

	opensshConfigPath := filepath.Join(testDir, "sshd_config")
	f, err := os.Create(opensshConfigPath)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	restartPath := filepath.Join(testDir, "restarted")
	teleportDataDir := filepath.Join(testDir, "teleport_openssh")

	openSSHCfg := servicecfg.MakeDefaultConfig()

	openSSHCfg.OpenSSH.Enabled = true
	err = config.ConfigureOpenSSH(&config.CommandLineFlags{
		DataDir:           teleportDataDir,
		ProxyServer:       rc.Web,
		AuthToken:         "token",
		JoinMethod:        string(types.JoinMethodToken),
		OpenSSHConfigPath: opensshConfigPath,
		RestartOpenSSH:    true,
		RestartCommand:    fmt.Sprintf("touch %q", restartPath),
		CheckCommand:      "echo okay",
		Labels:            "hello=true",
		Address:           "127.0.0.1:22",
		InsecureMode:      true,
		Debug:             true,
	}, openSSHCfg)
	require.NoError(t, err)

	err = service.RunWithSignalChannel(ctx, *openSSHCfg, nil, nil)
	require.NoError(t, err)

	client := rc.GetSiteAPI(rc.Secrets.SiteName)

	sshdConf, err := os.ReadFile(opensshConfigPath)
	require.NoError(t, err)
	require.Contains(t, string(sshdConf), fmt.Sprintf("Include %s", filepath.Join(teleportDataDir, "sshd.conf")))

	// check a node with the flags specified exists
	require.Eventually(t, helpers.FindNodeWithLabel(t, ctx, client, "hello", "true"), time.Second*2, time.Millisecond*50)
	// check the mock sshd RestartCommand command was in fact called
	require.FileExists(t, restartPath)

	keysDir := filepath.Join(teleportDataDir, "openssh")
	// check all the appropriate key files were made
	require.DirExists(t, keysDir)
	cabytes, err := os.ReadFile(filepath.Join(keysDir, openssh.TeleportOpenSSHCA))
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(keysDir, openssh.TeleportCert))
	require.FileExists(t, filepath.Join(keysDir, openssh.TeleportKey))

	allOpenSSHCAs := getOpenSSHCAs(t, ctx, client)
	require.ElementsMatch(t, bytes.Split(bytes.TrimSpace(cabytes), []byte("\n")), allOpenSSHCAs)
}

func getOpenSSHCAs(t *testing.T, ctx context.Context, cl authclient.ClientI) [][]byte {
	t.Helper()
	cas, err := cl.GetCertAuthorities(ctx, types.OpenSSHCA, false)
	require.NoError(t, err)
	var caBytes [][]byte
	for _, ca := range cas {
		for _, key := range ca.GetTrustedSSHKeyPairs() {
			caBytes = append(caBytes, bytes.TrimSpace(key.PublicKey))
		}
	}
	return caBytes
}
