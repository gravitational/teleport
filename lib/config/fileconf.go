/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/automaticupgrades"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/utils"
)

// FileConfig structure represents the teleport configuration stored in a config file
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

	// Debug is the "debug_service" section that defines the configuration for
	// the Debug service.
	Debug DebugService `yaml:"debug_service,omitempty"`

	// WindowsDesktop is the "windows_desktop_service" that defines the
	// configuration for Windows Desktop Access.
	WindowsDesktop WindowsDesktopService `yaml:"windows_desktop_service,omitempty"`

	// Tracing is the "tracing_service" section in Teleport configuration file
	Tracing TracingService `yaml:"tracing_service,omitempty"`

	// Discovery is the "discovery_service" section in the Teleport
	// configuration file
	Discovery Discovery `yaml:"discovery_service,omitempty"`

	// Okta is the "okta_service" section in the Teleport configuration file
	Okta Okta `yaml:"okta_service,omitempty"`

	// Jamf is the "jamf_service" section in the config file.
	Jamf JamfService `yaml:"jamf_service,omitempty"`

	// Plugins is the section of the config for configuring the plugin service.
	Plugins PluginService `yaml:"plugin_service,omitempty"`

	// AccessGraph is the section of the config describing AccessGraph service
	AccessGraph AccessGraph `yaml:"access_graph,omitempty"`
}

