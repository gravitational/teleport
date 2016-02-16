/*
Copyright 2015-16 Gravitational, Inc.

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
package config

import (
	"path/filepath"
	"strings"
	"time"

	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/gravitational/trace"
)

// FileConfig structre represents the teleport configuration stored in a config file
// in YAML format (usually /etc/teleport.yaml)
//
// Use config.ReadFromFile() to read the parsed FileConfig from a YAML file.
type FileConfig struct {
	Global `yaml:"teleport,omitempty"`
	Auth   Auth  `yaml:"auth_service,omitempty"`
	SSH    SSH   `yaml:"ssh_service,omitempty"`
	Proxy  Proxy `yaml:"proxy_service,omitempty"`
}

// ReadFromFile reads Teleport configuration from a file. Currently only YAML
// format is supported
func ReadFromFile(fp string) (fc *FileConfig, err error) {
	ext := strings.ToLower(filepath.Ext(fp))
	if ext != ".yaml" && ext != ".yml" {
		return nil, trace.Errorf("invalid configuration file type: '%v'. Only .yml is supported", fp)
	}

	fc = &FileConfig{}
	// read & parse YAML config:
	bytes, err := ioutil.ReadFile(fp)
	if err != nil {
		return nil, trace.Wrap(err, "failed reading Teleport configuration: %v", fp)
	}
	if err = yaml.Unmarshal(bytes, fc); err != nil {
		return nil, trace.Wrap(err, "failed to parse Teleport configuration: %v", fp)
	}
	return fc, nil
}

// DebugDump allows for quick YAML dumping of the config
func (conf *FileConfig) DebugDumpToYAML() string {
	bytes, err := yaml.Marshal(&conf)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

type AuthServer struct {
	Address string `yaml:"address"` // "tcp://127.0.0.1:3024"
	Token   string `yaml:"token"`   // "xxxxxxx"
}

type ConnectionRate struct {
	Period  time.Duration `yaml:"period"`
	Average int           `yaml:"average"`
	Burst   int           `yaml:"burst"`
}

type ConnectionLimits struct {
	MaxConnections int              `yaml:"max_connections"`
	MaxUsers       int              `yaml:"max_users"`
	Rates          []ConnectionRate `yaml:"rates,omitempty"`
}

type Log struct {
	Output   string `yaml:"output,omitempty"`
	Severity string `yaml:"severity,omitempty"`
}

type StorageBackend struct {
	Type  string `yaml:"type"`
	Param string `yaml:"param"`
}

// 'teleport' (global) section of the config file
type Global struct {
	NodeName    string           `yaml:"nodename,omitempty"`
	AuthServers []AuthServer     `yaml:"auth_servers,omitempty"`
	Limits      ConnectionLimits `yaml:"connection_limits,omitempty"`
	Logger      Log              `yaml:"log,omitempty"`
	Storage     StorageBackend   `yaml:"storage,omitempty"`
}

type Service struct {
	EnabledFlag   string `yaml:"enabled,omitempty"`
	ListenAddress string `yaml:"listen_addr,omitempty"`
}

// Configured determines if a given "_service" section has been specified
func (s *Service) Configured() bool {
	return s.EnabledFlag != ""
}

// Enabled determines if a given "_service" section has been set to 'true'
func (s *Service) Enabled() bool {
	switch strings.ToLower(s.EnabledFlag) {
	case "", "yes", "yeah", "y", "true", "1":
		return true
	}
	return false
}

// 'auth_service' section of the config file
type Auth struct {
	Service `yaml:",inline"`
}

// 'ssh_service' section of the config file
type SSH struct {
	Service  `yaml:",inline"`
	Labels   map[string]string `yaml:"labels,omitempty"`
	Commands []CommandLabel    `yaml:"commands,omitempty"`
}

// `command` section of `ssh_service` in the config file
type CommandLabel struct {
	Name    string        `yaml:"name"`
	Command string        `yaml:"command"`
	Period  time.Duration `yaml:"period"`
}

// `proxy_service` section of the config file:
type Proxy struct {
	Service  `yaml:",inline"`
	WebAddr  string `yaml:"web_listen_addr,omitempty"`
	SSHAddr  string `yaml:"ssh_listen_addr,omitempty"`
	KeyFile  string `yaml:"https_key_file,omitempty"`
	CertFile string `yaml:"https_cert_file,omitempty"`
}
