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
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

var (
	// all possible valid YAML config keys
	validKeys = map[string]bool{
		"teleport":          true,
		"enabled":           true,
		"ssh_service":       true,
		"proxy_service":     true,
		"auth_service":      true,
		"auth_token":        true,
		"auth_servers":      true,
		"domain_name":       true,
		"storage":           true,
		"nodename":          true,
		"log":               true,
		"period":            true,
		"connection_limits": true,
		"max_connections":   true,
		"max_users":         true,
		"rates":             true,
		"commands":          true,
		"labels":            false,
		"output":            true,
		"severity":          true,
		"role":              true,
		"name":              true,
		"type":              true,
		"data_dir":          true,
		"peers":             true,
		"prefix":            true,
		"web_listen_addr":   true,
		"ssh_listen_addr":   true,
		"listen_addr":       true,
		"https_key_file":    true,
		"https_cert_file":   true,
		"advertise_ip":      true,
		"tls_key_file":      true,
		"tls_cert_file":     true,
		"tls_ca_file":       true,
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
		var recursive, ok bool
		var key string
		for k, v := range m {
			if key, ok = k.(string); ok {
				if recursive, ok = validKeys[key]; !ok {
					return trace.Wrap(teleport.BadParameter(key, "this configuration key is unknown"))
				}
				if recursive {
					if m2, ok := v.(YAMLMap); ok {
						if err := validateKeys(m2); err != nil {
							return err
						}
					}
				}
			}
		}
		return nil
	}
	// validate configuration keys:
	var tmp YAMLMap
	if err = yaml.Unmarshal(bytes, &tmp); err != nil {
		return nil, trace.Errorf("error parsing YAML config")
	}
	if err = validateKeys(tmp); err != nil {
		return nil, trace.Wrap(err)
	}
	return fc, nil
}

// MakeSampleFileConfig returns a sample config structure populated by defaults,
// useful to generate sample configuration files
func MakeSampleFileConfig() (fc *FileConfig) {
	conf := service.MakeDefaultConfig()

	// sample global config:
	var g Global
	g.NodeName = conf.Hostname
	g.AuthToken = "xxxx-token-xxxx"
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.AuthServers = []string{defaults.AuthListenAddr().Addr}
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

// MakeAuthPeerFileConfig returns a sample configuration for auth
// server peer that shares etcd backend
func MakeAuthPeerFileConfig(domainName string, token string) (fc *FileConfig) {
	conf := service.MakeDefaultConfig()

	// sample global config:
	var g Global
	g.NodeName = conf.Hostname
	g.AuthToken = token
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.AuthServers = []string{"<insert auth server peer address here>"}
	g.Limits.MaxConnections = defaults.LimiterMaxConnections
	g.Limits.MaxUsers = defaults.LimiterMaxConcurrentUsers
	g.Storage.DirName = defaults.DataDir
	g.Storage.Type = teleport.ETCDBackendType
	g.Storage.Prefix = defaults.ETCDPrefix
	g.Storage.Peers = []string{"insert ETCD peers addresses here"}

	// sample Auth config:
	var a Auth
	a.ListenAddress = conf.Auth.SSHAddr.Addr
	a.EnabledFlag = "yes"
	a.DomainName = domainName

	var p Proxy
	p.EnabledFlag = "no"

	var s SSH
	s.EnabledFlag = "no"

	fc = &FileConfig{
		Global: g,
		Auth:   a,
		Proxy:  p,
		SSH:    s,
	}
	return fc
}

// DebugDumpToYAML allows for quick YAML dumping of the config
func (conf *FileConfig) DebugDumpToYAML() string {
	bytes, err := yaml.Marshal(&conf)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// ConnectionRate configures rate limiter
type ConnectionRate struct {
	Period  time.Duration `yaml:"period"`
	Average int64         `yaml:"average"`
	Burst   int64         `yaml:"burst"`
}

// ConnectionLimits sets up connection limiter
type ConnectionLimits struct {
	MaxConnections int64            `yaml:"max_connections"`
	MaxUsers       int              `yaml:"max_users"`
	Rates          []ConnectionRate `yaml:"rates,omitempty"`
}

// Log configures teleport logging
type Log struct {
	Output   string `yaml:"output,omitempty"`
	Severity string `yaml:"severity,omitempty"`
}

// StorageBackend is used for 'storage' config section. stores values for 'boltdb' and 'etcd'
type StorageBackend struct {
	// Type can be "bolt" or "etcd"
	Type string `yaml:"type,omitempty"`
	// DirName is valid only for bolt
	DirName string `yaml:"data_dir,omitempty"`
	// Peers is a lsit of etcd peers,  valid only for etcd
	Peers []string `yaml:"peers,omitempty"`
	// Prefix is etcd key prefix, valid only for etcd
	Prefix string `yaml:"prefix,omitempty"`
	// TLSCertFile is a tls client cert file, used for etcd
	TLSCertFile string `yaml:"tls_cert_file,omitempty"`
	// TLSKeyFile is a file with TLS private key for client auth
	TLSKeyFile string `yaml:"tls_key_file,omitempty"`
	// TLSCAFile is a tls client trusted CA file, used for etcd
	TLSCAFile string `yaml:"tls_ca_file,omitempty"`
}

// Global is 'teleport' (global) section of the config file
type Global struct {
	NodeName    string           `yaml:"nodename,omitempty"`
	AuthToken   string           `yaml:"auth_token,omitempty"`
	AuthServers []string         `yaml:"auth_servers,omitempty"`
	Limits      ConnectionLimits `yaml:"connection_limits,omitempty"`
	Logger      Log              `yaml:"log,omitempty"`
	Storage     StorageBackend   `yaml:"storage,omitempty"`
	AdvertiseIP net.IP           `yaml:"advertise_ip,omitempty"`
}

// Service is a common configuration of a teleport service
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

// Auth is 'auth_service' section of the config file
type Auth struct {
	Service `yaml:",inline"`
	// DomainName is the name of the certificate authority
	// managed by this domain
	DomainName string `yaml:"domain_name,omitempty"`
}

// SSH is 'ssh_service' section of the config file
type SSH struct {
	Service  `yaml:",inline"`
	Labels   map[string]string `yaml:"labels,omitempty"`
	Commands []CommandLabel    `yaml:"commands,omitempty"`
}

// CommandLabel is `command` section of `ssh_service` in the config file
type CommandLabel struct {
	Name    string        `yaml:"name"`
	Command []string      `yaml:"command,flow"`
	Period  time.Duration `yaml:"period"`
}

// Proxy is `proxy_service` section of the config file:
type Proxy struct {
	Service  `yaml:",inline"`
	WebAddr  string `yaml:"web_listen_addr,omitempty"`
	KeyFile  string `yaml:"https_key_file,omitempty"`
	CertFile string `yaml:"https_cert_file,omitempty"`
}
