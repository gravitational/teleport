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
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/client"
)

func TestSetDefaultPreset(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), client.TSHConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o700))
	require.NoError(t, os.WriteFile(configPath, []byte(`# keep this comment
aliases:
  prod-login: login --proxy=prod.example.com
presets:
  prod:
    proxy: prod.example.com:443
default_preset: old
`), 0o600))

	require.NoError(t, setDefaultPreset(configPath, "prod"))

	contents, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(contents), "# keep this comment")
	require.Contains(t, string(contents), "prod-login:")

	config, err := client.LoadTSHConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "prod", config.DefaultPreset)
	require.Contains(t, config.Presets, "prod")
}

func TestPresetLSCommand(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	command := &presetLSCommand{format: "text"}
	cf := &CLIConf{
		OverrideStdout: stdout,
		TSHConfig: client.TSHConfig{
			Presets: map[string]client.TSHConfigPreset{
				"staging": {Proxy: "staging.example.com:443"},
				"prod":    {Proxy: "prod.example.com:443", Cluster: "prod", User: "alice"},
			},
			DefaultPreset: "prod",
		},
	}

	require.NoError(t, command.run(cf))
	require.Contains(t, stdout.String(), "Preset")
	require.Contains(t, stdout.String(), "prod")
	require.Contains(t, stdout.String(), "prod.example.com:443")
	require.Contains(t, stdout.String(), "*")
	require.Less(t, bytes.Index(stdout.Bytes(), []byte("prod")), bytes.Index(stdout.Bytes(), []byte("staging")))
}

func TestPresetUseCommand(t *testing.T) {
	t.Parallel()

	tshHome := t.TempDir()
	configPath := filepath.Join(tshHome, client.TSHConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o700))
	require.NoError(t, os.WriteFile(configPath, []byte(`
presets:
  prod:
    proxy: prod.example.com:443
`), 0o600))
	stdout := new(bytes.Buffer)
	command := &presetUseCommand{name: "prod"}
	cf := &CLIConf{
		HomePath:       tshHome,
		OverrideStdout: stdout,
		TSHConfig: client.TSHConfig{Presets: map[string]client.TSHConfigPreset{
			"prod": {Proxy: "prod.example.com:443"},
		}},
	}

	require.NoError(t, command.run(cf))
	require.Contains(t, stdout.String(), `Default preset set to "prod".`)

	config, err := client.LoadTSHConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "prod", config.DefaultPreset)
}

func TestPresetManagementDoesNotApplyDefaultPreset(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	err := Run(context.Background(), []string{"preset", "ls"},
		setHomePath(t.TempDir()),
		func(cf *CLIConf) error {
			cf.OverrideStdout = stdout
			cf.TSHConfig = client.TSHConfig{
				Presets: map[string]client.TSHConfigPreset{
					"prod": {Proxy: "prod.example.com:443"},
				},
				// If preset management accidentally applies the default, this missing
				// name causes the command to fail before it can list presets.
				DefaultPreset: "missing",
			}
			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "prod")
}
