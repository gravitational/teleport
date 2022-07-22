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
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/pam"
	restricted "github.com/gravitational/teleport/lib/restrictedsession"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// FileConfig structre represents the teleport configuration stored in a config file
// in YAML format (usually /etc/teleport.yaml)
//
// Use config.ReadFromFile() to read the parsed FileConfig from a YAML file.
type FileConfig struct {
	Version string `yaml:"version,omitempty"`
	Global  `yaml:"teleport,omitempty"`
	Auth    Auth  `yaml:"auth_service,omitempty"`
	SSH     SSH   `yaml:"ssh_service,omitempty"`
	Proxy   Proxy `yaml:"proxy_service,omitempty"`
	Kube    Kube  `yaml:"kubernetes_service,omitempty"`

	// Apps is the "app_service" section in Teleport file configuration which
	// defines application access configuration.
	Apps Apps `yaml:"app_service,omitempty"`

	// Databases is the "db_service" section in Teleport configuration file
	// that defines database access configuration.
	Databases Databases `yaml:"db_service,omitempty"`

	// Metrics is the "metrics_service" section in Teleport configuration file
	// that defines the metrics service configuration
	Metrics Metrics `yaml:"metrics_service,omitempty"`

	// WindowsDesktop is the "windows_desktop_service" that defines the
	// configuration for Windows Desktop Access.
	WindowsDesktop WindowsDesktopService `yaml:"windows_desktop_service,omitempty"`

	// Tracing is the "tracing_service" section in Teleport configuration file
	Tracing TracingService `yaml:"tracing_service,omitempty"`

	//TenantUrl configures the tenant url for which the cluster is serving.
	TenantUrl string `yaml:"tenant_url,omitempty"`
}

// ReadFromFile reads Teleport configuration from a file. Currently only YAML
// format is supported
func ReadFromFile(filePath string) (*FileConfig, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, trace.Wrap(err, "failed to open file for Teleport configuration: %v. Ensure that you are running as a user with appropriate permissions.", filePath)
		}
		return nil, trace.Wrap(err, "failed to open file for Teleport configuration at %v", filePath)
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
	bytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, trace.Wrap(err, "failed reading Teleport configuration")
	}
	var fc FileConfig

	if err := yaml.UnmarshalStrict(bytes, &fc); err != nil {
		// Remove all newlines in the YAML error, to avoid escaping when printing.
		return nil, trace.BadParameter("failed parsing the config file: %s", strings.Replace(err.Error(), "\n", "", -1))
	}
	if err := fc.CheckAndSetDefaults(); err != nil {
		return nil, trace.BadParameter("failed to parse Teleport configuration: %v", err)
	}
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
	// Version is the Teleport Configuration version.
	Version string
	// PublicAddr sets the hostport the proxy advertises for the HTTP endpoint.
	PublicAddr string
	// KeyFile is a TLS key file
	KeyFile string
	// CertFile is a TLS Certificate file
	CertFile string
	// DataDir is a path to a directory where Teleport keep its data
	DataDir string
	// AuthToken is a token to register with an auth server
	AuthToken string
	// Roles is a list of comma-separated roles to create a config file with
	Roles string
	// AuthServer is the address of the auth server
	AuthServer string
	// AppName is the name of the application to start
	AppName string
	// AppURI is the internal address of the application to proxy
	AppURI string
	// NodeLabels is list of labels in the format `foo=bar,baz=bax` to add to newly created nodes.
	NodeLabels string
	// CAPin is the SKPI hash of the CA used to verify the Auth Server. Can be
	// a single value or a list.
	CAPin string
	// JoinMethod is the method that will be used to join the cluster, either "token", "iam" or "ec2"
	JoinMethod string
	// NodeName is the name of the teleport node
	NodeName string
}

// MakeSampleFileConfig returns a sample config to start
// a standalone server
func MakeSampleFileConfig(flags SampleFlags) (fc *FileConfig, err error) {
	if (flags.KeyFile == "") != (flags.CertFile == "") { // xor
		return nil, trace.BadParameter("please provide both --key-file and --cert-file")
	}

	if flags.ACMEEnabled {
		if flags.ClusterName == "" {
			return nil, trace.BadParameter("please provide --cluster-name when using ACME, for example --cluster-name=example.com")
		}
		if flags.CertFile != "" {
			return nil, trace.BadParameter("could not use --key-file/--cert-file when ACME is enabled")
		}
	}

	conf := service.MakeDefaultConfig()

	var g Global

	if flags.NodeName != "" {
		g.NodeName = flags.NodeName
	} else {
		g.NodeName = conf.Hostname
	}
	g.Logger.Output = "stderr"
	g.Logger.Severity = "INFO"
	g.Logger.Format.Output = "text"

	g.DataDir = flags.DataDir
	if g.DataDir == "" {
		g.DataDir = defaults.DataDir
	}

	joinMethod := flags.JoinMethod
	if joinMethod == "" && flags.AuthToken != "" {
		joinMethod = string(types.JoinMethodToken)
	}
	g.JoinParams = JoinParams{
		TokenName: flags.AuthToken,
		Method:    types.JoinMethod(joinMethod),
	}

	if flags.AuthServer != "" {
		g.AuthServers = []string{flags.AuthServer}
	}

	g.CAPin = strings.Split(flags.CAPin, ",")

	roles := roleMapFromFlags(flags)

	// SSH config:
	s, err := makeSampleSSHConfig(conf, flags, roles[defaults.RoleNode])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Auth config:
	a := makeSampleAuthConfig(conf, flags, roles[defaults.RoleAuthService])

	// sample proxy config:
	p, err := makeSampleProxyConfig(conf, flags, roles[defaults.RoleProxy])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Apps config:
	apps, err := makeSampleAppsConfig(conf, flags, roles[defaults.RoleApp])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// DB config:
	var dbs Databases
	if roles[defaults.RoleDatabase] {
		// keep it disable since `teleport configure` don't have all the necessary flags
		// for this kind of resource
		dbs.EnabledFlag = "no"
	}

	// WindowsDesktop config:
	var d WindowsDesktopService
	if roles[defaults.RoleWindowsDesktop] {
		// keep it disable since `teleport configure` don't have all the necessary flags
		// for this kind of resource
		d.EnabledFlag = "no"
	}

	fc = &FileConfig{
		Version:        flags.Version,
		Global:         g,
		Proxy:          p,
		SSH:            s,
		Auth:           a,
		Apps:           apps,
		Databases:      dbs,
		WindowsDesktop: d,
	}
	return fc, nil
}

