/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package teleterm

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

// Config describes teleterm configuration
type Config struct {
	// Addr is the bind address for the server
	Addr string
	// PrehogAddr is the URL where prehog events should be submitted.
	PrehogAddr string
	// HomeDir is the directory to store cluster profiles
	HomeDir string
	// Directory containing certs used to create secure gRPC connection with daemon service
	CertsDir string
	// InsecureSkipVerify is an option to skip HTTPS cert check
	InsecureSkipVerify bool
	// ListeningC propagates the address on which the gRPC server listens. Mostly useful in tests, as
	// the Electron app gets the server port from stdout.
	ListeningC chan<- utils.NetAddr
	// KubeconfigsDir is the directory containing kubeconfigs for Kubernetes
	// Acesss.
	KubeconfigsDir string
	// AgentsDir contains agent config files and data directories for Connect My Computer.
	AgentsDir string
	// InstallationID is a unique ID identifying a specific Teleport Connect installation.
	InstallationID string
	// AddKeysToAgent is passed to [client.Config].
	AddKeysToAgent string
}

// CheckAndSetDefaults checks and sets default config values.
func (c *Config) CheckAndSetDefaults() error {
	if c.HomeDir == "" {
		return trace.BadParameter("missing home directory")
	}

	if c.CertsDir == "" {
		return trace.BadParameter("missing certs directory")
	}

	if c.Addr == "" {
		return trace.BadParameter("missing network address")
	}

	addr, err := utils.ParseAddr(c.Addr)
	if err != nil {
		return trace.Wrap(err)
	}

	if !(addr.Network() == "unix" || addr.Network() == "tcp") {
		return trace.BadParameter("network address should start with unix:// or tcp:// or be empty (tcp:// is used in that case)")
	}

	if c.KubeconfigsDir == "" {
		return trace.BadParameter("missing kubeconfigs directory")
	}

	if c.AgentsDir == "" {
		return trace.BadParameter("missing agents directory")
	}

	if c.InstallationID == "" {
		return trace.BadParameter("missing installation ID")
	}

	if c.AddKeysToAgent == "" {
		c.AddKeysToAgent = client.AddKeysToAgentAuto
	}

	return nil
}
