/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package workloadattest

import (
	"net/url"
	"path/filepath"

	"github.com/gravitational/trace"
)

// DefaultDockerAddr is the default address we will use to connect to the Docker
// daemon.
const DefaultDockerAddr = "unix:///var/run/docker.sock"

// DockerAttestorConfig holds the configuration for the Docker workload attestor.
type DockerAttestorConfig struct {
	// Enabled determines whether Docker workload attestation will be performed.
	Enabled bool `yaml:"enabled"`

	// Addr is the address at which the [Docker Engine API] can be reached.
	//
	// It must be in the form `unix://path/to/socket`, as connecting via TCP is
	// not yet supported. It's also currently assumed that the Docker daemon is
	// running on the same host as `tbot`.
	//
	// By default, `tbot` will attempt to use the standard "rootful" socket
	// location: `/var/run/docker.sock`. When running `tbot` as a non-root user,
	// remember to add the user to the docker group.
	//
	// # Rootless Docker
	//
	// When running the Docker daemon as a non-root user, it can be reached at
	// `unix://$XDG_RUNTIME_DIR/docker.sock` but only by the docker user or root.
	//
	// If you do not want to run `tbot` as the same user or as root, you can
	// override this by creating a configuration file under `~/.config/docker`
	// with the following contents:
	//
	// 	{
	// 		"hosts": ["unix://path/to/socket"]
	// 	}
	//
	// [Docker Engine API]: https://docs.docker.com/reference/api/engine/
	Addr string `yaml:"addr,omitempty"`
}

func (c DockerAttestorConfig) CheckAndSetDefaults() error {
	if !c.Enabled {
		return nil
	}

	if c.Addr != "" {
		u, err := url.Parse(c.Addr)
		if err != nil {
			return trace.Wrap(err, "invalid addr")
		}
		if u.Scheme != "unix" || (u.Path == "" && u.Host == "") {
			return trace.BadParameter("addr must be in the form `unix://path/to/socket`")
		}

		// Forgetting the leading slash is a common typo with `unix://` addresses.
		if u.Host != "" {
			suggestedAddr := &url.URL{
				Scheme: u.Scheme,
				Path:   filepath.Join("/", u.Host, u.Path),
			}
			return trace.BadParameter(
				"addr host segment must be empty, did you forget a leading slash in the socket path? (i.e. `%s`)",
				suggestedAddr.String(),
			)
		}
	}

	return nil
}