func makeSampleSSHConfig(conf *service.Config, flags SampleFlags, enabled bool) (SSH, error) {
	var s SSH
	if enabled {
		s.EnabledFlag = "yes"
		s.ListenAddress = conf.SSH.Addr.Addr
		s.Commands = []CommandLabel{
			{
				Name:    "hostname",
				Command: []string{"hostname"},
				Period:  time.Minute,
			},
		}
		labels, err := client.ParseLabelSpec(flags.NodeLabels)
		if err != nil {
			return s, trace.Wrap(err)
		}
		s.Labels = labels
	} else {
		s.EnabledFlag = "no"
	}

	return s, nil
}

func makeSampleAuthConfig(conf *service.Config, flags SampleFlags, enabled bool) Auth {
	var a Auth
	if enabled {
		a.ListenAddress = conf.Auth.ListenAddr.Addr
		a.ClusterName = ClusterName(flags.ClusterName)
		a.EnabledFlag = "yes"

		if flags.LicensePath != "" {
			a.LicenseFile = flags.LicensePath
		}

		if flags.Version == defaults.TeleportConfigVersionV2 {
			a.ProxyListenerMode = types.ProxyListenerMode_Multiplex
		}
	} else {
		a.EnabledFlag = "no"
	}

	return a
}

func makeSampleProxyConfig(conf *service.Config, flags SampleFlags, enabled bool) (Proxy, error) {
	var p Proxy
	if enabled {
		p.EnabledFlag = "yes"
		p.ListenAddress = conf.Proxy.SSHAddr.Addr
		if flags.ACMEEnabled {
			p.ACME.EnabledFlag = "yes"
			p.ACME.Email = flags.ACMEEmail
			// ACME uses TLS-ALPN-01 challenge that requires port 443
			// https://letsencrypt.org/docs/challenge-types/#tls-alpn-01
			p.PublicAddr = apiutils.Strings{net.JoinHostPort(flags.ClusterName, fmt.Sprintf("%d", teleport.StandardHTTPSPort))}
			p.WebAddr = net.JoinHostPort(defaults.BindIP, fmt.Sprintf("%d", teleport.StandardHTTPSPort))
		}
		if flags.PublicAddr != "" {
			// default to 443 if port is not specified
			publicAddr, err := utils.ParseHostPortAddr(flags.PublicAddr, teleport.StandardHTTPSPort)
			if err != nil {
				return Proxy{}, trace.Wrap(err)
			}
			p.PublicAddr = apiutils.Strings{publicAddr.String()}

			// use same port for web addr
			webPort := publicAddr.Port(teleport.StandardHTTPSPort)
			p.WebAddr = net.JoinHostPort(defaults.BindIP, fmt.Sprintf("%d", webPort))
		}
		if flags.KeyFile != "" && flags.CertFile != "" {
			if _, err := tls.LoadX509KeyPair(flags.CertFile, flags.KeyFile); err != nil {
				return Proxy{}, trace.Wrap(err, "failed to load x509 key pair from --key-file and --cert-file")
			}

			p.KeyPairs = append(p.KeyPairs, KeyPair{
				PrivateKey:  flags.KeyFile,
				Certificate: flags.CertFile,
			})
		}
	} else {
		p.EnabledFlag = "no"
	}

	return p, nil
}

func makeSampleAppsConfig(conf *service.Config, flags SampleFlags, enabled bool) (Apps, error) {
	var apps Apps
	// assume users want app role if they added app name and/or uri but didn't add app role
	if enabled || flags.AppURI != "" || flags.AppName != "" {
		if flags.AppURI == "" || flags.AppName == "" {
			return Apps{}, trace.BadParameter("please provide both --app-name and --app-uri")
		}

		apps.EnabledFlag = "yes"
		apps.Apps = []*App{
			{
				Name: flags.AppName,
				URI:  flags.AppURI,
			},
		}
	}

	return apps, nil
}

func roleMapFromFlags(flags SampleFlags) map[string]bool {
	// if no roles are provided via CLI, return the default roles
	if flags.Roles == "" {
		return map[string]bool{
			defaults.RoleProxy:       true,
			defaults.RoleNode:        true,
			defaults.RoleAuthService: true,
		}
	}

	roles := splitRoles(flags.Roles)
	m := make(map[string]bool)
	for _, r := range roles {
		m[r] = true
	}

	return m
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
	if conf.Version == "" {
		conf.Version = defaults.TeleportConfigVersionV1
	}

	var sc ssh.Config
	sc.SetDefaults()

	for _, c := range conf.Ciphers {
		if !apiutils.SliceContainsStr(sc.Ciphers, c) {
			return trace.BadParameter("cipher algorithm %q is not supported; supported algorithms: %q", c, sc.Ciphers)
		}
	}
	for _, k := range conf.KEXAlgorithms {
		if !apiutils.SliceContainsStr(sc.KeyExchanges, k) {
			return trace.BadParameter("KEX algorithm %q is not supported; supported algorithms: %q", k, sc.KeyExchanges)
		}
	}
	for _, m := range conf.MACAlgorithms {
		if !apiutils.SliceContainsStr(sc.MACs, m) {
			return trace.BadParameter("MAC algorithm %q is not supported; supported algorithms: %q", m, sc.MACs)
		}
	}

	return nil
}

