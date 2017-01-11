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

	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

var (
	// all possible valid YAML config keys
	validKeys = map[string]bool{
		"namespace":          true,
		"seed_config":        true,
		"cluster_name":       true,
		"trusted_clusters":   true,
		"pid_file":           true,
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
		"storage":            false,
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
		"web_listen_addr":    true,
		"tunnel_listen_addr": true,
		"ssh_listen_addr":    true,
		"listen_addr":        true,
		"https_key_file":     true,
		"https_cert_file":    true,
		"advertise_ip":       true,
		"authorities":        true,
		"keys":               true,
		"reverse_tunnels":    true,
		"addresses":          true,
		"oidc_connectors":    true,
		"id":                 true,
		"issuer_url":         true,
		"client_id":          true,
		"client_secret":      true,
		"redirect_url":       true,
		"tokens":             true,
		"region":             true,
		"table_name":         true,
		"access_key":         true,
		"secret_key":         true,
		"u2f":                true,
		"app_id":             true,
		"facets":             true,
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
func ReadFromFile(filePath string) (*FileConfig, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".yaml" && ext != ".yml" {
		return nil, trace.BadParameter(
			"'%v' invalid configuration file type: '%v'. Only .yml is supported", filePath, ext)
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
		return nil, trace.BadParameter(
			"confiugraion should be base64 encoded: %v", err)
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
					return trace.BadParameter("unrecognized configuration key: '%v'", key)
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
		return nil, trace.BadParameter("error parsing YAML config")
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
	g.AuthToken = "cluster-join-token"
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.AuthServers = []string{defaults.AuthListenAddr().Addr}
	g.Limits.MaxConnections = defaults.LimiterMaxConnections
	g.Limits.MaxUsers = defaults.LimiterMaxConcurrentUsers
	g.DataDir = defaults.DataDir
	g.Storage.Type = conf.Auth.KeysBackend.Type
	g.PIDFile = "/var/run/teleport.pid"

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
	a.StaticTokens = []StaticToken{"proxy,node:cluster-join-token"}

	a.U2F.EnabledFlag = "yes"
	a.U2F.AppID = conf.Auth.U2F.AppID
	a.U2F.Facets = conf.Auth.U2F.Facets

	// sample proxy config:
	var p Proxy
	p.EnabledFlag = "yes"
	p.ListenAddress = conf.Proxy.SSHAddr.Addr
	p.WebAddr = conf.Proxy.WebAddr.Addr
	p.TunAddr = conf.Proxy.ReverseTunnelListenAddr.Addr
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
	// Output defines where logs go. It can be one of the following: "stderr", "stdout" or
	// a path to a log file
	Output string `yaml:"output,omitempty"`
	// Severity defines how verbose the log will be. Possible valus are "error", "info", "warn"
	Severity string `yaml:"severity,omitempty"`
}

// Global is 'teleport' (global) section of the config file
type Global struct {
	NodeName    string           `yaml:"nodename,omitempty"`
	DataDir     string           `yaml:"data_dir,omitempty"`
	PIDFile     string           `yaml:"pid_file,omitempty"`
	AuthToken   string           `yaml:"auth_token,omitempty"`
	AuthServers []string         `yaml:"auth_servers,omitempty"`
	Limits      ConnectionLimits `yaml:"connection_limits,omitempty"`
	Logger      Log              `yaml:"log,omitempty"`
	Storage     backend.Config   `yaml:"storage,omitempty"`
	AdvertiseIP net.IP           `yaml:"advertise_ip,omitempty"`

	// Keys holds the list of SSH key/cert pairs used by all services
	// Each service (like proxy, auth, node) can find the key it needs
	// by looking into certificate
	Keys []KeyPair `yaml:"keys,omitempty"`

	// SeedConfig [GRAVITATIONAL USE] when set to true, Teleport treats
	// its configuration file simply as a seed data on initial start-up.
	// For OSS Teleport it should always be 'false' by default.
	SeedConfig bool `yaml:"seed_config,omitempty"`
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
	// DomainName is the name of the CA who manages this cluster
	DomainName string `yaml:"cluster_name,omitempty"`

	// TrustedClustersFile is a file path to a file containing public CA keys
	// of clusters we trust. One key per line, those starting with '#' are comments
	TrustedClusters []TrustedCluster `yaml:"trusted_clusters,omitempty"`

	// FOR INTERNAL USE:
	// Authorities : 3rd party certificate authorities (CAs) this auth service trusts.
	Authorities []Authority `yaml:"authorities,omitempty"`

	// FOR INTERNAL USE:
	// ReverseTunnels is a list of SSH tunnels to 3rd party proxy services (used to talk
	// to 3rd party auth servers we trust)
	ReverseTunnels []ReverseTunnel `yaml:"reverse_tunnels,omitempty"`

	// OIDCConnectors is a list of trusted OpenID Connect Identity providers
	OIDCConnectors []OIDCConnector `yaml:"oidc_connectors"`

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	//
	// Each token string has the following format: "role1,role2,..:token",
	// for exmple: "auth,proxy,node:MTIzNGlvemRmOWE4MjNoaQo"
	StaticTokens []StaticToken `yaml:"tokens,omitempty"`

	// Configuration for "universal 2nd factor"
	U2F U2F `yaml:"u2f,omitempty"`
}

// TrustedCluster struct holds configuration values under "trusted_clusters" key
type TrustedCluster struct {
	// KeyFile is a path to a remote authority (AKA "trusted cluster") public keys
	KeyFile string `yaml:"key_file,omitempty"`
	// AllowedLogins is a comma-separated list of user logins allowed from that cluster
	AllowedLogins string `yaml:"allow_logins,omitempty"`
	// TunnelAddr is a comma-separated list of reverse tunnel addressess to
	// connect to
	TunnelAddr string `yaml:"tunnel_addr,omitempty"`
}

type StaticToken string

// SSH is 'ssh_service' section of the config file
type SSH struct {
	Service   `yaml:",inline"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Commands  []CommandLabel    `yaml:"commands,omitempty"`
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
	TunAddr  string `yaml:"tunnel_listen_addr,omitempty"`
	KeyFile  string `yaml:"https_key_file,omitempty"`
	CertFile string `yaml:"https_cert_file,omitempty"`
}

// ReverseTunnel is a SSH reverse tunnel mantained by one cluster's
// proxy to remote Teleport proxy
type ReverseTunnel struct {
	DomainName string   `yaml:"domain_name"`
	Addresses  []string `yaml:"addresses"`
}

// ConvertAndValidate returns validated services.ReverseTunnel or nil and error otherwize
func (t *ReverseTunnel) ConvertAndValidate() (services.ReverseTunnel, error) {
	for i := range t.Addresses {
		addr, err := utils.ParseHostPortAddr(t.Addresses[i], defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return nil, trace.Wrap(err, "Invalid address for tunnel %v", t.DomainName)
		}
		t.Addresses[i] = addr.String()
	}

	out := services.NewReverseTunnel(t.DomainName, t.Addresses)
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
			return nil, trace.ConvertSystemError(err)
		}
	} else {
		keyBytes = []byte(k.PrivateKey)
	}
	var certBytes []byte
	if k.CertFile != "" {
		certBytes, err = ioutil.ReadFile(k.CertFile)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
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
func (a *Authority) Parse() (services.CertAuthority, services.Role, error) {
	ca := &services.CertAuthorityV1{
		AllowedLogins: a.AllowedLogins,
		DomainName:    a.DomainName,
		Type:          a.Type,
	}

	for _, path := range a.CheckingKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ca.CheckingKeys = append(ca.CheckingKeys, keyBytes)
	}

	for _, val := range a.CheckingKeys {
		ca.CheckingKeys = append(ca.CheckingKeys, []byte(val))
	}

	for _, path := range a.SigningKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ca.SigningKeys = append(ca.SigningKeys, keyBytes)
	}

	for _, val := range a.SigningKeys {
		ca.SigningKeys = append(ca.SigningKeys, []byte(val))
	}

	new, role := services.ConvertV1CertAuthority(ca)
	return new, role, nil
}

// ClaimMapping is OIDC claim mapping that maps
// claim name to teleport roles
type ClaimMapping struct {
	// Claim is OIDC claim name
	Claim string `yaml:"claim"`
	// Value is claim value to match
	Value string `yaml:"value"`
	// Roles is a list of teleport roles to match
	Roles []string `yaml:"roles"`
}

// OIDCConnector specifies configuration fo Open ID Connect compatible external
// identity provider, e.g. google in some organisation
type OIDCConnector struct {
	// ID is a provider id, 'e.g.' google, used internally
	ID string `yaml:"id"`
	// Issuer URL is the endpoint of the provider, e.g. https://accounts.google.com
	IssuerURL string `yaml:"issuer_url"`
	// ClientID is id for authentication client (in our case it's our Auth server)
	ClientID string `yaml:"client_id"`
	// ClientSecret is used to authenticate our client and should not
	// be visible to end user
	ClientSecret string `yaml:"client_secret"`
	// RedirectURL - Identity provider will use this URL to redirect
	// client's browser back to it after successfull authentication
	// Should match the URL on Provider's side
	RedirectURL string `yaml:"redirect_url"`
	// Display controls how this connector is displayed
	Display string `yaml:"display"`
	// Scope is a list of additional scopes to request from OIDC
	// note that oidc and email scopes are always requested
	Scope []string `yaml:"scope"`
	// ClaimsToRoles is a list of mappings of claims to roles
	ClaimsToRoles []ClaimMapping `yaml:"claims_to_roles"`
}

// Parse parses config struct into services connector and checks if it's valid
func (o *OIDCConnector) Parse() (services.OIDCConnector, error) {
	if o.Display == "" {
		o.Display = o.ID
	}

	var mappings []services.ClaimMapping
	for _, c := range o.ClaimsToRoles {
		roles := make([]string, len(c.Roles))
		copy(roles, c.Roles)
		mappings = append(mappings, services.ClaimMapping{
			Claim: c.Claim,
			Value: c.Value,
			Roles: roles,
		})
	}

	other := &services.OIDCConnectorV1{
		ID:            o.ID,
		Display:       o.Display,
		IssuerURL:     o.IssuerURL,
		ClientID:      o.ClientID,
		ClientSecret:  o.ClientSecret,
		RedirectURL:   o.RedirectURL,
		Scope:         o.Scope,
		ClaimsToRoles: mappings,
	}
	v2 := other.V2()
	if err := v2.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return v2, nil
}

// Parse() is applied to a string in "role,role,role:token" format. It breaks it
// apart into a slice of roles, token and optional error
func (t StaticToken) Parse() (roles teleport.Roles, token string, err error) {
	parts := strings.Split(string(t), ":")
	if len(parts) != 2 {
		return nil, "", trace.Errorf("invalid static token spec: '%s'", t)
	}
	roles, err = teleport.ParseRoles(parts[0])
	return roles, parts[1], trace.Wrap(err)
}

type U2F struct {
	EnabledFlag string   `yaml:"enabled"`
	AppID       string   `yaml:"app_id,omitempty"`
	Facets      []string `yaml:"facets,omitempty"`
}

// Parse parses the values in 'u2f' configuration section of 'auth' and
// validates its content:
func (u *U2F) Parse() (*services.U2F, error) {
	enabled := false
	switch strings.ToLower(u.EnabledFlag) {
	case "yes", "yeah", "y", "true", "1":
		enabled = true
	}
	appID := u.AppID
	// If no appID specified, default to hostname
	if appID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, trace.Wrap(err, "failed to automatically determine U2F AppID from hostname")
		}
		appID = fmt.Sprintf("https://%s:%d", strings.ToLower(hostname), defaults.HTTPListenPort)
	}
	facets := u.Facets
	// If no facets specified, default to AppID
	if len(facets) == 0 {
		facets = []string{appID}
	}
	other := &services.U2F{
		Enabled: enabled,
		AppID:   appID,
		Facets:  facets,
	}
	if err := other.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return other, nil
}
