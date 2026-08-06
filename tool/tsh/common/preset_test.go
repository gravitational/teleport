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

func TestNormalizePresetProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		proxy     string
		expected  string
		assertErr require.ErrorAssertionFunc
	}{
		{name: "status URL", proxy: "HTTPS://Proxy.Example.com:443", expected: "proxy.example.com:443", assertErr: require.NoError},
		{name: "HTTPS default port", proxy: "https://proxy.example.com", expected: "proxy.example.com:443", assertErr: require.NoError},
		{name: "HTTP default port", proxy: "http://proxy.example.com", expected: "proxy.example.com:80", assertErr: require.NoError},
		{name: "proxy address", proxy: "proxy.example.com:443", expected: "proxy.example.com:443", assertErr: require.NoError},
		{name: "Teleport default port", proxy: "proxy.example.com", expected: "proxy.example.com:3080", assertErr: require.NoError},
		{name: "legacy web and SSH ports", proxy: "proxy.example.com:443,3023", expected: "proxy.example.com:443", assertErr: require.NoError},
		{name: "URL path", proxy: "https://proxy.example.com/web", assertErr: require.Error},
		{name: "URL credentials", proxy: "https://alice@proxy.example.com", assertErr: require.Error},
		{name: "unsupported scheme", proxy: "ssh://proxy.example.com:3023", assertErr: require.Error},
		{name: "empty URL port", proxy: "https://proxy.example.com:", assertErr: require.Error},
		{name: "invalid port", proxy: "proxy.example.com:70000", assertErr: require.Error},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			actual, err := normalizePresetProxy(test.proxy)
			test.assertErr(t, err)
			require.Equal(t, test.expected, actual)
		})
	}
}

func TestPresetAddCommand(t *testing.T) {
	t.Parallel()

	tshHome := t.TempDir()
	configPath := filepath.Join(tshHome, client.TSHConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o700))
	require.NoError(t, os.WriteFile(configPath, []byte(`# keep this comment
aliases:
  prod-login: login --proxy=prod.example.com
presets:
  staging:
    proxy: staging.example.com:443
`), 0o600))

	stdout := new(bytes.Buffer)
	command := &presetAddCommand{name: "prod", proxyURL: "https://Prod.Example.com:443"}
	cf := &CLIConf{
		HomePath:       tshHome,
		OverrideStdout: stdout,
		TSHConfig: client.TSHConfig{Presets: map[string]client.TSHConfigPreset{
			"staging": {Proxy: "staging.example.com:443"},
		}},
	}

	require.NoError(t, command.run(cf))
	require.Contains(t, stdout.String(), `Preset "prod" added for proxy "prod.example.com:443".`)

	contents, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(contents), "# keep this comment")
	require.Contains(t, string(contents), "prod-login:")

	config, err := client.LoadTSHConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "prod.example.com:443", config.Presets["prod"].Proxy)
	require.Contains(t, config.Presets, "staging")
}

func TestPresetAddRejectsExistingMergedPreset(t *testing.T) {
	t.Parallel()

	command := &presetAddCommand{name: "prod", proxyURL: "https://prod.example.com:443"}
	err := command.run(&CLIConf{
		HomePath: t.TempDir(),
		TSHConfig: client.TSHConfig{Presets: map[string]client.TSHConfigPreset{
			"prod": {Proxy: "prod.example.com:443"},
		}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `preset "prod" already exists`)
}

func TestStatusShowsMatchingPresets(t *testing.T) {
	t.Parallel()

	profile := &profileInfo{ProxyURL: "https://proxy.example.com:443"}
	setMatchingPresets(map[string]client.TSHConfigPreset{
		"z-last":      {Proxy: "https://PROXY.example.com:443"},
		"a-first":     {Proxy: "proxy.example.com"},
		"explicit":    {Proxy: "proxy.example.com:443,3023"},
		"wrong-port":  {Proxy: "proxy.example.com:3080"},
		"other":       {Proxy: "other.example.com:443"},
		"no-proxy":    {},
		"invalid-url": {Proxy: "https://proxy.example.com/path"},
	}, profile)
	require.Equal(t, []string{"a-first", "explicit", "z-last"}, profile.Presets)

	stdout := new(bytes.Buffer)
	printStatus(stdout, false, profile, nil, true)
	require.Contains(t, stdout.String(), "Presets:            a-first, explicit, z-last")

	jsonOutput, err := serializeProfiles(profile, nil, nil, "json")
	require.NoError(t, err)
	require.Contains(t, jsonOutput, `"presets": [`)
	require.Contains(t, jsonOutput, `"a-first"`)
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

func TestPresetAddDoesNotApplyDefaultPreset(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	home := t.TempDir()
	err := Run(context.Background(), []string{"preset", "add", "prod", "https://prod.example.com:443"},
		setHomePath(home),
		func(cf *CLIConf) error {
			cf.OverrideStdout = stdout
			cf.TSHConfig = client.TSHConfig{
				Presets:       map[string]client.TSHConfigPreset{},
				DefaultPreset: "missing",
			}
			return nil
		},
	)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), `Preset "prod" added`)

	config, err := client.LoadTSHConfig(filepath.Join(home, client.TSHConfigPath))
	require.NoError(t, err)
	require.Equal(t, "prod.example.com:443", config.Presets["prod"].Proxy)
}
