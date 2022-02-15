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

	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/sshutils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

const (
	// profileDir is the default root directory where tsh stores profiles.
	profileDir = ".tsh"
	// currentProfileFilename is a file which stores the name of the
	// currently active profile.
	currentProfileFilename = "current-profile"
)

// Profile is a collection of most frequently used CLI flags
// for "tsh".
//
// Profiles can be stored in a profile file, allowing TSH users to
// type fewer CLI args.
//
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

	// AuthType (like "google")
	AuthType string `yaml:"auth_type,omitempty"`

	// SiteName is equivalient to --cluster argument
	SiteName string `yaml:"cluster,omitempty"`

	// ForwardedPorts is the list of ports to forward to the target node.
	ForwardedPorts []string `yaml:"forward_ports,omitempty"`

	// DynamicForwardedPorts is a list of ports to use for dynamic port
	// forwarding (SOCKS5).
	DynamicForwardedPorts []string `yaml:"dynamic_forward_ports,omitempty"`

	// Dir is the directory of this profile.
	Dir string

	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool `yaml:"tls_routing_enabled,omitempty"`
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
	cert, err := tls.LoadX509KeyPair(p.TLSCertPath(), p.UserKeyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	err = filepath.Walk(p.TLSClusterCASDir(), func(path string, info fs.FileInfo, err error) error {
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

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// SSHClientConfig returns the profile's associated SSHClientConfig.
func (p *Profile) SSHClientConfig() (*ssh.ClientConfig, error) {
	cert, err := os.ReadFile(p.SSHCertPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := os.ReadFile(p.UserKeyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := os.ReadFile(p.KnownHostsPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ssh, err := sshutils.ProxyClientSSHConfig(cert, key, [][]byte{caCerts})
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

	path := filepath.Join(dir, currentProfileFilename)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(name)+"\n"), 0660); err != nil {
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

	data, err := os.ReadFile(filepath.Join(dir, currentProfileFilename))
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
	home := os.TempDir()
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
	p, err := profileFromFile(filepath.Join(dir, name+".yaml"))
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
	var p *Profile
	if err := yaml.Unmarshal(bytes, &p); err != nil {
		return nil, trace.Wrap(err)
	}
	p.Dir = filepath.Dir(filePath)

	// Older versions of tsh did not always store the cluster name in the
	// profile. If no cluster name is found, fallback to the name of the profile
	// for backward compatibility.
	if p.SiteName == "" {
		p.SiteName = p.Name()
	}
	return p, nil
}

// SaveToDir saves this profile to the specified directory.
// If makeCurrent is true, it makes this profile current.
func (p *Profile) SaveToDir(dir string, makeCurrent bool) error {
	if dir == "" {
		return trace.BadParameter("cannot save profile: missing dir")
	}
	if err := p.saveToFile(filepath.Join(dir, p.Name()+".yaml")); err != nil {
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
	if err = os.WriteFile(filepath, bytes, 0660); err != nil {
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

// UserKeyPath returns the path to the profile's private key.
func (p *Profile) UserKeyPath() string {
	return keypaths.UserKeyPath(p.Dir, p.Name(), p.Username)
}

// TLSCertPath returns the path to the profile's TLS certificate.
func (p *Profile) TLSCertPath() string {
	return keypaths.TLSCertPath(p.Dir, p.Name(), p.Username)
}

// TLSCAPathCluster returns CA for particular cluster.
func (p *Profile) TLSCAPathCluster(cluster string) string {
	return keypaths.TLSCAsPathCluster(p.Dir, p.Name(), cluster)
}

// TLSClusterCASDir returns CAS directory where cluster CAs are stored.
func (p *Profile) TLSClusterCASDir() string {
	return keypaths.CAsDir(p.Dir, p.Name())
}

// SSHDir returns the path to the profile's ssh directory.
func (p *Profile) SSHDir() string {
	return keypaths.SSHDir(p.Dir, p.Name(), p.Username)
}

// SSHCertPath returns the path to the profile's ssh certificate.
func (p *Profile) SSHCertPath() string {
	return keypaths.SSHCertPath(p.Dir, p.Name(), p.Username, p.SiteName)
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
