/*
Copyright 2021 Gravitational, Inc.

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

// Package profile handles management of the Teleport profile directory (~/.tsh).
package profile

import (
	"crypto/tls"
	"crypto/x509"
	"io/fs"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/sshutils"
)

const (
	// profileDir is the default root directory where tsh stores profiles.
	profileDir = ".tsh"
)

// Profile is a collection of most frequently used CLI flags
// for "tsh".
//
// Profiles can be stored in a profile file, allowing TSH users to
// type fewer CLI args.
type Profile struct {
	// WebProxyAddr is the host:port the web proxy can be accessed at.
	WebProxyAddr string `yaml:"web_proxy_addr,omitempty"`

	// SSHProxyAddr is the host:port the SSH proxy can be accessed at.
	SSHProxyAddr string `yaml:"ssh_proxy_addr,omitempty"`

	// KubeProxyAddr is the host:port the Kubernetes proxy can be accessed at.
	KubeProxyAddr string `yaml:"kube_proxy_addr,omitempty"`

	// PostgresProxyAddr is the host:port the Postgres proxy can be accessed at.
	PostgresProxyAddr string `yaml:"postgres_proxy_addr,omitempty"`

	// MySQLProxyAddr is the host:port the MySQL proxy can be accessed at.
	MySQLProxyAddr string `yaml:"mysql_proxy_addr,omitempty"`

	// MongoProxyAddr is the host:port the Mongo proxy can be accessed at.
	MongoProxyAddr string `yaml:"mongo_proxy_addr,omitempty"`

	// Username is the Teleport username for the client.
	Username string `yaml:"user,omitempty"`

	// SiteName is equivalent to the --cluster flag
	SiteName string `yaml:"cluster,omitempty"`

	// DynamicForwardedPorts is a list of ports to use for dynamic port
	// forwarding (SOCKS5).
	DynamicForwardedPorts []string `yaml:"dynamic_forward_ports,omitempty"`

	// Dir is the directory of this profile.
	Dir string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool `yaml:"tls_routing_enabled,omitempty"`

	// TLSRoutingConnUpgradeRequired indicates that ALPN connection upgrades
	// are required for making TLS routing requests.
	//
	// Note that this is applicable to the Proxy's Web port regardless of
	// whether the Proxy is in single-port or multi-port configuration.
	TLSRoutingConnUpgradeRequired bool `yaml:"tls_routing_conn_upgrade_required,omitempty"`

	// AuthConnector (like "google", "passwordless").
	// Equivalent to the --auth tsh flag.
	AuthConnector string `yaml:"auth_connector,omitempty"`

	// LoadAllCAs indicates that tsh should load the CAs of all clusters
	// instead of just the current cluster.
	LoadAllCAs bool `yaml:"load_all_cas,omitempty"`

	// MFAMode ("auto", "platform", "cross-platform").
	// Equivalent to the --mfa-mode tsh flag.
	MFAMode string `yaml:"mfa_mode,omitempty"`

	// PrivateKeyPolicy is a key policy enforced for this profile.
	PrivateKeyPolicy keys.PrivateKeyPolicy `yaml:"private_key_policy"`

	// PIVSlot is a specific piv slot that Teleport clients should use for hardware key support.
	PIVSlot keys.PIVSlot `yaml:"piv_slot"`

	// MissingClusterDetails means this profile was created with limited cluster details.
	// Missing cluster details should be loaded into the profile by pinging the proxy.
	MissingClusterDetails bool

	// SAMLSingleLogoutEnabled is whether SAML SLO (single logout) is enabled, this can only be true if this is a SAML SSO session
	// using an auth connector with a SAML SLO URL configured.
	SAMLSingleLogoutEnabled bool `yaml:"saml_slo_enabled,omitempty"`

	// SSHDialTimeout is the timeout value that should be used for SSH connections.
	SSHDialTimeout time.Duration `yaml:"ssh_dial_timeout,omitempty"`

	// SSOHost is the host of the SSO provider used to log in. Clients can check this value, along
	// with WebProxyAddr, to determine if a webpage is safe to open. Currently used by Teleport
	// Connect in the proxy host allow list.
	SSOHost string `yaml:"sso_host,omitempty"`
}

// Copy returns a shallow copy of p, or nil if p is nil.
func (p *Profile) Copy() *Profile {
	if p == nil {
		return nil
	}
	copy := *p
	return &copy
}

// Name returns the name of the profile.
func (p *Profile) Name() string {
	addr, _, err := net.SplitHostPort(p.WebProxyAddr)
	if err != nil {
		return p.WebProxyAddr
	}

	return addr
}

// TLSConfig returns the profile's associated TLSConfig.
func (p *Profile) TLSConfig() (*tls.Config, error) {
	cert, err := keys.LoadX509KeyPair(p.TLSCertPath(), p.UserTLSKeyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool, err := certPoolFromProfile(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// Expiry returns the credential expiry.
func (p *Profile) Expiry() (time.Time, bool) {
	certPEMBlock, err := os.ReadFile(p.TLSCertPath())
	if err != nil {
		return time.Time{}, false
	}
	cert, _, err := keys.X509Certificate(certPEMBlock)
	if err != nil {
		return time.Time{}, false
	}
	return cert.NotAfter, true
}

// RequireKubeLocalProxy returns true if this profile indicates a local proxy
// is required for kube access.
func (p *Profile) RequireKubeLocalProxy() bool {
	return p.KubeProxyAddr == p.WebProxyAddr && p.TLSRoutingConnUpgradeRequired
}

func certPoolFromProfile(p *Profile) (*x509.CertPool, error) {
	// Check if CAS dir exist if not try to load certs from legacy certs.pem file.
	if _, err := os.Stat(p.TLSClusterCASDir()); err != nil {
		if !os.IsNotExist(err) {
			return nil, trace.Wrap(err)
		}
		pool, err := certPoolFromLegacyCAFile(p)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return pool, nil
	}

	// Load CertPool from CAS directory.
	pool, err := certPoolFromCASDir(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pool, nil
}

func certPoolFromCASDir(p *Profile) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	err := filepath.Walk(p.TLSClusterCASDir(), func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		if info.IsDir() {
			return nil
		}
		cert, err := os.ReadFile(path)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if !pool.AppendCertsFromPEM(cert) {
			return trace.BadParameter("invalid CA cert PEM %s", path)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pool, nil
}

func certPoolFromLegacyCAFile(p *Profile) (*x509.CertPool, error) {
	caCerts, err := os.ReadFile(p.TLSCAsPath())
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCerts) {
		return nil, trace.BadParameter("invalid CA cert PEM")
	}
	return pool, nil
}

// SSHClientConfig returns the profile's associated SSHClientConfig.
func (p *Profile) SSHClientConfig() (*ssh.ClientConfig, error) {
	cert, err := os.ReadFile(p.SSHCertPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshCert, err := sshutils.ParseCertificate(cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := os.ReadFile(p.KnownHostsPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	priv, err := keys.LoadPrivateKey(p.UserSSHKeyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ssh, err := sshutils.ProxyClientSSHConfig(sshCert, priv, caCerts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ssh, nil
}

// SetCurrentProfileName attempts to set the current profile name.
func SetCurrentProfileName(dir string, name string) error {
	if dir == "" {
		return trace.BadParameter("cannot set current profile: missing dir")
	}

	path := keypaths.CurrentProfileFilePath(dir)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(name)+"\n"), 0o660); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RemoveProfile removes cluster profile file
func RemoveProfile(dir, name string) error {
	profilePath := filepath.Join(dir, name+".yaml")
	if err := os.Remove(profilePath); err != nil {
		return trace.ConvertSystemError(err)
	}

	return nil
}

// GetCurrentProfileName attempts to load the current profile name.
func GetCurrentProfileName(dir string) (name string, err error) {
	if dir == "" {
		return "", trace.BadParameter("cannot get current profile: missing dir")
	}

	data, err := os.ReadFile(keypaths.CurrentProfileFilePath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return "", trace.NotFound("current-profile is not set")
		}
		return "", trace.ConvertSystemError(err)
	}
	name = strings.TrimSpace(string(data))
	if name == "" {
		return "", trace.NotFound("current-profile is not set")
	}
	return name, nil
}

// ListProfileNames lists all available profiles.
func ListProfileNames(dir string) ([]string, error) {
	if dir == "" {
		return nil, trace.BadParameter("cannot list profiles: missing dir")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var names []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if file.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(file.Name(), ".yaml"))
	}
	return names, nil
}

// FullProfilePath returns the full path to the user profile directory.
// If the parameter is empty, it returns expanded "~/.tsh", otherwise
// returns its unmodified parameter
func FullProfilePath(dir string) string {
	if dir != "" {
		return dir
	}
	return defaultProfilePath()
}

// defaultProfilePath retrieves the default path of the TSH profile.
func defaultProfilePath() string {
	// start with UserHomeDir, which is the fastest option as it
	// relies only on environment variables and does not perform
	// a user lookup (which can be very slow on large AD environments)
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, profileDir)
	}

	home = os.TempDir()
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		home = u.HomeDir
	}
	return filepath.Join(home, profileDir)
}

// FromDir reads the user profile from a given directory. If dir is empty,
// this function defaults to the default tsh profile directory. If name is empty,
// this function defaults to loading the currently active profile (if any).
func FromDir(dir string, name string) (*Profile, error) {
	dir = FullProfilePath(dir)
	var err error
	if name == "" {
		name, err = GetCurrentProfileName(dir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	p, err := profileFromFile(keypaths.ProfileFilePath(dir, name))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return p, nil
}

// profileFromFile loads the profile from a YAML file.
func profileFromFile(filePath string) (*Profile, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var p Profile
	if err := yaml.Unmarshal(bytes, &p); err != nil {
		return nil, trace.Wrap(err)
	}

	if p.Name() == "" {
		return nil, trace.NotFound("invalid or empty profile at %q", filePath)
	}

	p.Dir = filepath.Dir(filePath)

	// Older versions of tsh did not always store the cluster name in the
	// profile. If no cluster name is found, fallback to the name of the profile
	// for backward compatibility.
	if p.SiteName == "" {
		p.SiteName = p.Name()
	}

	// For backwards compatibility, if the dial timeout was not set,
	// then use the DefaultIOTimeout.
	if p.SSHDialTimeout == 0 {
		p.SSHDialTimeout = defaults.DefaultIOTimeout
	}

	return &p, nil
}

// SaveToDir saves this profile to the specified directory.
// If makeCurrent is true, it makes this profile current.
func (p *Profile) SaveToDir(dir string, makeCurrent bool) error {
	if dir == "" {
		return trace.BadParameter("cannot save profile: missing dir")
	}
	if err := p.saveToFile(keypaths.ProfileFilePath(dir, p.Name())); err != nil {
		return trace.Wrap(err)
	}
	if makeCurrent {
		return trace.Wrap(SetCurrentProfileName(dir, p.Name()))
	}
	return nil
}

// saveToFile saves this profile to the specified file.
func (p *Profile) saveToFile(filepath string) error {
	bytes, err := yaml.Marshal(&p)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = os.WriteFile(filepath, bytes, 0o660); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// KeyDir returns the path to the profile's directory.
func (p *Profile) KeyDir() string {
	return keypaths.KeyDir(p.Dir)
}

// ProxyKeyDir returns the path to the profile's key directory.
func (p *Profile) ProxyKeyDir() string {
	return keypaths.ProxyKeyDir(p.Dir, p.Name())
}

// UserSSHKeyPath returns the path to the profile's SSH private key.
func (p *Profile) UserSSHKeyPath() string {
	return keypaths.UserSSHKeyPath(p.Dir, p.Name(), p.Username)
}

// UserTLSKeyPath returns the path to the profile's TLS private key.
func (p *Profile) UserTLSKeyPath() string {
	return keypaths.UserTLSKeyPath(p.Dir, p.Name(), p.Username)
}

// TLSCertPath returns the path to the profile's TLS certificate.
func (p *Profile) TLSCertPath() string {
	return keypaths.TLSCertPath(p.Dir, p.Name(), p.Username)
}

// TLSCAsLegacyPath returns the path to the profile's TLS certificate authorities.
func (p *Profile) TLSCAsLegacyPath() string {
	return keypaths.TLSCAsPath(p.Dir, p.Name())
}

// TLSCAPathCluster returns CA for particular cluster.
func (p *Profile) TLSCAPathCluster(cluster string) string {
	return keypaths.TLSCAsPathCluster(p.Dir, p.Name(), cluster)
}

// TLSClusterCASDir returns CAS directory where cluster CAs are stored.
func (p *Profile) TLSClusterCASDir() string {
	return keypaths.CAsDir(p.Dir, p.Name())
}

// TLSCAsPath returns the legacy path to the profile's TLS certificate authorities.
func (p *Profile) TLSCAsPath() string {
	return keypaths.TLSCAsPath(p.Dir, p.Name())
}

// SSHDir returns the path to the profile's ssh directory.
func (p *Profile) SSHDir() string {
	return keypaths.SSHDir(p.Dir, p.Name(), p.Username)
}

// SSHCertPath returns the path to the profile's ssh certificate.
func (p *Profile) SSHCertPath() string {
	return keypaths.SSHCertPath(p.Dir, p.Name(), p.Username, p.SiteName)
}

// PPKFilePath returns the path to the profile's PuTTY PPK-formatted keypair.
func (p *Profile) PPKFilePath() string {
	return keypaths.PPKFilePath(p.Dir, p.Name(), p.Username)
}

// KnownHostsPath returns the path to the profile's ssh certificate authorities.
func (p *Profile) KnownHostsPath() string {
	return keypaths.KnownHostsPath(p.Dir)
}

// AppCertPath returns the path to the profile's certificate for a given
// application. Note that this function merely constructs the path - there
// is no guarantee that there is an actual certificate at that location.
func (p *Profile) AppCertPath(appName string) string {
	return keypaths.AppCertPath(p.Dir, p.Name(), p.Username, p.SiteName, appName)
}

// AppKeyPath returns the path to the profile's private key for a given
// application. Note that this function merely constructs the path - there
// is no guarantee that there is an actual key at that location.
func (p *Profile) AppKeyPath(appName string) string {
	return keypaths.AppKeyPath(p.Dir, p.Name(), p.Username, p.SiteName, appName)
}
