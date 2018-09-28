package client

import (
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"

	"github.com/gravitational/trace"

	"gopkg.in/yaml.v2"
)

type ProfileOptions int

const (
	// ProfileCreateNew creates new profile, but does not update current profile
	ProfileCreateNew = 0
	// ProfileMakeCurrent creates a new profile and makes it current
	ProfileMakeCurrent = 1 << iota
)

// CurrentProfileSymlink is a filename which is a symlink to the
// current profile, usually something like this:
//
// ~/.tsh/profile -> ~/.tsh/staging.yaml
//
const CurrentProfileSymlink = "profile"

// ClientProfile is a collection of most frequently used CLI flags
// for "tsh".
//
// Profiles can be stored in a profile file, allowing TSH users to
// type fewer CLI args.
//
type ClientProfile struct {
	// WebProxyAddr is the host:port the web proxy can be accessed at.
	WebProxyAddr string `yaml:"web_proxy_addr,omitempty"`

	// SSHProxyAddr is the host:port the SSH proxy can be accessed at.
	SSHProxyAddr string `yaml:"ssh_proxy_addr,omitempty"`

	// KubeProxyAddr is the host:port the Kubernetes proxy can be accessed at.
	KubeProxyAddr string `yaml:"kube_proxy_addr,omitempty"`

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

	// DELETE IN: 3.1.0
	// The following fields have been deprecated and replaced with
	// "proxy_web_addr" and "proxy_ssh_addr".
	ProxyHost    string `yaml:"proxy_host,omitempty"`
	ProxySSHPort int    `yaml:"proxy_port,omitempty"`
	ProxyWebPort int    `yaml:"proxy_web_port,omitempty"`
}

// Name returns the name of the profile.
func (c *ClientProfile) Name() string {
	if c.ProxyHost != "" {
		return c.ProxyHost
	}

	addr, _, err := net.SplitHostPort(c.WebProxyAddr)
	if err != nil {
		return c.WebProxyAddr
	}

	return addr
}

// FullProfilePath returns the full path to the user profile directory.
// If the parameter is empty, it returns expanded "~/.tsh", otherwise
// returns its unmodified parameter
func FullProfilePath(pDir string) string {
	if pDir != "" {
		return pDir
	}
	// get user home dir:
	home := os.TempDir()
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	return filepath.Join(home, ProfileDir)

}

// If there's a current profile symlink, remove it
func UnlinkCurrentProfile() error {
	return trace.Wrap(os.Remove(filepath.Join(FullProfilePath(""), CurrentProfileSymlink)))
}

// ProfileFromDir reads the user (yaml) profile from a given directory. The
// default is to use the ~/<dir-path>/profile symlink unless another profile
// is explicitly asked for. It works by looking for a "profile" symlink in
// that directory pointing to the profile's YAML file first.
func ProfileFromDir(dirPath string, proxyName string) (*ClientProfile, error) {
	profilePath := filepath.Join(dirPath, CurrentProfileSymlink)
	if proxyName != "" {
		profilePath = filepath.Join(dirPath, proxyName+".yaml")
	}

	return ProfileFromFile(profilePath)
}

// ProfileFromFile loads the profile from a YAML file
func ProfileFromFile(filePath string) (*ClientProfile, error) {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cp *ClientProfile
	if err = yaml.Unmarshal(bytes, &cp); err != nil {
		return nil, trace.Wrap(err)
	}
	return cp, nil
}

// SaveTo saves the profile into a given filename, optionally overwriting it.
func (cp *ClientProfile) SaveTo(filePath string, opts ProfileOptions) error {
	bytes, err := yaml.Marshal(&cp)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = ioutil.WriteFile(filePath, bytes, 0660); err != nil {
		return trace.Wrap(err)
	}
	// set 'current' symlink:
	if opts&ProfileMakeCurrent != 0 {
		symlink := filepath.Join(filepath.Dir(filePath), CurrentProfileSymlink)
		os.Remove(symlink)
		err = os.Symlink(filepath.Base(filePath), symlink)
	}
	return trace.Wrap(err)
}