// ReadFromFile reads Teleport configuration from a file. Currently only YAML
// format is supported
func ReadFromFile(filePath string) (*FileConfig, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filePath)
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
		return nil, trace.BadParameter("failed parsing the config file: %s", strings.ReplaceAll(err.Error(), "\n", ""))
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
	// ProxyAddress is the address of the proxy
	ProxyAddress string
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
	// Silent suppresses user hint printed after config has been generated.
	Silent bool
	// AzureClientID is the client ID of the managed identity to use when joining
	// the cluster. Only applicable for the azure join method.
	AzureClientID string
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

	conf := servicecfg.MakeDefaultConfig()

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

	if err := setJoinParams(&g, flags); err != nil {
		return nil, trace.Wrap(err)
	}

	if flags.Version == defaults.TeleportConfigVersionV3 {
		if flags.AuthServer != "" && flags.ProxyAddress != "" {
			return nil, trace.BadParameter("--proxy and --auth-server cannot both be set")
		} else if flags.AuthServer != "" {
			g.AuthServer = flags.AuthServer
		} else if flags.ProxyAddress != "" {
			g.ProxyServer = flags.ProxyAddress
		}
	} else {
		if flags.AuthServer != "" {
			g.AuthServers = []string{flags.AuthServer}
		}
		if flags.ProxyAddress != "" {
			return nil, trace.BadParameter("--proxy cannot be used with configuration versions older than v3")
		}
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

func setJoinParams(g *Global, flags SampleFlags) error {
	joinMethod := flags.JoinMethod
	if joinMethod == "" && flags.AuthToken != "" {
		joinMethod = string(types.JoinMethodToken)
	}
	g.JoinParams = JoinParams{
		TokenName: flags.AuthToken,
		Method:    types.JoinMethod(joinMethod),
	}
	if flags.AzureClientID != "" {
		g.JoinParams.Azure = AzureJoinParams{
			ClientID: flags.AzureClientID,
		}
	}
	return nil
}

func makeSampleSSHConfig(conf *servicecfg.Config, flags SampleFlags, enabled bool) (SSH, error) {
	var s SSH
	if enabled {
		s.EnabledFlag = "yes"
		s.ListenAddress = conf.SSH.Addr.Addr
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

func makeSampleAuthConfig(conf *servicecfg.Config, flags SampleFlags, enabled bool) Auth {
	var a Auth
	if enabled {
		a.ListenAddress = conf.Auth.ListenAddr.Addr
		a.ClusterName = ClusterName(flags.ClusterName)
		a.EnabledFlag = "yes"

		if flags.LicensePath != "" {
			a.LicenseFile = flags.LicensePath
		}

		// from config v2 onwards, we support `proxy_listener_mode`, so we set it to `multiplex`
		if flags.Version != defaults.TeleportConfigVersionV1 {
			a.ProxyListenerMode = types.ProxyListenerMode_Multiplex
		}
	} else {
		a.EnabledFlag = "no"
	}

	return a
}

func makeSampleProxyConfig(conf *servicecfg.Config, flags SampleFlags, enabled bool) (Proxy, error) {
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

func makeSampleAppsConfig(conf *servicecfg.Config, flags SampleFlags, enabled bool) (Apps, error) {
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
	conf.Okta.defaultEnabled = false
	conf.Debug.defaultEnabled = true
	if conf.Version == "" {
		conf.Version = defaults.TeleportConfigVersionV1
	}

	var sc ssh.Config
	sc.SetDefaults()

	for _, c := range conf.Ciphers {
		if !slices.Contains(sc.Ciphers, c) {
			return trace.BadParameter("cipher algorithm %q is not supported; supported algorithms: %q", c, sc.Ciphers)
		}
	}
	for _, k := range conf.KEXAlgorithms {
		if !slices.Contains(sc.KeyExchanges, k) {
			return trace.BadParameter("KEX algorithm %q is not supported; supported algorithms: %q", k, sc.KeyExchanges)
		}
	}
	for _, m := range conf.MACAlgorithms {
		if !slices.Contains(sc.MACs, m) {
			return trace.BadParameter("MAC algorithm %q is not supported; supported algorithms: %q", m, sc.MACs)
		}
	}

	return nil
}

// JoinParams configures the parameters for Simplified Node Joining.
type JoinParams struct {
	TokenName string           `yaml:"token_name"`
	Method    types.JoinMethod `yaml:"method"`
	Azure     AzureJoinParams  `yaml:"azure,omitempty"`
}

// AzureJoinParams configures the parameters specific to the Azure join method.
type AzureJoinParams struct {
	ClientID string `yaml:"client_id"`
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
	Rates          []ConnectionRate `yaml:"rates,omitempty"`

	// Deprecated: MaxUsers has no effect.
	MaxUsers int `yaml:"max_users"`
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
		var typeError *yaml.TypeError
		if !errors.As(err, &typeError) {
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

	JoinParams JoinParams `yaml:"join_params,omitempty"`

	// v1, v2
	AuthServers []string `yaml:"auth_servers,omitempty"`
	// AuthToken is the old way of configuring the token to be used by the
	// node to join the Teleport cluster. `JoinParams.TokenName` should be
	// used instead with `JoinParams.JoinMethod = types.JoinMethodToken`.
	AuthToken string `yaml:"auth_token,omitempty"`

	// v3
	AuthServer  string `yaml:"auth_server,omitempty"`
	ProxyServer string `yaml:"proxy_server,omitempty"`

	Limits      ConnectionLimits `yaml:"connection_limits,omitempty"`
	Logger      Log              `yaml:"log,omitempty"`
	Storage     backend.Config   `yaml:"storage,omitempty"`
	AdvertiseIP string           `yaml:"advertise_ip,omitempty"`
	CachePolicy CachePolicy      `yaml:"cache,omitempty"`

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
	// MaxBackoff sets the maximum backoff on error.
	MaxBackoff time.Duration `yaml:"max_backoff,omitempty"`
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
func (c *CachePolicy) Parse() (*servicecfg.CachePolicy, error) {
	out := servicecfg.CachePolicy{
		Enabled:        c.Enabled(),
		MaxRetryPeriod: c.MaxBackoff,
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

	// ProxyProtocol controls support for HAProxy PROXY protocol.
	// Possible values:
	// - 'on': one PROXY header is accepted and required per incoming connection.
	// - 'off': no PROXY headers are allows, otherwise connection is rejected.
	// If unspecified - one PROXY header is allowed, but not required. Connection is marked with source port set to 0
	// and IP pinning will not be allowed. It is supposed to be used only as default mode for test setups.
	// In production you should always explicitly set the mode based on your network setup - if you have L4 load balancer
	// with enabled PROXY protocol in front of Teleport you should set it to 'on', if you don't have it, set it to 'off'
	ProxyProtocol string `yaml:"proxy_protocol,omitempty"`

	// ClusterName is the name of the CA who manages this cluster
	ClusterName ClusterName `yaml:"cluster_name,omitempty"`

	// StaticTokens are pre-defined host provisioning tokens supplied via config file for
	// environments where paranoid security is not needed
	//
	// Each token string has the following format: "role1,role2,..:token",
	// for example: "auth,proxy,node:MTIzNGlvemRmOWE4MjNoaQo"
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

	// ProxyPingInterval defines in which interval the TLS routing ping message
	// should be sent. This is applicable only when using ping-wrapped
	// connections, regular TLS routing connections are not affected.
	ProxyPingInterval types.Duration `yaml:"proxy_ping_interval,omitempty"`

	// CaseInsensitiveRouting causes proxies to use case-insensitive hostname matching.
	CaseInsensitiveRouting bool `yaml:"case_insensitive_routing,omitempty"`

	// SSHDialTimeout is the timeout value that should be used for SSH connections.
	SSHDialTimeout types.Duration `yaml:"ssh_dial_timeout,omitempty"`

	// LoadAllCAs tells tsh to load the CAs for all clusters when trying
	// to ssh into a node, instead of just the CA for the current cluster.
	LoadAllCAs bool `yaml:"load_all_cas,omitempty"`

	// HostedPlugins configures the hosted plugins runtime.
	// This is currently Cloud-specific.
	HostedPlugins HostedPlugins `yaml:"hosted_plugins,omitempty"`

	// AccessMonitoring is a set of options related to the Access Monitoring feature.
	AccessMonitoring *servicecfg.AccessMonitoringOptions `yaml:"access_monitoring,omitempty"`
}

// PluginService represents the configuration for the plugin service.
type PluginService struct {
	Enabled bool `yaml:"enabled"`
	// Plugins is a map of matchers for enabled plugin resources.
	Plugins map[string]string `yaml:"plugins,omitempty"`
}

// AccessGraph represents the configuration for the AccessGraph service.
type AccessGraph struct {
	// Enabled enables the AccessGraph service.
	Enabled bool `yaml:"enabled"`
	// Endpoint is the endpoint of the AccessGraph service.
	Endpoint string `yaml:"endpoint"`
	// CA is the path to the CA certificate for the AccessGraph service.
	CA string `yaml:"ca"`
	// Insecure is true if the AccessGraph service should not verify the CA.
	Insecure bool `yaml:"insecure"`
	// AuditLog contains audit log export details.
	AuditLog AuditLogConfig `yaml:"audit_log"`
}

// AuditLogConfig specifies the audit log event export setup.
type AuditLogConfig struct {
	// Enabled indicates if Audit Log event exporting is enabled.
	Enabled bool `yaml:"enabled"`
	// StartDate is the start date for exporting audit logs. It defaults to 90 days ago on the first export.
	StartDate time.Time `yaml:"start_date"`
}

// Opsgenie represents the configuration for the Opsgenie plugin.
type Opsgenie struct {
	// APIKeyFile is the path to a file containing an Opsgenie API key.
	APIKeyFile string `yaml:"api_key_file"`
}

// hasCustomNetworkingConfig returns true if any of the networking
// configuration fields have values different from an empty Auth.
func (a *Auth) hasCustomNetworkingConfig() bool {
	empty := Auth{}
	return a.ClientIdleTimeout != empty.ClientIdleTimeout ||
		a.ClientIdleTimeoutMessage != empty.ClientIdleTimeoutMessage ||
		a.WebIdleTimeout != empty.WebIdleTimeout ||
		a.KeepAliveInterval != empty.KeepAliveInterval ||
		a.KeepAliveCountMax != empty.KeepAliveCountMax ||
		a.SessionControlTimeout != empty.SessionControlTimeout ||
		a.ProxyListenerMode != empty.ProxyListenerMode ||
		a.RoutingStrategy != empty.RoutingStrategy ||
		a.TunnelStrategy != empty.TunnelStrategy ||
		a.ProxyPingInterval != empty.ProxyPingInterval ||
		a.SSHDialTimeout != empty.SSHDialTimeout
}

// hasCustomSessionRecording returns true if any of the session recording
// configuration fields have values different from an empty Auth.
func (a *Auth) hasCustomSessionRecording() bool {
	empty := Auth{}
	return a.SessionRecording != empty.SessionRecording ||
		a.ProxyChecksHostKeys != empty.ProxyChecksHostKeys
}

// CAKeyParams configures how CA private keys will be created and stored.
type CAKeyParams struct {
	// PKCS11 configures a PKCS#11 HSM to be used for all CA private key generation and
	// storage.
	PKCS11 *PKCS11 `yaml:"pkcs11,omitempty"`
	// GoogleCloudKMS configures Google Cloud Key Management Service to to be used for
	// all CA private key crypto operations.
	GoogleCloudKMS *GoogleCloudKMS `yaml:"gcp_kms,omitempty"`
	// AWSKMS configures AWS Key Management Service to to be used for
	// all CA private key crypto operations.
	AWSKMS *AWSKMS `yaml:"aws_kms,omitempty"`
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
	// PIN is the raw pin for connecting to the HSM. Set this or PINPath to set
	// the pin.
	PIN string `yaml:"pin,omitempty"`
	// PINPath is a path to a file containing a pin for connecting to the HSM.
	// Trailing newlines will be removed, other whitespace will be left. Set
	// this or Pin to set the pin.
	PINPath string `yaml:"pin_path,omitempty"`
	// MaxSessions is the upper limit of sessions allowed by the HSM.
	MaxSessions int `yaml:"max_sessions"`
}

// GoogleCloudKMS configures Google Cloud Key Management Service to to be used for
// all CA private key crypto operations.
type GoogleCloudKMS struct {
	// KeyRing is the GCP key ring where all keys generated by this auth server
	// should be held. This must be the fully qualified resource name of the key
	// ring, including the project and location, e.g.
	// projects/teleport-project/locations/us-west1/keyRings/teleport-keyring
	KeyRing string `yaml:"keyring"`
	// ProtectionLevel specifies how cryptographic operations are performed.
	// For more information, see https://cloud.google.com/kms/docs/algorithms#protection_levels
	// Supported options are "HSM" and "SOFTWARE".
	ProtectionLevel string `yaml:"protection_level"`
}

// AWSKMS configures AWS Key Management Service to to be used for all CA private
// key crypto operations.
type AWSKMS struct {
	// Account is the AWS account to use.
	Account string `yaml:"account"`
	// Region is the AWS region to use.
	Region string `yaml:"region"`
	// MultiRegion contains configuration for multi-region AWS KMS.
	MultiRegion servicecfg.MultiRegionKeyStore `yaml:"multi_region,omitempty"`
	// Tags are key/value pairs used as AWS resource tags. The 'TeleportCluster'
	// tag is added automatically if not specified in the set of tags. Changing tags
	// after Teleport has already created KMS keys may require manually updating
	// the tags of existing keys.
	Tags map[string]string `yaml:"tags,omitempty"`
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
	Type           string                     `yaml:"type"`
	SecondFactor   constants.SecondFactorType `yaml:"second_factor,omitempty"`
	SecondFactors  []types.SecondFactorType   `yaml:"second_factors,omitempty"`
	ConnectorName  string                     `yaml:"connector_name,omitempty"`
	U2F            *UniversalSecondFactor     `yaml:"u2f,omitempty"`
	Webauthn       *Webauthn                  `yaml:"webauthn,omitempty"`
	RequireMFAType types.RequireMFAType       `yaml:"require_session_mfa,omitempty"`
	LockingMode    constants.LockingMode      `yaml:"locking_mode,omitempty"`

	// LocalAuth controls if local authentication is allowed.
	LocalAuth *types.BoolOption `yaml:"local_auth"`

	// Passwordless enables/disables passwordless support.
	// Requires Webauthn to work.
	// Defaults to true if the Webauthn is configured, defaults to false
	// otherwise.
	Passwordless *types.BoolOption `yaml:"passwordless"`

	// Headless enables/disables headless support.
	// Requires Webauthn to work.
	// Defaults to true if the Webauthn is configured, defaults to false
	// otherwise.
	Headless *types.BoolOption `yaml:"headless"`

	// DeviceTrust holds settings related to trusted device verification.
	// Requires Teleport Enterprise.
	DeviceTrust *DeviceTrust `yaml:"device_trust,omitempty"`

	// DefaultSessionTTL is the default cluster max session ttl
	DefaultSessionTTL types.Duration `yaml:"default_session_ttl"`

	// Deprecated. HardwareKey.PIVSlot should be used instead.
	PIVSlot hardwarekey.PIVSlotKeyString `yaml:"piv_slot,omitempty"`

	// HardwareKey holds settings related to hardware key support.
	// Requires Teleport Enterprise.
	HardwareKey *HardwareKey `yaml:"hardware_key,omitempty"`

	// SignatureAlgorithmSuite is the configured signature algorithm suite for the cluster.
	SignatureAlgorithmSuite types.SignatureAlgorithmSuite `yaml:"signature_algorithm_suite"`

	// StableUNIXUserConfig is [types.AuthPreferenceSpecV2.StableUnixUserConfig].
	StableUNIXUserConfig *StableUNIXUserConfig `yaml:"stable_unix_user_config,omitempty"`
}

// Parse returns valid types.AuthPreference instance.
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

	var dt *types.DeviceTrust
	if a.DeviceTrust != nil {
		dt, err = a.DeviceTrust.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	var h *types.HardwareKey
	switch {
	case a.HardwareKey != nil:
		if a.PIVSlot != "" {
			slog.WarnContext(context.Background(), `Both "piv_slot" and "hardware_key" settings were populated, using "hardware_key" setting`)
		}
		h, err = a.HardwareKey.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	case a.HardwareKey == nil && a.PIVSlot != "":
		if err = a.PIVSlot.Validate(); err != nil {
			return nil, trace.Wrap(err, "failed to parse piv_slot")
		}

		h = &types.HardwareKey{
			PIVSlot: string(a.PIVSlot),
		}
	default:
	}

	if a.SecondFactor != "" && a.SecondFactors != nil {
		const msg = `second_factor and second_factors are both set. second_factors will take precedence. ` +
			`second_factor should be unset to remove this warning.`
		slog.WarnContext(context.Background(), msg)
	}

	stableUNIXUserConfig, err := a.StableUNIXUserConfig.Parse()
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse stable_unix_user_config")
	}

	ap, err := types.NewAuthPreferenceFromConfigFile(types.AuthPreferenceSpecV2{
		Type:                    a.Type,
		SecondFactor:            a.SecondFactor,
		SecondFactors:           a.SecondFactors,
		ConnectorName:           a.ConnectorName,
		U2F:                     u,
		Webauthn:                w,
		RequireMFAType:          a.RequireMFAType,
		LockingMode:             a.LockingMode,
		AllowLocalAuth:          a.LocalAuth,
		AllowPasswordless:       a.Passwordless,
		AllowHeadless:           a.Headless,
		DeviceTrust:             dt,
		DefaultSessionTTL:       a.DefaultSessionTTL,
		HardwareKey:             h,
		SignatureAlgorithmSuite: a.SignatureAlgorithmSuite,
		StableUnixUserConfig:    stableUNIXUserConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := services.ValidateAuthPreference(ap); err != nil {
		return nil, trace.Wrap(err)
	}

	return ap, nil
}

type UniversalSecondFactor struct {
	AppID string `yaml:"app_id"`
	// Facets kept only to avoid breakages during Teleport updates.
	// Webauthn is now used instead of U2F.
	Facets               []string `yaml:"facets"`
	DeviceAttestationCAs []string `yaml:"device_attestation_cas"`
}

func (u *UniversalSecondFactor) Parse() (*types.U2F, error) {
	attestationCAs, err := getCertificatePEMs(u.DeviceAttestationCAs)
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
	// Deprecated: Disabled has no effect, it is kept solely to not break existing
	// configurations.
	Disabled bool `yaml:"disabled,omitempty"`
}

func (w *Webauthn) Parse() (*types.Webauthn, error) {
	allowedCAs, err := getCertificatePEMs(w.AttestationAllowedCAs)
	if err != nil {
		return nil, trace.BadParameter("webauthn.attestation_allowed_cas: %v", err)
	}
	deniedCAs, err := getCertificatePEMs(w.AttestationDeniedCAs)
	if err != nil {
		return nil, trace.BadParameter("webauthn.attestation_denied_cas: %v", err)
	}
	if w.Disabled {
		const msg = `The "webauthn.disabled" setting is marked for removal and currently has no effect. ` +
			`Please update your configuration to use WebAuthn. ` +
			`Refer to https://goteleport.com/docs/admin-guides/access-controls/guides/webauthn/`
		slog.WarnContext(context.Background(), msg)
	}
	return &types.Webauthn{
		// Allow any RPID to go through, we rely on
		// types.Webauthn.CheckAndSetDefaults to correct it.
		RPID:                  w.RPID,
		AttestationAllowedCAs: allowedCAs,
		AttestationDeniedCAs:  deniedCAs,
	}, nil
}

func getCertificatePEMs(certOrPaths []string) ([]string, error) {
	res := make([]string, len(certOrPaths))
	for i, certOrPath := range certOrPaths {
		pem, err := getCertificatePEM(certOrPath)
		if err != nil {
			return nil, err
		}
		res[i] = pem
	}
	return res, nil
}

func getCertificatePEM(certOrPath string) (string, error) {
	_, parseErr := tlsutils.ParseCertificatePEM([]byte(certOrPath))
	if parseErr == nil {
		return certOrPath, nil // OK, valid inline PEM
	}

	// Try reading as a file and parsing that.
	data, err := os.ReadFile(certOrPath)
	if err != nil {
		// Don't use trace in order to keep a clean error message.
		return "", fmt.Errorf("%q is not a valid x509 certificate (%w) and can't be read as a file (%w)", certOrPath, parseErr, err)
	}
	if _, err := tlsutils.ParseCertificatePEM(data); err != nil {
		// Don't use trace in order to keep a clean error message.
		return "", fmt.Errorf("file %q contains an invalid x509 certificate: %w", certOrPath, err)
	}

	return string(data), nil // OK, valid PEM file
}

// DeviceTrust holds settings related to trusted device verification.
// Requires Teleport Enterprise.
type DeviceTrust struct {
	// Mode is the trusted device verification mode.
	// Mirrors types.DeviceTrust.Mode.
	Mode string `yaml:"mode,omitempty"`
	// AutoEnroll is the toggle for the device auto-enroll feature.
	AutoEnroll string `yaml:"auto_enroll,omitempty"`
	// EKCertAllowedCAs is an allow list of EKCert CAs. These may be specified
	// as a PEM encoded certificate or as a path to a PEM encoded certificate.
	//
	// If present, only TPM devices that present an EKCert that is signed by a
	// CA specified here may be enrolled (existing enrollments are
	// unchanged).
	//
	// If not present, then the CA of TPM EKCerts will not be checked during
	// enrollment, this allows any device to enroll.
	EKCertAllowedCAs []string `yaml:"ekcert_allowed_cas,omitempty"`
}

func (dt *DeviceTrust) Parse() (*types.DeviceTrust, error) {
	autoEnroll := false
	if dt.AutoEnroll != "" {
		var err error
		autoEnroll, err = apiutils.ParseBool(dt.AutoEnroll)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	allowedCAs, err := getCertificatePEMs(dt.EKCertAllowedCAs)
	if err != nil {
		return nil, trace.BadParameter("device_trust.ekcert_allowed_cas: %v", err)
	}

	return &types.DeviceTrust{
		Mode:             dt.Mode,
		AutoEnroll:       autoEnroll,
		EKCertAllowedCAs: allowedCAs,
	}, nil
}

// HardwareKey holds settings related to hardware key support.
// Requires Teleport Enterprise.
type HardwareKey struct {
	// PIVSlot is a PIV slot that Teleport clients should use instead of the
	// default based on private key policy. For example, "9a" or "9e".
	PIVSlot hardwarekey.PIVSlotKeyString `yaml:"piv_slot,omitempty"`

	// SerialNumberValidation contains optional settings for hardware key
	// serial number validation, including whether it is enabled.
	SerialNumberValidation *HardwareKeySerialNumberValidation `yaml:"serial_number_validation,omitempty"`

	// PINCacheTTL specifies how long to cache the user's PIV PIN.
	PINCacheTTL time.Duration `yaml:"pin_cache_ttl,omitempty"`
}

func (h *HardwareKey) Parse() (*types.HardwareKey, error) {
	if h.PIVSlot != "" {
		if err := h.PIVSlot.Validate(); err != nil {
			return nil, trace.Wrap(err, "failed to parse hardware_key.piv_slot")
		}
	}

	hk := &types.HardwareKey{
		PIVSlot:     string(h.PIVSlot),
		PinCacheTTL: types.Duration(h.PINCacheTTL),
	}

	if h.SerialNumberValidation != nil {
		var err error
		hk.SerialNumberValidation, err = h.SerialNumberValidation.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return hk, nil
}

// HardwareKeySerialNumberValidation holds settings related to hardware key serial number validation.
// Requires Teleport Enterprise.
type HardwareKeySerialNumberValidation struct {
	// Enabled indicates whether hardware key serial number validation is enabled.
	Enabled string `yaml:"enabled"`

	// SerialNumberTraitName is an optional custom user trait name for hardware key
	// serial numbers to replace the default: "hardware_key_serial_numbers".
	SerialNumberTraitName string `yaml:"serial_number_trait_name"`
}

func (h *HardwareKeySerialNumberValidation) Parse() (*types.HardwareKeySerialNumberValidation, error) {
	enabled, err := apiutils.ParseBool(h.Enabled)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &types.HardwareKeySerialNumberValidation{
		Enabled:               enabled,
		SerialNumberTraitName: h.SerialNumberTraitName,
	}, nil
}

// StableUNIXUserConfig is [types.StableUNIXUserConfig].
type StableUNIXUserConfig struct {
	// Enabled is [types.StableUNIXUserConfig.Enabled].
	Enabled bool `yaml:"enabled"`
	// FirstUID is [types.StableUNIXUserConfig.FirstUid].
	FirstUID int32 `yaml:"first_uid"`
	// LastUID is [types.StableUNIXUserConfig.LastUid].
	LastUID int32 `yaml:"last_uid"`
}

func (s *StableUNIXUserConfig) Parse() (*types.StableUNIXUserConfig, error) {
	if s == nil {
		return nil, nil
	}

	c := &types.StableUNIXUserConfig{
		Enabled:  s.Enabled,
		FirstUid: s.FirstUID,
		LastUid:  s.LastUID,
	}

	if err := services.ValidateStableUNIXUserConfig(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return c, nil
}

// HostedPlugins defines 'auth_service/plugins' Enterprise extension
type HostedPlugins struct {
	Enabled        bool                 `yaml:"enabled"`
	OAuthProviders PluginOAuthProviders `yaml:"oauth_providers,omitempty"`
}

// PluginOAuthProviders holds application credentials for each
// 3rd party API provider.
type PluginOAuthProviders struct {
	Slack *OAuthClientCredentials `yaml:"slack,omitempty"`
}

func (p *PluginOAuthProviders) Parse() (servicecfg.PluginOAuthProviders, error) {
	out := servicecfg.PluginOAuthProviders{}
	if p.Slack != nil {
		slack, err := p.Slack.Parse()
		if err != nil {
			return out, trace.Wrap(err)
		}
		out.SlackCredentials = slack
	}
	return out, nil
}

// OAuthClientCredentials holds paths from which to read
// client credentials for Teleport's OAuth app.
type OAuthClientCredentials struct {
	// ClientID is the path to the file containing the Client ID
	ClientID string `yaml:"client_id"`
	// ClientSecret is the path to the file containing the Client Secret
	ClientSecret string `yaml:"client_secret"`
}

func (o *OAuthClientCredentials) Parse() (*servicecfg.OAuthClientCredentials, error) {
	if o.ClientID == "" || o.ClientSecret == "" {
		return nil, trace.BadParameter("both client_id and client_secret paths must be specified")
	}

	var clientID, clientSecret string

	content, err := os.ReadFile(o.ClientID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientID = strings.TrimSpace(string(content))

	content, err = os.ReadFile(o.ClientSecret)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientSecret = strings.TrimSpace(string(content))

	return &servicecfg.OAuthClientCredentials{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, nil
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

	// MaybeSSHFileCopy enables or disables remote file operations via SCP/SFTP.
	// We're using a pointer-to-bool here because the system default is to allow
	// SCP/SFTP, we need to distinguish between an unset value and a false
	// value so we can an override unset value with `true`.
	//
	// Don't read this value directly: call the SSHFileCopy method
	// instead.
	MaybeSSHFileCopy *bool `yaml:"ssh_file_copy,omitempty"`

	// DisableCreateHostUser disables automatic user provisioning on this
	// SSH node.
	DisableCreateHostUser bool `yaml:"disable_create_host_user,omitempty"`

	// ForceListen enables listening on the configured ListenAddress
	// when connected to the cluster via a reverse tunnel. If no ListenAddress is
	// configured, the default address is used.
	//
	// This allows the service to be connectable by users with direct network access.
	// All connections still require a valid user certificate to be presented and will
	// not permit any additional access. This is intended to provide an optional connection
	// path to reduce latency if the Proxy is not co-located with the user and service.
	ForceListen bool `yaml:"force_listen,omitempty"`
}

// AllowTCPForwarding checks whether the config file allows TCP forwarding or not.
func (ssh *SSH) AllowTCPForwarding() bool {
	if ssh.MaybeAllowTCPForwarding == nil {
		return true
	}
	return *ssh.MaybeAllowTCPForwarding
}

// SSHFileCopy checks whether the config file allows for file copying
// via SCP/SFTP.
func (ssh *SSH) SSHFileCopy() bool {
	if ssh.MaybeSSHFileCopy == nil {
		return true
	}
	return *ssh.MaybeSSHFileCopy
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

// Discovery represents a discovery_service section in the config file.
type Discovery struct {
	Service `yaml:",inline"`

	// AWSMatchers are used to match EC2 instances
	AWSMatchers []AWSMatcher `yaml:"aws,omitempty"`

	// AzureMatchers are used to match Azure resources.
	AzureMatchers []AzureMatcher `yaml:"azure,omitempty"`

	// GCPMatchers are used to match GCP resources.
	GCPMatchers []GCPMatcher `yaml:"gcp,omitempty"`

	// KubernetesMatchers are used to match services inside Kubernetes cluster for auto discovery
	KubernetesMatchers []KubernetesMatcher `yaml:"kubernetes,omitempty"`

	// AccessGraph is used to configure the cloud sync into AccessGraph.
	AccessGraph *AccessGraphSync `yaml:"access_graph,omitempty"`

	// DiscoveryGroup is the name of the discovery group that the current
	// discovery service is a part of.
	// It is used to filter out discovered resources that belong to another
	// discovery services. When running in high availability mode and the agents
	// have access to the same cloud resources, this field value must be the same
	// for all discovery services. If different agents are used to discover different
	// sets of cloud resources, this field must be different for each set of agents.
	DiscoveryGroup string `yaml:"discovery_group,omitempty"`
	// PollInterval is the cadence at which the discovery server will run each of its
	// discovery cycles.
	// Default: [github.com/gravitational/teleport/lib/srv/discovery/common.DefaultDiscoveryPollInterval]
	PollInterval time.Duration `yaml:"poll_interval,omitempty"`
}

// GCPMatcher matches GCP resources.
type GCPMatcher struct {
	// Types are GKE resource types to match: "gke", "gce".
	Types []string `yaml:"types,omitempty"`
	// Locations are GKE locations to search resources for.
	Locations []string `yaml:"locations,omitempty"`
	// Labels are GCP labels to match.
	Labels map[string]apiutils.Strings `yaml:"labels,omitempty"`
	// Tags are an alias for Labels, for backwards compatibility.
	Tags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// ProjectIDs are the GCP project ID where the resources are deployed.
	ProjectIDs []string `yaml:"project_ids,omitempty"`
	// ServiceAccounts are the emails of service accounts attached to VMs.
	ServiceAccounts []string `yaml:"service_accounts,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered GCP VMs.
	InstallParams *InstallParams `yaml:"install,omitempty"`
}

// AccessGraphSync represents the configuration for the AccessGraph Sync service.
type AccessGraphSync struct {
	// AWS is the AWS configuration for the AccessGraph Sync service.
	AWS []AccessGraphAWSSync `yaml:"aws,omitempty"`
	// Azure is the Azure configuration for the AccessGraph Sync service.
	Azure []AccessGraphAzureSync `yaml:"azure,omitempty"`
	// PollInterval is the frequency at which to poll for AWS resources
	PollInterval time.Duration `yaml:"poll_interval,omitempty"`
}

// AccessGraphAWSSyncCloudTrailLogs represents the configuration for the SQS queue
// to poll for CloudTrail notifications.
type AccessGraphAWSSyncCloudTrailLogs struct {
	// QueueURL is the URL of the SQS queue to poll for AWS resources.
	QueueURL string `yaml:"queue_url,omitempty"`
	// QueueRegion is the AWS region of the SQS queue to poll for AWS resources.
	QueueRegion string `yaml:"queue_region,omitempty"`
}

// AccessGraphAWSSync represents the configuration for the AWS AccessGraph Sync service.
type AccessGraphAWSSync struct {
	// Regions are AWS regions to poll for resources.
	Regions []string `yaml:"regions,omitempty"`
	// AssumeRoleARN is the AWS role to assume for database discovery.
	AssumeRoleARN string `yaml:"assume_role_arn,omitempty"`
	// ExternalID is the AWS external ID to use when assuming a role for
	// database discovery in an external AWS account.
	ExternalID string `yaml:"external_id,omitempty"`
	// CloudTrailLogs is the configuration for the SQS queue to poll for
	// CloudTrail logs.
	CloudTrailLogs *AccessGraphAWSSyncCloudTrailLogs `yaml:"cloud_trail_logs,omitempty"`
}

// AccessGraphAzureSync represents the configuration for the Azure AccessGraph Sync service.
type AccessGraphAzureSync struct {
	// SubscriptionID is the Azure subscription ID configured for syncing
	SubscriptionID string `yaml:"subscription_id,omitempty"`
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

// Parse returns a parsed PAM config.
func (p *PAM) Parse() *servicecfg.PAMConfig {
	serviceName := p.ServiceName
	if serviceName == "" {
		serviceName = defaults.PAMServiceName
	}
	enabled, _ := apiutils.ParseBool(p.Enabled)
	return &servicecfg.PAMConfig{
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

	// RootPath root directory for the Teleport cgroups.
	// Optional, defaults to /teleport
	RootPath string `yaml:"root_path"`
}

// Parse will parse the enhanced session recording configuration.
func (b *BPF) Parse() *servicecfg.BPFConfig {
	enabled, _ := apiutils.ParseBool(b.Enabled)
	return &servicecfg.BPFConfig{
		Enabled:           enabled,
		CommandBufferSize: b.CommandBufferSize,
		DiskBufferSize:    b.DiskBufferSize,
		NetworkBufferSize: b.NetworkBufferSize,
		CgroupPath:        b.CgroupPath,
		RootPath:          b.RootPath,
	}
}

// RestrictedSession is a configuration for limiting access to kernel objects
type RestrictedSession struct {
	// Enabled enables or disables enforcement for this node.
	Enabled string `yaml:"enabled"`

	// EventsBufferSize is the size in bytes of the channel to report events
	// from the kernel to us.
	EventsBufferSize *int `yaml:"events_buffer_size,omitempty"`
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
	// AWSMatchers match AWS-hosted databases.
	AWSMatchers []AWSMatcher `yaml:"aws,omitempty"`
	// AzureMatchers match Azure hosted databases.
	AzureMatchers []AzureMatcher `yaml:"azure,omitempty"`
}

// ResourceMatcher matches cluster resources.
type ResourceMatcher struct {
	// Labels match resource labels.
	Labels map[string]apiutils.Strings `yaml:"labels,omitempty"`
	// AWS contains AWS specific settings.
	AWS ResourceMatcherAWS `yaml:"aws,omitempty"`
}

// ResourceMatcherAWS contains AWS specific settings for resource matcher.
type ResourceMatcherAWS struct {
	// AssumeRoleARN is the AWS role to assume to before accessing the
	// database.
	AssumeRoleARN string `yaml:"assume_role_arn,omitempty"`
	// ExternalID is an optional AWS external ID used to enable assuming an AWS
	// role across accounts.
	ExternalID string `yaml:"external_id,omitempty"`
}

// AWSMatcher matches AWS EC2 instances and AWS Databases
type AWSMatcher struct {
	// Types are AWS database types to match, "ec2", "rds", "redshift", "elasticache",
	// or "memorydb".
	Types []string `yaml:"types,omitempty"`
	// Regions are AWS regions to query for databases.
	Regions []string `yaml:"regions,omitempty"`
	// AssumeRoleARN is the AWS role to assume for database discovery.
	AssumeRoleARN string `yaml:"assume_role_arn,omitempty"`
	// ExternalID is the AWS external ID to use when assuming a role for
	// database discovery in an external AWS account.
	ExternalID string `yaml:"external_id,omitempty"`
	// Tags are AWS tags to match.
	Tags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered EC2 nodes
	InstallParams *InstallParams `yaml:"install,omitempty"`
	// SSM provides options to use when sending a document command to
	// an EC2 node
	SSM AWSSSM `yaml:"ssm,omitempty"`
	// Integration is the integration name used to generate credentials to interact with AWS APIs.
	// Environment credentials will not be used when this value is set.
	Integration string `yaml:"integration"`
	// KubeAppDiscovery controls whether Kubernetes App Discovery will be enabled for agents running on
	// discovered clusters, currently only affects AWS EKS discovery in integration mode.
	KubeAppDiscovery bool `yaml:"kube_app_discovery"`
	// SetupAccessForARN is the role that the discovery service should create EKS Access Entries for.
	SetupAccessForARN string `yaml:"setup_access_for_arn"`
}

// InstallParams sets join method to use on discovered nodes
type InstallParams struct {
	// JoinParams sets the token and method to use when generating
	// config on cloud instances
	JoinParams JoinParams `yaml:"join_params,omitempty"`
	// ScriptName is the name of the teleport installer script
	// resource for the cloud instance to execute
	ScriptName string `yaml:"script_name,omitempty"`
	// InstallTeleport disables agentless discovery
	InstallTeleport string `yaml:"install_teleport,omitempty"`
	// SSHDConfig provides the path to write sshd configuration changes
	SSHDConfig string `yaml:"sshd_config,omitempty"`
	// PublicProxyAddr is the address of the proxy the discovered node should use
	// to connect to the cluster.
	PublicProxyAddr string `yaml:"public_proxy_addr,omitempty"`
	// Azure is te set of installation parameters specific to Azure.
	Azure *AzureInstallParams `yaml:"azure,omitempty"`
	// EnrollMode indicates the mode used to enroll the node into Teleport.
	// Valid values: script, eice.
	// Optional.
	EnrollMode string `yaml:"enroll_mode"`
}

const (
	installEnrollModeEICE   = "eice"
	installEnrollModeScript = "script"
)

var validInstallEnrollModes = []string{installEnrollModeEICE, installEnrollModeScript}

func (ip *InstallParams) parse() (*types.InstallerParams, error) {
	install := &types.InstallerParams{
		JoinMethod:      ip.JoinParams.Method,
		JoinToken:       ip.JoinParams.TokenName,
		ScriptName:      ip.ScriptName,
		InstallTeleport: true,
		SSHDConfig:      ip.SSHDConfig,
		EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_UNSPECIFIED,
	}

	switch ip.EnrollMode {
	case installEnrollModeEICE:
		install.EnrollMode = types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE
	case installEnrollModeScript:
		install.EnrollMode = types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT
	case "":
		install.EnrollMode = types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_UNSPECIFIED
	default:
		return nil, trace.BadParameter("enroll mode %q is invalid, valid values: %v", ip.EnrollMode, validInstallEnrollModes)
	}

	if ip.InstallTeleport == "" {
		return install, nil
	}

	var err error
	install.InstallTeleport, err = apiutils.ParseBool(ip.InstallTeleport)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return install, nil
}

// AWSSSM provides options to use when executing SSM documents
type AWSSSM struct {
	// DocumentName is the name of the document to use when executing an
	// SSM command
	DocumentName string `yaml:"document_name,omitempty"`
}

// Azure is te set of installation parameters specific to Azure.
type AzureInstallParams struct {
	// ClientID is the client ID of the managed identity to use for installation.
	ClientID string `yaml:"client_id"`
}

// AzureMatcher matches Azure resources.
type AzureMatcher struct {
	// Subscriptions are Azure subscriptions to query for resources.
	Subscriptions []string `yaml:"subscriptions,omitempty"`
	// ResourceGroups are Azure resource groups to query for resources.
	ResourceGroups []string `yaml:"resource_groups,omitempty"`
	// Types are Azure types to match: "mysql", "postgres", "aks", "vm"
	Types []string `yaml:"types,omitempty"`
	// Regions are Azure locations to match for databases.
	Regions []string `yaml:"regions,omitempty"`
	// ResourceTags are Azure tags on resources to match.
	ResourceTags map[string]apiutils.Strings `yaml:"tags,omitempty"`
	// InstallParams sets the join method when installing on
	// discovered Azure nodes.
	InstallParams *InstallParams `yaml:"install,omitempty"`
}

// KubernetesMatcher matches Kubernetes resources.
type KubernetesMatcher struct {
	// Types are Kubernetes services types to match. Currently only 'app' is supported.
	Types []string `yaml:"types,omitempty"`
	// Namespaces are Kubernetes namespaces in which to discover services
	Namespaces []string `yaml:"namespaces,omitempty"`
	// Labels are Kubernetes services labels to match.
	Labels map[string]apiutils.Strings `yaml:"labels,omitempty"`
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
	// AWS contains AWS specific settings for AWS-hosted databases.
	AWS DatabaseAWS `yaml:"aws"`
	// GCP contains GCP specific settings for Cloud SQL databases.
	GCP DatabaseGCP `yaml:"gcp"`
	// AD contains Active Directory database configuration.
	AD DatabaseAD `yaml:"ad"`
	// Azure contains Azure database configuration.
	Azure DatabaseAzure `yaml:"azure"`
	// AdminUser describes database privileged user for auto-provisioning.
	AdminUser DatabaseAdminUser `yaml:"admin_user"`
	// Oracle is Database Oracle settings
	Oracle DatabaseOracle `yaml:"oracle,omitempty"`
}

// DatabaseAdminUser describes database privileged user for auto-provisioning.
type DatabaseAdminUser struct {
	// Name is the database admin username (e.g. "postgres").
	Name string `yaml:"name"`
	// DefaultDatabase is the database that the admin user logs into by
	// default.
	//
	// Depending on the database type, this database may be used to store
	// procedures or data for managing database users.
	DefaultDatabase string `yaml:"default_database"`
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
	// LDAPCert is a certificate from Windows LDAP/AD, optional; only for x509 Authentication.
	LDAPCert string `yaml:"ldap_cert,omitempty"`
	// KDCHostName is the host name for a KDC for x509 Authentication.
	KDCHostName string `yaml:"kdc_host_name,omitempty"`
	// LDAPServiceAccountName is the name of service account for performing LDAP queries. Required for x509 Auth / PKINIT.
	LDAPServiceAccountName string `yaml:"ldap_service_account_name,omitempty"`
	// LDAPServiceAccountSID is the SID of service account for performing LDAP queries. Required for x509 Auth / PKINIT.
	LDAPServiceAccountSID string `yaml:"ldap_service_account_sid,omitempty"`
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
	// TrustSystemCertPool allows Teleport to trust certificate authorities
	// available on the host system.
	TrustSystemCertPool bool `yaml:"trust_system_cert_pool,omitempty"`
}

// DatabaseMySQL are an additional MySQL database options.
type DatabaseMySQL struct {
	// ServerVersion is the MySQL version reported by DB proxy instead of default Teleport string.
	ServerVersion string `yaml:"server_version,omitempty"`
}

// DatabaseOracle are an additional Oracle database options.
type DatabaseOracle struct {
	// AuditUser is the Oracle database user privilege to access internal Oracle audit trail.
	AuditUser string `yaml:"audit_user,omitempty"`
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
	// AccountID is the AWS account ID.
	AccountID string `yaml:"account_id,omitempty"`
	// AssumeRoleARN is the AWS role to assume to before accessing the database.
	AssumeRoleARN string `yaml:"assume_role_arn,omitempty"`
	// ExternalID is an optional AWS external ID used to enable assuming an AWS role across accounts.
	ExternalID string `yaml:"external_id,omitempty"`
	// RedshiftServerless contains RedshiftServerless specific settings.
	RedshiftServerless DatabaseAWSRedshiftServerless `yaml:"redshift_serverless"`
	// SessionTags is a list of AWS STS session tags.
	SessionTags map[string]string `yaml:"session_tags,omitempty"`
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

// DatabaseAWSRedshiftServerless contains AWS Redshift Serverless specific settings.
type DatabaseAWSRedshiftServerless struct {
	// WorkgroupName is the Redshift Serverless workgroup name.
	WorkgroupName string `yaml:"workgroup_name,omitempty"`
	// EndpointName is the Redshift Serverless VPC endpoint name.
	EndpointName string `yaml:"endpoint_name,omitempty"`
}

// DatabaseGCP contains GCP specific settings for Cloud SQL databases.
type DatabaseGCP struct {
	// ProjectID is the GCP project ID where the database is deployed.
	ProjectID string `yaml:"project_id,omitempty"`
	// InstanceID is the Cloud SQL database instance ID.
	InstanceID string `yaml:"instance_id,omitempty"`
}

// DatabaseAzure contains Azure database configuration.
type DatabaseAzure struct {
	// ResourceID is the Azure fully qualified ID for the resource.
	ResourceID string `yaml:"resource_id,omitempty"`
	// IsFlexiServer is true if the database is an Azure Flexible server.
	IsFlexiServer bool `yaml:"is_flexi_server,omitempty"`
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

	// AWS contains additional options for AWS applications.
	AWS *AppAWS `yaml:"aws,omitempty"`

	// Cloud identifies the cloud instance the app represents.
	Cloud string `yaml:"cloud,omitempty"`

	// RequiredApps is a list of app names that are required for this app to function. Any app listed here will
	// be part of the authentication redirect flow and authenticate along side this app.
	RequiredApps []string `yaml:"required_apps,omitempty"`

	// UseAnyProxyPublicAddr will rebuild this app's fqdn based on the proxy public addr that the
	// request originated from. This should be true if your proxy has multiple proxy public addrs and you
	// want the app to be accessible from any of them. If `public_addr` is explicitly set in the app spec,
	// setting this value to true will overwrite that public address in the web UI.
	UseAnyProxyPublicAddr bool `yaml:"use_any_proxy_public_addr"`

	// CORS defines the Cross-Origin Resource Sharing configuration for the app,
	// controlling how resources are shared across different origins.
	CORS *CORS `yaml:"cors,omitempty"`

	// TCPPorts is a list of ports and port ranges that an app agent can forward connections to.
	// Only applicable to TCP App Access.
	// If this field is not empty, URI is expected to contain no port number and start with the tcp
	// protocol.
	TCPPorts []PortRange `yaml:"tcp_ports,omitempty"`

	// MCP contains MCP server-related configurations.
	MCP *MCP `yaml:"mcp,omitempty"`
}

// CORS represents the configuration for Cross-Origin Resource Sharing (CORS)
// settings that control how the app responds to requests from different origins.
type CORS struct {
	// AllowedOrigins specifies the list of origins that are allowed to access the app.
	// Example: "https://client.teleport.example.com:3080"
	AllowedOrigins []string `yaml:"allowed_origins"`

	// AllowedMethods specifies the HTTP methods that are allowed when accessing the app.
	// Example: "POST", "GET", "OPTIONS", "PUT", "DELETE"
	AllowedMethods []string `yaml:"allowed_methods"`

	// AllowedHeaders specifies the HTTP headers that can be used when making requests to the app.
	// Example: "Content-Type", "Authorization", "X-Custom-Header"
	AllowedHeaders []string `yaml:"allowed_headers"`

	// ExposedHeaders indicate which response headers should be made available to scripts running in
	// the browser, in response to a cross-origin request.
	ExposedHeaders []string `yaml:"exposed_headers"`

	// AllowCredentials indicates whether credentials such as cookies or authorization headers
	// are allowed to be included in the requests.
	AllowCredentials bool `yaml:"allow_credentials"`

	// MaxAge specifies how long (in seconds) the results of a preflight request can be cached.
	// Example: 86400 (which equals 24 hours)
	MaxAge uint `yaml:"max_age"`
}

// Rewrite is a list of rewriting rules to apply to requests and responses.
type Rewrite struct {
	// Redirect is a list of hosts that should be rewritten to the public address.
	Redirect []string `yaml:"redirect"`
	// Headers is a list of extra headers to inject in the request.
	Headers []string `yaml:"headers,omitempty"`
	// JWTClaims configures whether roles/traits are included in the JWT token
	JWTClaims string `yaml:"jwt_claims,omitempty"`
}

// AppAWS contains additional options for AWS applications.
type AppAWS struct {
	// ExternalID is the AWS External ID used when assuming roles in this app.
	ExternalID string `yaml:"external_id,omitempty"`
}

// PortRange describes a port range for TCP apps. The range starts with Port and ends with EndPort.
// PortRange can be used to describe a single port in which case the Port field is the port and the
// EndPort field is 0.
type PortRange struct {
	// Port describes the start of the range. It must be between 1 and 65535.
	Port int `yaml:"port"`
	// EndPort describes the end of the range, inclusive. When describing a port range, it must be
	// greater than Port and less than or equal to 65535. When describing a single port, it must be
	// set to 0.
	EndPort int `yaml:"end_port,omitempty"`
}

// MCP contains MCP server-related configurations.
type MCP struct {
	// Command to launch stdio-based MCP servers.
	Command string `yaml:"command,omitempty"`
	// Args to execute with the command.
	Args []string `yaml:"args,omitempty"`
	// RunAsHostUser is the host user account under which the command will be
	// executed. Required for stdio-based MCP servers.
	RunAsHostUser string `yaml:"run_as_host_user,omitempty"`
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
	// PeerPublicAddr is the hostport the proxy advertises for peer proxy
	// client connections.
	PeerPublicAddr string `yaml:"peer_public_addr,omitempty"`
	// KeyFile is a TLS key file
	KeyFile string `yaml:"https_key_file,omitempty"`
	// CertFile is a TLS Certificate file
	CertFile string `yaml:"https_cert_file,omitempty"`
	// ProxyProtocol turns on support for HAProxy PROXY protocol
	// this is the option that has be turned on only by administrator,
	// as only admin knows whether service is in front of trusted load balancer
	// or not.
	ProxyProtocol string `yaml:"proxy_protocol,omitempty"`
	// ProxyProtocolAllowDowngrade controls support for downgrading IPv6 source addresses in PROXY headers to pseudo IPv4
	// addresses when connecting to an IPv4 destination
	ProxyProtocolAllowDowngrade *types.BoolOption `yaml:"proxy_protocol_allow_downgrade,omitempty"`
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

	// KeyPairsReloadInterval is the interval between attempts to reload
	// x509 key pairs. If set to 0, then periodic reloading is disabled.
	KeyPairsReloadInterval time.Duration `yaml:"https_keypairs_reload_interval"`

	// ACME configures ACME protocol support
	ACME ACME `yaml:"acme"`

	// MySQLAddr is MySQL proxy listen address.
	MySQLAddr string `yaml:"mysql_listen_addr,omitempty"`
	// MySQLPublicAddr is the hostport the proxy advertises for MySQL
	// client connections.
	MySQLPublicAddr apiutils.Strings `yaml:"mysql_public_addr,omitempty"`

	// MySQLServerVersion allow to overwrite proxy default mysql engine version reported by Teleport proxy.
	MySQLServerVersion string `yaml:"mysql_server_version,omitempty"`

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

	// IdP is configuration for identity providers.
	//
	//nolint:revive // Because we want this to be IdP.
	IdP IdP `yaml:"idp,omitempty"`

	// UI provides config options for the web UI
	UI *UIConfig `yaml:"ui,omitempty"`

	// TrustXForwardedFor enables the service to take client source IPs from
	// the "X-Forwarded-For" headers for web APIs received from layer 7 load
	// balancers or reverse proxies.
	TrustXForwardedFor types.Bool `yaml:"trust_x_forwarded_for,omitempty"`

	// AutomaticUpgradesChannels is a map of all version channels used by the
	// proxy built-in version server to retrieve target versions. This is part
	// of the automatic upgrades.
	AutomaticUpgradesChannels automaticupgrades.Channels `yaml:"automatic_upgrades_channels,omitempty"`
}

// UIConfig provides config options for the web UI served by the proxy service.
type UIConfig struct {
	// ScrollbackLines is the max number of lines the UI terminal can display in its history
	ScrollbackLines int `yaml:"scrollback_lines,omitempty"`
	// ShowResources determines which resources are shown in the web UI. Default if unset is "requestable"
	// which means resources the user has access to and resources they can request will be shown in the
	// resources UI. If set to `accessible_only`, only resources the user already has access to will be shown.
	ShowResources constants.ShowResources `yaml:"show_resources,omitempty"`
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
func (a ACME) Parse() (*servicecfg.ACME, error) {
	// ACME is disabled by default
	out := servicecfg.ACME{}
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

// IdP represents the configuration for identity providers running within the
// proxy.
//
//nolint:revive // Because we want this to be IdP.
type IdP struct {
	// SAMLIdP represents configuratino options for the SAML identity provider.
	SAMLIdP SAMLIdP `yaml:"saml,omitempty"`
}

// SAMLIdP represents the configuration for the SAML identity provider.
type SAMLIdP struct {
	// Enabled turns the SAML IdP on or off for this process.
	EnabledFlag string `yaml:"enabled,omitempty"`

	// BaseURL is the base URL to provide to the SAML IdP.
	BaseURL string `yaml:"base_url,omitempty"`
}

// Enabled returns true if the SAML IdP is enabled or if the enabled flag is unset.
func (s *SAMLIdP) Enabled() bool {
	if s.EnabledFlag == "" {
		return true
	}
	v, err := apiutils.ParseBool(s.EnabledFlag)
	if err != nil {
		return false
	}
	return v
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
	// StaticLabels are the static labels for RBAC on kubernetes clusters.
	StaticLabels map[string]string `yaml:"labels,omitempty"`
	// DynamicLabels are the dynamic labels for RBAC on kubernetes clusters.
	DynamicLabels []CommandLabel `yaml:"commands,omitempty"`
	// ResourceMatchers match cluster kube_cluster resources.
	ResourceMatchers []ResourceMatcher `yaml:"resources,omitempty"`
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

	// GRPCServerLatency enables histogram metrics for each gRPC endpoint on the auth server
	GRPCServerLatency bool `yaml:"grpc_server_latency,omitempty"`

	// GRPCServerLatency enables histogram metrics for each gRPC endpoint on the auth server
	GRPCClientLatency bool `yaml:"grpc_client_latency,omitempty"`
}

// MTLSEnabled returns whether mtls is enabled or not in the metrics service config.
func (m *Metrics) MTLSEnabled() bool {
	return len(m.KeyPairs) > 0 && len(m.CACerts) > 0
}

// DebugService is a `debug_service` section of the config file.
type DebugService struct {
	// Service is a generic service configuration section
	Service `yaml:",inline"`
}

// WindowsDesktopService contains configuration for windows_desktop_service.
type WindowsDesktopService struct {
	Service `yaml:",inline"`
	// Labels are the configured windows deesktops service labels.
	Labels map[string]string `yaml:"labels,omitempty"`
	// PublicAddr is a list of advertised public addresses of this service.
	PublicAddr apiutils.Strings `yaml:"public_addr,omitempty"`
	// ShowDesktopWallpaper determines whether desktop sessions will show a
	// user-selected wallpaper vs a system-default, single-color wallpaper.
	ShowDesktopWallpaper bool `yaml:"show_desktop_wallpaper,omitempty"`
	// LDAP is the LDAP connection parameters.
	LDAP LDAPConfig `yaml:"ldap"`
	// PKIDomain optionally configures a separate Active Directory domain
	// for PKI operations. If empty, the domain from the LDAP config is used.
	// This can be useful for cases where PKI is configured in a root domain
	// but Teleport is used to provide access to users and computers in a child
	// domain.
	PKIDomain string `yaml:"pki_domain"`
	// KDCAddress optionally configures the address of the Kerberos Key Distribution Center,
	// which is used to support RDP Network Level Authentication (NLA).
	// If empty, the LDAP address will be used instead.
	// Note: NLA is only supported in Active Directory environments - this field has
	// no effect when connecting to desktops as local Windows users.
	KDCAddress string `yaml:"kdc_address"`
	// Discovery configures desktop discovery via LDAP.
	// New usages should prever DiscoveryConfigs instead, which allows for multiple searches.
	Discovery LDAPDiscoveryConfig `yaml:"discovery,omitempty"`
	// DiscoveryConfigs configures desktop discovery via LDAP.
	DiscoveryConfigs []LDAPDiscoveryConfig `yaml:"discovery_configs,omitempty"`
	// DiscoveryInterval controls how frequently the discovery process runs.
	DiscoveryInterval time.Duration `yaml:"discovery_interval"`
	// ADHosts is a list of static, AD-connected Windows hosts. This gives users
	// a way to specify AD-connected hosts that won't be found by the filters
	// specified in `discovery` (or if `discovery` is omitted).
	//
	// Deprecated: prefer StaticHosts instead.
	ADHosts []string `yaml:"hosts,omitempty"`
	// NonADHosts is a list of standalone Windows hosts that are not
	// jointed to an Active Directory domain.
	//
	// Deprecated: prefer StaticHosts instead.
	NonADHosts []string `yaml:"non_ad_hosts,omitempty"`
	// StaticHosts is a list of Windows hosts (both AD-connected and standalone).
	// User can specify name for each host and labels specific to it.
	StaticHosts []WindowsHost `yaml:"static_hosts,omitempty"`
	// HostLabels optionally applies labels to Windows hosts for RBAC.
	// A host can match multiple rules and will get a union of all
	// the matched labels.
	HostLabels []WindowsHostLabelRule `yaml:"host_labels,omitempty"`
	// ResourceMatchers match dynamic Windows desktop resources.
	ResourceMatchers []ResourceMatcher `yaml:"resources,omitempty"`
}

// Check checks whether the WindowsDesktopService is valid or not
func (wds *WindowsDesktopService) Check() error {
	hasAD := len(wds.ADHosts) > 0 || slices.ContainsFunc(wds.StaticHosts, func(host WindowsHost) bool {
		return host.AD
	})

	if hasAD && wds.LDAP.Addr == "" {
		return trace.BadParameter("if Active Directory hosts are specified in the windows_desktop_service, " +
			"the ldap configuration must also be specified")
	}

	if wds.Discovery.BaseDN != "" && wds.LDAP.Addr == "" {
		return trace.BadParameter("if discovery is specified in the windows_desktop_service, " +
			"ldap configuration must also be specified")
	}

	return nil
}

// WindowsHostLabelRule describes how a set of labels should be applied to
// a Windows host.
type WindowsHostLabelRule struct {
	// Match is a regexp that is checked against the Windows host's DNS name.
	// If the regexp matches, this rule's labels will be applied to the host.
	Match string `yaml:"match"`
	// Labels is the set of labels to apply to hosts that match this rule.
	Labels map[string]string `yaml:"labels"`
}

// WindowsHost describes single host in configuration
type WindowsHost struct {
	// Name of the host
	Name string `yaml:"name"`
	// Address of the host, with an optional port.
	// 10.1.103.4 or 10.1.103.4:3389, for example.
	Address string `yaml:"addr"`
	// Labels is the set of labels to apply to this host
	Labels map[string]string `yaml:"labels"`
	// AD tells if host is part of Active Directory domain
	AD bool `yaml:"ad"`
}

// LDAPConfig is the LDAP connection parameters.
type LDAPConfig struct {
	// Addr is the host:port of the LDAP server (typically port 389).
	Addr string `yaml:"addr"`
	// Domain is the ActiveDirectory domain name.
	Domain string `yaml:"domain"`
	// Username for LDAP authentication.
	Username string `yaml:"username"`
	// SID is the Security Identifier for the service account specified by Username.
	SID string `yaml:"sid"`
	// InsecureSkipVerify decides whether whether we skip verifying with the LDAP server's CA when making the LDAPS connection.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
	// ServerName is the name of the LDAP server for TLS.
	ServerName string `yaml:"server_name,omitempty"`
	// DEREncodedCAFile is the filepath to an optional DER encoded CA cert to be used for verification (if InsecureSkipVerify is set to false).
	DEREncodedCAFile string `yaml:"der_ca_file,omitempty"`
	// PEMEncodedCACert is an optional PEM encoded CA cert to be used for verification (if InsecureSkipVerify is set to false).
	PEMEncodedCACert string `yaml:"ldap_ca_cert,omitempty"`
}

// LDAPDiscoveryConfig is LDAP discovery configuration for windows desktop discovery service.
type LDAPDiscoveryConfig struct {
	// BaseDN is the base DN to search for desktops.
	// Use the value '*' to search from the root of the domain,
	// or leave blank to disable desktop discovery.
	BaseDN string `yaml:"base_dn"`
	// Filters are additional LDAP filters to apply to the search.
	// See: https://ldap.com/ldap-filters/
	Filters []string `yaml:"filters"`
	// LabelAttributes are LDAP attributes to apply to hosts discovered
	// via LDAP. Teleport labels hosts by prefixing the attribute with
	// "ldap/" - for example, a value of "location" here would result in
	// discovered desktops having a label with key "ldap/location" and
	// the value being the value of the "location" attribute.
	LabelAttributes []string `yaml:"label_attributes"`
	// RDPPort is the port to use for RDP for hosts discovered with this configuration.
	// Optional, defaults to 3389 if unspecified.
	RDPPort int `yaml:"rdp_port"`
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

// Okta represents an okta_service section in the config file.
type Okta struct {
	Service `yaml:",inline"`

	// APIEndpoint is the Okta API endpoint to use.
	APIEndpoint string `yaml:"api_endpoint,omitempty"`

	// APITokenPath is the path to the Okta API token.
	APITokenPath string `yaml:"api_token_path,omitempty"`

	// SyncPeriod is the duration between synchronization calls for synchronizing Okta applications and groups..
	// Deprecated. Please use sync.app_group_sync_period instead.
	SyncPeriod time.Duration `yaml:"sync_period,omitempty"`

	// Import is the import settings for the Okta service.
	Sync OktaSync `yaml:"sync,omitempty"`
}

// OktaSync represents the import subsection of the okta_service section in the config file.
type OktaSync struct {
	// AppGroupSyncPeriod is the duration between synchronization calls for synchronizing Okta applications and groups.
	AppGroupSyncPeriod time.Duration `yaml:"app_group_sync_period,omitempty"`

	// SyncAccessLists will enable or disable the Okta importing of access lists. Defaults to false.
	SyncAccessListsFlag string `yaml:"sync_access_lists,omitempty"`

	// DefaultOwners are the default owners for all imported access lists.
	DefaultOwners []string `yaml:"default_owners,omitempty"`

	// GroupFilters are filters for which Okta groups to synchronize as access lists.
	// These are globs/regexes.
	GroupFilters []string `yaml:"group_filters,omitempty"`

	// AppFilters are filters for which Okta applications to synchronize as access lists.
	// These are globs/regexes.
	AppFilters []string `yaml:"app_filters,omitempty"`
}

func (o *OktaSync) SyncAccessLists() bool {
	if o.SyncAccessListsFlag == "" {
		return false
	}
	enabled, _ := apiutils.ParseBool(o.SyncAccessListsFlag)
	return enabled
}

func (o *OktaSync) Parse() (*servicecfg.OktaSyncSettings, error) {
	enabled := o.SyncAccessLists()
	if enabled && len(o.DefaultOwners) == 0 {
		return nil, trace.BadParameter("default owners must be set when access list import is enabled")
	}

	for _, filter := range o.GroupFilters {
		_, err := utils.CompileExpression(filter)
		if err != nil {
			return nil, trace.Wrap(err, "error parsing group filter: %s", filter)
		}
	}

	for _, filter := range o.AppFilters {
		_, err := utils.CompileExpression(filter)
		if err != nil {
			return nil, trace.Wrap(err, "error parsing app filter: %s", filter)
		}
	}

	return &servicecfg.OktaSyncSettings{
		AppGroupSyncPeriod: o.AppGroupSyncPeriod,
		SyncAccessLists:    o.SyncAccessLists(),
		DefaultOwners:      o.DefaultOwners,
		GroupFilters:       o.GroupFilters,
		AppFilters:         o.AppFilters,
	}, nil
}

// JamfService is the yaml representation of jamf_service.
// Corresponds to [types.JamfSpecV1].
type JamfService struct {
	Service `yaml:",inline"`
	// Name is the name of the sync device source.
	Name string `yaml:"name,omitempty"`
	// SyncDelay is the initial sync delay.
	// Zero means "server default", negative means "immediate".
	SyncDelay time.Duration `yaml:"sync_delay,omitempty"`
	// ExitOnSync tells the service to exit immediately after the first sync.
	ExitOnSync bool `yaml:"exit_on_sync,omitempty"`
	// APIEndpoint is the Jamf Pro API endpoint.
	// Example: "https://yourtenant.jamfcloud.com/api".
	APIEndpoint string `yaml:"api_endpoint,omitempty"`
	// Username is the Jamf Pro API username.
	Username string `yaml:"username,omitempty"`
	// PasswordFile is a file containing the Jamf Pro API password.
	// A single trailing newline is trimmed, anything else is taken literally.
	PasswordFile string `yaml:"password_file,omitempty"`
	// ClientID is the Jamf API Client ID.
	ClientID string `yaml:"client_id,omitempty"`
	// ClientSecretFile is a file containing the Jamf API client secret.
	// A single trailing newline is trimmed, anything else is taken literally.
	ClientSecretFile string `yaml:"client_secret_file,omitempty"`
	// Inventory are the entries for inventory sync.
	Inventory []*JamfInventoryEntry `yaml:"inventory,omitempty"`
}

// JamfInventoryEntry is the yaml representation of a jamf_service.inventory
// entry.
// Corresponds to [types.JamfInventoryEntry].
type JamfInventoryEntry struct {
	// FilterRSQL is a Jamf Pro API RSQL filter string.
	FilterRSQL string `yaml:"filter_rsql,omitempty"`
	// SyncPeriodPartial is the period for PARTIAL syncs.
	// Zero means "server default", negative means "disabled".
	SyncPeriodPartial time.Duration `yaml:"sync_period_partial,omitempty"`
	// SyncPeriodFull is the period for FULL syncs.
	// Zero means "server default", negative means "disabled".
	SyncPeriodFull time.Duration `yaml:"sync_period_full,omitempty"`
	// OnMissing is the trigger for devices missing from the MDM inventory view.
	// See [types.JamfInventoryEntry.OnMissing].
	OnMissing string `yaml:"on_missing,omitempty"`
	// Custom page size for inventory queries.
	// A server default is used if zeroed or negative.
	PageSize int32 `yaml:"page_size,omitempty"`
}

func (j *JamfService) toJamfSpecV1() (*types.JamfSpecV1, error) {
	switch {
	case j == nil:
		return nil, trace.BadParameter("jamf_service is nil")
	case j.ListenAddress != "":
		return nil, trace.BadParameter("jamf listen_addr not supported")
	}

	// Assemble spec.
	inventory := make([]*types.JamfInventoryEntry, len(j.Inventory))
	for i, e := range j.Inventory {
		inventory[i] = &types.JamfInventoryEntry{
			FilterRsql:        e.FilterRSQL,
			SyncPeriodPartial: types.Duration(e.SyncPeriodPartial),
			SyncPeriodFull:    types.Duration(e.SyncPeriodFull),
			OnMissing:         e.OnMissing,
			PageSize:          e.PageSize,
		}
	}
	spec := &types.JamfSpecV1{
		Enabled:     j.Enabled(),
		Name:        j.Name,
		SyncDelay:   types.Duration(j.SyncDelay),
		ApiEndpoint: j.APIEndpoint,
		Inventory:   inventory,
	}

	// Validate.
	if err := types.ValidateJamfSpecV1(spec); err != nil {
		return nil, trace.BadParameter("jamf_service %v", err)
	}

	return spec, nil
}

func (j *JamfService) readJamfCredentials() (*servicecfg.JamfCredentials, error) {
	password, err := readJamfPasswordFile(j.PasswordFile, "password_file")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientSecret, err := readJamfPasswordFile(j.ClientSecretFile, "client_secret_file")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds := &servicecfg.JamfCredentials{
		Username:     j.Username,
		Password:     password,
		ClientID:     j.ClientID,
		ClientSecret: clientSecret,
	}

	// Validate.
	if err := servicecfg.ValidateJamfCredentials(creds); err != nil {
		return nil, trace.BadParameter("jamf_service %v", err)
	}

	return creds, nil
}

func readJamfPasswordFile(path, key string) (string, error) {
	if path == "" {
		return "", nil
	}

	pwdBytes, err := os.ReadFile(path)
	if err != nil {
		return "", trace.BadParameter("jamf %v: %v", key, err)
	}
	pwd := string(pwdBytes)
	if pwd == "" {
		return "", trace.BadParameter("jamf %v is empty", key)
	}
	// Trim exactly one trailing \n, if present.
	if l := len(pwd); pwd[l-1] == '\n' {
		pwd = pwd[:l-1]
	}

	return pwd, nil
}
