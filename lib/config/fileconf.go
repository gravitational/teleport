/*
Copyright 2015 Gravitational, Inc.

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
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"gopkg.in/yaml.v2"
)

var (
	// all possible valid YAML config keys
	// true  = has sub-keys
	// false = does not have sub-keys (a leaf)
	validKeys = map[string]bool{
		"proxy_protocol":         false,
		"namespace":              true,
		"cluster_name":           true,
		"trusted_clusters":       true,
		"pid_file":               true,
		"cert_file":              true,
		"private_key_file":       true,
		"cert":                   true,
		"private_key":            true,
		"checking_keys":          true,
		"checking_key_files":     true,
		"signing_keys":           true,
		"signing_key_files":      true,
		"allowed_logins":         true,
		"teleport":               true,
		"enabled":                true,
		"ssh_service":            true,
		"proxy_service":          true,
		"auth_service":           true,
		"auth_token":             true,
		"auth_servers":           true,
		"domain_name":            true,
		"storage":                false,
		"nodename":               true,
		"log":                    true,
		"period":                 true,
		"connection_limits":      true,
		"max_connections":        true,
		"max_users":              true,
		"rates":                  true,
		"commands":               true,
		"labels":                 false,
		"output":                 true,
		"severity":               true,
		"role":                   true,
		"name":                   true,
		"type":                   true,
		"data_dir":               true,
		"web_listen_addr":        true,
		"tunnel_listen_addr":     true,
		"ssh_listen_addr":        true,
		"listen_addr":            true,
		"https_key_file":         true,
		"https_cert_file":        true,
		"advertise_ip":           true,
		"authorities":            true,
		"keys":                   true,
		"reverse_tunnels":        true,
		"addresses":              true,
		"oidc_connectors":        true,
		"id":                     true,
		"issuer_url":             true,
		"client_id":              true,
		"client_secret":          true,
		"redirect_url":           true,
		"acr_values":             true,
		"provider":               true,
		"tokens":                 true,
		"region":                 true,
		"table_name":             true,
		"access_key":             true,
		"secret_key":             true,
		"u2f":                    true,
		"app_id":                 true,
		"facets":                 true,
		"authentication":         true,
		"second_factor":          false,
		"oidc":                   true,
		"display":                false,
		"scope":                  false,
		"claims_to_roles":        true,
		"dynamic_config":         false,
		"seed_config":            false,
		"public_addr":            false,
		"cache":                  true,
		"ttl":                    false,
		"issuer":                 false,
		"permit_user_env":        false,
		"ciphers":                false,
		"kex_algos":              false,
		"mac_algos":              false,
		"connector_name":         false,
		"session_recording":      false,
		"read_capacity_units":    false,
		"write_capacity_units":   false,
		"license_file":           false,
		"proxy_checks_host_keys": false,
		"audit_table_name":       false,
		"audit_sessions_uri":     false,
		"pam":                    true,
		"service_name":           false,
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
		return nil, trace.BadParameter("failed to parse Teleport configuration: %v", err)
	}
	// don't start Teleport with invalid ciphers, kex algorithms, or mac algorithms.
	err = fc.Check()
	if err != nil {
		return nil, trace.BadParameter("failed to parse Teleport configuration: %v", err)
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

	// sample proxy config:
	var p Proxy
	p.EnabledFlag = "yes"
	p.ListenAddress = conf.Proxy.SSHAddr.Addr
	p.WebAddr = conf.Proxy.WebAddr.Addr
	p.TunAddr = conf.Proxy.ReverseTunnelListenAddr.Addr
	p.CertFile = "/var/lib/teleport/webproxy_cert.pem"
	p.KeyFile = "/var/lib/teleport/webproxy_key.pem"

	fc = &FileConfig{
		Global: g,
		Proxy:  p,
		SSH:    s,
		Auth:   a,
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

// Check ensures that the ciphers, kex algorithms, and mac algorithms set
// are supported by golang.org/x/crypto/ssh. This ensures we don't start
// Teleport with invalid configuration.
func (conf *FileConfig) Check() error {
	var sc ssh.Config
	sc.SetDefaults()

	for _, c := range conf.Ciphers {
		if utils.SliceContainsStr(sc.Ciphers, c) == false {
			return trace.BadParameter("cipher %q not supported", c)
		}
	}
	for _, k := range conf.KEXAlgorithms {
		if utils.SliceContainsStr(sc.KeyExchanges, k) == false {
			return trace.BadParameter("KEX %q not supported", k)
		}
	}
	for _, m := range conf.MACAlgorithms {
		if utils.SliceContainsStr(sc.MACs, m) == false {
			return trace.BadParameter("MAC %q not supported", m)
		}
	}

	return nil
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
	CachePolicy CachePolicy      `yaml:"cache,omitempty"`
	SeedConfig  *bool            `yaml:"seed_config,omitempty"`

	// Keys holds the list of SSH key/cert pairs used by all services
	// Each service (like proxy, auth, node) can find the key it needs
	// by looking into certificate
	Keys []KeyPair `yaml:"keys,omitempty"`

	// Ciphers is a list of ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string `yaml:"ciphers,omitempty"`

	// KEXAlgorithms is a list of key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string `yaml:"kex_algos,omitempty"`

	// MACAlgorithms is a list of message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string `yaml:"mac_algos,omitempty"`
}

// CachePolicy is used to control  local cache
type CachePolicy struct {
	// EnabledFlag enables or disables cache
	EnabledFlag string `yaml:"enabled,omitempty"`
	// TTL sets maximum TTL for the cached values
	TTL string `yaml:"ttl,omitempty"`
}

func isTrue(v string) bool {
	switch v {
	case "yes", "yeah", "y", "true", "1":
		return true
	}
	return false
}

func isNever(v string) bool {
	switch v {
	case "never", "no", "0":
		return true
	}
	return false
}

// Enabled determines if a given "_service" section has been set to 'true'
func (c *CachePolicy) Enabled() bool {
	return c.EnabledFlag == "" || isTrue(c.EnabledFlag)
}

// NeverExpires returns if cache never expires by itself
func (c *CachePolicy) NeverExpires() bool {
	if isNever(c.TTL) {
		return true
	}
	return false
}

// Parse parses cache policy from Teleport config
func (c *CachePolicy) Parse() (*service.CachePolicy, error) {
	out := service.CachePolicy{
		Enabled:      c.Enabled(),
		NeverExpires: c.NeverExpires(),
	}
	if out.NeverExpires {
		return &out, nil
	}
	var err error
	if c.TTL != "" {
		out.TTL, err = time.ParseDuration(c.TTL)
		if err != nil {
			return nil, trace.BadParameter("cache.ttl invalid duration: %v, accepted format '10h'", c.TTL)
		}
	}
	return &out, nil
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

// Disabled returns 'true' if the service has been deliberately turned off
func (s *Service) Disabled() bool {
	return s.Configured() && !s.Enabled()
}

// Auth is 'auth_service' section of the config file
type Auth struct {
	Service `yaml:",inline"`

	// ProxyProtocol turns on support for HAProxy proxy protocol
	// this is the option that has be turned on only by administrator,
	// as only admin knows whether service is in front of trusted load balancer
	// or not.
	ProxyProtocol string `yaml:"proxy_protocol,omitempty"`

	// ClusterName is the name of the CA who manages this cluster
	ClusterName ClusterName `yaml:"cluster_name,omitempty"`

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	//
	// Each token string has the following format: "role1,role2,..:token",
	// for exmple: "auth,proxy,node:MTIzNGlvemRmOWE4MjNoaQo"
	StaticTokens StaticTokens `yaml:"tokens,omitempty"`

	// Authentication holds authentication configuration information like authentication
	// type, second factor type, specific connector information, etc.
	Authentication *AuthenticationConfig `yaml:"authentication,omitempty"`

	// SessionRecording determines where the session is recorded: node, proxy, or off.
	SessionRecording string `yaml:"session_recording"`

	// ProxyChecksHostKeys is used when the proxy is in recording mode and
	// determines if the proxy will check the host key of the client or not.
	ProxyChecksHostKeys string `yaml:"proxy_checks_host_keys,omitempty"`

	// LicenseFile is a path to the license file. The path can be either absolute or
	// relative to the global data dir
	LicenseFile string `yaml:"license_file,omitempty"`

	// FOR INTERNAL USE:
	// Authorities : 3rd party certificate authorities (CAs) this auth service trusts.
	Authorities []Authority `yaml:"authorities,omitempty"`

	// FOR INTERNAL USE:
	// ReverseTunnels is a list of SSH tunnels to 3rd party proxy services (used to talk
	// to 3rd party auth servers we trust)
	ReverseTunnels []ReverseTunnel `yaml:"reverse_tunnels,omitempty"`

	// TrustedClustersFile is a file path to a file containing public CA keys
	// of clusters we trust. One key per line, those starting with '#' are comments
	// Deprecated: Remove in Teleport 2.4.1.
	TrustedClusters []TrustedCluster `yaml:"trusted_clusters,omitempty"`

	// OIDCConnectors is a list of trusted OpenID Connect Identity providers
	// Deprecated: Remove in Teleport 2.4.1.
	OIDCConnectors []OIDCConnector `yaml:"oidc_connectors,omitempty"`

	// Configuration for "universal 2nd factor"
	// Deprecated: Remove in Teleport 2.4.1.
	U2F U2F `yaml:"u2f,omitempty"`

	// DynamicConfig determines when file configuration is pushed to the backend. Setting
	// it here overrides defaults.
	// Deprecated: Remove in Teleport 2.4.1.
	DynamicConfig *bool `yaml:"dynamic_config,omitempty"`
}

// TrustedCluster struct holds configuration values under "trusted_clusters" key
type TrustedCluster struct {
	// KeyFile is a path to a remote authority (AKA "trusted cluster") public keys
	KeyFile string `yaml:"key_file,omitempty"`
	// AllowedLogins is a comma-separated list of user logins allowed from that cluster
	AllowedLogins string `yaml:"allow_logins,omitempty"`
	// TunnelAddr is a comma-separated list of reverse tunnel addresses to
	// connect to
	TunnelAddr string `yaml:"tunnel_addr,omitempty"`
}

type ClusterName string

func (c ClusterName) Parse() (services.ClusterName, error) {
	if string(c) == "" {
		return nil, nil
	}
	return services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: string(c),
	})
}

type StaticTokens []StaticToken

func (t StaticTokens) Parse() (services.StaticTokens, error) {
	staticTokens := []services.ProvisionToken{}

	for _, token := range t {
		st, err := token.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		staticTokens = append(staticTokens, st)
	}

	return services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: staticTokens,
	})
}

type StaticToken string

// Parse is applied to a string in "role,role,role:token" format. It breaks it
// apart and constructs a services.ProvisionToken which contains the token,
// role, and expiry (infinite).
func (t StaticToken) Parse() (services.ProvisionToken, error) {
	parts := strings.Split(string(t), ":")
	if len(parts) != 2 {
		return services.ProvisionToken{}, trace.BadParameter("invalid static token spec: %q", t)
	}

	roles, err := teleport.ParseRoles(parts[0])
	if err != nil {
		return services.ProvisionToken{}, trace.Wrap(err)
	}

	return services.ProvisionToken{
		Token:   parts[1],
		Roles:   roles,
		Expires: time.Unix(0, 0).UTC(),
	}, nil
}

// AuthenticationConfig describes the auth_service/authentication section of teleport.yaml
type AuthenticationConfig struct {
	Type          string                 `yaml:"type"`
	SecondFactor  string                 `yaml:"second_factor,omitempty"`
	ConnectorName string                 `yaml:"connector_name,omitempty"`
	U2F           *UniversalSecondFactor `yaml:"u2f,omitempty"`

	// TODO: OIDC connection is DEPRECATED!!!! Users are supposed to use resources
	// for configuring OIDC connectors
	OIDC *OIDCConnector `yaml:"oidc,omitempty"`
}

// Parse returns the Authentication Configuration in two parts: AuthPreference
// (type, second factor, u2f) and OIDCConnector.
func (a *AuthenticationConfig) Parse() (services.AuthPreference, services.OIDCConnector, error) {
	var err error

	var u services.U2F
	if a.U2F != nil {
		u = a.U2F.Parse()
	}

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:          a.Type,
		SecondFactor:  a.SecondFactor,
		ConnectorName: a.ConnectorName,
		U2F:           &u,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// check to make sure the configuration is valid
	err = ap.CheckAndSetDefaults()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var oidcConnector services.OIDCConnector
	if a.OIDC != nil {
		oidcConnector, err = a.OIDC.Parse()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	return ap, oidcConnector, nil
}

type UniversalSecondFactor struct {
	AppID  string   `yaml:"app_id"`
	Facets []string `yaml:"facets"`
}

func (u *UniversalSecondFactor) Parse() services.U2F {
	return services.U2F{
		AppID:  u.AppID,
		Facets: u.Facets,
	}
}

// SSH is 'ssh_service' section of the config file
type SSH struct {
	Service               `yaml:",inline"`
	Namespace             string            `yaml:"namespace,omitempty"`
	Labels                map[string]string `yaml:"labels,omitempty"`
	Commands              []CommandLabel    `yaml:"commands,omitempty"`
	PermitUserEnvironment bool              `yaml:"permit_user_env,omitempty"`
	PAM                   *PAM              `yaml:"pam,omitempty"`
}

// CommandLabel is `command` section of `ssh_service` in the config file
type CommandLabel struct {
	Name    string        `yaml:"name"`
	Command []string      `yaml:"command,flow"`
	Period  time.Duration `yaml:"period"`
}

// PAM is configuration for Pluggable Authentication Modules (PAM).
type PAM struct {
	// Enabled controls if PAM will be used or not.
	Enabled string `yaml:"enabled"`

	// ServiceName is the name of the PAM policy to apply.
	ServiceName string `yaml:"service_name"`
}

// Parse returns a parsed pam.Config.
func (p *PAM) Parse() *pam.Config {
	serviceName := p.ServiceName
	if serviceName == "" {
		serviceName = defaults.ServiceName
	}

	return &pam.Config{
		Enabled:     isTrue(p.Enabled),
		ServiceName: serviceName,
	}
}

// Proxy is a `proxy_service` section of the config file:
type Proxy struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`
	// WebAddr is a web UI listen address
	WebAddr string `yaml:"web_listen_addr,omitempty"`
	// TunAddr is a reverse tunnel address
	TunAddr string `yaml:"tunnel_listen_addr,omitempty"`
	// KeyFile is a TLS key file
	KeyFile string `yaml:"https_key_file,omitempty"`
	// CertFile is a TLS Certificate file
	CertFile string `yaml:"https_cert_file,omitempty"`
	// PublicAddr is a publicly advertised address of the proxy
	PublicAddr string `yaml:"public_addr,omitempty"`
	// ProxyProtocol turns on support for HAProxy proxy protocol
	// this is the option that has be turned on only by administrator,
	// as only admin knows whether service is in front of trusted load balancer
	// or not.
	ProxyProtocol string `yaml:"proxy_protocol,omitempty"`
}

// ReverseTunnel is a SSH reverse tunnel maintained by one cluster's
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
	// TLSCert is TLS certificate in PEM format
	TLSCert string `yaml:"tls_cert"`
	// TLSCACert is TLS certificate in PEM format for trusted CA
	TLSCACert string `yaml:"tls_ca_cert"`
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
	return auth.ReadIdentityFromKeyPair(keyBytes, certBytes, []byte(k.TLSCert), [][]byte{[]byte(k.TLSCACert)})
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
	Roles []string `yaml:"roles,omitempty"`
	// RoleTemplate is a template for a role that will be filled
	// with data from claims.
	RoleTemplate *services.RoleV2 `yaml:"role_template,omitempty"`
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
	// client's browser back to it after successful authentication
	// Should match the URL on Provider's side
	RedirectURL string `yaml:"redirect_url"`
	// ACR is the acr_values parameter to be sent with an authorization request.
	ACR string `yaml:"acr_values,omitempty"`
	// Provider is the identity provider we connect to. This field is
	// only required if using acr_values.
	Provider string `yaml:"provider,omitempty"`
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
		var roles []string
		if len(c.Roles) > 0 {
			roles = append(roles, c.Roles...)
		}

		mappings = append(mappings, services.ClaimMapping{
			Claim:        c.Claim,
			Value:        c.Value,
			Roles:        roles,
			RoleTemplate: c.RoleTemplate,
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
	v2.SetACR(o.ACR)
	v2.SetProvider(o.Provider)
	if err := v2.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return v2, nil
}

type U2F struct {
	AppID  string   `yaml:"app_id,omitempty"`
	Facets []string `yaml:"facets,omitempty"`
}

// Parse parses values in the U2F configuration section and validates its content.
func (u *U2F) Parse() (*services.U2F, error) {
	// If no appID specified, default to hostname
	appID := u.AppID
	if appID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, trace.Wrap(err, "failed to automatically determine U2F AppID from hostname")
		}
		appID = fmt.Sprintf("https://%s:%d", strings.ToLower(hostname), defaults.HTTPListenPort)
	}

	// If no facets specified, default to AppID
	facets := u.Facets
	if len(facets) == 0 {
		facets = []string{appID}
	}

	return &services.U2F{
		AppID:  appID,
		Facets: facets,
	}, nil
}