// JoinParams configures the parameters for Simplified Node Joining.
type JoinParams struct {
	TokenName string           `yaml:"token_name"`
	Method    types.JoinMethod `yaml:"method"`
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

// LegacyLog contains the old format of the 'format' field
// It is kept here for backwards compatibility and should always be maintained
// The custom yaml unmarshaler should automatically convert it into the new
// expected format.
type LegacyLog struct {
	// Output defines where logs go. It can be one of the following: "stderr", "stdout" or
	// a path to a log file
	Output string `yaml:"output,omitempty"`
	// Severity defines how verbose the log will be. Possible values are "error", "info", "warn"
	Severity string `yaml:"severity,omitempty"`
	// Format lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller]
	Format []string `yaml:"format,omitempty"`
}

// Log configures teleport logging
type Log struct {
	// Output defines where logs go. It can be one of the following: "stderr", "stdout" or
	// a path to a log file
	Output string `yaml:"output,omitempty"`
	// Severity defines how verbose the log will be. Possible values are "error", "info", "warn"
	Severity string `yaml:"severity,omitempty"`
	// Format defines the logs output format and extra fields
	Format LogFormat `yaml:"format,omitempty"`
}

// LogFormat specifies the logs output format and extra fields
type LogFormat struct {
	// Output defines the output format. Possible values are 'text' and 'json'.
	Output string `yaml:"output,omitempty"`
	// ExtraFields lists the output fields from KnownFormatFields. Example format: [timestamp, component, caller]
	ExtraFields []string `yaml:"extra_fields,omitempty"`
}

func (l *Log) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// the next two lines are needed because of an infinite loop issue
	// https://github.com/go-yaml/yaml/issues/107
	type logYAML Log
	log := (*logYAML)(l)
	if err := unmarshal(log); err != nil {
		if _, ok := err.(*yaml.TypeError); !ok {
			return err
		}

		var legacyLog LegacyLog
		if lerr := unmarshal(&legacyLog); lerr != nil {
			// return the original unmarshal error
			return err
		}

		l.Output = legacyLog.Output
		l.Severity = legacyLog.Severity
		l.Format.Output = "text"
		l.Format.ExtraFields = legacyLog.Format
		return nil
	}

	return nil
}

