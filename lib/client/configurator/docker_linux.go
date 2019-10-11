/*
Copyright 2019 Gravitational, Inc.

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

package configurator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

type dockerConfigurator struct {
	// debug enables verbose logging mode for debugging.
	debug bool
}

// Configure configures local Docker with client key/certificate specified
// in the provided configuration.
func (c *dockerConfigurator) Configure(config Config) error {
	if !hasDocker() {
		log.Debug("Will not configure Docker registy: docker not available.")
		return nil
	}
	// Docker configuration works by symlinking client key/certificate to
	// a location inside /etc/docker/certs.d which requires root privileges
	// so if we're not running as root, invoke a subcommand as root that
	// will relaunch this configurator and perform the symlinking below.
	if os.Geteuid() != 0 {
		return c.runAsRoot(config)
	}
	// Ensure /etc/docker/certs.d/<proxy> directory exists.
	certsDir, err := utils.SafeFilepathJoin(DockerCerts, config.ProxyAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(certsDir, 0755); err != nil {
		return trace.ConvertSystemError(err)
	}
	// Symlink user's key/certificate to /etc/docker/certs.d/<proxy>.
	symlinks, err := c.getSymlinks(config)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := ensureSymlinks(symlinks); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// IsConfigured returns true if the local Docker is already configured with
// the specified client key/certificate.
func (c *dockerConfigurator) IsConfigured(config Config) (bool, error) {
	symlinks, err := c.getSymlinks(config)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return verifySymlinks(symlinks)
}

// getSymlinks returns a map of symlinks that need to be configured in order
// to let local Docker access registry provided by the proxy.
func (c *dockerConfigurator) getSymlinks(config Config) (map[string]string, error) {
	certsDir, err := utils.SafeFilepathJoin(DockerCerts, config.ProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return map[string]string{
		config.CertificatePath: filepath.Join(certsDir, DockerClientCertificate),
		config.KeyPath:         filepath.Join(certsDir, DockerClientKey),
	}, nil
}

// runAsRoot executes "tsh configure-docker" subcommand as a root.
//
// The command needs root privileges in order to be able to symlink
// certificates to /etc/docker/certs.d.
func (c *dockerConfigurator) runAsRoot(config Config) error {
	tshPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	// Invoke "sudo tsh configure-docker" command, it will run as root so
	// it needs to be passed the profile directory of the current user in
	// order to be able to find proper certificates.
	args := []string{"configure-docker", "--profile-dir", config.ProfileDir}
	if c.debug {
		args = append(args, "--debug")
	}
	fmt.Printf("Will configure access to Docker registry provided by %v, "+
		"you may be prompted for password.\n", config.ProxyAddress)
	err = runCommandSudo(tshPath, args...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

const (
	// DockerCerts is the directory where Docker keeps client certificates.
	DockerCerts = "/etc/docker/certs.d"
	// DockerClientKey is the client private key filename.
	DockerClientKey = "client.key"
	// DockerClientCertificate is the client certificate filename.
	DockerClientCertificate = "client.cert"
)
