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

// Package 'config' provides facilities for configuring Teleport daemons
// including
//	- parsing YAML configuration
//	- parsing CLI flags
package config

import (
	"bufio"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

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
	// CAPin is the hash of the SKPI of the root CA. Used to verify the cluster
	// being joined is the one expected.
	CAPin string
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

	// AppName is the name of the application to proxy.
	AppName string

	// AppURI is the internal address of the application to proxy.
	AppURI string

	// AppPublicAddr is the public address of the application to proxy.
	AppPublicAddr string

	// DatabaseName is the name of the database to proxy.
	DatabaseName string
	// DatabaseProtocol is the type of the proxied database e.g. postgres or mysql.
	DatabaseProtocol string
	// DatabaseURI is the address to connect to the proxied database.
	DatabaseURI string
	// DatabaseCACertFile is the database CA cert path.
	DatabaseCACertFile string
	// DatabaseAWSRegion is an optional database cloud region e.g. when using AWS RDS.
	DatabaseAWSRegion string
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
func ReadResources(filePath string) ([]services.Resource, error) {
	reader, err := utils.OpenFile(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.LookaheadBufSize)
	var resources []services.Resource
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
	if _, err := cfg.ApplyToken(fc.AuthToken); err != nil {
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
	logger := utils.NewLogger()
	err = applyLogConfig(fc.Logger, logger)
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Log = logger

	// Apply logging configuration for the global logger instance
	// DELETE this when global logger instance is no longer in use.
	//
	// Logging configuration has already been validated above
	_ = applyLogConfig(fc.Logger, log.StandardLogger())

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
		cfg.CASignatureAlgorithm = fc.CASignatureAlgorithm
	}

	// Read in how nodes will validate the CA.
	if fc.CAPin != "" {
		cfg.CAPin = fc.CAPin
	}

	// apply connection throttling:
	limiters := []*limiter.Config{
		&cfg.SSH.Limiter,
		&cfg.Auth.Limiter,
		&cfg.Proxy.Limiter,
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

	return nil
}

func applyLogConfig(loggerConfig Log, logger *log.Logger) error {
	switch loggerConfig.Output {
	case "":
		break // not set
	case "stderr", "error", "2":
		logger.SetOutput(os.Stderr)
	case "stdout", "out", "1":
		logger.SetOutput(os.Stdout)
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
	case "":
		break // not set
	case "info":
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
		cfg.Auth.SSHAddr = *addr
		cfg.AuthServers = append(cfg.AuthServers, *addr)
	}

	// INTERNAL: Authorities (plus Roles) and ReverseTunnels don't follow the
	// same pattern as the rest of the configuration (they are not configuration
	// singletons). However, we need to keep them around while Telekube uses them.
	for _, authority := range fc.Auth.Authorities {
		ca, role, err := authority.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.Authorities = append(cfg.Auth.Authorities, ca)
		cfg.Auth.Roles = append(cfg.Auth.Roles, role)
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
		authPreference, err := fc.Auth.Authentication.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.Preference = authPreference
	}

	var localAuth services.Bool
	if fc.Auth.Authentication == nil || fc.Auth.Authentication.LocalAuth == nil {
		localAuth = services.NewBool(true)
	} else {
		localAuth = *fc.Auth.Authentication.LocalAuth
	}

	if !localAuth.Value() && fc.Auth.Authentication.SecondFactor != "" {
		warningMessage := "Second factor settings will have no affect because local " +
			"authentication is disabled. Update file configuration and remove " +
			"\"second_factor\" field to get rid of this error message."
		log.Warnf(warningMessage)
	}

	auditConfig, err := services.AuditConfigFromObject(fc.Storage.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	auditConfig.Type = fc.Storage.Type

	// Set cluster-wide configuration from file configuration.
	cfg.Auth.ClusterConfig, err = services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording:      fc.Auth.SessionRecording,
		ProxyChecksHostKeys:   fc.Auth.ProxyChecksHostKeys,
		Audit:                 *auditConfig,
		ClientIdleTimeout:     fc.Auth.ClientIdleTimeout,
		DisconnectExpiredCert: fc.Auth.DisconnectExpiredCert,
		KeepAliveInterval:     fc.Auth.KeepAliveInterval,
		KeepAliveCountMax:     fc.Auth.KeepAliveCountMax,
		LocalAuth:             localAuth,
		SessionControlTimeout: fc.Auth.SessionControlTimeout,
	})
	if err != nil {
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

// applyProxyConfig applies file configuration for the "proxy_service" section.
func applyProxyConfig(fc *FileConfig, cfg *service.Config) error {
	var err error

	cfg.Proxy.EnableProxyProtocol, err = utils.ParseOnOff("proxy_protocol", fc.Proxy.ProxyProtocol, true)
	if err != nil {
		return trace.Wrap(err)
	}
	if fc.Proxy.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.ListenAddress, int(defaults.SSHProxyListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHAddr = *addr
	}
	if fc.Proxy.WebAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.WebAddr, int(defaults.HTTPListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.WebAddr = *addr
	}
	if fc.Proxy.TunAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.TunAddr, int(defaults.SSHProxyTunnelListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.ReverseTunnelListenAddr = *addr
	}
	if fc.Proxy.MySQLAddr != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.MySQLAddr, int(defaults.MySQLListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.MySQLAddr = *addr
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

		// Read in certificate from disk. If Teleport finds a self signed
		// certificate chain, log a warning, and then accept whatever certificate
		// was passed. If the certificate is not self signed, verify the certificate
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
		// Nothing enabled, this is just for completeness.
	}
	if len(fc.Proxy.PublicAddr) != 0 {
		addrs, err := utils.AddrsFromStrings(fc.Proxy.PublicAddr, defaults.HTTPListenPort)
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

	acme, err := fc.Proxy.ACME.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Proxy.ACME = *acme

	return nil

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
			cfg.SSH.CmdLabels[cmdLabel.Name] = &services.CommandLabelV2{
				Period:  services.NewDuration(cmdLabel.Period),
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

	if proxyAddr := os.Getenv(defaults.TunnelPublicAddrEnvar); proxyAddr != "" {
		cfg.SSH.ProxyReverseTunnelFallbackAddr, err = utils.ParseHostPortAddr(proxyAddr, defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err, "invalid reverse tunnel address format %q", proxyAddr)
		}
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
			cfg.Kube.DynamicLabels[cmdLabel.Name] = &services.CommandLabelV2{
				Period:  services.NewDuration(cmdLabel.Period),
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
	for _, database := range fc.Databases.Databases {
		staticLabels := make(map[string]string)
		if database.StaticLabels != nil {
			staticLabels = database.StaticLabels
		}
		dynamicLabels := make(services.CommandLabels)
		if database.DynamicLabels != nil {
			for _, v := range database.DynamicLabels {
				dynamicLabels[v.Name] = &services.CommandLabelV2{
					Period:  services.NewDuration(v.Period),
					Command: v.Command,
					Result:  "",
				}
			}
		}
		var caBytes []byte
		var err error
		if database.CACertFile != "" {
			caBytes, err = ioutil.ReadFile(database.CACertFile)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		db := service.Database{
			Name:          database.Name,
			Description:   database.Description,
			Protocol:      database.Protocol,
			URI:           database.URI,
			StaticLabels:  staticLabels,
			DynamicLabels: dynamicLabels,
			CACert:        caBytes,
			AWS: service.DatabaseAWS{
				Region: database.AWS.Region,
			},
			GCP: service.DatabaseGCP{
				ProjectID:  database.GCP.ProjectID,
				InstanceID: database.GCP.InstanceID,
			},
		}
		if err := db.Check(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Databases.Databases = append(cfg.Databases.Databases, db)
	}
	return nil
}

// applyAppsConfig applies file configuration for the "app_service" section.
func applyAppsConfig(fc *FileConfig, cfg *service.Config) error {
	// Apps are enabled.
	cfg.Apps.Enabled = true

	// Enable debugging application if requested.
	cfg.Apps.DebugApp = fc.Apps.DebugApp

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
				dynamicLabels[v.Name] = &services.CommandLabelV2{
					Period:  services.NewDuration(v.Period),
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
			app.Rewrite = &service.Rewrite{
				Redirect: application.Rewrite.Redirect,
			}
		}
		if err := app.Check(); err != nil {
			return trace.Wrap(err)
		}
		cfg.Apps.Apps = append(cfg.Apps.Apps, app)
	}

	return nil
}

// parseAuthorizedKeys parses keys in the authorized_keys format and
// returns a services.CertAuthority.
func parseAuthorizedKeys(bytes []byte, allowedLogins []string) (services.CertAuthority, services.Role, error) {
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
	ca := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:         services.UserCA,
		ClusterName:  clusterName,
		SigningKeys:  nil,
		CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(pubkey)},
		Roles:        nil,
		SigningAlg:   services.CertAuthoritySpecV2_UNKNOWN,
	})

	// transform old allowed logins into roles
	role := services.RoleForCertAuthority(ca)
	role.SetLogins(services.Allow, allowedLogins)
	ca.AddRole(role.GetName())

	return ca, role, nil
}

// parseKnownHosts parses keys in known_hosts format and returns a
// services.CertAuthority.
func parseKnownHosts(bytes []byte, allowedLogins []string) (services.CertAuthority, services.Role, error) {
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
	authType := services.CertAuthType(teleportOpts.Get("type"))
	if authType != services.HostCA && authType != services.UserCA {
		return nil, nil, trace.BadParameter("unsupported CA type: '%s'", authType)
	}
	if len(options) == 0 {
		return nil, nil, trace.BadParameter("key without cluster_name")
	}
	const prefix = "*."
	domainName := strings.TrimPrefix(options[0], prefix)

	ca := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:         authType,
		ClusterName:  domainName,
		CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(pubKey)},
	})

	// transform old allowed logins into roles
	role := services.RoleForCertAuthority(ca)
	role.SetLogins(services.Allow, utils.CopyStrings(allowedLogins))
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
// and returns a services.CertAuthority.
func parseCAKey(bytes []byte, allowedLogins []string) (services.CertAuthority, services.Role, error) {
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
		var authorities []services.CertAuthority
		var roles []services.Role
		scanner := bufio.NewScanner(f)
		for line := 0; scanner.Scan(); {
			ca, role, err := parseCAKey(scanner.Bytes(), allowedLogins)
			if err != nil {
				return trace.BadParameter("%s:L%d. %v", tc.KeyFile, line, err)
			}
			if ca.GetType() == services.UserCA && len(allowedLogins) == 0 && len(tc.TunnelAddr) > 0 {
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
			conf.ReverseTunnels = append(conf.ReverseTunnels, services.NewReverseTunnel(clusterName, tunnelAddresses))
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
	if utils.SliceContainsStr(splitRoles(clf.Roles), defaults.RoleApp) &&
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
		if err := app.Check(); err != nil {
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
			caBytes, err = ioutil.ReadFile(clf.DatabaseCACertFile)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		db := service.Database{
			Name:          clf.DatabaseName,
			Protocol:      clf.DatabaseProtocol,
			URI:           clf.DatabaseURI,
			StaticLabels:  staticLabels,
			DynamicLabels: dynamicLabels,
			CACert:        caBytes,
			AWS: service.DatabaseAWS{
				Region: clf.DatabaseAWSRegion,
			},
		}
		if err := db.Check(); err != nil {
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
			if cfg.Auth.ClusterConfig.GetLocalAuth() {
				return trace.BadParameter("non-FIPS compliant authentication setting: \"local_auth\" must be false")
			}

			// If sessions are being recorded at the proxy host key checking must be
			// enabled. This make sure the host certificate key algorithm is FIPS
			// compliant.
			if services.IsRecordAtProxy(cfg.Auth.ClusterConfig.GetSessionRecording()) &&
				cfg.Auth.ClusterConfig.GetProxyChecksHostKeys() == services.HostKeyCheckNo {
				return trace.BadParameter("non-FIPS compliant proxy settings: \"proxy_checks_host_keys\" must be true")
			}
		}
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
		cfg.Console = ioutil.Discard
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
	if clf.CAPin != "" {
		cfg.CAPin = clf.CAPin
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
		cfg.AuthServers = append(cfg.AuthServers, cfg.Auth.SSHAddr)
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
func isCmdLabelSpec(spec string) (services.CommandLabel, error) {
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
		var openQuote bool = false
		return &services.CommandLabelV2{
			Period: services.NewDuration(period),
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
		&cfg.Auth.SSHAddr,
		&cfg.Auth.SSHAddr,
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

// validateRoles makes sure that value upassed to --roles flag is valid
func validateRoles(roles string) error {
	for _, role := range splitRoles(roles) {
		switch role {
		case defaults.RoleAuthService,
			defaults.RoleNode,
			defaults.RoleProxy,
			defaults.RoleApp,
			defaults.RoleDatabase:
			break
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
