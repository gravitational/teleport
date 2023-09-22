/*
Copyright 2016-2021 Gravitational, Inc.

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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// bootstrap check
func TestTeleportMain(t *testing.T) {
	// get the hostname
	hostname, err := os.Hostname()
	require.NoError(t, err)

	fixtureDir := t.TempDir()
	// generate the fixture config file
	configFile := filepath.Join(fixtureDir, "teleport.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configData), 0660))

	// generate the fixture bootstrap file
	bootstrapEntries := []struct{ fileName, kind, name string }{
		{"role.yaml", types.KindRole, "role_name"},
		{"github.yaml", types.KindGithubConnector, "github"},
		{"user.yaml", types.KindRole, "user"},
	}
	var bootstrapData []byte
	for _, entry := range bootstrapEntries {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "resources", entry.fileName))
		require.NoError(t, err)
		bootstrapData = append(bootstrapData, data...)
		bootstrapData = append(bootstrapData, "\n---\n"...)
	}
	bootstrapFile := filepath.Join(fixtureDir, "bootstrap.yaml")
	require.NoError(t, os.WriteFile(bootstrapFile, bootstrapData, 0660))

	// set defaults to test-mode (non-existing files&locations)
	defaults.ConfigFilePath = "/tmp/teleport/etc/teleport.yaml"
	defaults.DataDir = "/tmp/teleport/var/lib/teleport"

	t.Run("Default", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start"},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Equal(t, hostname, conf.Hostname)
		require.Equal(t, "/tmp/teleport/var/lib/teleport", conf.DataDir)
		require.True(t, conf.Auth.Enabled)
		require.True(t, conf.SSH.Enabled)
		require.True(t, conf.Proxy.Enabled)
		require.Equal(t, os.Stdout, conf.Console)
		require.Equal(t, log.ErrorLevel, log.GetLevel())
	})

	t.Run("RolesFlag", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node"},
			InitOnly: true,
		})
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		_, cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=proxy"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.True(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		_, cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=auth"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.True(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)
	})

	t.Run("ConfigFile", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node", "--labels=a=a1,b=b1", "--config=" + configFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, log.DebugLevel, conf.Log.GetLevel())
		require.Equal(t, "hvostongo.example.org", conf.Hostname)

		token, err := conf.Token()
		require.NoError(t, err)
		require.Equal(t, "xxxyyy", token)
		require.Equal(t, "10.5.5.5", conf.AdvertiseIP)
		require.Equal(t, map[string]string{"a": "a1", "b": "b1"}, conf.SSH.Labels)
	})

	t.Run("Bootstrap", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--bootstrap", bootstrapFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Equal(t, len(bootstrapEntries), len(conf.Auth.BootstrapResources))
		for i, entry := range bootstrapEntries {
			require.Equal(t, entry.kind, conf.Auth.BootstrapResources[i].GetKind(), entry.fileName)
			require.Equal(t, entry.name, conf.Auth.BootstrapResources[i].GetName(), entry.fileName)
			require.NoError(t, conf.Auth.BootstrapResources[i].CheckAndSetDefaults(), entry.fileName)
		}
	})
	t.Run("ApplyOnStartup", func(t *testing.T) {
		_, cmd, conf := Run(Options{
			Args:     []string{"start", "--apply-on-startup", bootstrapFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.Equal(t, len(bootstrapEntries), len(conf.Auth.ApplyOnStartupResources))
		for i, entry := range bootstrapEntries {
			require.Equal(t, entry.kind, conf.Auth.ApplyOnStartupResources[i].GetKind(), entry.fileName)
			require.Equal(t, entry.name, conf.Auth.ApplyOnStartupResources[i].GetName(), entry.fileName)
			require.NoError(t, conf.Auth.ApplyOnStartupResources[i].CheckAndSetDefaults(), entry.fileName)
		}
	})
}

func TestConfigure(t *testing.T) {
	t.Run("Dump", func(t *testing.T) {
		err := onConfigDump(dumpFlags{
			// typo
			output: "sddout",
		})
		require.IsType(t, trace.BadParameter(""), err)

		err = onConfigDump(dumpFlags{
			output: "file://" + filepath.Join(t.TempDir(), "test"),
			SampleFlags: config.SampleFlags{
				ClusterName: "example.com",
			},
		})
		require.NoError(t, err)

		// stdout
		err = onConfigDump(dumpFlags{
			output: "stdout",
		})
		require.NoError(t, err)
	})

	t.Run("Defaults", func(t *testing.T) {
		flags := dumpFlags{}
		err := flags.CheckAndSetDefaults()
		require.NoError(t, err)
	})
}

func TestDumpConfigFile(t *testing.T) {
	tt := []struct {
		name      string
		outputURI string
		contents  string
		comment   string
		assert    require.ErrorAssertionFunc
	}{
		{
			name:      "errors on relative path",
			assert:    require.Error,
			outputURI: "../",
		},
		{
			name:      "doesn't error on unexisting config path",
			assert:    require.NoError,
			outputURI: fmt.Sprintf("%s/unexisting/dir/%s", t.TempDir(), "config.yaml"),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dumpConfigFile(tc.outputURI, tc.contents, tc.comment)
			tc.assert(t, err)
		})
	}
}

const configData = `
version: v3
teleport:
  advertise_ip: 10.5.5.5
  nodename: hvostongo.example.org
  auth_server: auth.server.example.org:3024
  auth_token: xxxyyy
  log:
    output: stderr
    severity: DEBUG
  connection_limits:
    max_connections: 90
    max_users: 91
    rates:
    - period: 1m1s
      average: 70
      burst: 71
    - period: 10m10s
      average: 170
      burst: 171

auth_service:
  enabled: yes
  listen_addr: tcp://auth

ssh_service:
  enabled: no
  listen_addr: tcp://ssh
  labels:
    name: mondoserver
    role: follower
  commands:
  - name: hostname
    command: [/bin/hostname]
    period: 10ms
  - name: date
    command: [/bin/date]
    period: 20ms
`
