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

package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestLoadConfigNonExistingFile(t *testing.T) {
	t.Parallel()

	fullFilePath := "/tmp/doesntexist." + uuid.NewString()
	gotConfig, gotErr := LoadTSHConfig(fullFilePath)
	require.NoError(t, gotErr)
	require.Equal(t, &TSHConfig{}, gotConfig)
}

func TestLoadConfigEmptyFile(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "test-telelport")
	require.NoError(t, err)

	_, err = file.Write([]byte(" "))
	require.NoError(t, err)

	gotConfig, gotErr := LoadTSHConfig(file.Name())
	require.NoError(t, gotErr)
	require.Equal(t, &TSHConfig{}, gotConfig)
}

func TestLoadAllConfigs(t *testing.T) {
	t.Parallel()

	writeConf := func(fn string, config TSHConfig) {
		dir, _ := filepath.Split(fn)
		err := os.MkdirAll(dir, 0777)
		require.NoError(t, err)
		out, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(fn, out, 0777)
		require.NoError(t, err)
	}

	tmp := t.TempDir()

	globalPath := filepath.Join(tmp, "etc", "tsh_global.yaml")
	globalConf := TSHConfig{
		ExtraHeaders: []ExtraProxyHeaders{{
			Proxy:   "global",
			Headers: map[string]string{"bar": "123"},
		}},
	}

	homeDir := filepath.Join(tmp, "home", "myuser", ".tsh")
	userPath := filepath.Join(homeDir, "config", "config.yaml")
	userConf := TSHConfig{
		ExtraHeaders: []ExtraProxyHeaders{{
			Proxy:   "user",
			Headers: map[string]string{"bar": "456"},
		}},
	}

	writeConf(globalPath, globalConf)
	writeConf(userPath, userConf)

	config, err := LoadAllConfigs(globalPath, homeDir)
	require.NoError(t, err)
	require.Equal(t, &TSHConfig{
		ExtraHeaders: []ExtraProxyHeaders{
			{
				Proxy:   "user",
				Headers: map[string]string{"bar": "456"},
			},
			{
				Proxy:   "global",
				Headers: map[string]string{"bar": "123"},
			},
		},
		Aliases: map[string]string{},
	}, config)

}

func TestTshConfigMerge(t *testing.T) {
	t.Parallel()

	sampleConfig := TSHConfig{
		ExtraHeaders: []ExtraProxyHeaders{{
			Proxy: "foo",
			Headers: map[string]string{
				"bar": "baz",
			},
		}},
		Aliases: map[string]string{},
	}

	tests := []struct {
		name    string
		config1 *TSHConfig
		config2 *TSHConfig
		want    TSHConfig
	}{
		{
			name:    "empty + empty = empty",
			config1: nil,
			config2: nil,
			want:    TSHConfig{Aliases: map[string]string{}},
		},
		{
			name:    "empty + x = x",
			config1: &sampleConfig,
			config2: nil,
			want:    sampleConfig,
		},
		{
			name:    "x + empty = x",
			config1: nil,
			config2: &sampleConfig,
			want:    sampleConfig,
		},
		{
			name: "headers combine different proxies",
			config1: &TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{{
					Proxy: "foo",
					Headers: map[string]string{
						"bar": "123",
					},
				}}},
			config2: &TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{{
					Proxy: "bar",
					Headers: map[string]string{
						"baz": "456",
					},
				}},
			},
			want: TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{
					{
						Proxy: "bar",
						Headers: map[string]string{
							"baz": "456",
						},
					},
					{
						Proxy: "foo",
						Headers: map[string]string{
							"bar": "123",
						},
					},
				},
				Aliases: map[string]string{},
			},
		},
		{
			name: "headers combine same proxy",
			config1: &TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{{
					Proxy: "foo",
					Headers: map[string]string{
						"bar": "123",
					},
				}}},
			config2: &TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{{
					Proxy: "foo",
					Headers: map[string]string{
						"bar": "456",
					},
				}}},
			want: TSHConfig{
				ExtraHeaders: []ExtraProxyHeaders{
					{
						Proxy: "foo",
						Headers: map[string]string{
							"bar": "456",
						},
					},
					{
						Proxy: "foo",
						Headers: map[string]string{
							"bar": "123",
						},
					},
				},
				Aliases: map[string]string{},
			},
		},
		{
			name: "aliases combine",
			config1: &TSHConfig{
				ExtraHeaders:   nil,
				ProxyTemplates: nil,
				Aliases: map[string]string{
					"foo": "foo1",
					"bar": "bar1",
				},
			},
			config2: &TSHConfig{
				ExtraHeaders:   nil,
				ProxyTemplates: nil,
				Aliases: map[string]string{
					"baz": "baz2",
					"bar": "bar2",
				},
			},
			want: TSHConfig{
				ExtraHeaders:   nil,
				ProxyTemplates: nil,
				Aliases: map[string]string{
					"foo": "foo1",
					"baz": "baz2",
					"bar": "bar2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config3 := tt.config1.Merge(tt.config2)
			require.Equal(t, tt.want, config3)
		})
	}
}

