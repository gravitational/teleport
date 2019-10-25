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
	"fmt"
	"net"

	"github.com/gravitational/trace"
)

type helmConfigurator struct{}

// Configure configures Helm chart repository specified in the config.
func (c *helmConfigurator) Configure(config Config) error {
	if !hasHelm() {
		log.Debug("Can not configure Helm repository: helm not available.")
		return nil
	}
	log.Debugf("Configuring Helm repository for %v.", config.ProxyAddress)
	if err := c.addRepository(config); err != nil {
		return trace.Wrap(err)
	}
	if err := c.updateRepository(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Deconfigure removes Helm chart repository specified in the config.
func (c *helmConfigurator) Deconfigure(config Config) error {
	if !hasHelm() {
		log.Debug("Can not deconfigure Helm repository: helm not available.")
		return nil
	}
	if err := c.removeRepository(config); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// String returns human-friendly description of the configurator.
func (c *helmConfigurator) String() string {
	return "Helm repository"
}

// addRepository adds Helm chart repository specified by the config to
// the local Helm client.
func (c *helmConfigurator) addRepository(config Config) error {
	// Make Helm chart repository name as a proxy hostname without port.
	chartRepository, _, err := net.SplitHostPort(config.ProxyAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	err = runCommand("helm", "repo", "add", chartRepository,
		fmt.Sprintf("https://%v/charts", config.ProxyAddress),
		"--cert-file", config.CertificatePath,
		"--key-file", config.KeyPath)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// updateRepository updates Helm chart repository cache.
func (c *helmConfigurator) updateRepository() error {
	err := runCommand("helm", "repo", "update")
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeRepository removes the specified Helm chart repository.
func (c *helmConfigurator) removeRepository(config Config) error {
	chartRepository, _, err := net.SplitHostPort(config.ProxyAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	err = runCommand("helm", "repo", "remove", chartRepository)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
