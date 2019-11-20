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

package extensions

import (
	"os"
	"os/user"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/fatih/color"
	"github.com/gravitational/trace"
)

type dockerConfigurator struct{}

// Configure configures local Docker with client key/certificate specified
// in the provided configuration.
func (c *dockerConfigurator) Configure(config Config) error {
	if !hasDocker() {
		log.Debug("Can not configure Docker registy: docker not available.")
		return nil
	}
	configured, err := c.isConfigured(config)
	if err != nil {
		return trace.Wrap(err)
	}
	if configured { // Nothing to do.
		log.Debug("Docker registry is already configured.")
		return nil
	}
	log.Debugf("Configuring Docker registry for %v.", config.ProxyAddress)
	err = c.tryConfigure(config)
	if err == nil {
		return nil
	}
	if !trace.IsAccessDenied(err) {
		return trace.Wrap(err)
	}
	// Output a message for the user informing them that tsh doesn't have
	// enough permissions to configure Docker and the way to do it.
	printError(config)
	return nil
}

// Deconfigure removes configuration for the local Docker client by removing
// previously created symlinks.
func (c *dockerConfigurator) Deconfigure(config Config) error {
	if !hasDocker() {
		log.Debug("Can not deconfigure Docker registry: docker not available.")
		return nil
	}
	configured, err := c.isConfigured(config)
	if err != nil {
		return trace.Wrap(err)
	}
	if !configured { // Nothing to do.
		log.Debug("Docker registry is not configured.")
		return nil
	}
	err = c.tryDeconfigure(config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// String returns human-friendly description of the configurator.
func (c *dockerConfigurator) String() string {
	return "Docker registry"
}

// tryConfigure attempts to symlink user certificates to /etc/docker/certs.d.
func (c *dockerConfigurator) tryConfigure(config Config) error {
	// Ensure /etc/docker/certs.d/<proxy> directory exists.
	certsDir, err := securejoin.SecureJoin(dockerCerts, config.ProxyAddress)
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

// tryDeconfigure attempts to remove user certificates symlinked to /etc/docker/certs.d.
func (c *dockerConfigurator) tryDeconfigure(config Config) error {
	certsDir, err := securejoin.SecureJoin(dockerCerts, config.ProxyAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.RemoveAll(certsDir)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// isConfigured returns true if the local Docker is already configured with
// the specified client key/certificate.
func (c *dockerConfigurator) isConfigured(config Config) (bool, error) {
	symlinks, err := c.getSymlinks(config)
	if err != nil {
		return false, trace.Wrap(err)
	}
	return verifySymlinks(symlinks)
}

// getSymlinks returns a map of symlinks that need to be configured in order
// to let local Docker access registry provided by the proxy.
func (c *dockerConfigurator) getSymlinks(config Config) (map[string]string, error) {
	certsDir, err := securejoin.SecureJoin(dockerCerts, config.ProxyAddress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return map[string]string{
		config.CertificatePath: filepath.Join(certsDir, dockerClientCertificate),
		config.KeyPath:         filepath.Join(certsDir, dockerClientKey),
	}, nil
}

// printError outputs a message indicating that tsh does not have permissions
// to create Docker certificate symlinks.
func printError(config Config) {
	currentUser, err := user.Current()
	if err != nil {
		log.WithError(err).Warn("Failed to determine the current user.")
		color.Yellow(errorMessage, config.ProxyAddress, "<user>")
	}
	color.Yellow(errorMessage, config.ProxyAddress, currentUser.Username)
}

const (
	// dockerCerts is the directory where Docker keeps client certificates.
	dockerCerts = "/etc/docker/certs.d"
	// dockerClientKey is the client private key filename.
	dockerClientKey = "client.key"
	// dockerClientCertificate is the client certificate filename.
	dockerClientCertificate = "client.cert"
	// errorMessage is a message that gets shown to a user if tsh wasn't
	// unable to configure Docker certificates due to permissions issue.
	errorMessage = `
The server %[1]v provides Docker registry support but tsh was unable to configure your local Docker client due to insufficient permissions.

To configure your local Docker client tsh needs to symlink obtained certificates to /etc/docker/certs.d.
See https://docs.docker.com/engine/security/certificates/ for details.

If you'd like to configure your local Docker client, please create the following directory and make sure tsh has write permissions for it, for example:

  sudo mkdir -p /etc/docker/certs.d/%[1]v && sudo chown -R %[2]v /etc/docker/certs.d/%[1]v

And then execute the tsh login command again.

`
)
