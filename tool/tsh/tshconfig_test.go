/*
Copyright 2022 Gravitational, Inc.

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

package main

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
	gotConfig, gotErr := loadConfig(fullFilePath)
	require.NoError(t, gotErr)
	require.Equal(t, &TSHConfig{}, gotConfig)
}

func TestLoadConfigEmptyFile(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "test-telelport")
	require.NoError(t, err)

	_, err = file.Write([]byte(" "))
	require.NoError(t, err)

	gotConfig, gotErr := loadConfig(file.Name())
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

	config, err := loadAllConfigs(CLIConf{
		GlobalTshConfigPath: globalPath,
		HomePath:            homeDir,
	})

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

// TestProxyTemplates verifies proxy templates matching functionality.
func TestProxyTemplates(t *testing.T) {
	t.Parallel()

	tshConfig := TSHConfig{
		ProxyTemplates: ProxyTemplates{
			{
				Template: `^(.+)\.(us.example.com):(.+)$`,
				Proxy:    "$2:443",
				Host:     "$1:$3",
			},
			{
				Template: `^(.+)\.(eu.example.com):(.+)$`,
				Proxy:    "$2:3080",
			},
		},
	}
	require.NoError(t, tshConfig.Check())
	tests := []struct {
		testName       string
		inFullHostname string
		outProxy       string
		outHost        string
		outMatch       bool
	}{
		{
			testName:       "matches first template",
			inFullHostname: "node-1.us.example.com:3022",
			outProxy:       "us.example.com:443",
			outHost:        "node-1:3022",
			outMatch:       true,
		},
		{
			testName:       "matches second template",
			inFullHostname: "node-1.eu.example.com:3022",
			outProxy:       "eu.example.com:3080",
			outHost:        "node-1.eu.example.com:3022",
			outMatch:       true,
		},
		{
			testName:       "does not match templates",
			inFullHostname: "node-1.cn.example.com:3022",
			outMatch:       false,
		},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			proxy, host, match := tshConfig.ProxyTemplates.Apply(test.inFullHostname)
			require.Equal(t, test.outProxy, proxy)
			require.Equal(t, test.outHost, host)
			require.Equal(t, test.outMatch, match)
		})
	}
}
