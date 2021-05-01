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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/trace"
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

	// generate the fixture config file
	configFile := filepath.Join(t.TempDir(), "teleport.yaml")
	err = ioutil.WriteFile(configFile, []byte(YAMLConfig), 0660)
	require.NoError(t, err)

	// set defaults to test-mode (non-existing files&locations)
	defaults.ConfigFilePath = "/tmp/teleport/etc/teleport.yaml"
	defaults.DataDir = "/tmp/teleport/var/lib/teleport"

	t.Run("Default", func(t *testing.T) {
		cmd, conf := Run(Options{
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
		cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node"},
			InitOnly: true,
		})
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=proxy"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.True(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)

		cmd, conf = Run(Options{
			Args:     []string{"start", "--roles=auth"},
			InitOnly: true,
		})
		require.False(t, conf.SSH.Enabled)
		require.True(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, "start", cmd)
	})

	t.Run("ConfigFile", func(t *testing.T) {
		cmd, conf := Run(Options{
			Args:     []string{"start", "--roles=node", "--labels=a=a1,b=b1", "--config=" + configFile},
			InitOnly: true,
		})
		require.Equal(t, "start", cmd)
		require.True(t, conf.SSH.Enabled)
		require.False(t, conf.Auth.Enabled)
		require.False(t, conf.Proxy.Enabled)
		require.Equal(t, log.DebugLevel, conf.Log.GetLevel())
		require.Equal(t, "hvostongo.example.org", conf.Hostname)
		require.Equal(t, "xxxyyy", conf.Token)
		require.Equal(t, "10.5.5.5", conf.AdvertiseIP)
		require.Equal(t, map[string]string{"a": "a1", "b": "b1"}, conf.SSH.Labels)
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

const YAMLConfig = `
teleport:
  advertise_ip: 10.5.5.5
  nodename: hvostongo.example.org
  auth_servers:
    - auth.server.example.org:3024
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
