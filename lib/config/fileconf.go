/*
Copyright 2015-2021 Gravitational, Inc.

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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var (
	// all possible valid YAML config keys
	// true  = has sub-keys
	// false = does not have sub-keys (a leaf)
	validKeys = map[string]bool{
		"proxy_protocol":          false,
		"namespace":               true,
		"cluster_name":            true,
		"trusted_clusters":        true,
		"pid_file":                true,
		"cert_file":               true,
		"private_key_file":        true,
		"cert":                    true,
		"private_key":             true,
		"checking_keys":           true,
		"checking_key_files":      true,
		"signing_keys":            true,
		"signing_key_files":       true,
		"allowed_logins":          true,
		"teleport":                true,
		"enabled":                 true,
		"ssh_service":             true,
		"proxy_service":           true,
		"auth_service":            true,
		"kubernetes":              true,
		"kubeconfig_file":         true,
		"auth_token":              true,
		"auth_servers":            true,
		"domain_name":             true,
		"storage":                 false,
		"nodename":                true,
		"log":                     true,
		"period":                  true,
		"connection_limits":       true,
		"max_connections":         true,
		"max_users":               true,
		"rates":                   true,
		"commands":                true,
		"labels":                  false,
		"output":                  true,
		"severity":                true,
		"role":                    true,
		"name":                    true,
		"type":                    true,
		"data_dir":                true,
		"web_listen_addr":         true,
		"tunnel_listen_addr":      true,
		"ssh_listen_addr":         true,
		"listen_addr":             true,
		"ca_cert_file":            false,
		"https_key_file":          true,
		"https_cert_file":         true,
		"advertise_ip":            true,
		"authorities":             true,
		"keys":                    true,
		"reverse_tunnels":         true,
		"addresses":               true,
		"oidc_connectors":         true,
		"id":                      true,
		"issuer_url":              true,
		"client_id":               true,
		"client_secret":           true,
		"redirect_url":            true,
		"acr_values":              true,
		"provider":                true,
		"tokens":                  true,
		"region":                  true,
		"table_name":              true,
		"access_key":              true,
		"secret_key":              true,
		"u2f":                     true,
		"app_id":                  true,
		"facets":                  true,
		"device_attestation_cas":  true,
		"authentication":          true,
		"second_factor":           false,
		"oidc":                    true,
		"display":                 false,
		"scope":                   false,
		"claims_to_roles":         true,
		"dynamic_config":          false,
		"seed_config":             false,
		"public_addr":             false,
		"ssh_public_addr":         false,
		"tunnel_public_addr":      false,
		"cache":                   true,
		"ttl":                     false,
		"issuer":                  false,
		"permit_user_env":         false,
		"ciphers":                 false,
		"kex_algos":               false,
		"mac_algos":               false,
		"ca_signature_algo":       false,
		"connector_name":          false,
		"session_recording":       false,
		"read_capacity_units":     false,
		"write_capacity_units":    false,
		"license_file":            false,
		"proxy_checks_host_keys":  false,
		"audit_table_name":        false,
		"audit_sessions_uri":      false,
		"audit_events_uri":        false,
		"pam":                     true,
		"use_pam_auth":            false,
		"service_name":            false,
		"client_idle_timeout":     false,
		"session_control_timeout": false,
		"disconnect_expired_cert": false,
		"ciphersuites":            false,
		"ca_pin":                  false,
		"keep_alive_interval":     false,
		"keep_alive_count_max":    false,
		"local_auth":              false,
		"enhanced_recording":      false,
		"command_buffer_size":     false,
		"disk_buffer_size":        false,
		"network_buffer_size":     false,
		"cgroup_path":             false,
		"kubernetes_service":      true,
		"kube_cluster_name":       false,
		"kube_listen_addr":        false,
		"kube_public_addr":        false,
		"app_service":             true,
		"db_service":              true,
		"protocol":                false,
		"uri":                     false,
		"apps":                    false,
		"databases":               false,
		"https_keypairs":          true,
		"key_file":                false,
		"insecure_skip_verify":    false,
		"rewrite":                 false,
		"redirect":                false,
		"debug_app":               false,
		"acme":                    true,
		"email":                   false,
		"mysql_listen_addr":       false,
	}
)

var validCASigAlgos = []string{
	ssh.SigAlgoRSA,
	ssh.SigAlgoRSASHA2256,
	ssh.SigAlgoRSASHA2512,
}

// FileConfig structre represents the teleport configuration stored in a config file
// in YAML format (usually /etc/teleport.yaml)
//
// Use config.ReadFromFile() to read the parsed FileConfig from a YAML file.
type FileConfig struct {
	Global `yaml:"teleport,omitempty"`
	Auth   Auth  `yaml:"auth_service,omitempty"`
	SSH    SSH   `yaml:"ssh_service,omitempty"`
	Proxy  Proxy `yaml:"proxy_service,omitempty"`
	Kube   Kube  `yaml:"kubernetes_service,omitempty"`

	// Apps is the "app_service" section in Teleport file configuration which
	// defines application access configuration.
	Apps Apps `yaml:"app_service,omitempty"`

	// Databases is the "db_service" section in Teleport configuration file
	// that defined database access configuration.
	Databases Databases `yaml:"db_service,omitempty"`
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
			"configuration should be base64 encoded: %v", err)
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

	// New validation in 6.0:
	//
	// Try strict unmarshal first (fails if any yaml entry doesn't map to a
	// FileConfig field).
	//
	// If strict unmarshal failed, there may be some innocent mis-placed config
	// fields. Fall back to the old validation first.
	//
	// If the old validation fails too, then we'll report the above error
	// because the config is definitely invalid.
	//
	// If the old validation succeeds, we'll log the above error, but won't
	// enforce it yet to let users fix the problem
	strictUnmarshalErr := yaml.UnmarshalStrict(bytes, &fc)
	if strictUnmarshalErr == nil {
		// don't start Teleport with invalid ciphers, kex algorithms, or mac algorithms.
		if err = fc.CheckAndSetDefaults(); err != nil {
			return nil, trace.BadParameter("failed to parse Teleport configuration: %v", err)
		}
		return &fc, nil
	}
	// Remove all newlines in the YAML error, to avoid escaping when printing.
	strictUnmarshalErr = errors.New(strings.Replace(strictUnmarshalErr.Error(), "\n", "", -1))
	// DELETE IN 7.0: during 6.0, users should notice any issues that passed
	// old validation but not the new strict one. With 7.0, we should always
	// enforce the strict validation.
	if err = yaml.Unmarshal(bytes, &fc); err != nil {
		return nil, trace.BadParameter("failed to parse Teleport configuration: %v", strictUnmarshalErr)
	}
	// don't start Teleport with invalid ciphers, kex algorithms, or mac algorithms.
	err = fc.CheckAndSetDefaults()
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
		return nil, trace.BadParameter("error parsing YAML config: %v", err)
	}
	if err = validateKeys(tmp); err != nil {
		// Both old an new validations failed. Report the new strict validation
		// error.
		return nil, trace.Wrap(strictUnmarshalErr)
	}
	// New strict validation failed but old one succeeded. There's something
	// wrong with the config, but don't prevent it from starting up.
	logrus.Errorf("Teleport configuration is invalid: %v.", strictUnmarshalErr)
	logrus.Error("This error will be enforced in the next Teleport release.")
	// Also add a short but noticeable sleep, to nudge users to pay attention
	// to logs.
	time.Sleep(5 * time.Second)
	return &fc, nil
}

// SampleFlags specifies standalone configuration parameters
type SampleFlags struct {
	// ClusterName is an optional cluster name
	ClusterName string
	// LicensePath adds license path to config
	LicensePath string
	// ACMEEmail is acme email
	ACMEEmail string
	// ACMEEnabled turns on ACME
	ACMEEnabled bool
}

// MakeSampleFileConfig returns a sample config to start
// a standalone server
func MakeSampleFileConfig(flags SampleFlags) (fc *FileConfig, err error) {
	if flags.ACMEEnabled && flags.ClusterName == "" {
		return nil, trace.BadParameter("please provide --cluster-name when using acme, for example --cluster-name=example.com")
	}

	conf := service.MakeDefaultConfig()

	var g Global
	g.NodeName = conf.Hostname
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.DataDir = defaults.DataDir

	// SSH config:
	var s SSH
	s.EnabledFlag = "yes"
	s.ListenAddress = conf.SSH.Addr.Addr
	s.Commands = []CommandLabel{
		{
			Name:    "hostname",
			Command: []string{"hostname"},
			Period:  time.Minute,
		},
	}
	s.Labels = map[string]string{
		"env": "example",
	}

	// Auth config:
	var a Auth
	a.ListenAddress = conf.Auth.SSHAddr.Addr
	a.ClusterName = ClusterName(flags.ClusterName)
	a.EnabledFlag = "yes"

	if flags.LicensePath != "" {
		a.LicenseFile = flags.LicensePath
	}

	// sample proxy config:
	var p Proxy
	p.EnabledFlag = "yes"
	p.ListenAddress = conf.Proxy.SSHAddr.Addr
	if flags.ACMEEnabled {
		p.ACME.EnabledFlag = "yes"
		p.ACME.Email = flags.ACMEEmail
		// ACME uses TLS-ALPN-01 challenge that requires port 443
		// https://letsencrypt.org/docs/challenge-types/#tls-alpn-01
		p.PublicAddr = utils.Strings{net.JoinHostPort(flags.ClusterName, fmt.Sprintf("%d", teleport.StandardHTTPSPort))}
		p.WebAddr = fmt.Sprintf(":%d", teleport.StandardHTTPSPort)
	}

	fc = &FileConfig{
		Global: g,
		Proxy:  p,
		SSH:    s,
		Auth:   a,
	}
	return fc, nil
}

// DebugDumpToYAML allows for quick YAML dumping of the config
func (conf *FileConfig) DebugDumpToYAML() string {
	bytes, err := yaml.Marshal(&conf)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}

// CheckAndSetDefaults sets defaults and ensures that the ciphers, kex
// algorithms, and mac algorithms set are supported by golang.org/x/crypto/ssh.
// This ensures we don't start Teleport with invalid configuration.
func (conf *FileConfig) CheckAndSetDefaults() error {
	conf.Auth.defaultEnabled = true
	conf.Proxy.defaultEnabled = true
	conf.SSH.defaultEnabled = true
	conf.Kube.defaultEnabled = false

	var sc ssh.Config
	sc.SetDefaults()

	for _, c := range conf.Ciphers {
		if !utils.SliceContainsStr(sc.Ciphers, c) {
			return trace.BadParameter("cipher algorithm %q is not supported; supported algorithms: %q", c, sc.Ciphers)
		}
	}
	for _, k := range conf.KEXAlgorithms {
		if !utils.SliceContainsStr(sc.KeyExchanges, k) {
			return trace.BadParameter("KEX algorithm %q is not supported; supported algorithms: %q", k, sc.KeyExchanges)
		}
	}
	for _, m := range conf.MACAlgorithms {
		if !utils.SliceContainsStr(sc.MACs, m) {
			return trace.BadParameter("MAC algorithm %q is not supported; supported algorithms: %q", m, sc.MACs)
		}
	}
	if conf.CASignatureAlgorithm != nil && !utils.SliceContainsStr(validCASigAlgos, *conf.CASignatureAlgorithm) {
		return trace.BadParameter("CA signature algorithm %q is not supported; supported algorithms: %q", *conf.CASignatureAlgorithm, validCASigAlgos)
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
	// Format lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller]
	Format []string `yaml:"format,omitempty"`
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
	AdvertiseIP string           `yaml:"advertise_ip,omitempty"`
	CachePolicy CachePolicy      `yaml:"cache,omitempty"`
	SeedConfig  *bool            `yaml:"seed_config,omitempty"`

	// CipherSuites is a list of TLS ciphersuites that Teleport supports. If
	// omitted, a Teleport selected list of defaults will be used.
	CipherSuites []string `yaml:"ciphersuites,omitempty"`

	// Ciphers is a list of SSH ciphers that the server supports. If omitted,
	// the defaults will be used.
	Ciphers []string `yaml:"ciphers,omitempty"`

	// KEXAlgorithms is a list of SSH key exchange (KEX) algorithms that the
	// server supports. If omitted, the defaults will be used.
	KEXAlgorithms []string `yaml:"kex_algos,omitempty"`

	// MACAlgorithms is a list of SSH message authentication codes (MAC) that
	// the server supports. If omitted the defaults will be used.
	MACAlgorithms []string `yaml:"mac_algos,omitempty"`

	// CASignatureAlgorithm is an SSH Certificate Authority (CA) signature
	// algorithm that the server uses for signing user and host certificates.
	// If omitted, the default will be used.
	CASignatureAlgorithm *string `yaml:"ca_signature_algo,omitempty"`

	// CAPin is the SKPI hash of the CA used to verify the Auth Server.
	CAPin string `yaml:"ca_pin"`
}

// CachePolicy is used to control  local cache
type CachePolicy struct {
	// Type is for cache type `sqlite` or `in-memory`
	Type string `yaml:"type,omitempty"`
	// EnabledFlag enables or disables cache
	EnabledFlag string `yaml:"enabled,omitempty"`
	// TTL sets maximum TTL for the cached values
	TTL string `yaml:"ttl,omitempty"`
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
	if c.EnabledFlag == "" {
		return true
	}
	enabled, _ := utils.ParseBool(c.EnabledFlag)
	return enabled
}

// NeverExpires returns if cache never expires by itself
func (c *CachePolicy) NeverExpires() bool {
	return isNever(c.TTL)
}

// Parse parses cache policy from Teleport config
func (c *CachePolicy) Parse() (*service.CachePolicy, error) {
	out := service.CachePolicy{
		Type:         c.Type,
		Enabled:      c.Enabled(),
		NeverExpires: c.NeverExpires(),
	}
	if !out.NeverExpires {
		var err error
		if c.TTL != "" {
			out.TTL, err = time.ParseDuration(c.TTL)
			if err != nil {
				return nil, trace.BadParameter("cache.ttl invalid duration: %v, accepted format '10h'", c.TTL)
			}
		}
	}
	if err := out.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// Service is a common configuration of a teleport service
type Service struct {
	defaultEnabled bool
	EnabledFlag    string `yaml:"enabled,omitempty"`
	ListenAddress  string `yaml:"listen_addr,omitempty"`
}

// Configured determines if a given "_service" section has been specified
func (s *Service) Configured() bool {
	return s.EnabledFlag != ""
}

// Enabled determines if a given "_service" section has been set to 'true'
func (s *Service) Enabled() bool {
	if !s.Configured() {
		return s.defaultEnabled
	}
	v, err := utils.ParseBool(s.EnabledFlag)
	if err != nil {
		return false
	}
	return v
}

// Disabled returns 'true' if the service has been deliberately turned off
func (s *Service) Disabled() bool {
	if !s.Configured() {
		return !s.defaultEnabled
	}
	return !s.Enabled()
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
	SessionRecording string `yaml:"session_recording,omitempty"`

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

	// DynamicConfig determines when file configuration is pushed to the backend. Setting
	// it here overrides defaults.
	// Deprecated: Remove in Teleport 2.4.1.
	DynamicConfig *bool `yaml:"dynamic_config,omitempty"`

	// PublicAddr sets SSH host principals and TLS DNS names to auth
	// server certificates
	PublicAddr utils.Strings `yaml:"public_addr,omitempty"`

	// ClientIdleTimeout sets global cluster default setting for client idle timeouts
	ClientIdleTimeout services.Duration `yaml:"client_idle_timeout,omitempty"`

	// DisconnectExpiredCert provides disconnect expired certificate setting -
	// if true, connections with expired client certificates will get disconnected
	DisconnectExpiredCert services.Bool `yaml:"disconnect_expired_cert,omitempty"`

	// SessionControlTimeout specifies the maximum amount of time a node can be out
	// of contact with the auth server before it starts terminating controlled sessions.
	SessionControlTimeout services.Duration `yaml:"session_control_timeout,omitempty"`

	// KubeconfigFile is an optional path to kubeconfig file,
	// if specified, teleport will use API server address and
	// trusted certificate authority information from it
	KubeconfigFile string `yaml:"kubeconfig_file,omitempty"`

	// KeepAliveInterval set the keep-alive interval for server to client
	// connections.
	KeepAliveInterval services.Duration `yaml:"keep_alive_interval,omitempty"`

	// KeepAliveCountMax set the number of keep-alive messages that can be
	// missed before the server disconnects the client.
	KeepAliveCountMax int64 `yaml:"keep_alive_count_max,omitempty"`
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
	staticTokens := []services.ProvisionTokenV1{}

	for _, token := range t {
		st, err := token.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		staticTokens = append(staticTokens, *st)
	}

	return services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: staticTokens,
	})
}

type StaticToken string

// Parse is applied to a string in "role,role,role:token" format. It breaks it
// apart and constructs a services.ProvisionToken which contains the token,
// role, and expiry (infinite).
func (t StaticToken) Parse() (*services.ProvisionTokenV1, error) {
	parts := strings.Split(string(t), ":")
	if len(parts) != 2 {
		return nil, trace.BadParameter("invalid static token spec: %q", t)
	}

	roles, err := teleport.ParseRoles(parts[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := utils.ReadToken(parts[1])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &services.ProvisionTokenV1{
		Token:   token,
		Roles:   roles,
		Expires: time.Unix(0, 0).UTC(),
	}, nil
}

// AuthenticationConfig describes the auth_service/authentication section of teleport.yaml
type AuthenticationConfig struct {
	Type              string                     `yaml:"type"`
	SecondFactor      constants.SecondFactorType `yaml:"second_factor,omitempty"`
	ConnectorName     string                     `yaml:"connector_name,omitempty"`
	U2F               *UniversalSecondFactor     `yaml:"u2f,omitempty"`
	RequireSessionMFA bool                       `yaml:"require_session_mfa,omitempty"`

	// LocalAuth controls if local authentication is allowed.
	LocalAuth *services.Bool `yaml:"local_auth"`
}

// Parse returns a services.AuthPreference (type, second factor, U2F).
func (a *AuthenticationConfig) Parse() (services.AuthPreference, error) {
	var err error

	var u services.U2F
	if a.U2F != nil {
		u, err = a.U2F.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:              a.Type,
		SecondFactor:      a.SecondFactor,
		ConnectorName:     a.ConnectorName,
		U2F:               &u,
		RequireSessionMFA: a.RequireSessionMFA,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// check to make sure the configuration is valid
	err = ap.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ap, nil
}

type UniversalSecondFactor struct {
	AppID                string   `yaml:"app_id"`
	Facets               []string `yaml:"facets"`
	DeviceAttestationCAs []string `yaml:"device_attestation_cas"`
}

func (u *UniversalSecondFactor) Parse() (services.U2F, error) {
	res := services.U2F{
		AppID:  u.AppID,
		Facets: u.Facets,
	}
	// DeviceAttestationCAs are either file paths or raw PEM blocks.
	for _, ca := range u.DeviceAttestationCAs {
		_, parseErr := tlsutils.ParseCertificatePEM([]byte(ca))
		if parseErr == nil {
			// Successfully parsed as a PEM block, add it.
			res.DeviceAttestationCAs = append(res.DeviceAttestationCAs, ca)
			continue
		}

		// Try reading as a file and parsing that.
		data, err := ioutil.ReadFile(ca)
		if err != nil {
			return res, trace.BadParameter("device_attestation_cas value %q is not a valid x509 certificate (%v) and can't be read as a file (%v)", ca, parseErr, err)
		}

		if _, err := tlsutils.ParseCertificatePEM(data); err != nil {
			return res, trace.BadParameter("device_attestation_cas file %q contains an invalid x509 certificate: %v", ca, err)
		}
		res.DeviceAttestationCAs = append(res.DeviceAttestationCAs, string(data))
	}
	return res, nil
}

// SSH is 'ssh_service' section of the config file
type SSH struct {
	Service               `yaml:",inline"`
	Namespace             string            `yaml:"namespace,omitempty"`
	Labels                map[string]string `yaml:"labels,omitempty"`
	Commands              []CommandLabel    `yaml:"commands,omitempty"`
	PermitUserEnvironment bool              `yaml:"permit_user_env,omitempty"`
	PAM                   *PAM              `yaml:"pam,omitempty"`
	// PublicAddr sets SSH host principals for SSH service
	PublicAddr utils.Strings `yaml:"public_addr,omitempty"`

	// BPF is used to configure BPF-based auditing for this node.
	BPF *BPF `yaml:"enhanced_recording,omitempty"`
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

	// UsePAMAuth specifies whether to trigger the "auth" PAM modules from the
	// policy.
	UsePAMAuth bool `yaml:"use_pam_auth"`
}

// Parse returns a parsed pam.Config.
func (p *PAM) Parse() *pam.Config {
	serviceName := p.ServiceName
	if serviceName == "" {
		serviceName = defaults.ServiceName
	}
	enabled, _ := utils.ParseBool(p.Enabled)
	return &pam.Config{
		Enabled:     enabled,
		ServiceName: serviceName,
		UsePAMAuth:  p.UsePAMAuth,
	}
}

// BPF is configuration for BPF-based auditing.
type BPF struct {
	// Enabled enables or disables enhanced session recording for this node.
	Enabled string `yaml:"enabled"`

	// CommandBufferSize is the size of the perf buffer for command events.
	CommandBufferSize *int `yaml:"command_buffer_size,omitempty"`

	// DiskBufferSize is the size of the perf buffer for disk events.
	DiskBufferSize *int `yaml:"disk_buffer_size,omitempty"`

	// NetworkBufferSize is the size of the perf buffer for network events.
	NetworkBufferSize *int `yaml:"network_buffer_size,omitempty"`

	// CgroupPath controls where cgroupv2 hierarchy is mounted.
	CgroupPath string `yaml:"cgroup_path"`
}

// Parse will parse the enhanced session recording configuration.
func (b *BPF) Parse() *bpf.Config {
	enabled, _ := utils.ParseBool(b.Enabled)
	return &bpf.Config{
		Enabled:           enabled,
		CommandBufferSize: b.CommandBufferSize,
		DiskBufferSize:    b.DiskBufferSize,
		NetworkBufferSize: b.NetworkBufferSize,
		CgroupPath:        b.CgroupPath,
	}
}

// Databases represents the database proxy service configuration.
//
// In the configuration file this section will be "db_service".
type Databases struct {
	// Service contains common service fields.
	Service `yaml:",inline"`
	// Databases is a list of databases proxied by the service.
	Databases []*Database `yaml:"databases"`
}

// Database represents a single database proxied by the service.
type Database struct {
	// Name is the name for the database proxy service.
	Name string `yaml:"name"`
	// Description is an optional free-form database description.
	Description string `yaml:"description,omitempty"`
	// Protocol is the database type e.g. postgres, mysql, etc.
	Protocol string `yaml:"protocol"`
	// URI is the database address to connect to.
	URI string `yaml:"uri"`
	// CACertFile is an optional path to the database CA certificate.
	CACertFile string `yaml:"ca_cert_file,omitempty"`
	// StaticLabels is a map of database static labels.
	StaticLabels map[string]string `yaml:"static_labels,omitempty"`
	// DynamicLabels is a list of database dynamic labels.
	DynamicLabels []CommandLabel `yaml:"dynamic_labels,omitempty"`
	// AWS contains AWS specific settings for RDS/Aurora databases.
	AWS DatabaseAWS `yaml:"aws"`
	// GCP contains GCP specific settings for Cloud SQL databases.
	GCP DatabaseGCP `yaml:"gcp"`
}

// DatabaseAWS contains AWS specific settings for RDS/Aurora databases.
type DatabaseAWS struct {
	// Region is a cloud region for RDS/Aurora database endpoint.
	Region string `yaml:"region,omitempty"`
}

// DatabaseGCP contains GCP specific settings for Cloud SQL databases.
type DatabaseGCP struct {
	// ProjectID is the GCP project ID where the database is deployed.
	ProjectID string `yaml:"project_id,omitempty"`
	// InstanceID is the Cloud SQL database instance ID.
	InstanceID string `yaml:"instance_id,omitempty"`
}

// Apps represents the configuration for the collection of applications this
// service will start. In file configuration this would be the "app_service"
// section.
type Apps struct {
	// Service contains fields common to all services like "enabled" and
	// "listen_addr".
	Service `yaml:",inline"`

	// DebugApp turns on a header debugging application.
	DebugApp bool `yaml:"debug_app"`

	// Apps is a list of applications that will be run by this service.
	Apps []*App `yaml:"apps"`
}

// App is the specific application that will be proxied by the application
// service.
type App struct {
	// Name of the application.
	Name string `yaml:"name"`

	// Description is an optional free-form app description.
	Description string `yaml:"description,omitempty"`

	// URI is the internal address of the application.
	URI string `yaml:"uri"`

	// Public address of the application. This is the address users will access
	// the application at.
	PublicAddr string `yaml:"public_addr"`

	// Labels is a map of static labels to apply to this application.
	StaticLabels map[string]string `yaml:"labels,omitempty"`

	// Commands is a list of dynamic labels to apply to this application.
	DynamicLabels []CommandLabel `yaml:"commands,omitempty"`

	// InsecureSkipVerify is used to skip validating the servers certificate.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// Rewrite defines a block that is used to rewrite requests and responses.
	Rewrite *Rewrite `yaml:"rewrite,omitempty"`
}

// Rewrite is a list of rewriting rules to apply to requests and responses.
type Rewrite struct {
	// Redirect is a list of hosts that should be rewritten to the public address.
	Redirect []string `yaml:"redirect"`
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
	// ProxyProtocol turns on support for HAProxy proxy protocol
	// this is the option that has be turned on only by administrator,
	// as only admin knows whether service is in front of trusted load balancer
	// or not.
	ProxyProtocol string `yaml:"proxy_protocol,omitempty"`
	// KubeProxy configures kubernetes protocol support of the proxy
	Kube KubeProxy `yaml:"kubernetes,omitempty"`
	// KubeAddr is a shorthand for enabling the Kubernetes endpoint without a
	// local Kubernetes cluster.
	KubeAddr string `yaml:"kube_listen_addr,omitempty"`
	// KubePublicAddr is a public address of the kubernetes endpoint.
	KubePublicAddr utils.Strings `yaml:"kube_public_addr,omitempty"`

	// PublicAddr sets the hostport the proxy advertises for the HTTP endpoint.
	// The hosts in PublicAddr are included in the list of host principals
	// on the SSH certificate.
	PublicAddr utils.Strings `yaml:"public_addr,omitempty"`

	// SSHPublicAddr sets the hostport the proxy advertises for the SSH endpoint.
	// The hosts in PublicAddr are included in the list of host principals
	// on the SSH certificate.
	SSHPublicAddr utils.Strings `yaml:"ssh_public_addr,omitempty"`

	// TunnelPublicAddr sets the hostport the proxy advertises for the tunnel
	// endpoint. The hosts in PublicAddr are included in the list of host
	// principals on the SSH certificate.
	TunnelPublicAddr utils.Strings `yaml:"tunnel_public_addr,omitempty"`

	// KeyPairs is a list of x509 key pairs the proxy will load.
	KeyPairs []KeyPair `yaml:"https_keypairs"`

	// ACME configures ACME protocol support
	ACME ACME `yaml:"acme"`

	// MySQLAddr is MySQL proxy listen address.
	MySQLAddr string `yaml:"mysql_listen_addr,omitempty"`
}

// ACME configures ACME protocol - automatic X.509 certificates
type ACME struct {
	// EnabledFlag is whether ACME should be enabled
	EnabledFlag string `yaml:"enabled,omitempty"`
	// Email is the email that will receive problems with certificate renewals
	Email string `yaml:"email,omitempty"`
	// URI is ACME server URI
	URI string `yaml:"uri,omitempty"`
}

// Parse parses ACME section values
func (a ACME) Parse() (*service.ACME, error) {
	// ACME is disabled by default
	out := service.ACME{}
	if a.EnabledFlag == "" {
		return &out, nil
	}

	var err error
	out.Enabled, err = utils.ParseBool(a.EnabledFlag)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out.Email = a.Email
	if a.URI != "" {
		_, err := url.Parse(a.URI)
		if err != nil {
			return nil, trace.Wrap(err, "acme.uri should be a valid URI, for example %v", acme.LetsEncryptURL)
		}
	}
	out.URI = a.URI

	return &out, nil
}

// KeyPair represents a path on disk to a private key and certificate.
type KeyPair struct {
	// PrivateKey is the path on disk to a PEM encoded private key,
	PrivateKey string `yaml:"key_file"`
	// Certificate is the path on disk to a PEM encoded x509 certificate.
	Certificate string `yaml:"cert_file"`
}

// KubeProxy is a `kubernetes` section in `proxy_service`.
type KubeProxy struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`
	// PublicAddr is a publicly advertised address of the kubernetes proxy
	PublicAddr utils.Strings `yaml:"public_addr,omitempty"`
	// KubeconfigFile is an optional path to kubeconfig file,
	// if specified, teleport will use API server address and
	// trusted certificate authority information from it
	KubeconfigFile string `yaml:"kubeconfig_file,omitempty"`
	// ClusterName is the name of a kubernetes cluster this proxy is running
	// in. If set, this proxy will handle kubernetes requests for the cluster.
	ClusterName string `yaml:"cluster_name,omitempty"`
}

// Kube is a `kubernetes_service`
type Kube struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`
	// PublicAddr is a publicly advertised address of the kubernetes service
	PublicAddr utils.Strings `yaml:"public_addr,omitempty"`
	// KubeconfigFile is an optional path to kubeconfig file,
	// if specified, teleport will use API server address and
	// trusted certificate authority information from it
	KubeconfigFile string `yaml:"kubeconfig_file,omitempty"`
	// KubeClusterName is the name of a kubernetes cluster this service is
	// running in. If set, this proxy will handle kubernetes requests for the
	// cluster.
	KubeClusterName string `yaml:"kube_cluster_name,omitempty"`
	// Static and dynamic labels for RBAC on kubernetes clusters.
	StaticLabels  map[string]string `yaml:"labels,omitempty"`
	DynamicLabels []CommandLabel    `yaml:"commands,omitempty"`
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
	if err := auth.ValidateReverseTunnel(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
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
	ca := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        a.Type,
		ClusterName: a.DomainName,
	})

	// transform old allowed logins into roles
	role := auth.RoleForCertAuthority(ca)
	role.SetLogins(services.Allow, a.AllowedLogins)
	ca.AddRole(role.GetName())

	for _, path := range a.CheckingKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ca.SetCheckingKeys(append(ca.GetCheckingKeys(), keyBytes))
	}

	for _, val := range a.CheckingKeys {
		ca.SetCheckingKeys(append(ca.GetCheckingKeys(), []byte(val)))
	}

	for _, path := range a.SigningKeyFiles {
		keyBytes, err := utils.ReadPath(path)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		ca.SetSigningKeys(append(ca.GetSigningKeys(), keyBytes))
	}

	for _, val := range a.SigningKeys {
		ca.SetSigningKeys(append(ca.GetSigningKeys(), []byte(val)))
	}

	return ca, role, nil
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
			Claim: c.Claim,
			Value: c.Value,
			Roles: roles,
		})
	}

	v2 := services.NewOIDCConnector(o.ID, services.OIDCConnectorSpecV2{
		IssuerURL:     o.IssuerURL,
		ClientID:      o.ClientID,
		ClientSecret:  o.ClientSecret,
		RedirectURL:   o.RedirectURL,
		Display:       o.Display,
		Scope:         o.Scope,
		ClaimsToRoles: mappings,
	})

	v2.SetACR(o.ACR)
	v2.SetProvider(o.Provider)
	if err := v2.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return v2, nil
}