// Global is 'teleport' (global) section of the config file
type Global struct {
	NodeName string `yaml:"nodename,omitempty"`
	DataDir  string `yaml:"data_dir,omitempty"`
	PIDFile  string `yaml:"pid_file,omitempty"`

	// AuthToken is the old way of configuring the token to be used by the
	// node to join the Teleport cluster. `JoinParams.TokenName` should be
	// used instead with `JoinParams.JoinMethod = types.JoinMethodToken`.
	AuthToken   string           `yaml:"auth_token,omitempty"`
	JoinParams  JoinParams       `yaml:"join_params,omitempty"`
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

	// CASignatureAlgorithm is ignored but ketp for config backwards compat
	CASignatureAlgorithm *string `yaml:"ca_signature_algo,omitempty"`

	// CAPin is the SKPI hash of the CA used to verify the Auth Server. Can be
	// a single value or a list.
	CAPin apiutils.Strings `yaml:"ca_pin"`

	// DiagAddr is the address to expose a diagnostics HTTP endpoint.
	DiagAddr string `yaml:"diag_addr"`
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

// Enabled determines if a given "_service" section has been set to 'true'
func (c *CachePolicy) Enabled() bool {
	if c.EnabledFlag == "" {
		return true
	}
	enabled, _ := apiutils.ParseBool(c.EnabledFlag)
	return enabled
}

// Parse parses cache policy from Teleport config
func (c *CachePolicy) Parse() (*service.CachePolicy, error) {
	out := service.CachePolicy{
		Enabled: c.Enabled(),
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
	v, err := apiutils.ParseBool(s.EnabledFlag)
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

	// ProxyProtocol enables support for HAProxy proxy protocol version 1 when it is turned 'on'.
	// Verify whether the service is in front of a trusted load balancer.
	// The default value is 'on'.
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

	// SessionRecording determines where the session is recorded:
	// node, node-sync, proxy, proxy-sync, or off.
	SessionRecording string `yaml:"session_recording,omitempty"`

	// ProxyChecksHostKeys is used when the proxy is in recording mode and
	// determines if the proxy will check the host key of the client or not.
	ProxyChecksHostKeys *types.BoolOption `yaml:"proxy_checks_host_keys,omitempty"`

	// LicenseFile is a path to the license file. The path can be either absolute or
	// relative to the global data dir
	LicenseFile string `yaml:"license_file,omitempty"`

	// FOR INTERNAL USE:
	// ReverseTunnels is a list of SSH tunnels to 3rd party proxy services (used to talk
	// to 3rd party auth servers we trust)
	ReverseTunnels []ReverseTunnel `yaml:"reverse_tunnels,omitempty"`

	// PublicAddr sets SSH host principals and TLS DNS names to auth
	// server certificates
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`

	// ClientIdleTimeout sets global cluster default setting for client idle timeouts
	ClientIdleTimeout types.Duration `yaml:"client_idle_timeout,omitempty"`

	// DisconnectExpiredCert provides disconnect expired certificate setting -
	// if true, connections with expired client certificates will get disconnected
	DisconnectExpiredCert *types.BoolOption `yaml:"disconnect_expired_cert,omitempty"`

	// SessionControlTimeout specifies the maximum amount of time a node can be out
	// of contact with the auth server before it starts terminating controlled sessions.
	SessionControlTimeout types.Duration `yaml:"session_control_timeout,omitempty"`

	// KubeconfigFile is an optional path to kubeconfig file,
	// if specified, teleport will use API server address and
	// trusted certificate authority information from it
	KubeconfigFile string `yaml:"kubeconfig_file,omitempty"`

	// KeepAliveInterval set the keep-alive interval for server to client
	// connections.
	KeepAliveInterval types.Duration `yaml:"keep_alive_interval,omitempty"`

	// KeepAliveCountMax set the number of keep-alive messages that can be
	// missed before the server disconnects the client.
	KeepAliveCountMax int64 `yaml:"keep_alive_count_max,omitempty"`

	// ClientIdleTimeoutMessage is sent to the client when the inactivity timeout
	// expires. The empty string implies no message should be sent prior to
	// disconnection.
	ClientIdleTimeoutMessage string `yaml:"client_idle_timeout_message,omitempty"`

	// MessageOfTheDay is a banner that a user must acknowledge during a `tsh login`.
	MessageOfTheDay string `yaml:"message_of_the_day,omitempty"`

	// WebIdleTimeout sets global cluster default setting for WebUI client
	// idle timeouts
	WebIdleTimeout types.Duration `yaml:"web_idle_timeout,omitempty"`

	// CAKeyParams configures how CA private keys will be created and stored.
	CAKeyParams *CAKeyParams `yaml:"ca_key_params,omitempty"`

	// ProxyListenerMode is a listener mode user by the proxy.
	ProxyListenerMode types.ProxyListenerMode `yaml:"proxy_listener_mode,omitempty"`

	// RoutingStrategy configures the routing strategy to nodes.
	RoutingStrategy types.RoutingStrategy `yaml:"routing_strategy,omitempty"`

	// TunnelStrategy configures the tunnel strategy used by the cluster.
	TunnelStrategy *types.TunnelStrategyV1 `yaml:"tunnel_strategy,omitempty"`
}

// CAKeyParams configures how CA private keys will be created and stored.
type CAKeyParams struct {
	// PKCS11 configures a PKCS#11 HSM to be used for private key generation and
	// storage.
	PKCS11 PKCS11 `yaml:"pkcs11"`
}

// PKCS11 configures a PKCS#11 HSM to be used for private key generation and
// storage.
type PKCS11 struct {
	// ModulePath is the path to the PKCS#11 library.
	ModulePath string `yaml:"module_path"`
	// TokenLabel is the CKA_LABEL of the HSM token to use. Set this or
	// SlotNumber to select a token.
	TokenLabel string `yaml:"token_label,omitempty"`
	// SlotNumber is the slot number of the HSM token to use. Set this or
	// TokenLabel to select a token.
	SlotNumber *int `yaml:"slot_number,omitempty"`
	// Pin is the raw pin for connecting to the HSM. Set this or PinPath to set
	// the pin.
	Pin string `yaml:"pin,omitempty"`
	// PinPath is a path to a file containing a pin for connecting to the HSM.
	// Trailing newlines will be removed, other whitespace will be left. Set
	// this or Pin to set the pin.
	PinPath string `yaml:"pin_path,omitempty"`
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

func (c ClusterName) Parse() (types.ClusterName, error) {
	if string(c) == "" {
		return nil, nil
	}
	return services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: string(c),
	})
}

type StaticTokens []StaticToken

func (t StaticTokens) Parse() (types.StaticTokens, error) {
	var provisionTokens []types.ProvisionTokenV1

	for _, st := range t {
		tokens, err := st.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		provisionTokens = append(provisionTokens, tokens...)
	}

	return types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: provisionTokens,
	})
}

type StaticToken string

// Parse is applied to a string in "role,role,role:token" format. It breaks it
// apart and constructs a list of services.ProvisionToken which contains the token,
// role, and expiry (infinite).
// If the token string is a file path, the file may contain multiple newline delimited
// tokens, in which case each token is used to construct a services.ProvisionToken
// with the same roles.
func (t StaticToken) Parse() ([]types.ProvisionTokenV1, error) {
	// Split only on the first ':', for future cross platform compat with windows paths
	parts := strings.SplitN(string(t), ":", 2)
	if len(parts) != 2 {
		return nil, trace.BadParameter("invalid static token spec: %q", t)
	}

	roles, err := types.ParseTeleportRoles(parts[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenPart, err := utils.TryReadValueAsFile(parts[1])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tokens := strings.Split(tokenPart, "\n")
	provisionTokens := make([]types.ProvisionTokenV1, 0, len(tokens))

	for _, token := range tokens {
		provisionTokens = append(provisionTokens, types.ProvisionTokenV1{
			Token:   token,
			Roles:   roles,
			Expires: time.Unix(0, 0).UTC(),
		})
	}
	return provisionTokens, nil
}

// AuthenticationConfig describes the auth_service/authentication section of teleport.yaml
type AuthenticationConfig struct {
	Type              string                     `yaml:"type"`
	SecondFactor      constants.SecondFactorType `yaml:"second_factor,omitempty"`
	ConnectorName     string                     `yaml:"connector_name,omitempty"`
	U2F               *UniversalSecondFactor     `yaml:"u2f,omitempty"`
	Webauthn          *Webauthn                  `yaml:"webauthn,omitempty"`
	RequireSessionMFA bool                       `yaml:"require_session_mfa,omitempty"`
	LockingMode       constants.LockingMode      `yaml:"locking_mode,omitempty"`

	// LocalAuth controls if local authentication is allowed.
	LocalAuth *types.BoolOption `yaml:"local_auth"`

	// Passwordless enables/disables passwordless support.
	// Requires Webauthn to work.
	// Defaults to true if the Webauthn is configured, defaults to false
	// otherwise.
	Passwordless *types.BoolOption `yaml:"passwordless"`
}

// Parse returns a types.AuthPreference (type, second factor, U2F).
func (a *AuthenticationConfig) Parse() (types.AuthPreference, error) {
	var err error

	var u *types.U2F
	if a.U2F != nil {
		u, err = a.U2F.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var w *types.Webauthn
	if a.Webauthn != nil {
		w, err = a.Webauthn.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:              a.Type,
		SecondFactor:      a.SecondFactor,
		ConnectorName:     a.ConnectorName,
		U2F:               u,
		Webauthn:          w,
		RequireSessionMFA: a.RequireSessionMFA,
		LockingMode:       a.LockingMode,
		AllowLocalAuth:    a.LocalAuth,
		AllowPasswordless: a.Passwordless,
	})
}

type UniversalSecondFactor struct {
	AppID string `yaml:"app_id"`
	// Facets kept only to avoid breakages during Teleport updates.
	// Webauthn is now used instead of U2F.
	Facets               []string `yaml:"facets"`
	DeviceAttestationCAs []string `yaml:"device_attestation_cas"`
}

func (u *UniversalSecondFactor) Parse() (*types.U2F, error) {
	attestationCAs, err := getAttestationPEMs(u.DeviceAttestationCAs)
	if err != nil {
		return nil, trace.BadParameter("u2f.device_attestation_cas: %v", err)
	}
	return &types.U2F{
		AppID:                u.AppID,
		DeviceAttestationCAs: attestationCAs,
	}, nil
}

type Webauthn struct {
	RPID                  string   `yaml:"rp_id,omitempty"`
	AttestationAllowedCAs []string `yaml:"attestation_allowed_cas,omitempty"`
	AttestationDeniedCAs  []string `yaml:"attestation_denied_cas,omitempty"`
	// Disabled has no effect, it is kept solely to not break existing
	// configurations.
	// DELETE IN 11.0, time to sunset U2F (codingllama).
	Disabled bool `yaml:"disabled,omitempty"`
}

func (w *Webauthn) Parse() (*types.Webauthn, error) {
	allowedCAs, err := getAttestationPEMs(w.AttestationAllowedCAs)
	if err != nil {
		return nil, trace.BadParameter("webauthn.attestation_allowed_cas: %v", err)
	}
	deniedCAs, err := getAttestationPEMs(w.AttestationDeniedCAs)
	if err != nil {
		return nil, trace.BadParameter("webauthn.attestation_denied_cas: %v", err)
	}
	if w.Disabled {
		log.Warnf(`` +
			`The "webauthn.disabled" setting is marked for removal and currently has no effect. ` +
			`Please update your configuration to use WebAuthn. ` +
			`Refer to https://goteleport.com/docs/access-controls/guides/webauthn/`)
	}
	return &types.Webauthn{
		// Allow any RPID to go through, we rely on
		// types.Webauthn.CheckAndSetDefaults to correct it.
		RPID:                  w.RPID,
		AttestationAllowedCAs: allowedCAs,
		AttestationDeniedCAs:  deniedCAs,
	}, nil
}

func getAttestationPEMs(certOrPaths []string) ([]string, error) {
	res := make([]string, len(certOrPaths))
	for i, certOrPath := range certOrPaths {
		pem, err := getAttestationPEM(certOrPath)
		if err != nil {
			return nil, err
		}
		res[i] = pem
	}
	return res, nil
}

func getAttestationPEM(certOrPath string) (string, error) {
	_, parseErr := tlsutils.ParseCertificatePEM([]byte(certOrPath))
	if parseErr == nil {
		return certOrPath, nil // OK, valid inline PEM
	}

	// Try reading as a file and parsing that.
	data, err := os.ReadFile(certOrPath)
	if err != nil {
		// Don't use trace in order to keep a clean error message.
		return "", fmt.Errorf("%q is not a valid x509 certificate (%v) and can't be read as a file (%v)", certOrPath, parseErr, err)
	}
	if _, err := tlsutils.ParseCertificatePEM(data); err != nil {
		// Don't use trace in order to keep a clean error message.
		return "", fmt.Errorf("file %q contains an invalid x509 certificate: %v", certOrPath, err)
	}

	return string(data), nil // OK, valid PEM file
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
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`

	// BPF is used to configure BPF-based auditing for this node.
	BPF *BPF `yaml:"enhanced_recording,omitempty"`

	// RestrictedSession is used to restrict access to kernel objects
	RestrictedSession *RestrictedSession `yaml:"restricted_session,omitempty"`

	// MaybeAllowTCPForwarding enables or disables TCP port forwarding. We're
	// using a pointer-to-bool here because the system default is to allow TCP
	// forwarding, we need to distinguish between an unset value and a false
	// value so we can an override unset value with `true`.
	//
	// Don't read this value directly: call the AllowTCPForwarding method
	// instead.
	MaybeAllowTCPForwarding *bool `yaml:"port_forwarding,omitempty"`

	// X11 is used to configure X11 forwarding settings
	X11 *X11 `yaml:"x11,omitempty"`

	// DisableCreateHostUser disables automatic user provisioning on this
	// SSH node.
	DisableCreateHostUser bool `yaml:"disable_create_host_user,omitempty"`

	// AWSMatchers are used to match EC2 instances
	AWSMatchers []AWSMatcher `yaml:"aws,omitempty"`
}

// AllowTCPForwarding checks whether the config file allows TCP forwarding or not.
func (ssh *SSH) AllowTCPForwarding() bool {
	if ssh.MaybeAllowTCPForwarding == nil {
		return true
	}
	return *ssh.MaybeAllowTCPForwarding
}

// X11ServerConfig returns the X11 forwarding server configuration.
func (ssh *SSH) X11ServerConfig() (*x11.ServerConfig, error) {
	// Start with default configuration
	cfg := &x11.ServerConfig{Enabled: false}
	if ssh.X11 == nil {
		return cfg, nil
	}

	var err error
	cfg.Enabled, err = apiutils.ParseBool(ssh.X11.Enabled)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	cfg.DisplayOffset = x11.DefaultDisplayOffset
	if ssh.X11.DisplayOffset != nil {
		cfg.DisplayOffset = int(*ssh.X11.DisplayOffset)

		if cfg.DisplayOffset > x11.MaxDisplayNumber {
			cfg.DisplayOffset = x11.MaxDisplayNumber
		}
	}

	cfg.MaxDisplay = cfg.DisplayOffset + x11.DefaultMaxDisplays
	if ssh.X11.MaxDisplay != nil {
		cfg.MaxDisplay = int(*ssh.X11.MaxDisplay)

		if cfg.MaxDisplay < cfg.DisplayOffset {
			return nil, trace.BadParameter("x11.MaxDisplay cannot be smaller than x11.DisplayOffset")
		}
	}

	if cfg.MaxDisplay > x11.MaxDisplayNumber {
		cfg.MaxDisplay = x11.MaxDisplayNumber
	}

	return cfg, nil
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

	// Environment represents environment variables to pass to PAM.
	// These may contain role-style interpolation syntax.
	Environment map[string]string `yaml:"environment,omitempty"`
}

// Parse returns a parsed pam.Config.
func (p *PAM) Parse() *pam.Config {
	serviceName := p.ServiceName
	if serviceName == "" {
		serviceName = defaults.ServiceName
	}
	enabled, _ := apiutils.ParseBool(p.Enabled)
	return &pam.Config{
		Enabled:     enabled,
		ServiceName: serviceName,
		UsePAMAuth:  p.UsePAMAuth,
		Environment: p.Environment,
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
	enabled, _ := apiutils.ParseBool(b.Enabled)
	return &bpf.Config{
		Enabled:           enabled,
		CommandBufferSize: b.CommandBufferSize,
		DiskBufferSize:    b.DiskBufferSize,
		NetworkBufferSize: b.NetworkBufferSize,
		CgroupPath:        b.CgroupPath,
	}
}

// RestrictedSession is a configuration for limiting access to kernel objects
type RestrictedSession struct {
	// Enabled enables or disables enforcemant for this node.
	Enabled string `yaml:"enabled"`

	// EventsBufferSize is the size in bytes of the channel to report events
	// from the kernel to us.
	EventsBufferSize *int `yaml:"events_buffer_size,omitempty"`
}

// Parse will parse the enhanced session recording configuration.
func (r *RestrictedSession) Parse() (*restricted.Config, error) {
	enabled, err := apiutils.ParseBool(r.Enabled)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &restricted.Config{
		Enabled:          enabled,
		EventsBufferSize: r.EventsBufferSize,
	}, nil
}

// X11 is a configuration for X11 forwarding
type X11 struct {
	// Enabled controls whether X11 forwarding requests can be granted by the server.
	Enabled string `yaml:"enabled"`
	// DisplayOffset tells the server what X11 display number to start from when
	// searching for an open X11 unix socket for XServer proxies.
	DisplayOffset *uint `yaml:"display_offset,omitempty"`
	// MaxDisplay tells the server what X11 display number to stop at when
	// searching for an open X11 unix socket for XServer proxies.
	MaxDisplay *uint `yaml:"max_display,omitempty"`
}

// Databases represents the database proxy service configuration.
//
// In the configuration file this section will be "db_service".
type Databases struct {
	// Service contains common service fields.
	Service `yaml:",inline"`
	// Databases is a list of databases proxied by the service.
	Databases []*Database `yaml:"databases"`
	// ResourceMatchers match cluster database resources.
	ResourceMatchers []ResourceMatcher `yaml:"resources,omitempty"`
	// AWSMatchers match AWS hosted databases.
	AWSMatchers []AWSMatcher `yaml:"aws,omitempty"`
}

// ResourceMatcher matches cluster resources.
type ResourceMatcher struct {
	// Labels match resource labels.
	Labels map[string]apiutils.Strings `yaml:"labels,omitempty"`
}

// AWSMatcher matches AWS databases.
type AWSMatcher struct {
	// Types are AWS database types to match, "rds", "redshift", "elasticache",
	// or "memorydb".
	Types []string `yaml:"types,omitempty"`
	// Regions are AWS regions to query for databases.
	Regions []string `yaml:"regions,omitempty"`
	// Tags are AWS tags to match.
	Tags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// SSMDocument is the ssm command document to execute for EC2
	// installation
	SSMDocument string `yaml:"ssm_command_document"`
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
	// Deprecated in favor of TLS.CACertFile.
	CACertFile string `yaml:"ca_cert_file,omitempty"`
	// TLS keeps an optional TLS configuration options.
	TLS DatabaseTLS `yaml:"tls"`
	// MySQL are additional database options.
	MySQL DatabaseMySQL `yaml:"mysql"`
	// StaticLabels is a map of database static labels.
	StaticLabels map[string]string `yaml:"static_labels,omitempty"`
	// DynamicLabels is a list of database dynamic labels.
	DynamicLabels []CommandLabel `yaml:"dynamic_labels,omitempty"`
	// AWS contains AWS specific settings for RDS/Aurora/Redshift databases.
	AWS DatabaseAWS `yaml:"aws"`
	// GCP contains GCP specific settings for Cloud SQL databases.
	GCP DatabaseGCP `yaml:"gcp"`
	// AD contains Active Directory database configuration.
	AD DatabaseAD `yaml:"ad"`
}

// DatabaseAD contains database Active Directory configuration.
type DatabaseAD struct {
	// KeytabFile is the path to the Kerberos keytab file.
	KeytabFile string `yaml:"keytab_file"`
	// Krb5File is the path to the Kerberos configuration file. Defaults to /etc/krb5.conf.
	Krb5File string `yaml:"krb5_file,omitempty"`
	// Domain is the Active Directory domain the database resides in.
	Domain string `yaml:"domain"`
	// SPN is the service principal name for the database.
	SPN string `yaml:"spn"`
}

// DatabaseTLS keeps TLS settings used when connecting to database.
type DatabaseTLS struct {
	// Mode is a TLS verification mode. Available options are 'verify-full', 'verify-ca' or 'insecure',
	// 'verify-full' is the default option.
	Mode string `yaml:"mode"`
	// ServerName allows providing custom server name.
	// This name will override DNS name when validating certificate presented by the database.
	ServerName string `yaml:"server_name,omitempty"`
	// CACertFile is an optional path to the database CA certificate.
	CACertFile string `yaml:"ca_cert_file,omitempty"`
}

// DatabaseMySQL are an additional MySQL database options.
type DatabaseMySQL struct {
	// ServerVersion is the MySQL version reported by DB proxy instead of default Teleport string.
	ServerVersion string `yaml:"server_version,omitempty"`
}

// SecretStore contains settings for managing secrets.
type SecretStore struct {
	// KeyPrefix specifies the secret key prefix.
	KeyPrefix string `yaml:"key_prefix,omitempty"`
	// KMSKeyID specifies the KMS key used to encrypt and decrypt the secret.
	KMSKeyID string `yaml:"kms_key_id,omitempty"`
}

// DatabaseAWS contains AWS specific settings for RDS/Aurora databases.
type DatabaseAWS struct {
	// Region is a cloud region for RDS/Aurora database endpoint.
	Region string `yaml:"region,omitempty"`
	// Redshift contains Redshift specific settings.
	Redshift DatabaseAWSRedshift `yaml:"redshift"`
	// RDS contains RDS specific settings.
	RDS DatabaseAWSRDS `yaml:"rds"`
	// ElastiCache contains ElastiCache specific settings.
	ElastiCache DatabaseAWSElastiCache `yaml:"elasticache"`
	// SecretStore contains settings for managing secrets.
	SecretStore SecretStore `yaml:"secret_store"`
	// MemoryDB contains MemoryDB specific settings.
	MemoryDB DatabaseAWSMemoryDB `yaml:"memorydb"`
}

// DatabaseAWSRedshift contains AWS Redshift specific settings.
type DatabaseAWSRedshift struct {
	// ClusterID is the Redshift cluster identifier.
	ClusterID string `yaml:"cluster_id,omitempty"`
}

// DatabaseAWSRDS contains settings for RDS databases.
type DatabaseAWSRDS struct {
	// InstanceID is the RDS instance identifier.
	InstanceID string `yaml:"instance_id,omitempty"`
	// ClusterID is the RDS cluster (Aurora) identifier.
	ClusterID string `yaml:"cluster_id,omitempty"`
}

// DatabaseAWSElastiCache contains settings for ElastiCache databases.
type DatabaseAWSElastiCache struct {
	// ReplicationGroupID is the ElastiCache replication group ID.
	ReplicationGroupID string `yaml:"replication_group_id,omitempty"`
}

// DatabaseAWSMemoryDB contains settings for MemoryDB databases.
type DatabaseAWSMemoryDB struct {
	// ClusterName is the MemoryDB cluster name.
	ClusterName string `yaml:"cluster_name,omitempty"`
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

	// ResourceMatchers match cluster application resources.
	ResourceMatchers []ResourceMatcher `yaml:"resources,omitempty"`
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

	// StaticLabels is a map of static labels to apply to this application.
	StaticLabels map[string]string `yaml:"labels,omitempty"`

	// DynamicLabels is a list of commands that generate dynamic labels
	// to apply to this application.
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
	// Headers is a list of extra headers to inject in the request.
	Headers []string `yaml:"headers,omitempty"`
}

// Proxy is a `proxy_service` section of the config file:
type Proxy struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`
	// WebAddr is a web UI listen address
	WebAddr string `yaml:"web_listen_addr,omitempty"`
	// TunAddr is a reverse tunnel address
	TunAddr string `yaml:"tunnel_listen_addr,omitempty"`
	// PeerAddr is the address this proxy will be dialed at by its peers.
	PeerAddr string `yaml:"peer_listen_addr,omitempty"`
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
	KubePublicAddr apiutils.Strings `yaml:"kube_public_addr,omitempty"`

	// PublicAddr sets the hostport the proxy advertises for the HTTP endpoint.
	// The hosts in PublicAddr are included in the list of host principals
	// on the SSH certificate.
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`

	// SSHPublicAddr sets the hostport the proxy advertises for the SSH endpoint.
	// The hosts in PublicAddr are included in the list of host principals
	// on the SSH certificate.
	SSHPublicAddr apiutils.Strings `yaml:"ssh_public_addr,omitempty"`

	// TunnelPublicAddr sets the hostport the proxy advertises for the tunnel
	// endpoint. The hosts in PublicAddr are included in the list of host
	// principals on the SSH certificate.
	TunnelPublicAddr apiutils.Strings `yaml:"tunnel_public_addr,omitempty"`

	// KeyPairs is a list of x509 key pairs the proxy will load.
	KeyPairs []KeyPair `yaml:"https_keypairs"`

	// ACME configures ACME protocol support
	ACME ACME `yaml:"acme"`

	// MySQLAddr is MySQL proxy listen address.
	MySQLAddr string `yaml:"mysql_listen_addr,omitempty"`
	// MySQLPublicAddr is the hostport the proxy advertises for MySQL
	// client connections.
	MySQLPublicAddr apiutils.Strings `yaml:"mysql_public_addr,omitempty"`

	// PostgresAddr is Postgres proxy listen address.
	PostgresAddr string `yaml:"postgres_listen_addr,omitempty"`
	// PostgresPublicAddr is the hostport the proxy advertises for Postgres
	// client connections.
	PostgresPublicAddr apiutils.Strings `yaml:"postgres_public_addr,omitempty"`

	// MongoAddr is Mongo proxy listen address.
	MongoAddr string `yaml:"mongo_listen_addr,omitempty"`
	// MongoPublicAddr is the hostport the proxy advertises for Mongo
	// client connections.
	MongoPublicAddr apiutils.Strings `yaml:"mongo_public_addr,omitempty"`
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
	out.Enabled, err = apiutils.ParseBool(a.EnabledFlag)
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
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`
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
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`
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
func (t *ReverseTunnel) ConvertAndValidate() (types.ReverseTunnel, error) {
	for i := range t.Addresses {
		addr, err := utils.ParseHostPortAddr(t.Addresses[i], defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return nil, trace.Wrap(err, "Invalid address for tunnel %v", t.DomainName)
		}
		t.Addresses[i] = addr.String()
	}

	out, err := types.NewReverseTunnel(t.DomainName, t.Addresses)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := services.ValidateReverseTunnel(out); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
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

// Metrics is a `metrics_service` section of the config file:
type Metrics struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`

	// KeyPairs is a list of x509 serving key pairs used for securing the metrics endpoint with mTLS.
	// mTLS will be enabled for the service if both 'keypairs' and 'ca_certs' fields are set.
	KeyPairs []KeyPair `yaml:"keypairs,omitempty"`

	// CACerts is a list of prometheus CA certificates to validate clients against.
	// mTLS will be enabled for the service if both 'keypairs' and 'ca_certs' fields are set.
	CACerts []string `yaml:"ca_certs,omitempty"`

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCServerLatency bool `yaml:"grpc_server_latency,omitempty"`

	// GRPCServerLatency enables histogram metrics for each grpc endpoint on the auth server
	GRPCClientLatency bool `yaml:"grpc_client_latency,omitempty"`
}

// MTLSEnabled returns whether mtls is enabled or not in the metrics service config.
func (m *Metrics) MTLSEnabled() bool {
	return len(m.KeyPairs) > 0 && len(m.CACerts) > 0
}

// WindowsDesktopService contains configuration for windows_desktop_service.
type WindowsDesktopService struct {
	Service `yaml:",inline"`
	// PublicAddr is a list of advertised public addresses of this service.
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`
	// LDAP is the LDAP connection parameters.
	LDAP LDAPConfig `yaml:"ldap"`
	// Discovery configures desktop discovery via LDAP.
	Discovery service.LDAPDiscoveryConfig `yaml:"discovery,omitempty"`
	// Hosts is a list of static Windows hosts connected to this service in
	// gateway mode.
	Hosts []string `yaml:"hosts,omitempty"`
	// HostLabels optionally applies labels to Windows hosts for RBAC.
	// A host can match multiple rules and will get a union of all
	// the matched labels.
	HostLabels []WindowsHostLabelRule `yaml:"host_labels,omitempty"`
}

// WindowsHostLabelRule describes how a set of labels should be a applied to
// a Windows host.
type WindowsHostLabelRule struct {
	// Match is a regexp that is checked against the Windows host's DNS name.
	// If the regexp matches, this rule's labels will be applied to the host.
	Match string `yaml:"match"`
	// Labels is the set of labels to apply to hosts that match this rule.
	Labels map[string]string `yaml:"labels"`
}

// LDAPConfig is the LDAP connection parameters.
type LDAPConfig struct {
	// Addr is the host:port of the LDAP server (typically port 389).
	Addr string `yaml:"addr"`
	// Domain is the ActiveDirectory domain name.
	Domain string `yaml:"domain"`
	// Username for LDAP authentication.
	Username string `yaml:"username"`
	// InsecureSkipVerify decides whether whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
	// DEREncodedCAFile is the filepath to an optional DER encoded CA cert to be used for verification (if InsecureSkipVerify is set to false).
	DEREncodedCAFile string `yaml:"der_ca_file,omitempty"`
}

// TracingService contains configuration for the tracing_service.
type TracingService struct {
	// Enabled turns the tracing service role on or off for this process
	EnabledFlag string `yaml:"enabled,omitempty"`

	// ExporterURL is the OTLP exporter URL to send spans to
	ExporterURL string `yaml:"exporter_url"`

	// KeyPairs is a list of x509 serving key pairs used for mTLS.
	KeyPairs []KeyPair `yaml:"keypairs,omitempty"`

	// CACerts are the exporter ca certs to use
	CACerts []string `yaml:"ca_certs,omitempty"`

	// SamplingRatePerMillion is the sampling rate for the exporter.
	// 1_000_000 means all spans will be sampled and 0 means none are sampled.
	SamplingRatePerMillion int `yaml:"sampling_rate_per_million"`
}

func (s *TracingService) Enabled() bool {
	if s.EnabledFlag == "" {
		return false
	}
	v, err := apiutils.ParseBool(s.EnabledFlag)
	if err != nil {
		return false
	}
	return v
}
