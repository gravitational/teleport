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

// Package config provides facilities for configuring Teleport daemons
// including
//	- parsing YAML configuration
//	- parsing CLI flags
package config

import (
	"bufio"
	"crypto/x509"
	"io"
	stdlog "log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/go-ldap/ldap/v3"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/backend/postgres"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/sirupsen/logrus"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// CommandLineFlags stores command line flag values, it's a much simplified subset
// of Teleport configuration (which is fully expressed via YAML config file)
type CommandLineFlags struct {
	// --name flag
	NodeName string
	// --auth-server flag
	AuthServerAddr []string
	// --token flag
	AuthToken string
	// CAPins are the SKPI hashes of the CAs used to verify the Auth Server.
	CAPins []string
	// --listen-ip flag
	ListenIP net.IP
	// --advertise-ip flag
	AdvertiseIP string
	// --config flag
	ConfigFile string
	// Bootstrap flag contains a YAML file that defines a set of resources to bootstrap
	// a cluster.
	BootstrapFile string
	// ConfigString is a base64 encoded configuration string
	// set by --config-string or TELEPORT_CONFIG environment variable
	ConfigString string
	// --roles flag
	Roles string
	// -d flag
	Debug bool

	// --insecure-no-tls flag
	DisableTLS bool

	// --labels flag
	Labels string
	// --pid-file flag
	PIDFile string
	// DiagnosticAddr is listen address for diagnostic endpoint
	DiagnosticAddr string
	// PermitUserEnvironment enables reading of ~/.tsh/environment
	// when creating a new session.
	PermitUserEnvironment bool

	// Insecure mode is controlled by --insecure flag and in this mode
	// Teleport won't check certificates when connecting to trusted clusters
	// It's useful for learning Teleport (following quick starts, etc).
	InsecureMode bool

	// FIPS mode means Teleport starts in a FedRAMP/FIPS 140-2 compliant
	// configuration.
	FIPS bool

	// SkipVersionCheck allows Teleport to connect to auth servers that
	// have an earlier major version number.
	SkipVersionCheck bool

	// AppName is the name of the application to proxy.
	AppName string

	// AppURI is the internal address of the application to proxy.
	AppURI string

	// AppPublicAddr is the public address of the application to proxy.
	AppPublicAddr string

	// DatabaseName is the name of the database to proxy.
	DatabaseName string
	// DatabaseDescription is a free-form database description.
	DatabaseDescription string
	// DatabaseProtocol is the type of the proxied database e.g. postgres or mysql.
	DatabaseProtocol string
	// DatabaseURI is the address to connect to the proxied database.
	DatabaseURI string
	// DatabaseCACertFile is the database CA cert path.
	DatabaseCACertFile string
	// DatabaseAWSRegion is an optional database cloud region e.g. when using AWS RDS.
	DatabaseAWSRegion string
	// DatabaseAWSRedshiftClusterID is Redshift cluster identifier.
	DatabaseAWSRedshiftClusterID string
	// DatabaseAWSRDSInstanceID is RDS instance identifier.
	DatabaseAWSRDSInstanceID string
	// DatabaseAWSRDSClusterID is RDS cluster (Aurora) cluster identifier.
	DatabaseAWSRDSClusterID string
	// DatabaseGCPProjectID is GCP Cloud SQL project identifier.
	DatabaseGCPProjectID string
	// DatabaseGCPInstanceID is GCP Cloud SQL instance identifier.
	DatabaseGCPInstanceID string
	// DatabaseADKeytabFile is the path to Kerberos keytab file.
	DatabaseADKeytabFile string
	// DatabaseADKrb5File is the path to krb5.conf file.
	DatabaseADKrb5File string
	// DatabaseADDomain is the Active Directory domain for authentication.
	DatabaseADDomain string
	// DatabaseADSPN is the database Service Principal Name.
	DatabaseADSPN string
	// DatabaseMySQLServerVersion is the MySQL server version reported to a client
	// if the value cannot be obtained from the database.
	DatabaseMySQLServerVersion string
}

// ReadConfigFile reads /etc/teleport.yaml (or whatever is passed via --config flag)
// and overrides values in 'cfg' structure
func ReadConfigFile(cliConfigPath string) (*FileConfig, error) {
	configFilePath := defaults.ConfigFilePath
	// --config tells us to use a specific conf. file:
	if cliConfigPath != "" {
		configFilePath = cliConfigPath
		if !utils.FileExists(configFilePath) {
			return nil, trace.NotFound("file %s is not found", configFilePath)
		}
	}
	// default config doesn't exist? quietly return:
	if !utils.FileExists(configFilePath) {
		log.Info("not using a config file")
		return nil, nil
	}
	log.Debug("reading config file: ", configFilePath)
	return ReadFromFile(configFilePath)
}

// ReadResources loads a set of resources from a file.
func ReadResources(filePath string) ([]types.Resource, error) {
	reader, err := utils.OpenFile(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	var resources []types.Resource
	for {
		var raw services.UnknownResource
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, trace.Wrap(err)
		}
		rsc, err := services.UnmarshalResource(raw.Kind, raw.Raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, rsc)
	}
	return resources, nil
}

// ApplyFileConfig applies configuration from a YAML file to Teleport
// runtime config
func ApplyFileConfig(fc *FileConfig, cfg *service.Config) error {
	var err error

	// no config file? no problem
	if fc == nil {
		return nil
	}
	// merge file-based config with defaults in 'cfg'
	if fc.Auth.Disabled() {
		cfg.Auth.Enabled = false
	}
	if fc.SSH.Disabled() {
		cfg.SSH.Enabled = false
	}
	if fc.Proxy.Disabled() {
		cfg.Proxy.Enabled = false
	}
	if fc.Kube.Enabled() {
		cfg.Kube.Enabled = true
	}
	if fc.Apps.Disabled() {
		cfg.Apps.Enabled = false
	}
	if fc.Databases.Disabled() {
		cfg.Databases.Enabled = false
	}
	if fc.Metrics.Disabled() {
		cfg.Metrics.Enabled = false
	}
	if fc.WindowsDesktop.Disabled() {
		cfg.WindowsDesktop.Enabled = false
	}
	applyString(fc.NodeName, &cfg.Hostname)

	// apply "advertise_ip" setting:
	advertiseIP := fc.AdvertiseIP
	if advertiseIP != "" {
		if _, _, err := utils.ParseAdvertiseAddr(advertiseIP); err != nil {
			return trace.Wrap(err)
		}
		cfg.AdvertiseIP = advertiseIP
	}
	cfg.PIDFile = fc.PIDFile

	// config file has auth servers in there?
	if len(fc.AuthServers) > 0 {
		cfg.AuthServers = make([]utils.NetAddr, 0, len(fc.AuthServers))
		for _, as := range fc.AuthServers {
			addr, err := utils.ParseHostPortAddr(as, defaults.AuthListenPort)
			if err != nil {
				return trace.Wrap(err)
			}

			if err != nil {
				return trace.Errorf("cannot parse auth server address: '%v'", as)
			}
			cfg.AuthServers = append(cfg.AuthServers, *addr)
		}
	}

	if err := applyTokenConfig(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	if fc.Global.DataDir != "" {
		cfg.DataDir = fc.Global.DataDir
		cfg.Auth.StorageConfig.Params["path"] = cfg.DataDir
	}

	// If a backend is specified, override the defaults.
	if fc.Storage.Type != "" {
		// If the alternative name "dir" is given, update it to "lite".
		if fc.Storage.Type == lite.AlternativeName {
			fc.Storage.Type = lite.GetName()
		}
		// If the alternative name "cockroachdb" is given, update it to "postgres".
		if fc.Storage.Type == postgres.AlternativeName {
			fc.Storage.Type = postgres.GetName()
		}

		// Fix yamlv2 issue with nested storage sections.
		fc.Storage.Params.Cleanse()

		cfg.Auth.StorageConfig = fc.Storage
		// backend is specified, but no path is set, set a reasonable default
		_, pathSet := cfg.Auth.StorageConfig.Params[defaults.BackendPath]
		if cfg.Auth.StorageConfig.Type == lite.GetName() && !pathSet {
			if cfg.Auth.StorageConfig.Params == nil {
				cfg.Auth.StorageConfig.Params = make(backend.Params)
			}
			cfg.Auth.StorageConfig.Params[defaults.BackendPath] = filepath.Join(cfg.DataDir, defaults.BackendDir)
		}
	} else {
		// Set a reasonable default.
		cfg.Auth.StorageConfig.Params[defaults.BackendPath] = filepath.Join(cfg.DataDir, defaults.BackendDir)
	}

	// apply logger settings
	err = applyLogConfig(fc.Logger, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	if fc.CachePolicy.TTL != "" {
		log.Warn("cache.ttl config option is deprecated and will be ignored, caches no longer attempt to anticipate resource expiration.")
	}
	if fc.CachePolicy.Type == memory.GetName() {
		log.Debugf("cache.type config option is explicitly set to %v.", memory.GetName())
	} else if fc.CachePolicy.Type != "" {
		log.Warn("cache.type config option is deprecated and will be ignored, caches are always in memory in this version.")
	}

	// apply cache policy for node and proxy
	cachePolicy, err := fc.CachePolicy.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.CachePolicy = *cachePolicy

	// Apply (TLS) cipher suites and (SSH) ciphers, KEX algorithms, and MAC
	// algorithms.
	if len(fc.CipherSuites) > 0 {
		cipherSuites, err := utils.CipherSuiteMapping(fc.CipherSuites)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.CipherSuites = cipherSuites
	}
	if fc.Ciphers != nil {
		cfg.Ciphers = fc.Ciphers
	}
	if fc.KEXAlgorithms != nil {
		cfg.KEXAlgorithms = fc.KEXAlgorithms
	}
	if fc.MACAlgorithms != nil {
		cfg.MACAlgorithms = fc.MACAlgorithms
	}
	if fc.CASignatureAlgorithm != nil {
		log.Warn("ca_signing_algo config option is deprecated and will be removed in a future release, Teleport defaults to rsa-sha2-512.")
	}

	// Read in how nodes will validate the CA. A single empty string in the file
	// conf should indicate no pins.
	if err = cfg.ApplyCAPins(fc.CAPin); err != nil {
		return trace.Wrap(err)
	}

	// Set diagnostic address
	if fc.DiagAddr != "" {
		// Validate address
		parsed, err := utils.ParseAddr(fc.DiagAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.DiagnosticAddr = *parsed
	}

	// apply connection throttling:
	limiters := []*limiter.Config{
		&cfg.SSH.Limiter,
		&cfg.Auth.Limiter,
		&cfg.Proxy.Limiter,
		&cfg.Databases.Limiter,
		&cfg.Kube.Limiter,
		&cfg.WindowsDesktop.ConnLimiter,
	}
	for _, l := range limiters {
		if fc.Limits.MaxConnections > 0 {
			l.MaxConnections = fc.Limits.MaxConnections
		}
		if fc.Limits.MaxUsers > 0 {
			l.MaxNumberOfUsers = fc.Limits.MaxUsers
		}
		for _, rate := range fc.Limits.Rates {
			l.Rates = append(l.Rates, limiter.Rate{
				Period:  rate.Period,
				Average: rate.Average,
				Burst:   rate.Burst,
			})
		}
	}

	applyConfigVersion(fc, cfg)

	// Apply configuration for "auth_service", "proxy_service", "ssh_service",
	// and "app_service" if they are enabled.
	if fc.Auth.Enabled() {
		err = applyAuthConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Proxy.Enabled() {
		err = applyProxyConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.SSH.Enabled() {
		err = applySSHConfig(fc, cfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Kube.Enabled() {
		if err := applyKubeConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Apps.Enabled() {
		if err := applyAppsConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Databases.Enabled() {
		if err := applyDatabasesConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Metrics.Enabled() {
		if err := applyMetricsConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.WindowsDesktop.Enabled() {
		if err := applyWindowsDesktopConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	if fc.Tracing.Enabled() {
		if err := applyTracingConfig(fc, cfg); err != nil {
			return trace.Wrap(err)
		}
	}

	if fc.TenantUrl != "" {
		cfg.TenantUrl = fc.TenantUrl
	}

	return nil
}

func applyLogConfig(loggerConfig Log, cfg *service.Config) error {
	logger := log.StandardLogger()

	switch loggerConfig.Output {
	case "":
		break // not set
	case "stderr", "error", "2":
		logger.SetOutput(os.Stderr)
		cfg.Console = io.Discard // disable console printing
	case "stdout", "out", "1":
		logger.SetOutput(os.Stdout)
		cfg.Console = io.Discard // disable console printing
	case teleport.Syslog:
		err := utils.SwitchLoggerToSyslog(logger)
		if err != nil {
			// this error will go to stderr
			log.Errorf("Failed to switch logging to syslog: %v.", err)
		}
	default:
		// assume it's a file path:
		logFile, err := os.Create(loggerConfig.Output)
		if err != nil {
			return trace.Wrap(err, "failed to create the log file")
		}
		logger.SetOutput(logFile)
	}

	switch strings.ToLower(loggerConfig.Severity) {
	case "", "info":
		logger.SetLevel(log.InfoLevel)
	case "err", "error":
		logger.SetLevel(log.ErrorLevel)
	case teleport.DebugLevel:
		logger.SetLevel(log.DebugLevel)
	case "warn", "warning":
		logger.SetLevel(log.WarnLevel)
	default:
		return trace.BadParameter("unsupported logger severity: %q", loggerConfig.Severity)
	}

	switch strings.ToLower(loggerConfig.Format.Output) {
	case "":
		fallthrough // not set. defaults to 'text'
	case "text":
		formatter := &utils.TextFormatter{
			ExtraFields:  loggerConfig.Format.ExtraFields,
			EnableColors: trace.IsTerminal(os.Stderr),
		}

		if err := formatter.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		logger.SetFormatter(formatter)
	case "json":
		formatter := &utils.JSONFormatter{
			ExtraFields: loggerConfig.Format.ExtraFields,
		}

		if err := formatter.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}

		logger.SetFormatter(formatter)
		stdlog.SetOutput(io.Discard) // disable the standard logger used by external dependencies
		stdlog.SetFlags(0)
	default:
		return trace.BadParameter("unsupported log output format : %q", loggerConfig.Format.Output)
	}

	cfg.Log = logger
	return nil
}

// applyAuthConfig applies file configuration for the "auth_service" section.
func applyAuthConfig(fc *FileConfig, cfg *service.Config) error {
	var err error

	if fc.Auth.KubeconfigFile != "" {
		warningMessage := "The auth_service no longer needs kubeconfig_file. It has " +
			"been moved to proxy_service section. This setting is ignored."
		log.Warning(warningMessage)
	}
	cfg.Auth.EnableProxyProtocol, err = utils.ParseOnOff("proxy_protocol", fc.Auth.ProxyProtocol, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if fc.Auth.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Auth.ListenAddress, int(defaults.AuthListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.ListenAddr = *addr
		cfg.AuthServers = append(cfg.AuthServers, *addr)
	}
	for _, t := range fc.Auth.ReverseTunnels {
		tun, err := t.ConvertAndValidate()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.ReverseTunnels = append(cfg.ReverseTunnels, tun)
	}
	if len(fc.Auth.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Auth.PublicAddr, defaults.AuthListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.PublicAddrs = addrs
	}
	// read in cluster name from file configuration and create services.ClusterName
	cfg.Auth.ClusterName, err = fc.Auth.ClusterName.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	// read in static tokens from file configuration and create services.StaticTokens
	if fc.Auth.StaticTokens != nil {
		cfg.Auth.StaticTokens, err = fc.Auth.StaticTokens.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// read in and set authentication preferences
	if fc.Auth.Authentication != nil {
		cfg.Auth.Preference, err = fc.Auth.Authentication.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if fc.Auth.MessageOfTheDay != "" {
		cfg.Auth.Preference.SetMessageOfTheDay(fc.Auth.MessageOfTheDay)
	}

	if fc.Auth.DisconnectExpiredCert != nil {
		cfg.Auth.Preference.SetOrigin(types.OriginConfigFile)
		cfg.Auth.Preference.SetDisconnectExpiredCert(fc.Auth.DisconnectExpiredCert.Value)
	}

	if !cfg.Auth.Preference.GetAllowLocalAuth() && cfg.Auth.Preference.GetSecondFactor() != constants.SecondFactorOff {
		warningMessage := "Second factor settings will have no affect because local " +
			"authentication is disabled. Update file configuration and remove " +
			"\"second_factor\" field to get rid of this error message."
		log.Warnf(warningMessage)
	}

	// Set cluster audit configuration from file configuration.
	auditConfigSpec, err := services.ClusterAuditConfigSpecFromObject(fc.Storage.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	auditConfigSpec.Type = fc.Storage.Type
	cfg.Auth.AuditConfig, err = types.NewClusterAuditConfig(*auditConfigSpec)
	if err != nil {
		return trace.Wrap(err)
	}

	// Set cluster networking configuration from file configuration.
	cfg.Auth.NetworkingConfig, err = types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		ClientIdleTimeout:        fc.Auth.ClientIdleTimeout,
		ClientIdleTimeoutMessage: fc.Auth.ClientIdleTimeoutMessage,
		WebIdleTimeout:           fc.Auth.WebIdleTimeout,
		KeepAliveInterval:        fc.Auth.KeepAliveInterval,
		KeepAliveCountMax:        fc.Auth.KeepAliveCountMax,
		SessionControlTimeout:    fc.Auth.SessionControlTimeout,
		ProxyListenerMode:        fc.Auth.ProxyListenerMode,
		RoutingStrategy:          fc.Auth.RoutingStrategy,
		TunnelStrategy:           fc.Auth.TunnelStrategy,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Only override session recording configuration if either field is
	// specified in file configuration.
	if fc.Auth.SessionRecording != "" || fc.Auth.ProxyChecksHostKeys != nil {
		cfg.Auth.SessionRecordingConfig, err = types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode:                fc.Auth.SessionRecording,
			ProxyChecksHostKeys: fc.Auth.ProxyChecksHostKeys,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := applyKeyStoreConfig(fc, cfg); err != nil {
		return trace.Wrap(err)
	}

	// read in and set the license file path (not used in open-source version)
	licenseFile := fc.Auth.LicenseFile
	if licenseFile != "" {
		if filepath.IsAbs(licenseFile) {
			cfg.Auth.LicenseFile = licenseFile
		} else {
			cfg.Auth.LicenseFile = filepath.Join(cfg.DataDir, licenseFile)
		}
	}

	return nil
}

func applyKeyStoreConfig(fc *FileConfig, cfg *service.Config) error {
	if fc.Auth.CAKeyParams == nil {
		return nil
	}

	if fc.Auth.CAKeyParams.PKCS11.ModulePath != "" {
		fi, err := utils.StatFile(fc.Auth.CAKeyParams.PKCS11.ModulePath)
		if err != nil {
			return trace.Wrap(err)
		}

		const worldWritableBits = 0o002
		if fi.Mode().Perm()&worldWritableBits != 0 {
			return trace.Errorf(
				"PKCS11 library (%s) must not be world-writable",
				fc.Auth.CAKeyParams.PKCS11.ModulePath,
			)
		}

		cfg.Auth.KeyStore.Path = fc.Auth.CAKeyParams.PKCS11.ModulePath
	}

	cfg.Auth.KeyStore.TokenLabel = fc.Auth.CAKeyParams.PKCS11.TokenLabel
	cfg.Auth.KeyStore.SlotNumber = fc.Auth.CAKeyParams.PKCS11.SlotNumber

	cfg.Auth.KeyStore.Pin = fc.Auth.CAKeyParams.PKCS11.Pin
	if fc.Auth.CAKeyParams.PKCS11.PinPath != "" {
		if fc.Auth.CAKeyParams.PKCS11.Pin != "" {
			return trace.BadParameter("can not set both pin and pin_path")
		}

		fi, err := utils.StatFile(fc.Auth.CAKeyParams.PKCS11.PinPath)
		if err != nil {
			return trace.Wrap(err)
		}

		const worldReadableBits = 0o004
		if fi.Mode().Perm()&worldReadableBits != 0 {
			return trace.Errorf(
				"HSM pin file (%s) must not be world-readable",
				fc.Auth.CAKeyParams.PKCS11.PinPath,
			)
		}

		pinBytes, err := os.ReadFile(fc.Auth.CAKeyParams.PKCS11.PinPath)
		if err != nil {
			return trace.Wrap(err)
		}
		pin := strings.TrimRight(string(pinBytes), "\r\n")
		cfg.Auth.KeyStore.Pin = pin
	}
	return nil
}

// applyProxyConfig applies file configuration for the "proxy_service" section.
func applyProxyConfig(fc *FileConfig, cfg *service.Config) error {
	var err error

	cfg.Proxy.EnableProxyProtocol, err = utils.ParseOnOff("proxy_protocol", fc.Proxy.ProxyProtocol, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if fc.Proxy.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.ListenAddress, defaults.SSHProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHAddr = *addr
	}
	if fc.Proxy.WebAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.WebAddr, defaults.HTTPListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.WebAddr = *addr
	}
	if fc.Proxy.TunAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.TunAddr, defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.ReverseTunnelListenAddr = *addr
	}
	if fc.Proxy.MySQLAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.MySQLAddr, defaults.MySQLListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MySQLAddr = *addr
	}
	if fc.Proxy.PostgresAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.PostgresAddr, defaults.PostgresListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PostgresAddr = *addr
	}
	if fc.Proxy.MongoAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.MongoAddr, defaults.MongoListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MongoAddr = *addr
	}
	if fc.Proxy.PeerAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.PeerAddr, int(defaults.ProxyPeeringListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PeerAddr = *addr
	}

	// This is the legacy format. Continue to support it forever, but ideally
	// users now use the list format below.
	if fc.Proxy.KeyFile != "" || fc.Proxy.CertFile != "" {
		cfg.Proxy.KeyPairs = append(cfg.Proxy.KeyPairs, service.KeyPairPath{
			PrivateKey:  fc.Proxy.KeyFile,
			Certificate: fc.Proxy.CertFile,
		})
	}
	for _, p := range fc.Proxy.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.Errorf("https private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.Errorf("https cert does not exist: %s", p.Certificate)
		}

		// Read in certificate from disk. If Teleport finds a self-signed
		// certificate chain, log a warning, and then accept whatever certificate
		// was passed. If the certificate is not self-signed, verify the certificate
		// chain from leaf to root with the trust store on the computer so browsers
		// don't complain.
		certificateChainBytes, err := utils.ReadPath(p.Certificate)
		if err != nil {
			return trace.Wrap(err)
		}
		certificateChain, err := utils.ReadCertificateChain(certificateChainBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		if utils.IsSelfSigned(certificateChain) {
			warningMessage := "Starting Teleport with a self-signed TLS certificate, this is " +
				"not safe for production clusters. Using a self-signed certificate opens " +
				"Teleport users to Man-in-the-Middle attacks."
			log.Warnf(warningMessage)
		} else {
			if err := utils.VerifyCertificateChain(certificateChain); err != nil {
				return trace.BadParameter("unable to verify HTTPS certificate chain in %v: %s",
					fc.Proxy.CertFile, utils.UserMessageFromError(err))
			}
		}

		cfg.Proxy.KeyPairs = append(cfg.Proxy.KeyPairs, service.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}

	// apply kubernetes proxy config, by default kube proxy is disabled
	legacyKube := fc.Proxy.Kube.Configured() && fc.Proxy.Kube.Enabled()
	newKube := fc.Proxy.KubeAddr != "" || len(fc.Proxy.KubePublicAddr) > 0
	switch {
	case legacyKube && !newKube:
		cfg.Proxy.Kube.Enabled = true
		cfg.Proxy.Kube.LegacyKubeProxy = true
		if fc.Proxy.Kube.KubeconfigFile != "" {
			cfg.Proxy.Kube.KubeconfigPath = fc.Proxy.Kube.KubeconfigFile
		}
		if fc.Proxy.Kube.ListenAddress != "" {
			addr, err := utils.ParseHostPortAddr(fc.Proxy.Kube.ListenAddress, int(defaults.KubeListenPort))
			if err != nil {
				return trace.Wrap(err)
			}
			cfg.Proxy.Kube.ListenAddr = *addr
		}
		if len(fc.Proxy.Kube.PublicAddr) != 0 {
			addrs, err := utils.AddrsFromStrings(fc.Proxy.Kube.PublicAddr, defaults.KubeListenPort)
			if err != nil {
				return trace.Wrap(err)
			}
			cfg.Proxy.Kube.PublicAddrs = addrs
		}
	case !legacyKube && newKube:
		// New kubernetes format (kubernetes_service +
		// proxy_service.kube_listen_addr) is only relevant in the config file
		// format. Under the hood, we use the same cfg.Proxy.Kube field to
		// enable it.
		if len(fc.Proxy.KubePublicAddr) > 0 && fc.Proxy.KubeAddr == "" {
			return trace.BadParameter("kube_listen_addr must be set when kube_public_addr is set")
		}
		cfg.Proxy.Kube.Enabled = true
		addr, err := utils.ParseHostPortAddr(fc.Proxy.KubeAddr, int(defaults.KubeListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.ListenAddr = *addr

		publicAddrs, err := utils.AddrsFromStrings(fc.Proxy.KubePublicAddr, defaults.KubeListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.PublicAddrs = publicAddrs
	case legacyKube && newKube:
		return trace.BadParameter("proxy_service should either set kube_listen_addr/kube_public_addr or kubernetes.enabled, not both; keep kubernetes.enabled if you don't enable kubernetes_service, or keep kube_listen_addr otherwise")
	case !legacyKube && !newKube:
		if fc.Version == defaults.TeleportConfigVersionV2 {
			// Always enable kube service if using config V2 (TLS routing is supported)
			cfg.Proxy.Kube.Enabled = true
		}
	}
	if len(fc.Proxy.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.PublicAddr, cfg.Proxy.WebAddr.Port(defaults.HTTPListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PublicAddrs = addrs
	}
	if len(fc.Proxy.SSHPublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.SSHPublicAddr, defaults.SSHProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHPublicAddrs = addrs
	}
	if len(fc.Proxy.TunnelPublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.TunnelPublicAddr, defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.TunnelPublicAddrs = addrs
	}
	if len(fc.Proxy.PostgresPublicAddr) != 0 {
		defaultPort := getPostgresDefaultPort(cfg)
		addrs, err := utils.AddrsFromStrings(fc.Proxy.PostgresPublicAddr, defaultPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PostgresPublicAddrs = addrs
	}

	if len(fc.Proxy.MySQLPublicAddr) != 0 {
		if fc.Proxy.MySQLAddr == "" {
			return trace.BadParameter("mysql_listen_addr must be set when mysql_public_addr is set")
		}
		// MySQL proxy is listening on a separate port.
		addrs, err := utils.AddrsFromStrings(fc.Proxy.MySQLPublicAddr, defaults.MySQLListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MySQLPublicAddrs = addrs
	}

	if len(fc.Proxy.MongoPublicAddr) != 0 {
		if fc.Proxy.MongoAddr == "" {
			return trace.BadParameter("mongo_listen_addr must be set when mongo_public_addr is set")
		}
		addrs, err := utils.AddrsFromStrings(fc.Proxy.MongoPublicAddr, defaults.MongoListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MongoPublicAddrs = addrs
	}

	acme, err := fc.Proxy.ACME.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Proxy.ACME = *acme

	applyDefaultProxyListenerAddresses(cfg)

	return nil
}

func getPostgresDefaultPort(cfg *service.Config) int {
	if !cfg.Proxy.PostgresAddr.IsEmpty() {
		// If the proxy.PostgresAddr flag was provided return port
		// from PostgresAddr address or default PostgresListenPort.
		return cfg.Proxy.PostgresAddr.Port(defaults.PostgresListenPort)
	}
	// Postgres proxy is multiplexed on the web proxy port. If the proxy is
	// not specified here explicitly, prefer defaults in the following
	// order, depending on what's set:
	//   1. Web proxy public port
	//   2. Web proxy listen port
	//   3. Web proxy default listen port
	if len(cfg.Proxy.PublicAddrs) != 0 {
		return cfg.Proxy.PublicAddrs[0].Port(defaults.HTTPListenPort)
	}
	return cfg.Proxy.WebAddr.Port(defaults.HTTPListenPort)
}

func applyDefaultProxyListenerAddresses(cfg *service.Config) {
	if cfg.Version == defaults.TeleportConfigVersionV2 {
		// For v2 configuration if an address is not provided don't fallback to the default values.
		return
	}

	// For v1 configuration check if address was set in config file if
	// not fallback to the default listener address.
	if cfg.Proxy.WebAddr.IsEmpty() {
		cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	}
	if cfg.Proxy.SSHAddr.IsEmpty() {
		cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	}
	if cfg.Proxy.ReverseTunnelListenAddr.IsEmpty() {
		cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnelListenAddr()
	}
	if cfg.Proxy.Kube.ListenAddr.IsEmpty() && cfg.Proxy.Kube.Enabled {
		cfg.Proxy.Kube.ListenAddr = *defaults.KubeProxyListenAddr()
	}
}

// applySSHConfig applies file configuration for the "ssh_service" section.
func applySSHConfig(fc *FileConfig, cfg *service.Config) (err error) {
	if fc.SSH.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.SSH.ListenAddress, int(defaults.SSHServerListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.Addr = *addr
	}
	if fc.SSH.Labels != nil {
		cfg.SSH.Labels = make(map[string]string)
		for k, v := range fc.SSH.Labels {
			cfg.SSH.Labels[k] = v
		}
	}
	if fc.SSH.Commands != nil {
		cfg.SSH.CmdLabels = make(services.CommandLabels)
		for _, cmdLabel := range fc.SSH.Commands {
			cfg.SSH.CmdLabels[cmdLabel.Name] = &types.CommandLabelV2{
				Period:  types.NewDuration(cmdLabel.Period),
				Command: cmdLabel.Command,
				Result:  "",
			}
		}
	}
	if fc.SSH.Namespace != "" {
		cfg.SSH.Namespace = fc.SSH.Namespace
	}
	if fc.SSH.PermitUserEnvironment {
		cfg.SSH.PermitUserEnvironment = true
	}
	if fc.SSH.DisableCreateHostUser || runtime.GOOS != constants.LinuxOS {
		cfg.SSH.DisableCreateHostUser = true
		if runtime.GOOS != constants.LinuxOS {
			log.Debugln("Disabling host user creation as this feature is only available on Linux")
		}
	}
	if fc.SSH.PAM != nil {
		cfg.SSH.PAM = fc.SSH.PAM.Parse()

		// If PAM is enabled, make sure that Teleport was built with PAM support
		// and the PAM library was found at runtime.
		if cfg.SSH.PAM.Enabled {
			if !pam.BuildHasPAM() {
				errorMessage := "Unable to start Teleport: PAM was enabled in file configuration but this \n" +
					"Teleport binary was built without PAM support. To continue either download a \n" +
					"Teleport binary build with PAM support from https://goteleport.com/teleport \n" +
					"or disable PAM in file configuration."
				return trace.BadParameter(errorMessage)
			}
			if !pam.SystemHasPAM() {
				errorMessage := "Unable to start Teleport: PAM was enabled in file configuration but this \n" +
					"system does not have the needed PAM library installed. To continue either \n" +
					"install libpam or disable PAM in file configuration."
				return trace.BadParameter(errorMessage)
			}
		}
	}
	if len(fc.SSH.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.SSH.PublicAddr, defaults.SSHServerListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.PublicAddrs = addrs
	}
	if fc.SSH.BPF != nil {
		cfg.SSH.BPF = fc.SSH.BPF.Parse()
	}
	if fc.SSH.RestrictedSession != nil {
		rs, err := fc.SSH.RestrictedSession.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.RestrictedSession = rs
	}

	cfg.SSH.AllowTCPForwarding = fc.SSH.AllowTCPForwarding()

	cfg.SSH.X11, err = fc.SSH.X11ServerConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, matcher := range fc.SSH.AWSMatchers {
		cfg.SSH.AWSMatchers = append(cfg.SSH.AWSMatchers,
			services.AWSMatcher{
				Types:       matcher.Types,
				Regions:     matcher.Regions,
				Tags:        matcher.Tags,
				SSMDocument: matcher.SSMDocument,
			})
	}

	return nil
}

// applyKubeConfig applies file configuration for the "kubernetes_service" section.
func applyKubeConfig(fc *FileConfig, cfg *service.Config) error {
	if fc.Kube.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Kube.ListenAddress, int(defaults.SSHProxyListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Kube.ListenAddr = addr
	}
	if len(fc.Kube.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Kube.PublicAddr, defaults.KubeListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Kube.PublicAddrs = addrs
	}

	if fc.Kube.KubeconfigFile != "" {
		cfg.Kube.KubeconfigPath = fc.Kube.KubeconfigFile
	}
	if fc.Kube.KubeClusterName != "" {
		cfg.Kube.KubeClusterName = fc.Kube.KubeClusterName
	}
	if fc.Kube.StaticLabels != nil {
		cfg.Kube.StaticLabels = make(map[string]string)
		for k, v := range fc.Kube.StaticLabels {
			cfg.Kube.StaticLabels[k] = v
		}
	}
	if fc.Kube.DynamicLabels != nil {
		cfg.Kube.DynamicLabels = make(services.CommandLabels)
		for _, cmdLabel := range fc.Kube.DynamicLabels {
			cfg.Kube.DynamicLabels[cmdLabel.Name] = &types.CommandLabelV2{
				Period:  types.NewDuration(cmdLabel.Period),
				Command: cmdLabel.Command,
				Result:  "",
			}
		}
	}

	// Sanity check the local proxy config, so that users don't forget to
	// enable the k8s endpoint there.
	if fc.Proxy.Enabled() && fc.Proxy.Kube.Disabled() && fc.Proxy.KubeAddr == "" {
		log.Warning("both kubernetes_service and proxy_service are enabled, but proxy_service doesn't set kube_listen_addr; consider setting kube_listen_addr on proxy_service, to handle incoming Kubernetes requests")
	}
	return nil
}

// applyDatabasesConfig applies file configuration for the "db_service" section.
func applyDatabasesConfig(fc *FileConfig, cfg *service.Config) error {
	cfg.Databases.Enabled = true
	for _, matcher := range fc.Databases.ResourceMatchers {
		cfg.Databases.ResourceMatchers = append(cfg.Databases.ResourceMatchers,
			services.ResourceMatcher{
				Labels: matcher.Labels,
			})
	}
	for _, matcher := range fc.Databases.AWSMatchers {
		cfg.Databases.AWSMatchers = append(cfg.Databases.AWSMatchers,
			services.AWSMatcher{
				Types:   matcher.Types,
				Regions: matcher.Regions,
				Tags:    matcher.Tags,
			})
	}
	for _, database := range fc.Databases.Databases {
		staticLabels := make(map[string]string)
		if database.StaticLabels != nil {
			staticLabels = database.StaticLabels
		}
		dynamicLabels := make(services.CommandLabels)
		if database.DynamicLabels != nil {
			for _, v := range database.DynamicLabels {
				dynamicLabels[v.Name] = &types.CommandLabelV2{
					Period:  types.NewDuration(v.Period),
					Command: v.Command,
					Result:  "",
				}
			}
		}

		caBytes, err := readCACert(database)
		if err != nil {
			return trace.Wrap(err)
		}

		db := service.Database{
			Name:          database.Name,
			Description:   database.Description,
			Protocol:      database.Protocol,
			URI:           database.URI,
			StaticLabels:  staticLabels,
			DynamicLabels: dynamicLabels,
			MySQL: service.MySQLOptions{
				ServerVersion: database.MySQL.ServerVersion,
			},
			TLS: service.DatabaseTLS{
				CACert:     caBytes,
				ServerName: database.TLS.ServerName,
				Mode:       service.TLSMode(database.TLS.Mode),
			},
			AWS: service.DatabaseAWS{
				Region: database.AWS.Region,
				Redshift: service.DatabaseAWSRedshift{
					ClusterID: database.AWS.Redshift.ClusterID,
				},
				RDS: service.DatabaseAWSRDS{
					InstanceID: database.AWS.RDS.InstanceID,
					ClusterID:  database.AWS.RDS.ClusterID,
				},
				ElastiCache: service.DatabaseAWSElastiCache{
					ReplicationGroupID: database.AWS.ElastiCache.ReplicationGroupID,
				},
				MemoryDB: service.DatabaseAWSMemoryDB{
					ClusterName: database.AWS.MemoryDB.ClusterName,
				},
				SecretStore: service.DatabaseAWSSecretStore{
					KeyPrefix: database.AWS.SecretStore.KeyPrefix,
					KMSKeyID:  database.AWS.SecretStore.KMSKeyID,
				},
			},
			GCP: service.DatabaseGCP{
				ProjectID:  database.GCP.ProjectID,
				InstanceID: database.GCP.InstanceID,
			},
			AD: service.DatabaseAD{
				KeytabFile: database.AD.KeytabFile,
				Krb5File:   database.AD.Krb5File,
				Domain:     database.AD.Domain,
				SPN:        database.AD.SPN,
			},
		}
		if err := db.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Databases.Databases = append(cfg.Databases.Databases, db)
	}
	return nil
}

// readCACert reads database CA certificate from the config file.
// First 'tls.ca_cert_file` is being read, then deprecated 'ca_cert_file' if
// the first one is not set.
func readCACert(database *Database) ([]byte, error) {
	var (
		caBytes []byte
		err     error
	)
	if database.TLS.CACertFile != "" {
		caBytes, err = os.ReadFile(database.TLS.CACertFile)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
	}

	// ca_cert_file is deprecated, but we still support it.
	// Print a warning if the old field is still being used.
	if database.CACertFile != "" {
		if database.TLS.CACertFile != "" {
			// New and old fields are set. Ignore the old field.
			log.Warnf("Ignoring deprecated ca_cert_file in %s configuration; using tls.ca_cert_file.", database.Name)
		} else {
			// Only old field is set, inform about deprecation.
			log.Warnf("ca_cert_file is deprecated, please use tls.ca_cert_file instead for %s.", database.Name)

			caBytes, err = os.ReadFile(database.CACertFile)
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}
		}
	}

	return caBytes, nil
}

// applyAppsConfig applies file configuration for the "app_service" section.
func applyAppsConfig(fc *FileConfig, cfg *service.Config) error {
	// Apps are enabled.
	cfg.Apps.Enabled = true

	// Enable debugging application if requested.
	cfg.Apps.DebugApp = fc.Apps.DebugApp

	// Configure resource watcher selectors if present.
	for _, matcher := range fc.Apps.ResourceMatchers {
		cfg.Apps.ResourceMatchers = append(cfg.Apps.ResourceMatchers,
			services.ResourceMatcher{
				Labels: matcher.Labels,
			})
	}

	// Loop over all apps and load app configuration.
	for _, application := range fc.Apps.Apps {
		// Parse the static labels of the application.
		staticLabels := make(map[string]string)
		if application.StaticLabels != nil {
			staticLabels = application.StaticLabels
		}

		// Parse the dynamic labels of the application.
		dynamicLabels := make(services.CommandLabels)
		if application.DynamicLabels != nil {
			for _, v := range application.DynamicLabels {
				dynamicLabels[v.Name] = &types.CommandLabelV2{
					Period:  types.NewDuration(v.Period),
					Command: v.Command,
				}
			}
		}

		// Add the application to the list of proxied applications.
		app := service.App{
			Name:               application.Name,
			Description:        application.Description,
			URI:                application.URI,
			PublicAddr:         application.PublicAddr,
			StaticLabels:       staticLabels,
			DynamicLabels:      dynamicLabels,
			InsecureSkipVerify: application.InsecureSkipVerify,
		}
		if application.Rewrite != nil {
			// Parse http rewrite headers if there are any.
			headers, err := service.ParseHeaders(application.Rewrite.Headers)
			if err != nil {
				return trace.Wrap(err, "failed to parse headers rewrite configuration for app %q",
					application.Name)
			}
			app.Rewrite = &service.Rewrite{
				Redirect: application.Rewrite.Redirect,
				Headers:  headers,
			}
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Apps.Apps = append(cfg.Apps.Apps, app)
	}

	return nil
}

// applyMetricsConfig applies file configuration for the "metrics_service" section.
func applyMetricsConfig(fc *FileConfig, cfg *service.Config) error {
	// Metrics is enabled.
	cfg.Metrics.Enabled = true

	addr, err := utils.ParseHostPortAddr(fc.Metrics.ListenAddress, int(defaults.MetricsListenPort))
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Metrics.ListenAddr = addr

	cfg.Metrics.GRPCServerLatency = fc.Metrics.GRPCServerLatency
	cfg.Metrics.GRPCClientLatency = fc.Metrics.GRPCClientLatency

	if !fc.Metrics.MTLSEnabled() {
		return nil
	}

	cfg.Metrics.MTLS = true

	if len(fc.Metrics.KeyPairs) == 0 {
		return trace.BadParameter("at least one keypair shoud be provided when mtls is enabled in the metrics config")
	}

	if len(fc.Metrics.CACerts) == 0 {
		return trace.BadParameter("at least one CA cert shoud be provided when mtls is enabled in the metrics config")
	}

	for _, p := range fc.Metrics.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.NotFound("metrics service private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.NotFound("metrics service cert does not exist: %s", p.Certificate)
		}

		certificateChainBytes, err := utils.ReadPath(p.Certificate)
		if err != nil {
			return trace.Wrap(err)
		}
		certificateChain, err := utils.ReadCertificateChain(certificateChainBytes)
		if err != nil {
			return trace.Wrap(err)
		}

		if !utils.IsSelfSigned(certificateChain) {
			if err := utils.VerifyCertificateChain(certificateChain); err != nil {
				return trace.BadParameter("unable to verify the metrics service certificate chain in %v: %s",
					p.Certificate, utils.UserMessageFromError(err))
			}
		}

		cfg.Metrics.KeyPairs = append(cfg.Metrics.KeyPairs, service.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}

	for _, caCert := range fc.Metrics.CACerts {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(caCert) {
			return trace.NotFound("metrics service ca cert does not exist: %s", caCert)
		}

		cfg.Metrics.CACerts = append(cfg.Metrics.CACerts, caCert)
	}

	return nil
}

// applyWindowsDesktopConfig applies file configuration for the "windows_desktop_service" section.
func applyWindowsDesktopConfig(fc *FileConfig, cfg *service.Config) error {
	cfg.WindowsDesktop.Enabled = true

	if fc.WindowsDesktop.ListenAddress != "" {
		listenAddr, err := utils.ParseHostPortAddr(fc.WindowsDesktop.ListenAddress, int(defaults.WindowsDesktopListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.WindowsDesktop.ListenAddr = *listenAddr
	}

	for _, filter := range fc.WindowsDesktop.Discovery.Filters {
		if _, err := ldap.CompileFilter(filter); err != nil {
			return trace.BadParameter("WindowsDesktopService specifies invalid LDAP filter %q", filter)
		}
	}

	for _, attributeName := range fc.WindowsDesktop.Discovery.LabelAttributes {
		if !types.IsValidLabelKey(attributeName) {
			return trace.BadParameter("WindowsDesktopService specifies label_attribute %q which is not a valid label key", attributeName)
		}
	}

	cfg.WindowsDesktop.Discovery = fc.WindowsDesktop.Discovery

	var err error
	cfg.WindowsDesktop.PublicAddrs, err = utils.AddrsFromStrings(fc.WindowsDesktop.PublicAddr, defaults.WindowsDesktopListenPort)
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.WindowsDesktop.Hosts, err = utils.AddrsFromStrings(fc.WindowsDesktop.Hosts, defaults.RDPListenPort)
	if err != nil {
		return trace.Wrap(err)
	}

	var cert *x509.Certificate
	if fc.WindowsDesktop.LDAP.DEREncodedCAFile != "" {
		rawCert, err := os.ReadFile(fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		if err != nil {
			return trace.WrapWithMessage(err, "loading the LDAP CA from file %v", fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		}

		cert, err = x509.ParseCertificate(rawCert)
		if err != nil {
			return trace.WrapWithMessage(err, "parsing the LDAP root CA file %v", fc.WindowsDesktop.LDAP.DEREncodedCAFile)
		}
	}

	cfg.WindowsDesktop.LDAP = service.LDAPConfig{
		Addr:               fc.WindowsDesktop.LDAP.Addr,
		Username:           fc.WindowsDesktop.LDAP.Username,
		Domain:             fc.WindowsDesktop.LDAP.Domain,
		InsecureSkipVerify: fc.WindowsDesktop.LDAP.InsecureSkipVerify,
		CA:                 cert,
	}

	for _, rule := range fc.WindowsDesktop.HostLabels {
		r, err := regexp.Compile(rule.Match)
		if err != nil {
			return trace.BadParameter("WindowsDesktopService specifies invalid regexp %q", rule.Match)
		}

		if len(rule.Labels) == 0 {
			return trace.BadParameter("WindowsDesktopService host regex %q has no labels", rule.Match)
		}

		for k := range rule.Labels {
			if !types.IsValidLabelKey(k) {
				return trace.BadParameter("WindowsDesktopService specifies invalid label %q", k)
			}
		}

		cfg.WindowsDesktop.HostLabels = append(cfg.WindowsDesktop.HostLabels, service.HostLabelRule{
			Regexp: r,
			Labels: rule.Labels,
		})
	}

	return nil
}

// applyTracingConfig applies file configuration for the "tracing_service" section.
func applyTracingConfig(fc *FileConfig, cfg *service.Config) error {
	// Tracing is enabled.
	cfg.Tracing.Enabled = true

	if fc.Tracing.ExporterURL == "" {
		return trace.BadParameter("tracing_service is enabled but no exporter_url is specified")
	}

	cfg.Tracing.ExporterURL = fc.Tracing.ExporterURL
	cfg.Tracing.SamplingRate = float64(fc.Tracing.SamplingRatePerMillion) / 1_000_000.0

	for _, p := range fc.Tracing.KeyPairs {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(p.PrivateKey) {
			return trace.NotFound("tracing_service private key does not exist: %s", p.PrivateKey)
		}
		if !utils.FileExists(p.Certificate) {
			return trace.NotFound("tracing_service cert does not exist: %s", p.Certificate)
		}

		cfg.Tracing.KeyPairs = append(cfg.Tracing.KeyPairs, service.KeyPairPath{
			PrivateKey:  p.PrivateKey,
			Certificate: p.Certificate,
		})
	}

	for _, caCert := range fc.Tracing.CACerts {
		// Check that the certificate exists on disk. This exists to provide the
		// user a sensible error message.
		if !utils.FileExists(caCert) {
			return trace.NotFound("tracing_service ca cert does not exist: %s", caCert)
		}

		cfg.Tracing.CACerts = append(cfg.Tracing.CACerts, caCert)
	}

	return nil
}

// parseAuthorizedKeys parses keys in the authorized_keys format and
// returns a types.CertAuthority.
func parseAuthorizedKeys(bytes []byte, allowedLogins []string) (types.CertAuthority, types.Role, error) {
	pubkey, comment, _, _, err := ssh.ParseAuthorizedKey(bytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	comments, err := url.ParseQuery(comment)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clusterName := comments.Get("clustername")
	if clusterName == "" {
		return nil, nil, trace.BadParameter("no clustername provided")
	}

	// create a new certificate authority
	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.UserCA,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PublicKey: ssh.MarshalAuthorizedKey(pubkey),
			}},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// transform old allowed logins into roles
	role := services.RoleForCertAuthority(ca)
	role.SetLogins(types.Allow, allowedLogins)
	ca.AddRole(role.GetName())

	return ca, role, nil
}

// parseKnownHosts parses keys in known_hosts format and returns a
// types.CertAuthority.
func parseKnownHosts(bytes []byte, allowedLogins []string) (types.CertAuthority, types.Role, error) {
	marker, options, pubKey, comment, _, err := ssh.ParseKnownHosts(bytes)
	if marker != "cert-authority" {
		return nil, nil, trace.BadParameter("invalid file format. expected '@cert-authority` marker")
	}
	if err != nil {
		return nil, nil, trace.BadParameter("invalid public key")
	}
	teleportOpts, err := url.ParseQuery(comment)
	if err != nil {
		return nil, nil, trace.BadParameter("invalid key comment: '%s'", comment)
	}
	authType := types.CertAuthType(teleportOpts.Get("type"))
	if authType != types.HostCA && authType != types.UserCA {
		return nil, nil, trace.BadParameter("unsupported CA type: '%s'", authType)
	}
	if len(options) == 0 {
		return nil, nil, trace.BadParameter("key without cluster_name")
	}
	const prefix = "*."
	domainName := strings.TrimPrefix(options[0], prefix)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        authType,
		ClusterName: domainName,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PublicKey: ssh.MarshalAuthorizedKey(pubKey),
			}},
		},
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// transform old allowed logins into roles
	role := services.RoleForCertAuthority(ca)
	role.SetLogins(types.Allow, apiutils.CopyStrings(allowedLogins))
	ca.AddRole(role.GetName())

	return ca, role, nil
}

// certificateAuthorityFormat parses bytes and determines if they are in
// known_hosts format or authorized_keys format.
func certificateAuthorityFormat(bytes []byte) (string, error) {
	_, _, _, _, err := ssh.ParseAuthorizedKey(bytes)
	if err != nil {
		_, _, _, _, _, err := ssh.ParseKnownHosts(bytes)
		if err != nil {
			return "", trace.BadParameter("unknown ca format")
		}
		return teleport.KnownHosts, nil
	}
	return teleport.AuthorizedKeys, nil
}

// parseCAKey parses bytes either in known_hosts or authorized_keys format
// and returns a types.CertAuthority.
func parseCAKey(bytes []byte, allowedLogins []string) (types.CertAuthority, types.Role, error) {
	caFormat, err := certificateAuthorityFormat(bytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if caFormat == teleport.AuthorizedKeys {
		return parseAuthorizedKeys(bytes, allowedLogins)
	}
	return parseKnownHosts(bytes, allowedLogins)
}

// readTrustedClusters parses the content of "trusted_clusters" YAML structure
// and modifies Teleport 'conf' by adding "authorities" and "reverse tunnels"
// to it
func readTrustedClusters(clusters []TrustedCluster, conf *service.Config) error {
	if len(clusters) == 0 {
		return nil
	}
	// go over all trusted clusters:
	for i := range clusters {
		tc := &clusters[i]
		// parse "allow_logins"
		var allowedLogins []string
		for _, login := range strings.Split(tc.AllowedLogins, ",") {
			login = strings.TrimSpace(login)
			if login != "" {
				allowedLogins = append(allowedLogins, login)
			}
		}
		// open the key file for this cluster:
		log.Debugf("reading trusted cluster key file %s", tc.KeyFile)
		if tc.KeyFile == "" {
			return trace.Errorf("key_file is missing for a trusted cluster")
		}
		f, err := os.Open(tc.KeyFile)
		if err != nil {
			return trace.Errorf("reading trusted cluster keys: %v", err)
		}
		defer f.Close()
		// read the keyfile for this cluster and get trusted CA keys:
		var authorities []types.CertAuthority
		var roles []types.Role
		scanner := bufio.NewScanner(f)
		for line := 0; scanner.Scan(); {
			ca, role, err := parseCAKey(scanner.Bytes(), allowedLogins)
			if err != nil {
				return trace.BadParameter("%s:L%d. %v", tc.KeyFile, line, err)
			}
			if ca.GetType() == types.UserCA && len(allowedLogins) == 0 && len(tc.TunnelAddr) > 0 {
				return trace.BadParameter("trusted cluster '%s' needs allow_logins parameter",
					ca.GetClusterName())
			}
			authorities = append(authorities, ca)
			if role != nil {
				roles = append(roles, role)
			}
		}
		conf.Auth.Authorities = append(conf.Auth.Authorities, authorities...)
		conf.Auth.Roles = append(conf.Auth.Roles, roles...)
		clusterName := authorities[0].GetClusterName()
		// parse "tunnel_addr"
		var tunnelAddresses []string
		for _, ta := range strings.Split(tc.TunnelAddr, ",") {
			ta := strings.TrimSpace(ta)
			if ta == "" {
				continue
			}
			addr, err := utils.ParseHostPortAddr(ta, defaults.SSHProxyTunnelListenPort)
			if err != nil {
				return trace.Wrap(err,
					"Invalid tunnel address '%s' for cluster '%s'. Expect host:port format",
					ta, clusterName)
			}
			tunnelAddresses = append(tunnelAddresses, addr.FullAddress())
		}
		if len(tunnelAddresses) > 0 {
			rt, err := types.NewReverseTunnel(clusterName, tunnelAddresses)
			if err != nil {
				return trace.Wrap(err)
			}
			conf.ReverseTunnels = append(conf.ReverseTunnels, rt)
		}
	}
	return nil
}

// applyString takes 'src' and overwrites target with it, unless 'src' is empty
// returns 'True' if 'src' was not empty
func applyString(src string, target *string) bool {
	if src != "" {
		*target = src
		return true
	}
	return false
}

// applyConfigVersion applies config version from parsed file. If config version is not
// present the v1 version will be used as default.
func applyConfigVersion(fc *FileConfig, cfg *service.Config) {
	cfg.Version = defaults.TeleportConfigVersionV1
	if fc.Version != "" {
		cfg.Version = fc.Version
	}
}

// Configure merges command line arguments with what's in a configuration file
// with CLI commands taking precedence
func Configure(clf *CommandLineFlags, cfg *service.Config) error {
	// pass the value of --insecure flag to the runtime
	lib.SetInsecureDevMode(clf.InsecureMode)

	// load /etc/teleport.yaml and apply it's values:
	fileConf, err := ReadConfigFile(clf.ConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if clf.ConfigString != "" {
		fileConf, err = ReadFromString(clf.ConfigString)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if clf.BootstrapFile != "" {
		resources, err := ReadResources(clf.BootstrapFile)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(resources) < 1 {
			return trace.BadParameter("no resources found: %q", clf.BootstrapFile)
		}
		cfg.Auth.Resources = resources
	}

	// Apply command line --debug flag to override logger severity.
	if clf.Debug {
		// If debug logging is requested and no file configuration exists, set the
		// log level right away. Otherwise allow the command line flag to override
		// logger severity in file configuration.
		if fileConf == nil {
			log.SetLevel(log.DebugLevel)
			cfg.Log.SetLevel(log.DebugLevel)
		} else {
			fileConf.Logger.Severity = teleport.DebugLevel
		}
	}

	// If this process is trying to join a cluster as an application service,
	// make sure application name and URI are provided.
	if apiutils.SliceContainsStr(splitRoles(clf.Roles), defaults.RoleApp) &&
		(clf.AppName == "" || clf.AppURI == "") {
		return trace.BadParameter("application name (--app-name) and URI (--app-uri) flags are both required to join application proxy to the cluster")
	}

	// If application name was specified on command line, add to file
	// configuration where it will be validated.
	if clf.AppName != "" {
		cfg.Apps.Enabled = true

		// Parse static and dynamic labels.
		static, dynamic, err := parseLabels(clf.Labels)
		if err != nil {
			return trace.BadParameter("labels invalid: %v", err)
		}

		// Create and validate application. If valid, add to list of applications.
		app := service.App{
			Name:          clf.AppName,
			URI:           clf.AppURI,
			PublicAddr:    clf.AppPublicAddr,
			StaticLabels:  static,
			DynamicLabels: dynamic,
		}
		if err := app.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Apps.Apps = append(cfg.Apps.Apps, app)
	}

	// If database name was specified on the command line, add to configuration.
	if clf.DatabaseName != "" {
		cfg.Databases.Enabled = true
		staticLabels, dynamicLabels, err := parseLabels(clf.Labels)
		if err != nil {
			return trace.Wrap(err)
		}
		var caBytes []byte
		if clf.DatabaseCACertFile != "" {
			caBytes, err = os.ReadFile(clf.DatabaseCACertFile)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		db := service.Database{
			Name:         clf.DatabaseName,
			Description:  clf.DatabaseDescription,
			Protocol:     clf.DatabaseProtocol,
			URI:          clf.DatabaseURI,
			StaticLabels: staticLabels,
			MySQL: service.MySQLOptions{
				ServerVersion: clf.DatabaseMySQLServerVersion,
			},
			DynamicLabels: dynamicLabels,
			TLS: service.DatabaseTLS{
				CACert: caBytes,
			},
			AWS: service.DatabaseAWS{
				Region: clf.DatabaseAWSRegion,
				Redshift: service.DatabaseAWSRedshift{
					ClusterID: clf.DatabaseAWSRedshiftClusterID,
				},
				RDS: service.DatabaseAWSRDS{
					InstanceID: clf.DatabaseAWSRDSInstanceID,
					ClusterID:  clf.DatabaseAWSRDSClusterID,
				},
			},
			GCP: service.DatabaseGCP{
				ProjectID:  clf.DatabaseGCPProjectID,
				InstanceID: clf.DatabaseGCPInstanceID,
			},
			AD: service.DatabaseAD{
				KeytabFile: clf.DatabaseADKeytabFile,
				Krb5File:   clf.DatabaseADKrb5File,
				Domain:     clf.DatabaseADDomain,
				SPN:        clf.DatabaseADSPN,
			},
		}
		if err := db.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Databases.Databases = append(cfg.Databases.Databases, db)
	}

	if err = ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
	}

	// If FIPS mode is specified, validate Teleport configuration is FedRAMP/FIPS
	// 140-2 compliant.
	if clf.FIPS {
		// Make sure all cryptographic primitives are FIPS compliant.
		err = utils.UintSliceSubset(defaults.FIPSCipherSuites, cfg.CipherSuites)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant TLS cipher suite selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSCiphers, cfg.Ciphers)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH cipher selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSKEXAlgorithms, cfg.KEXAlgorithms)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH kex algorithm selected: %v", err)
		}
		err = utils.StringSliceSubset(defaults.FIPSMACAlgorithms, cfg.MACAlgorithms)
		if err != nil {
			return trace.BadParameter("non-FIPS compliant SSH mac algorithm selected: %v", err)
		}

		// Make sure cluster settings are also FedRAMP/FIPS 140-2 compliant.
		if cfg.Auth.Enabled {
			// Only SSO based authentication is supported. The SSO provider is where
			// any FedRAMP/FIPS 140-2 compliance (like password complexity) should be
			// enforced.
			if cfg.Auth.Preference.GetAllowLocalAuth() {
				return trace.BadParameter("non-FIPS compliant authentication setting: \"local_auth\" must be false")
			}

			// If sessions are being recorded at the proxy host key checking must be
			// enabled. This make sure the host certificate key algorithm is FIPS
			// compliant.
			if services.IsRecordAtProxy(cfg.Auth.SessionRecordingConfig.GetMode()) &&
				!cfg.Auth.SessionRecordingConfig.GetProxyChecksHostKeys() {
				return trace.BadParameter("non-FIPS compliant proxy settings: \"proxy_checks_host_keys\" must be true")
			}
		}
	}

	// apply --skip-version-check flag.
	if clf.SkipVersionCheck {
		cfg.SkipVersionCheck = clf.SkipVersionCheck
	}

	// Apply diagnostic address flag.
	if clf.DiagnosticAddr != "" {
		addr, err := utils.ParseAddr(clf.DiagnosticAddr)
		if err != nil {
			return trace.Wrap(err, "failed to parse diag-addr")
		}
		cfg.DiagnosticAddr = *addr
	}

	// apply --insecure-no-tls flag:
	if clf.DisableTLS {
		cfg.Proxy.DisableTLS = clf.DisableTLS
	}

	// apply --debug flag to config:
	if clf.Debug {
		cfg.Console = io.Discard
		cfg.Debug = clf.Debug
	}

	// apply --roles flag:
	if clf.Roles != "" {
		if err := validateRoles(clf.Roles); err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.Enabled = strings.Contains(clf.Roles, defaults.RoleNode)
		cfg.Auth.Enabled = strings.Contains(clf.Roles, defaults.RoleAuthService)
		cfg.Proxy.Enabled = strings.Contains(clf.Roles, defaults.RoleProxy)
		cfg.Apps.Enabled = strings.Contains(clf.Roles, defaults.RoleApp)
		cfg.Databases.Enabled = strings.Contains(clf.Roles, defaults.RoleDatabase)
	}

	// apply --auth-server flag:
	if len(clf.AuthServerAddr) > 0 {
		if cfg.Auth.Enabled {
			log.Warnf("not starting the local auth service. --auth-server flag tells to connect to another auth server")
			cfg.Auth.Enabled = false
		}
		cfg.AuthServers = make([]utils.NetAddr, 0, len(clf.AuthServerAddr))
		for _, as := range clf.AuthServerAddr {
			addr, err := utils.ParseHostPortAddr(as, defaults.AuthListenPort)
			if err != nil {
				return trace.BadParameter("cannot parse auth server address: '%v'", as)
			}
			cfg.AuthServers = append(cfg.AuthServers, *addr)
		}
	}

	// apply --name flag:
	if clf.NodeName != "" {
		cfg.Hostname = clf.NodeName
	}

	// apply --pid-file flag
	if clf.PIDFile != "" {
		cfg.PIDFile = clf.PIDFile
	}

	// apply --token flag:
	if _, err := cfg.ApplyToken(clf.AuthToken); err != nil {
		return trace.Wrap(err)
	}

	// Apply flags used for the node to validate the Auth Server.
	if err = cfg.ApplyCAPins(clf.CAPins); err != nil {
		return trace.Wrap(err)
	}

	// apply --listen-ip flag:
	if clf.ListenIP != nil {
		applyListenIP(clf.ListenIP, cfg)
	}

	// --advertise-ip flag
	if clf.AdvertiseIP != "" {
		if _, _, err := utils.ParseAdvertiseAddr(clf.AdvertiseIP); err != nil {
			return trace.Wrap(err)
		}
		cfg.AdvertiseIP = clf.AdvertiseIP
	}

	// apply --labels flag
	if err := parseLabelsApply(clf.Labels, &cfg.SSH); err != nil {
		return trace.Wrap(err)
	}

	// --pid-file:
	if clf.PIDFile != "" {
		cfg.PIDFile = clf.PIDFile
	}

	// auth_servers not configured, but the 'auth' is enabled (auth is on localhost)?
	if len(cfg.AuthServers) == 0 && cfg.Auth.Enabled {
		cfg.AuthServers = append(cfg.AuthServers, cfg.Auth.ListenAddr)
	}

	// add data_dir to the backend config:
	if cfg.Auth.StorageConfig.Params == nil {
		cfg.Auth.StorageConfig.Params = backend.Params{}
	}
	cfg.Auth.StorageConfig.Params["data_dir"] = cfg.DataDir
	// command line flag takes precedence over file config
	if clf.PermitUserEnvironment {
		cfg.SSH.PermitUserEnvironment = true
	}

	return nil
}

// parseLabels parses the labels command line flag and returns static and
// dynamic labels.
func parseLabels(spec string) (map[string]string, services.CommandLabels, error) {
	// Base syntax parsing, the spec must be in the form of 'key=value,more="better"'.
	lmap, err := client.ParseLabelSpec(spec)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	static := make(map[string]string)
	dynamic := make(services.CommandLabels)

	if len(lmap) == 0 {
		return static, dynamic, nil
	}

	// Loop over all parsed labels and set either static or dynamic labels.
	for key, value := range lmap {
		dynamicLabel, err := isCmdLabelSpec(value)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if dynamicLabel != nil {
			dynamic[key] = dynamicLabel
		} else {
			static[key] = value
		}
	}

	return static, dynamic, nil
}

// parseLabelsApply reads in the labels command line flag and tries to
// correctly populate static and dynamic labels for the SSH service.
func parseLabelsApply(spec string, sshConf *service.SSHConfig) error {
	if spec == "" {
		return nil
	}

	var err error
	sshConf.Labels, sshConf.CmdLabels, err = parseLabels(spec)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// isCmdLabelSpec tries to interpret a given string as a "command label" spec.
// A command label spec looks like [time_duration:command param1 param2 ...] where
// time_duration is in "1h2m1s" form.
//
// Example of a valid spec: "[1h:/bin/uname -m]"
func isCmdLabelSpec(spec string) (types.CommandLabel, error) {
	// command spec? (surrounded by brackets?)
	if len(spec) > 5 && spec[0] == '[' && spec[len(spec)-1] == ']' {
		invalidSpecError := trace.BadParameter(
			"invalid command label spec: '%s'", spec)
		spec = strings.Trim(spec, "[]")
		idx := strings.IndexRune(spec, ':')
		if idx < 0 {
			return nil, trace.Wrap(invalidSpecError)
		}
		periodSpec := spec[:idx]
		period, err := time.ParseDuration(periodSpec)
		if err != nil {
			return nil, trace.Wrap(invalidSpecError)
		}
		cmdSpec := spec[idx+1:]
		if len(cmdSpec) < 1 {
			return nil, trace.Wrap(invalidSpecError)
		}
		openQuote := false
		return &types.CommandLabelV2{
			Period: types.NewDuration(period),
			Command: strings.FieldsFunc(cmdSpec, func(c rune) bool {
				if c == '"' {
					openQuote = !openQuote
				}
				return unicode.IsSpace(c) && !openQuote
			}),
		}, nil
	}
	// not a valid spec
	return nil, nil
}

// applyListenIP replaces all 'listen addr' settings for all services with
// a given IP
func applyListenIP(ip net.IP, cfg *service.Config) {
	listeningAddresses := []*utils.NetAddr{
		&cfg.Auth.ListenAddr,
		&cfg.Auth.ListenAddr,
		&cfg.Proxy.SSHAddr,
		&cfg.Proxy.WebAddr,
		&cfg.SSH.Addr,
		&cfg.Proxy.ReverseTunnelListenAddr,
	}
	for _, addr := range listeningAddresses {
		replaceHost(addr, ip.String())
	}
}

// replaceHost takes utils.NetAddr and replaces the hostname in it, preserving
// the original port
func replaceHost(addr *utils.NetAddr, newHost string) {
	_, port, err := net.SplitHostPort(addr.Addr)
	if err != nil {
		log.Errorf("failed parsing address: '%v'", addr.Addr)
	}
	addr.Addr = net.JoinHostPort(newHost, port)
}

// validateRoles makes sure that value passed to the --roles flag is valid
func validateRoles(roles string) error {
	for _, role := range splitRoles(roles) {
		switch role {
		case defaults.RoleAuthService,
			defaults.RoleNode,
			defaults.RoleProxy,
			defaults.RoleApp,
			defaults.RoleDatabase,
			defaults.RoleWindowsDesktop:
		default:
			return trace.Errorf("unknown role: '%s'", role)
		}
	}
	return nil
}

// splitRoles splits in the format roles expects.
func splitRoles(roles string) []string {
	return strings.Split(roles, ",")
}

// applyTokenConfig applies the auth_token and join_params to the config
func applyTokenConfig(fc *FileConfig, cfg *service.Config) error {
	if fc.AuthToken != "" {
		cfg.JoinMethod = types.JoinMethodToken
		_, err := cfg.ApplyToken(fc.AuthToken)
		return trace.Wrap(err)
	}
	if fc.JoinParams != (JoinParams{}) {
		if cfg.Token != "" {
			return trace.BadParameter("only one of auth_token or join_params should be set")
		}
		_, err := cfg.ApplyToken(fc.JoinParams.TokenName)
		if err != nil {
			return trace.Wrap(err)
		}
		switch fc.JoinParams.Method {
		case types.JoinMethodEC2, types.JoinMethodIAM, types.JoinMethodToken:
			cfg.JoinMethod = fc.JoinParams.Method
		default:
			return trace.BadParameter(`unknown value for join_params.method: %q, expected one of %v`, fc.JoinParams.Method, []types.JoinMethod{types.JoinMethodEC2, types.JoinMethodIAM, types.JoinMethodToken})
		}
	}
	return nil
}