// TestProxyTemplatesApply verifies proxy templates matching functionality.
func TestProxyTemplatesApply(t *testing.T) {
	t.Parallel()

	tshConfig := TSHConfig{
		ProxyTemplates: ProxyTemplates{
			{
				Template: `^(.+)\.(us.example.com):(.+)$`,
				Proxy:    "$2:443",
				Cluster:  "$2",
				Host:     "$1:$3",
			},
			{
				Template: `^(.+)\.(eu.example.com):(.+)$`,
				Proxy:    "$2:3080",
			},
			{
				Template: `^(.+)\.(private-leaf):(.+)$`,
				Cluster:  "$2",
			},
			{
				Template: `^(.+)\.(au.example.com):(.+)$`,
				Host:     "$1:4022",
			},
			{
				Template: `^(.+)\.search.example.com:(.+)$`,
				Search:   "$1",
			},
			{
				Template: `^(.+)\.query.example.com:(.+)$`,
				Query:    `labels.animal == "$1"`,
			},
		},
	}
	require.NoError(t, tshConfig.Check())

	tests := []struct {
		testName       string
		inFullHostname string
		outProxy       string
		outHost        string
		outCluster     string
		outSearch      string
		outQuery       string
		outMatch       bool
	}{
		{
			testName:       "matches first template",
			inFullHostname: "node-1.us.example.com:3022",
			outProxy:       "us.example.com:443",
			outCluster:     "us.example.com",
			outHost:        "node-1:3022",
			outMatch:       true,
		},
		{
			testName:       "matches second template",
			inFullHostname: "node-1.eu.example.com:3022",
			outProxy:       "eu.example.com:3080",
			outMatch:       true,
		},
		{
			testName:       "matches third template",
			inFullHostname: "node-1.private-leaf:3022",
			outCluster:     "private-leaf",
			outMatch:       true,
		},
		{
			testName:       "matches fourth template",
			inFullHostname: "node-1.au.example.com:3022",
			outHost:        "node-1:4022",
			outMatch:       true,
		},
		{
			testName:       "does not match templates",
			inFullHostname: "node-1.cn.example.com:3022",
			outMatch:       false,
		},
		{
			testName:       "matches search",
			inFullHostname: "llama.search.example.com:3022",
			outSearch:      "llama",
			outMatch:       true,
		},
		{
			testName:       "matches query",
			inFullHostname: "llama.query.example.com:3022",
			outQuery:       `labels.animal == "llama"`,
			outMatch:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			expanded, match := tshConfig.ProxyTemplates.Apply(test.inFullHostname)
			require.Equal(t, test.outMatch, match)
			if !match {
				require.Nil(t, expanded)
				return
			}

			require.Equal(t, test.outProxy, expanded.Proxy)
			require.Equal(t, test.outHost, expanded.Host)
			require.Equal(t, test.outCluster, expanded.Cluster)
			require.Equal(t, test.outSearch, expanded.Search)
			require.Equal(t, test.outQuery, expanded.Query)
		})
	}
}

func TestProfileCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		profile   Profile
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "valid profile with proxy only",
			profile:   Profile{Proxy: "proxy.example.com:443"},
			assertErr: require.NoError,
		},
		{
			name:      "valid mfa_mode sso",
			profile:   Profile{MFAMode: "sso"},
			assertErr: require.NoError,
		},
		{
			name:      "invalid mfa_mode",
			profile:   Profile{MFAMode: "bogus"},
			assertErr: require.Error,
		},
		{
			name:      "invalid add_keys_to_agent",
			profile:   Profile{AddKeysToAgent: "maybe"},
			assertErr: require.Error,
		},
		{
			name:      "empty profile",
			profile:   Profile{},
			assertErr: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			profile := test.profile
			test.assertErr(t, profile.Check())
		})
	}
}

