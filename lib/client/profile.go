package client

import (
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/gravitational/trace"

	"gopkg.in/yaml.v2"
)

// CurrentProfileSymlink is a filename which is a symlink to the
// current profile, usually something like this:
//
// ~/.tsh/profile -> ~/.tsh/staging.yaml
//
const CurrentProfileSymlink = "profile"

// CurrentProfileFilename is a file which stores the name of the
// currently active profile.
const CurrentProfileFilename = "current-profile"

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
}

// Name returns the name of the profile.
func (cp *Profile) Name() string {
	addr, _, err := net.SplitHostPort(cp.WebProxyAddr)
	if err != nil {
		return cp.WebProxyAddr
	}

	return addr
}

// migrateCurrentProfile makes a best-effort attempt to migrate
// the old symlink based current-profile link to the new
// file based current-profile link.
//
// DELETE IN: 6.0
func migrateCurrentProfile(dir string) {
	link := filepath.Join(dir, CurrentProfileSymlink)
	linfo, err := os.Lstat(link)
	if err != nil {
		return
	}
	if finfo, err := os.Stat(filepath.Join(dir, CurrentProfileFilename)); err == nil {
		if linfo.ModTime().Before(finfo.ModTime()) {
			// current-profile is as new or newer than the legacy symlink,
			// no migration necessary.
			return
		}
	}
	linked, err := os.Readlink(link)
	if err != nil || linked == "" {
		return
	}
	name := strings.TrimSuffix(filepath.Base(linked), ".yaml")
	if name == "" {
		return
	}
	if err := SetCurrentProfileName(dir, name); err != nil {
		return
	}

	// TODO IN 5.2: Re-enable removal after verifying that nothing else
	// relis on `link` (note: exact version that this happens doesn't matter
	// too much, but it should happen at least one version prior to removal
	// of the migration).
	//
	//os.Remove(link)
}

// DELETE IN: 6.0
func setLegacySymlink(dir string, name string) error {
	link := filepath.Join(dir, CurrentProfileSymlink)
	if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
		log.Warningf("Failed to remove legacy symlink: %v", err)
	}
	return trace.ConvertSystemError(os.Symlink(name+".yaml", link))
}

// SetCurrentProfileName attempts to set the current profile name.
func SetCurrentProfileName(dir string, name string) error {
	if dir == "" {
		return trace.BadParameter("cannot set current profile: missing dir")
	}

	// set legacy symlink first so that the current-profile file will have
	// a more recent modification time.
	if err := setLegacySymlink(dir, name); err != nil {
		log.Warningf("Failed to set legacy symlink: %v", err)
	}

	path := filepath.Join(dir, CurrentProfileFilename)
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
	// DELETE IN 6.0
	migrateCurrentProfile(dir)

	data, err := ioutil.ReadFile(filepath.Join(dir, CurrentProfileFilename))
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
	// get user home dir:
	home := os.TempDir()
	u, err := user.Current()
	if err == nil {
		home = u.HomeDir
	}
	return filepath.Join(home, ProfileDir)
}

// ProfileFromDir reads the user (yaml) profile from a given directory. If
// name is empty, this function defaults to loading the currently active
// profile (if any).
func ProfileFromDir(dir string, name string) (*Profile, error) {
	if dir == "" {
		return nil, trace.BadParameter("cannot load profile: missing dir")
	}
	var err error
	if name == "" {
		name, err = GetCurrentProfileName(dir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return ProfileFromFile(filepath.Join(dir, name+".yaml"))
}

// ProfileFromFile loads the profile from a YAML file
func ProfileFromFile(filePath string) (*Profile, error) {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var cp *Profile
	if err := yaml.Unmarshal(bytes, &cp); err != nil {
		return nil, trace.Wrap(err)
	}
	return cp, nil
}

func (cp *Profile) SaveToDir(dir string, makeCurrent bool) error {
	if dir == "" {
		return trace.BadParameter("cannot save profile: missing dir")
	}
	if err := cp.SaveToFile(filepath.Join(dir, cp.Name()+".yaml")); err != nil {
		return trace.Wrap(err)
	}
	if makeCurrent {
		return trace.Wrap(SetCurrentProfileName(dir, cp.Name()))
	}
	return nil
}

// SaveToFile saves Profile to the target file.
func (cp *Profile) SaveToFile(filepath string) error {
	bytes, err := yaml.Marshal(&cp)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = ioutil.WriteFile(filepath, bytes, 0660); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
