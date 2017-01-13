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
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/Sirupsen/logrus"
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
	// --listen-ip flag
	ListenIP net.IP
	// --advertise-ip flag
	AdvertiseIP net.IP
	// --config flag
	ConfigFile string
	// ConfigString is a base64 encoded configuration string
	// set by --config-string or TELEPORT_CONFIG environment variable
	ConfigString string
	// --roles flag
	Roles string
	// -d flag
	Debug bool
	// --labels flag
	Labels string
	// --httpprofile hidden flag
	HTTPProfileEndpoint bool
	// --pid-file flag
	PIDFile string
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

// ApplyFileConfig applies confniguration from a YAML file to Teleport
// runtime config
func ApplyFileConfig(fc *FileConfig, cfg *service.Config) error {
	// no config file? no problem
	if fc == nil {
		return nil
	}
	cfg.SeedConfig = fc.SeedConfig
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
	if advertiseIP != nil {
		if err := validateAdvertiseIP(advertiseIP); err != nil {
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
	cfg.Auth.DomainName = fc.Auth.DomainName

	// U2F (universal 2nd factor auth) configuration:
	u2f, err := fc.Auth.U2F.Parse()
	if err != nil {
		return trace.Wrap(err)
	}
	cfg.Auth.U2F = *u2f

	if fc.Global.DataDir != "" {
		cfg.DataDir = fc.Global.DataDir
	}
	// use bolt by default:
	if fc.Storage.Type == "" {
		fc.Storage.Type = boltbk.GetName()
	}
	if fc.Storage.Params == nil {
		fc.Storage.Params = make(backend.Params)
	}

	// forward storage config to 'auth backend' config (same thing)
	cfg.Auth.KeysBackend.BackendConf = &fc.Storage
	cfg.Auth.KeysBackend.Type = fc.Storage.Type

	// apply logger settings
	switch fc.Logger.Output {
	case "":
		break // not set
	case "stderr", "error", "2":
		log.SetOutput(os.Stderr)
	case "stdout", "out", "1":
		log.SetOutput(os.Stdout)
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
		return trace.Errorf("unsupported logger severity: '%v'", fc.Logger.Severity)
	}

	// apply connection throttling:
	limiters := []limiter.LimiterConfig{
		cfg.SSH.Limiter,
		cfg.Auth.Limiter,
		cfg.Proxy.Limiter,
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

	// add reverse tunnels supplied from configs
	for _, t := range fc.Auth.ReverseTunnels {
		tun, err := t.ConvertAndValidate()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.ReverseTunnels = append(cfg.ReverseTunnels, tun)
	}

	// add oidc connectors supplied from configs
	for _, c := range fc.Auth.OIDCConnectors {
		conn, err := c.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.OIDCConnectors = append(cfg.OIDCConnectors, conn)
	}

	// apply "proxy_service" section
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
		cfg.Proxy.TLSCert = fc.Proxy.CertFile
	}

	// apply "auth_service" section
	if fc.Auth.ListenAddress != "" {
		addr, err := utils.ParseHostPortAddr(fc.Auth.ListenAddress, int(defaults.AuthListenPort))
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.SSHAddr = *addr
		cfg.AuthServers = append(cfg.AuthServers, *addr)
	}
	for _, authority := range fc.Auth.Authorities {
		ca, role, err := authority.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.Authorities = append(cfg.Auth.Authorities, ca)
		cfg.Auth.Roles = append(cfg.Auth.Roles, role)
	}
	for _, token := range fc.Auth.StaticTokens {
		roles, tokenValue, err := token.Parse()
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Auth.StaticTokens = append(cfg.Auth.StaticTokens, services.ProvisionToken{Token: tokenValue, Roles: roles, Expires: time.Unix(0, 0)})
	}

	// apply "ssh_service" section
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
	// read 'trusted_clusters' section:
	if fc.Auth.Enabled() && len(fc.Auth.TrustedClusters) > 0 {
		if err := readTrustedClusters(fc.Auth.TrustedClusters, cfg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// parseCAKey() gets called for every line in a "CA key file" which is
// the same as 'known_hosts' format for openssh
func parseCAKey(bytes []byte, allowedLogins []string) (services.CertAuthority, services.Role, error) {
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
			roles = append(roles, role)
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

	// apply --debug flag:
	if clf.Debug {
		cfg.Console = ioutil.Discard
		utils.InitLoggerDebug()
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

	// apply --token flag:
	cfg.ApplyToken(clf.AuthToken)

	// apply --listen-ip flag:
	if clf.ListenIP != nil {
		applyListenIP(clf.ListenIP, cfg)
	}

	// --advertise-ip flag
	if clf.AdvertiseIP != nil {
		if err := validateAdvertiseIP(clf.AdvertiseIP); err != nil {
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

func validateAdvertiseIP(advertiseIP net.IP) error {
	if advertiseIP.IsLoopback() || advertiseIP.IsUnspecified() || advertiseIP.IsMulticast() {
		return trace.BadParameter("unreachable advertise IP: %v", advertiseIP)
	}
	return nil
}
