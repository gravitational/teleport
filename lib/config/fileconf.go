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
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

var (
	// all possible valid YAML config keys
	validKeys = map[string]bool{
		"cert_file":          true,
		"private_key_file":   true,
		"cert":               true,
		"private_key":        true,
		"checking_keys":      true,
		"checking_key_files": true,
		"signing_keys":       true,
		"signing_key_files":  true,
		"allowed_logins":     true,
		"teleport":           true,
		"enabled":            true,
		"ssh_service":        true,
		"proxy_service":      true,
		"auth_service":       true,
		"auth_token":         true,
		"auth_servers":       true,
		"domain_name":        true,
		"storage":            true,
		"nodename":           true,
		"log":                true,
		"period":             true,
		"connection_limits":  true,
		"max_connections":    true,
		"max_users":          true,
		"rates":              true,
		"commands":           true,
		"labels":             false,
		"output":             true,
		"severity":           true,
		"role":               true,
		"name":               true,
		"type":               true,
		"data_dir":           true,
		"peers":              true,
		"prefix":             true,
		"web_listen_addr":    true,
		"ssh_listen_addr":    true,
		"listen_addr":        true,
		"https_key_file":     true,
		"https_cert_file":    true,
		"advertise_ip":       true,
		"tls_key_file":       true,
		"tls_cert_file":      true,
		"tls_ca_file":        true,
		"authorities":        true,
		"keys":               true,
		"secrets":            true,
		"rts":                true,
		"addresses":          true,
	}
)

// FileConfig structre represents the teleport configuration stored in a config file
// in YAML format (usually /etc/teleport.yaml)
//
// Use config.ReadFromFile() to read the parsed FileConfig from a YAML file.
type FileConfig struct {
	Global         `yaml:"teleport,omitempty"`
	Auth           Auth            `yaml:"auth_service,omitempty"`
	SSH            SSH             `yaml:"ssh_service,omitempty"`
	Proxy          Proxy           `yaml:"proxy_service,omitempty"`
	Secrets        Secrets         `yaml:"secrets,omitempty"`
	ReverseTunnels []ReverseTunnel `yaml:"rts,omitempty"`
}

type YAMLMap map[interface{}]interface{}

// ReadFromFile reads Teleport configuration from a file. Currently only YAML
// format is supported
func ReadFromFile(filePath string) (*FileConfig, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".yaml" && ext != ".yml" {
		return nil, trace.Wrap(
			teleport.BadParameter(filePath,
				fmt.Sprintf("invalid configuration file type: '%v'. Only .yml is supported", ext)))
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, trace.Wrap(err, fmt.Sprintf("failed to open file: %v", filePath))
	}
	defer f.Close()
	return ReadConfig(f)
}

// ReadFromString reads values from base64 encoded byte string
func ReadFromString(configString string) (*FileConfig, error) {
	data, err := base64.StdEncoding.DecodeString(configString)
	if err != nil {
		return nil, trace.Wrap(teleport.BadParameter(
			"config", fmt.Sprintf("confiugraion should be base64 encoded: %v", err)))
	}
	return ReadConfig(bytes.NewBuffer(data))
}

// ReadConfig reads Teleport configuration from reader in YAML format
func ReadConfig(reader io.Reader) (*FileConfig, error) {
	// read & parse YAML config:
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, trace.Wrap(err, "failed reading Teleport configuration")
	}
	var fc FileConfig
	if err = yaml.Unmarshal(bytes, &fc); err != nil {
		return nil, trace.Wrap(err, "failed to parse Teleport configuration")
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
	return &fc, nil
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

// Secrets hold additional initialization secrets passed to the process
type Secrets struct {
	// Authorities is a list of authorities that auth server will add
	// to the backend on the first start
	Authorities []Authority `yaml:"authorities,omitempty"`
	// Keys is the list of keys set for this server
	Keys []KeyPair `yaml:"keys,omitempty"`
}

// ReverseTunnel is a SSH reverse tunnel mantained by one cluster's
// proxy to remote Teleport proxy
type ReverseTunnel struct {
	DomainName string   `yaml:"domain_name"`
	Addresses  []string `yaml:"addresses"`
}

// Tunnel returns validated services.ReverseTunnel or nil and error otherwize
func (t *ReverseTunnel) Tunnel() (*services.ReverseTunnel, error) {
	out := &services.ReverseTunnel{
		DomainName: t.DomainName,
		DialAddrs:  t.Addresses,
	}
	if err := out.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// KeyPair is a pair of private key and certificates
type KeyPair struct {
	// PrivateKeyFile is a path to file with private key
	PrivateKeyFile string `yaml:"private_key_file"`
	// CertFile is a path to file with OpenSSH certificate
	CertFile string `yaml:"cert_file"`
	// PrivateKey is PEM encoded OpenSSH private key
	PrivateKey string `yaml:"private_key"`
	// Cert is certificate in OpenSSH authorized keys format
	Cert string `yaml:"cert"`
}

// Identity parses keypair into auth server identity
func (k *KeyPair) Identity() (*auth.Identity, error) {
	var keyBytes []byte
	var err error
	if k.PrivateKeyFile != "" {
		keyBytes, err = ioutil.ReadFile(k.PrivateKeyFile)
		if err != nil {
			return nil, teleport.ConvertSystemError(err)
		}
	} else {
		keyBytes = []byte(k.PrivateKey)
	}
	var certBytes []byte
	if k.CertFile != "" {
		certBytes, err = ioutil.ReadFile(k.CertFile)
		if err != nil {
			return nil, teleport.ConvertSystemError(err)
		}
	} else {
		certBytes = []byte(k.Cert)
	}
	return auth.ReadIdentityFromKeyPair(keyBytes, certBytes)
}

// Authority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type Authority struct {
	// Type is either user or host certificate authority
	Type services.CertAuthType `yaml:"type"`
	// DomainName identifies domain name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	DomainName string `yaml:"domain_name"`
	// Checkers is a list of SSH public keys that can be used to check
	// certificate signatures in OpenSSH authorized keys format
	CheckingKeys []string `yaml:"checking_keys"`
	// CheckingKeyFiles is a list of files
	CheckingKeyFiles []string `yaml:"checking_key_files"`
	// SigningKeys is a list of PEM-encoded private keys used for signing
	SigningKeys []string `yaml:"signing_keys"`
	// SigningKeyFiles is a list of paths to PEM encoded private keys used for signing
	SigningKeyFiles []string `yaml:"signing_key_files"`
	// AllowedLogins is a list of allowed logins for users within
	// this certificate authority
	AllowedLogins []string `yaml:"allowed_logins"`
}

// Parse reads values and returns parsed CertAuthority
func (a *Authority) Parse() (*services.CertAuthority, error) {
	ca := &services.CertAuthority{
		AllowedLogins: a.AllowedLogins,
		DomainName:    a.DomainName,
		Type:          a.Type,
	}

	for _, path := range a.CheckingKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.CheckingKeys = append(ca.CheckingKeys, keyBytes)
	}

	for _, val := range a.CheckingKeys {
		ca.CheckingKeys = append(ca.CheckingKeys, []byte(val))
	}

	for _, path := range a.SigningKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ca.SigningKeys = append(ca.SigningKeys, keyBytes)
	}

	for _, val := range a.SigningKeys {
		ca.SigningKeys = append(ca.SigningKeys, []byte(val))
	}

	return ca, nil
}
