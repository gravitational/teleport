package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/gravitational/trace"

	"gopkg.in/yaml.v2"
)

type ProfileOptions int

const (
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
	//
	// proxy configuration
	//
	ProxyHost    string `yaml:"proxy_host,omitempty"`
	ProxySSHPort int    `yaml:"proxy_port,omitempty"`
	ProxyWebPort int    `yaml:"proxy_web_port,omitempty"`

	//
	// auth/identity
	//
	Username string `yaml:"user,omitempty"`

	// AuthType (like "google")
	AuthType string `yaml:"auth_type,omitempty"`

	// SiteName is equivalient to --cluster argument
	SiteName string `yaml:"cluster,omitempty"`

	//
	// other stuff
	//
	ForwardedPorts []string `yaml:"forward_ports,omitempty"`
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

// LogoutFromEverywhere looks at the list of proxy servers tsh is currently logged into
// by examining ~/.tsh and logs him out of them all
func LogoutFromEverywhere(username string) error {
	// if no --user flag was passed, get the current OS user:

	if username == "" {
		me, err := user.Current()
		if err != nil {
			return trace.Wrap(err)
		}
		username = me.Username
	}
	// load all current keys:
	agent, err := NewLocalAgent("", username)
	if err != nil {
		return trace.Wrap(err)
	}
	keys, err := agent.GetKeys(username)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(keys) == 0 {
		fmt.Printf("%s is not logged in, but logging out of everywhere\n", username)
		return agent.ClearAllKeys()
	}
	// ... and delete them:
	for _, key := range keys {

		err = agent.DeleteKey(key.ProxyHost, username)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error logging %s out of %s: %s\n",
				username, key.ProxyHost, err)
		} else {
			fmt.Printf("logged %s out of %s\n", username, key.ProxyHost)
		}
	}
	return nil
}
