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

package client

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport/api/constants"
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

	// MySQLProxyAddr is the host:port the MySQL proxy can be accessed at.
	MySQLProxyAddr string `yaml:"mysql_proxy_addr,omitempty"`

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
	cert, err := tls.LoadX509KeyPair(p.tlsCertPath(), p.keyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := ioutil.ReadFile(p.tlsCasPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCerts) {
		return nil, trace.BadParameter("invalid CA cert PEM")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

// SSHClientConfig returns the profile's associated SSHClientConfig.
func (p *Profile) SSHClientConfig() (*ssh.ClientConfig, error) {
	cert, err := ioutil.ReadFile(p.sshCertPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := ioutil.ReadFile(p.keyPath())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := ioutil.ReadFile(p.sshCasPath())
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
	if err := ioutil.WriteFile(path, []byte(strings.TrimSpace(name)+"\n"), 0660); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetCurrentProfileName attempts to load the current profile name.
func GetCurrentProfileName(dir string) (name string, err error) {
	if dir == "" {
		return "", trace.BadParameter("cannot get current profile: missing dir")
	}

	data, err := ioutil.ReadFile(filepath.Join(dir, currentProfileFilename))
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
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var names []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
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

// ProfileFromDir reads the user (yaml) profile from a given directory. If
// dir is empty, this function defaults to the default tsh profile directory.
// If name is empty, this function defaults to loading the currently active
// profile (if any).
func ProfileFromDir(dir string, name string) (*Profile, error) {
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
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var p *Profile
	if err := yaml.Unmarshal(bytes, &p); err != nil {
		return nil, trace.Wrap(err)
	}
	p.Dir = filepath.Dir(filePath)
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
	if err = ioutil.WriteFile(filepath, bytes, 0660); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *Profile) keyDir() string {
	return filepath.Join(p.Dir, constants.SessionKeyDir)
}

func (p *Profile) userKeyDir() string {
	return filepath.Join(p.keyDir(), p.Name())
}

func (p *Profile) keyPath() string {
	return filepath.Join(p.userKeyDir(), p.Username)
}

func (p *Profile) tlsCertPath() string {
	return filepath.Join(p.userKeyDir(), p.Username+constants.FileExtTLSCert)
}

func (p *Profile) tlsCasPath() string {
	return filepath.Join(p.userKeyDir(), constants.FileNameTLSCerts)
}

func (p *Profile) sshDir() string {
	return filepath.Join(p.userKeyDir(), p.Username+constants.SSHDirSuffix)
}

func (p *Profile) sshCertPath() string {
	return filepath.Join(p.sshDir(), p.SiteName+constants.FileExtSSHCert)
}

func (p *Profile) sshCasPath() string {
	return filepath.Join(p.Dir, constants.FileNameKnownHosts)
}
