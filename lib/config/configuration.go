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
	switch fc.Logger.Output {
	case "":
		break // not set
	case "stderr", "error", "2":
		log.SetOutput(os.Stderr)
	case "stdout", "out", "1":
		log.SetOutput(os.Stdout)
	case teleport.Syslog:
		err := utils.SwitchLoggingtoSyslog()
		if err != nil {
			// this error will go to stderr
			log.Errorf("Failed to switch logging to syslog: %v.", err)
		}
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
	case teleport.DebugLevel:
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
	if fc.CASignatureAlgorithm != nil {
		cfg.CASignatureAlgorithm = fc.CASignatureAlgorithm
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
		addrs, err := fc.Auth.PublicAddr.Addrs(defaults.AuthListenPort)
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
	if fc.Proxy.Kube.KubeconfigFile != "" {
		cfg.Proxy.Kube.KubeconfigPath = fc.Proxy.Kube.KubeconfigFile
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
	if len(fc.Proxy.TunnelPublicAddr) != 0 {
		addrs, err := fc.Proxy.TunnelPublicAddr.Addrs(defaults.SSHProxyTunnelListenPort)
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Proxy.TunnelPublicAddrs = addrs
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
	if fc.SSH.BPF != nil {
		cfg.SSH.BPF = fc.SSH.BPF.Parse()
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
		nil,
		services.CertAuthoritySpecV2_UNKNOWN,
	)

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
		} else {
			fileConf.Logger.Severity = teleport.DebugLevel
		}
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
			if cfg.Auth.ClusterConfig.GetSessionRecording() == services.RecordAtProxy &&
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
		sshConf.CmdLabels = make(services.CommandLabels)
		sshConf.Labels = make(map[string]string)
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
