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
	"path/filepath"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

// .tsh config must go in a subdir as all .yaml files in .tsh get
// parsed automatically by the profile loader and results in yaml
// unmarshal errors.
const tshConfigPath = "config/config.yaml"

// TshConfig represents configuration loaded from the tsh config file.
type TshConfig struct {
	// ExtraHeaders are additional http headers to be included in
	// webclient requests.
	ExtraHeaders []ExtraProxyHeaders `yaml:"add_headers"`
}

// ExtraProxyHeaders represents the headers to include with the
// webclient.
type ExtraProxyHeaders struct {
	// Proxy is the domain of the proxy for these set of Headers, can contain globs.
	Proxy string `yaml:"proxy"`
	// Headers are the http header key values.
	Headers map[string]string `yaml:"headers,omitempty"`
}

func loadConfig(homePath string) (*TshConfig, error) {
	confPath := filepath.Join(profile.FullProfilePath(homePath), tshConfigPath)
	configFile, err := os.Open(confPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer configFile.Close()
	cfg := TshConfig{}
	err = yaml.NewDecoder(configFile).Decode(&cfg)
	return &cfg, trace.Wrap(err)
}
