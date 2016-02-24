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

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

var (
	// all possible valid YAML config keys
	validKeys = map[string]int{
		"teleport":          1,
		"enabled":           1,
		"ssh_service":       1,
		"proxy_service":     1,
		"auth_service":      1,
		"auth_token":        1,
		"auth_servers":      1,
		"storage":           1,
		"nodename":          1,
		"log":               1,
		"period":            1,
		"connection_limits": 1,
		"max_connections":   1,
		"max_users":         1,
		"rates":             1,
		"commands":          1,
		"labels":            1,
		"output":            1,
		"severity":          1,
		"role":              1,
		"name":              1,
		"type":              1,
		"data_dir":          1,
		"peers":             1,
		"web_listen_addr":   1,
		"ssh_listen_addr":   1,
		"listen_addr":       1,
		"https_key_file":    1,
		"https_cert_file":   1,
	}
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

type YAMLMap map[interface{}]interface{}

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
	// now check for unknown (misspelled) config keys:
	var validateKeys func(m YAMLMap) error
	validateKeys = func(m YAMLMap) error {
		for k, value := range m {
			if key, ok := k.(string); ok {
				if _, ok := validKeys[key]; !ok {
					return trace.Errorf("unknown configuration key: '%v'", key)
				}
				if m, ok := value.(YAMLMap); ok {
					return validateKeys(m)
				}
			}
		}
		return nil
	}
	var m YAMLMap
	if err = yaml.Unmarshal(bytes, &m); err != nil {
		return nil, trace.Errorf("error parsing YAML config")
	}
	if err = validateKeys(m); err != nil {
		return nil, trace.Wrap(err)
	}
	return fc, nil
}

// makeSampleFileConfig returns a sample config structure populated by defaults,
// useful to generate sample configuration files
func MakeSampleFileConfig() (fc *FileConfig) {
	conf := service.MakeDefaultConfig()

	// sample global config:
	var g Global
	g.NodeName = conf.Hostname
	g.AuthToken = "xxxx-token-xxxx"
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.AuthServers = defaults.AuthListenAddr().Addr
	g.Limits.MaxConnections = defaults.LimiterMaxConnections
	g.Limits.MaxUsers = defaults.LimiterMaxConcurrentUsers
	g.Storage.DirName = defaults.DataDir
	g.Storage.Type = conf.Auth.RecordsBackend.Type

	// sample SSH config:
	var s SSH
	s.EnabledFlag = "yes"
	s.ListenAddress = conf.SSH.Addr.Addr
	s.Commands = []CommandLabel{
		{
			Name:    "hostname",
			Command: []string{"/usr/bin/hostname"},
			Period:  time.Minute,
		},
		{
			Name:    "arch",
			Command: []string{"/usr/bin/uname", "-p"},
			Period:  time.Hour,
		},
	}
	s.Labels = map[string]string{
		"db_type": "postgres",
		"db_role": "master",
	}

	// sample Auth config:
	var a Auth
	a.ListenAddress = conf.Auth.SSHAddr.Addr
	a.EnabledFlag = "yes"

	// sample proxy config:
	var p Proxy
	p.EnabledFlag = "yes"
	p.ListenAddress = conf.Proxy.SSHAddr.Addr
	p.WebAddr = conf.Proxy.WebAddr.Addr
	p.CertFile = "/etc/teleport/teleport.crt"
	p.KeyFile = "/etc/teleport/teleport.key"

	fc = &FileConfig{
		Global: g,
		Proxy:  p,
		SSH:    s,
		Auth:   a,
	}
	return fc
}

// DebugDump allows for quick YAML dumping of the config
func (conf *FileConfig) DebugDumpToYAML() string {
	bytes, err := yaml.Marshal(&conf)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

type ConnectionRate struct {
	Period  time.Duration `yaml:"period"`
	Average int64         `yaml:"average"`
	Burst   int64         `yaml:"burst"`
}

type ConnectionLimits struct {
	MaxConnections int64            `yaml:"max_connections"`
	MaxUsers       int              `yaml:"max_users"`
	Rates          []ConnectionRate `yaml:"rates,omitempty"`
}

type Log struct {
	Output   string `yaml:"output,omitempty"`
	Severity string `yaml:"severity,omitempty"`
}

// used for 'storage' config section. stores values for 'boltdb' and 'etcd'
type StorageBackend struct {
	Type    string `yaml:"type,omitempty"`     // can be "bolt" or "etcd"
	DirName string `yaml:"data_dir,omitempty"` // valid only for bolt
	Peers   string `yaml:"peers,omitempty"`    // valid only for etcd
}

// 'teleport' (global) section of the config file
type Global struct {
	NodeName    string           `yaml:"nodename,omitempty"`
	AuthToken   string           `yaml:"auth_token,omitempty"`
	AuthServers string           `yaml:"auth_servers,omitempty"`
	Limits      ConnectionLimits `yaml:"connection_limits,omitempty"`
	Logger      Log              `yaml:"log,omitempty"`
	Storage     StorageBackend   `yaml:"storage,omitempty"`
}

// returns a slice of the auth servers
func (g *Global) GetAuthServers() []string {
	return strings.Split(strings.Replace(g.AuthServers, " ", "", -1), ",")
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

// Disabled returns 'true' if the service has been deliverately turned off
func (s *Service) Disabled() bool {
	return s.Configured() && !s.Enabled()
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
	Command []string      `yaml:"command,flow"`
	Period  time.Duration `yaml:"period"`
}

// `proxy_service` section of the config file:
type Proxy struct {
	Service  `yaml:",inline"`
	WebAddr  string `yaml:"web_listen_addr,omitempty"`
	KeyFile  string `yaml:"https_key_file,omitempty"`
	CertFile string `yaml:"https_cert_file,omitempty"`
}