func TestTSHConfigCheckProfiles(t *testing.T) {
	t.Parallel()

	t.Run("valid profiles and default", func(t *testing.T) {
		t.Parallel()
		config := TSHConfig{
			Profiles: map[string]Profile{
				"prod": {Proxy: "prod.example.com:443", MFAMode: "sso"},
				"dev":  {Proxy: "dev.example.com:443"},
			},
			DefaultProfile: "prod",
		}
		require.NoError(t, config.Check())
	})

	t.Run("default_profile pointing to missing key fails", func(t *testing.T) {
		t.Parallel()
		config := TSHConfig{
			Profiles: map[string]Profile{
				"prod": {Proxy: "prod.example.com:443"},
			},
			DefaultProfile: "staging",
		}
		err := config.Check()
		require.Error(t, err)
		require.Contains(t, err.Error(), "staging")
	})

	t.Run("profile with bad mfa_mode fails with profile name in error", func(t *testing.T) {
		t.Parallel()
		config := TSHConfig{
			Profiles: map[string]Profile{
				"broken": {Proxy: "broken.example.com:443", MFAMode: "bogus"},
			},
		}
		err := config.Check()
		require.Error(t, err)
		require.Contains(t, err.Error(), "broken")
	})
}

func TestTSHConfigGetProfile(t *testing.T) {
	t.Parallel()

	prod := Profile{Proxy: "prod.example.com:443", Cluster: "prod"}
	config := TSHConfig{
		Profiles: map[string]Profile{
			"prod": prod,
			"dev":  {Proxy: "dev.example.com:443"},
		},
	}

	t.Run("empty name returns BadParameter", func(t *testing.T) {
		t.Parallel()
		_, err := config.GetProfile("")
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), "expected BadParameter, got %v", err)
	})

	t.Run("nil profiles returns NotFound", func(t *testing.T) {
		t.Parallel()
		empty := TSHConfig{}
		_, err := empty.GetProfile("prod")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
	})

	t.Run("existing profile is returned", func(t *testing.T) {
		t.Parallel()
		got, err := config.GetProfile("prod")
		require.NoError(t, err)
		require.Equal(t, prod, got)
	})

	t.Run("missing profile returns NotFound with available names", func(t *testing.T) {
		t.Parallel()
		_, err := config.GetProfile("missing")
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
		assert.Contains(t, err.Error(), "prod")
		assert.Contains(t, err.Error(), "dev")
	})
}

func TestTSHConfigMergeProfiles(t *testing.T) {
	t.Parallel()

	newBase := func() *TSHConfig {
		return &TSHConfig{
			Profiles: map[string]Profile{
				"a":      {Proxy: "a.example.com:443"},
				"shared": {Proxy: "X"},
			},
			DefaultProfile: "a",
		}
	}

	t.Run("profiles merged per-key, other wins on shared", func(t *testing.T) {
		t.Parallel()
		base := newBase()
		other := &TSHConfig{
			Profiles: map[string]Profile{
				"b":      {Proxy: "b.example.com:443"},
				"shared": {Proxy: "Y"},
			},
		}
		merged := base.Merge(other)
		require.Contains(t, merged.Profiles, "a")
		require.Contains(t, merged.Profiles, "b")
		require.Contains(t, merged.Profiles, "shared")
		require.Equal(t, "Y", merged.Profiles["shared"].Proxy)
	})

	t.Run("default_profile kept from base when other empty", func(t *testing.T) {
		t.Parallel()
		base := newBase()
		other := &TSHConfig{DefaultProfile: ""}
		merged := base.Merge(other)
		require.Equal(t, "a", merged.DefaultProfile)
	})

	t.Run("default_profile taken from other when set", func(t *testing.T) {
		t.Parallel()
		base := newBase()
		other := &TSHConfig{
			Profiles: map[string]Profile{
				"b": {Proxy: "b.example.com:443"},
			},
			DefaultProfile: "b",
		}
		merged := base.Merge(other)
		require.Equal(t, "b", merged.DefaultProfile)
	})
}

func TestProfileYAMLUnmarshal(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }

	const raw = `
default_profile: prod
profiles:
  prod:
    proxy: prod.example.com:443
    cluster: prod-cluster
    mfa_mode: sso
    headless: true
`

	var config TSHConfig
	require.NoError(t, yaml.Unmarshal([]byte(raw), &config))

	require.Equal(t, "prod", config.DefaultProfile)
	require.Contains(t, config.Profiles, "prod")

	prod := config.Profiles["prod"]
	require.Equal(t, "prod.example.com:443", prod.Proxy)
	require.Equal(t, "prod-cluster", prod.Cluster)
	require.Equal(t, "sso", prod.MFAMode)
	require.NotNil(t, prod.Headless)
	require.Equal(t, boolPtr(true), prod.Headless)
	require.True(t, *prod.Headless)
}
