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
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// CommandLineFlags stores command line flag values, it's a much simplified subset
// of Teleport configuration (which is fully expressed via YAML config file)
type CommandLineFlags struct {
	// --name flag
	NodeName string
	// --auth-server flag
	AuthServerAddr string
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
}

// readConfigFile reads /etc/teleport.yaml (or whatever is passed via --config flag)
// and overrides values in 'cfg' structure
func ReadConfigFile(cliConfigPath string) (*FileConfig, error) {
	configFilePath := defaults.ConfigFilePath
	// --config tells us to use a specific conf. file:
	if cliConfigPath != "" {
		configFilePath = cliConfigPath
		if !fileExists(configFilePath) {
			return nil, trace.Errorf("file not found: %s", configFilePath)
		}
	}
	// default config doesn't exist? quietly return:
	if !fileExists(configFilePath) {
		log.Info("not using a config file")
		return nil, nil
	}
	log.Debug("reading config file: ", configFilePath)
	return ReadFromFile(configFilePath)
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
	cfg.ApplyToken(fc.AuthToken)

	if fc.Global.DataDir != "" {
		cfg.DataDir = fc.Global.DataDir
		cfg.Auth.StorageConfig.Params["path"] = cfg.DataDir
	}

	// if a backend is specified, override the defaults
	if fc.Storage.Type != "" {
		cfg.Auth.StorageConfig = fc.Storage
		// backend is specified, but no path is set, set a reasonable default
		_, pathSet := cfg.Auth.StorageConfig.Params[defaults.BackendPath]
		if cfg.Auth.StorageConfig.Type == dir.GetName() && !pathSet {
			if cfg.Auth.StorageConfig.Params == nil {
				cfg.Auth.StorageConfig.Params = make(backend.Params)
			}
			cfg.Auth.StorageConfig.Params[defaults.BackendPath] = filepath.Join(cfg.DataDir, defaults.BackendDir)
		}
	} else {
		// bolt backend is deprecated, but is picked if it was setup before
		exists, err := boltbk.Exists(cfg.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}
		if exists {
			cfg.Auth.StorageConfig.Type = boltbk.GetName()
			cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: cfg.DataDir}
		} else {
			cfg.Auth.StorageConfig.Type = dir.GetName()
			cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
		}
	}

	// apply logger settings
	switch fc.Logger.Output {
	case "":
		break // not set
	case "stderr", "error", "2":
		log.SetOutput(os.Stderr)
	case "stdout", "out", "1":
		log.SetOutput(os.Stdout)
	case teleport.Syslog:
		utils.SwitchLoggingtoSyslog()
	default:
		// assume it's a file path:
		logFile, err := os.Create(fc.Logger.Output)
		if err != nil {
			return trace.Wrap(err, "failed to create the log file")
		}
		log.SetOutput(logFile)
	}
	switch strings.ToLower(fc.Logger.Severity) {
	case "":
		break // not set
	case "info":
		log.SetLevel(log.InfoLevel)
	case "err", "error":
		log.SetLevel(log.ErrorLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	default:
		return trace.BadParameter("unsupported logger severity: '%v'", fc.Logger.Severity)
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

	// Read in how nodes will validate the CA.
	if fc.CAPin != "" {
		cfg.CAPin = fc.CAPin
	}

	// apply connection throttling:
	limiters := []*limiter.LimiterConfig{
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
	// add static signed keypairs supplied from configs
	for i := range fc.Global.Keys {
		identity, err := fc.Global.Keys[i].Identity()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Identities = append(cfg.Identities, identity)
	}

	// Apply configuration for "auth_service", "proxy_service", and
	// "ssh_service" if it's enabled.
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

	return nil
}

// applyAuthConfig applies file configuration for the "auth_service" section.
func applyAuthConfig(fc *FileConfig, cfg *service.Config) error {
	var err error

	// passhtrough custom certificate authority file
	if fc.Auth.KubeconfigFile != "" {
		cfg.Auth.KubeconfigPath = fc.Auth.KubeconfigFile
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

	// DELETE IN: 2.7.0
	// We have converted this warning to error
	if fc.Auth.DynamicConfig != nil {
		warningMessage := "Dynamic configuration is no longer supported. Refer to " +
			"Teleport documentation for more details: " +
			"http://gravitational.com/teleport/docs/admin-guide"
		return trace.BadParameter(warningMessage)
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
		addrs, err := fc.Auth.PublicAddr.Addrs(defaults.AuthListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.PublicAddrs = addrs
	}
	// DELETE IN: 2.4.0
	if len(fc.Auth.TrustedClusters) > 0 {
		warningMessage := "Configuring Trusted Clusters using file configuration has " +
			"been deprecated, Trusted Clusters must now be configured using resources. " +
			"Existing Trusted Cluster relationships will be maintained, but you will " +
			"not be able to add or remove Trusted Clusters using file configuration " +
			"anymore. Refer to Teleport documentation for more details: " +
			"http://gravitational.com/teleport/docs/admin-guide"
		log.Warnf(warningMessage)
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
		authPreference, oidcConnector, err := fc.Auth.Authentication.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.Preference = authPreference

		// DELETE IN: 2.4.0
		if oidcConnector != nil {
			warningMessage := "Configuring OIDC connectors using file configuration is " +
				"no longer supported, OIDC connectors must be configured using resources. " +
				"Existing OIDC connectors will not be removed but you will not be able to " +
				"add, remove, or update them using file configuration. Refer to Teleport " +
				"documentation for more details: " +
				"http://gravitational.com/teleport/docs/admin-guide"
			log.Warnf(warningMessage)
		}
	}

	auditConfig, err := services.AuditConfigFromObject(fc.Storage.Params)
	if err != nil {
		return trace.Wrap(err)
	}
	auditConfig.Type = fc.Storage.Type

	// build cluster config from session recording and host key checking preferences
	cfg.Auth.ClusterConfig, err = services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording:      fc.Auth.SessionRecording,
		ProxyChecksHostKeys:   fc.Auth.ProxyChecksHostKeys,
		Audit:                 *auditConfig,
		ClientIdleTimeout:     fc.Auth.ClientIdleTimeout,
		DisconnectExpiredCert: fc.Auth.DisconnectExpiredCert,
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

	if fc.Proxy.KeyFile != "" {
		if !fileExists(fc.Proxy.KeyFile) {
			return trace.Errorf("https key does not exist: %s", fc.Proxy.KeyFile)
		}
		cfg.Proxy.TLSKey = fc.Proxy.KeyFile
	}
	if fc.Proxy.CertFile != "" {
		if !fileExists(fc.Proxy.CertFile) {
			return trace.Errorf("https cert does not exist: %s", fc.Proxy.CertFile)
		}

		// read in certificate chain from disk
		certificateChainBytes, err := utils.ReadPath(fc.Proxy.CertFile)
		if err != nil {
			return trace.Wrap(err)
		}

		// parse certificate chain into []*x509.Certificate
		certificateChain, err := utils.ReadCertificateChain(certificateChainBytes)
		if err != nil {
			return trace.Wrap(err)
		}

		// if starting teleport with a self signed certificate, print a warning, and
		// then take whatever was passed to us. otherwise verify the certificate
		// chain from leaf to root so browsers don't complain.
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

		cfg.Proxy.TLSCert = fc.Proxy.CertFile
	}

	// apply kubernetes proxy config, by default kube proxy is disabled
	if fc.Proxy.Kube.Configured() {
		cfg.Proxy.Kube.Enabled = fc.Proxy.Kube.Enabled()
	}
	if fc.Proxy.Kube.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Proxy.Kube.ListenAddress, int(defaults.KubeProxyListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.ListenAddr = *addr
	}
	if len(fc.Proxy.Kube.PublicAddr) != 0 {
		addrs, err := fc.Proxy.Kube.PublicAddr.Addrs(defaults.KubeProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.Kube.PublicAddrs = addrs
	}
	if len(fc.Proxy.PublicAddr) != 0 {
		addrs, err := fc.Proxy.PublicAddr.Addrs(defaults.HTTPListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.PublicAddrs = addrs
	}
	if len(fc.Proxy.SSHPublicAddr) != 0 {
		addrs, err := fc.Proxy.SSHPublicAddr.Addrs(defaults.SSHProxyListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.SSHPublicAddrs = addrs
	}

	return nil

}

// applySSHConfig applies file configuration for the "ssh_service" section.
func applySSHConfig(fc *FileConfig, cfg *service.Config) error {
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
					"Teleport binary build with PAM support from https://gravitational.com/teleport \n" +
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
		addrs, err := fc.SSH.PublicAddr.Addrs(defaults.SSHServerListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.PublicAddrs = addrs
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
	ca := services.NewCertAuthority(
		services.UserCA,
		clusterName,
		nil,
		[][]byte{ssh.MarshalAuthorizedKey(pubkey)},
		nil)

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

	v1 := &services.CertAuthorityV1{
		AllowedLogins: utils.CopyStrings(allowedLogins),
		DomainName:    domainName,
		Type:          authType,
		CheckingKeys:  [][]byte{ssh.MarshalAuthorizedKey(pubKey)},
	}
	ca, role := services.ConvertV1CertAuthority(v1)
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
	if err = ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
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

	// apply --debug flag:
	if clf.Debug {
		cfg.Console = ioutil.Discard
		utils.InitLogger(utils.LoggingForDaemon, log.DebugLevel)
		cfg.Debug = clf.Debug
	}

	// apply --roles flag:
	if clf.Roles != "" {
		if err := validateRoles(clf.Roles); err != nil {
			return trace.Wrap(err)
		}
		cfg.SSH.Enabled = strings.Index(clf.Roles, defaults.RoleNode) != -1
		cfg.Auth.Enabled = strings.Index(clf.Roles, defaults.RoleAuthService) != -1
		cfg.Proxy.Enabled = strings.Index(clf.Roles, defaults.RoleProxy) != -1
	}

	// apply --auth-server flag:
	if clf.AuthServerAddr != "" {
		if cfg.Auth.Enabled {
			log.Warnf("not starting the local auth service. --auth-server flag tells to connect to another auth server")
			cfg.Auth.Enabled = false
		}
		addr, err := utils.ParseHostPortAddr(clf.AuthServerAddr, int(defaults.AuthListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Using auth server: %v", addr.FullAddress())
		cfg.AuthServers = []utils.NetAddr{*addr}
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
	cfg.ApplyToken(clf.AuthToken)

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
	if err = parseLabels(clf.Labels, &cfg.SSH); err != nil {
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

// parseLabels takes the value of --labels flag and tries to correctly populate
// sshConf.Labels and sshConf.CmdLabels
func parseLabels(spec string, sshConf *service.SSHConfig) error {
	if spec == "" {
		return nil
	}
	// base syntax parsing, the spec must be in the form of 'key=value,more="better"`
	lmap, err := client.ParseLabelSpec(spec)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(lmap) > 0 {
		sshConf.CmdLabels = make(services.CommandLabels, 0)
		sshConf.Labels = make(map[string]string, 0)
	}
	// see which labels are actually command labels:
	for key, value := range lmap {
		cmdLabel, err := isCmdLabelSpec(value)
		if err != nil {
			return trace.Wrap(err)
		}
		if cmdLabel != nil {
			sshConf.CmdLabels[key] = cmdLabel
		} else {
			sshConf.Labels[key] = value
		}
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

func fileExists(fp string) bool {
	_, err := os.Stat(fp)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// validateRoles makes sure that value upassed to --roles flag is valid
func validateRoles(roles string) error {
	for _, role := range strings.Split(roles, ",") {
		switch role {
		case defaults.RoleAuthService,
			defaults.RoleNode,
			defaults.RoleProxy:
			break
		default:
			return trace.Errorf("unknown role: '%s'", role)
		}
	}
	return nil
}
