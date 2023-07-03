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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integration/helpers"
	"github.com/gravitational/teleport/lib/auth"
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
		Log:         utils.NewLoggerForTests(),
	}
	cfg.Listeners = helpers.StandardListenerSetup(t, &cfg.Fds)
	rc := helpers.NewInstance(t, cfg)
	var err error
	rcConf := servicecfg.MakeDefaultConfig()
	rcConf.DataDir = filepath.Join(testDir, "cluster")
	rcConf.Auth.Enabled = true
	rcConf.Proxy.Enabled = true
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

	err = service.Run(ctx, *openSSHCfg, nil)
	require.NoError(t, err)

	client := rc.GetSiteAPI(rc.Secrets.SiteName)

	sshdConf, err := os.ReadFile(opensshConfigPath)
	require.NoError(t, err)
	require.Contains(t, string(sshdConf), fmt.Sprintf("Include %s", filepath.Join(teleportDataDir, "sshd.conf")))

	// check a node with the flags specified exists
	require.Eventually(t, findNodeWithLabel(t, ctx, client, "hello"), time.Second*2, time.Millisecond*50)
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

func getOpenSSHCAs(t *testing.T, ctx context.Context, cl auth.ClientI) [][]byte {
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

func findNodeWithLabel(t *testing.T, ctx context.Context, cl auth.ClientI, key string) func() bool {
	t.Helper()
	return func() bool {
		servers, err := cl.ListResources(ctx, proto.ListResourcesRequest{
			ResourceType: types.KindNode,
			Namespace:    defaults.Namespace,
			Labels:       map[string]string{key: ""},
			Limit:        1,
		})
		require.NoError(t, err)
		return len(servers.Resources) >= 1
	}
}
