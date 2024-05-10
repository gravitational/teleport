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
	"path"
	"testing"

	"github.com/google/uuid"
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
		dir, _ := path.Split(fn)
		err := os.MkdirAll(dir, 0777)
		require.NoError(t, err)
		out, err := yaml.Marshal(config)
		require.NoError(t, err)
		err = os.WriteFile(fn, out, 0777)
		require.NoError(t, err)
	}

	tmp := t.TempDir()

	globalPath := path.Join(tmp, "etc", "tsh_global.yaml")
	globalConf := TSHConfig{
		ExtraHeaders: []ExtraProxyHeaders{{
			Proxy:   "global",
			Headers: map[string]string{"bar": "123"},
		}},
	}

	homeDir := path.Join(tmp, "home", "myuser", ".tsh")
	userPath := path.Join(homeDir, "config", "config.yaml")
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
